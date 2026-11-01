//go:build linux

package computecloud

// Offline operator scheduler. It has no HTTP wiring and no credentials,
// wallets, proof construction, marketplace calls, or persistence.

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"prism/internal/usefulwork"
)

var (
	ErrLocalGPUQueueFullV1 = errors.New("local GPU scheduler queue full")
	ErrLocalGPUClosedV1    = errors.New("local GPU scheduler closed")
)

// LocalGPURunnerV1 is deliberately injected by the operator-side caller.
// The scheduler itself cannot launch a process, dial a node, or dispatch a job.
// The callback MUST obey ctx cancellation and perform its own binary pinning,
// node identity checks, process cleanup, and output verification.
type LocalGPURunnerV1 func(context.Context, []byte, string) (CloudLocalBrokerGPUResultV1, error)

type localGPURequestV1 struct {
	caller    context.Context
	data      []byte
	taskID    string
	inputHash string
	done      chan localGPUReplyV1
}

type localGPUReplyV1 struct {
	value CloudLocalBrokerGPUResultV1
	err   error
}

// LocalGPUSchedulerV1 admits at most queueCapacity WAITING tasks, plus one
// running task. No additional worker goroutines execute callbacks in parallel.
// This prototype has no restart persistence or cross-process coordination.
type LocalGPUSchedulerV1 struct {
	mu         sync.Mutex
	closed     bool
	pending    chan localGPURequestV1
	runner     LocalGPURunnerV1
	jobTimeout time.Duration
	lifetime   context.Context
	cancel     context.CancelFunc
	done       chan struct{}
}

// NewLocalGPUSchedulerV1 is opt-in; constructing one only starts an idle
// single-worker goroutine. No CUDA execution occurs until Submit succeeds.
func NewLocalGPUSchedulerV1(queueCapacity int, jobTimeout time.Duration, runner LocalGPURunnerV1) (*LocalGPUSchedulerV1, error) {
	if runner == nil || queueCapacity < 1 || queueCapacity > 4 ||
		jobTimeout <= 0 || jobTimeout > 30*time.Second {
		return nil, errors.New("invalid offline GPU scheduler configuration")
	}
	lifetime, cancel := context.WithCancel(context.Background())
	s := &LocalGPUSchedulerV1{
		pending: make(chan localGPURequestV1, queueCapacity),
		runner:  runner, jobTimeout: jobTimeout,
		lifetime: lifetime, cancel: cancel, done: make(chan struct{}),
	}
	go s.worker()
	return s, nil
}

// Submit validates and confirms the canonical task ID BEFORE admission.
// It copies the request bytes to prevent caller mutation while queued.
// Full queues fail immediately: no implicit unbounded goroutine backlog.
func (s *LocalGPUSchedulerV1) Submit(ctx context.Context, body []byte, approvedTaskID string) (CloudLocalBrokerGPUResultV1, error) {
	var empty CloudLocalBrokerGPUResultV1
	if s == nil || ctx == nil {
		return empty, errors.New("missing scheduler or context")
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	task, err := ParseSubmitV1(body)
	if err != nil {
		return empty, err
	}
	approvedBytes, err := hex.DecodeString(approvedTaskID)
	if err != nil || len(approvedBytes) != 32 || hex.EncodeToString(approvedBytes) != approvedTaskID ||
		subtle.ConstantTimeCompare([]byte(task.ID), []byte(approvedTaskID)) != 1 {
		return empty, errors.New("scheduler requires matching operator task-ID approval")
	}
	req := localGPURequestV1{
		caller: ctx, data: append([]byte(nil), body...),
		taskID: task.ID, inputHash: task.InputHash,
		done: make(chan localGPUReplyV1, 1),
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return empty, ErrLocalGPUClosedV1
	}
	if err := ctx.Err(); err != nil {
		s.mu.Unlock()
		return empty, err
	}
	// The queue is never closed; shutdown cancels the lifetime instead.
	select {
	case s.pending <- req:
		s.mu.Unlock()
	default:
		s.mu.Unlock()
		return empty, ErrLocalGPUQueueFullV1
	}
	select {
	case <-ctx.Done():
		return empty, ctx.Err()
	case <-s.lifetime.Done():
		return empty, ErrLocalGPUClosedV1
	case reply := <-req.done:
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		if s.lifetime.Err() != nil {
			return empty, ErrLocalGPUClosedV1
		}
		return reply.value, reply.err
	}
}

func (s *LocalGPUSchedulerV1) worker() {
	defer close(s.done)
	for {
		select {
		case <-s.lifetime.Done():
			return
		default:
		}
		select {
		case <-s.lifetime.Done():
			return
		case req := <-s.pending:
			if s.lifetime.Err() != nil {
				return
			}
			if req.caller.Err() != nil {
				continue // expired while waiting, never invoke CUDA
			}
			runCtx, cancel := context.WithTimeout(s.lifetime, s.jobTimeout)
			stopCaller := context.AfterFunc(req.caller, cancel)
			value, err := s.runner(runCtx, req.data, req.taskID)
			runErr := runCtx.Err()
			stopCaller()
			cancel()
			if err == nil && runErr != nil {
				err = runErr
			}
			if err == nil {
				err = validateScheduledGPUResultV1(req, value)
			}
			if err != nil {
				value = CloudLocalBrokerGPUResultV1{}
			}
			req.done <- localGPUReplyV1{value: value, err: err}
		}
	}
}

func validateScheduledGPUResultV1(req localGPURequestV1, result CloudLocalBrokerGPUResultV1) error {
	if result.Mode != "operator_local_gpu_lab" ||
		result.Workload != usefulwork.TaskTypeMLInferenceQuantized ||
		result.ChainID == "" || result.TaskID != req.taskID ||
		result.InputHash != req.inputHash || !result.CPUVerified ||
		result.GPUAttestation || result.HTTPDispatchEnabled ||
		result.EscrowEnabled || result.ProofsAvailable {
		return errors.New("unsafe or mismatched offline scheduler result")
	}
	task, err := ParseSubmitV1(req.data)
	if err != nil {
		return fmt.Errorf("scheduler task revalidation: %w", err)
	}
	expected, err := usefulwork.ComputeMLInferenceQuantized(task)
	if err != nil {
		return fmt.Errorf("scheduler independent CPU check: %w", err)
	}
	if !slices.Equal(expected, result.ResultValues) {
		return errors.New("scheduler CPU prediction mismatch")
	}
	encoded, err := json.Marshal(result.ResultValues)
	if err != nil {
		return err
	}
	h := sha256.Sum256(encoded)
	if result.OutputHash != hex.EncodeToString(h[:]) {
		return errors.New("scheduler output hash mismatch")
	}
	return nil
}

// Close cancels the active callback and prevents new admissions. It waits for
// the callback to RETURN, bounded by the caller's shutdown context. A runner
// that ignores context may remain active; this is NOT a process sandbox.
func (s *LocalGPUSchedulerV1) Close(ctx context.Context) error {
	if s == nil || ctx == nil {
		return errors.New("missing scheduler or close context")
	}
	s.mu.Lock()
	if !s.closed {
		s.closed = true
		s.cancel()
	}
	s.mu.Unlock()
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
