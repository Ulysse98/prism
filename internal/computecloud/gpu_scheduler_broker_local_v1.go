//go:build linux

package computecloud

// Operator-only GPU queue integration. This file deliberately adds no HTTP
// routes, wallets, proof signing, marketplace calls or node writes.

import (
	"context"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ApprovedLocalGPUWorkV1 is a single operator-confirmed offline workload.
// The task ID MUST come from a separate CLI approval, not from its body.
type ApprovedLocalGPUWorkV1 struct {
	Body           []byte
	ApprovedTaskID string
}

// NewOperatorGPUQueueRunnerV1 binds the previously qualified local broker
// to the bounded scheduler. Each invocation independently verifies node
// status before and after the GPU process and compares against Go CPU.
func NewOperatorGPUQueueRunnerV1(observer *CloudNodeObserverV1, gpuBinary, expectedSHA256 string) (LocalGPURunnerV1, error) {
	if observer == nil || gpuBinary == "" || expectedSHA256 == "" {
		return nil, errors.New("offline GPU queue requires pinned node and worker")
	}
	return func(ctx context.Context, body []byte, approvedID string) (CloudLocalBrokerGPUResultV1, error) {
		return RunLocalBrokerGPULabV1(ctx, body, observer, approvedID, gpuBinary, expectedSHA256)
	}, nil
}

// NewOperatorGPUQueueRunnerWithGlobalLockV1 enables cross-process GPU
// exclusion for the operator queue. Unlocked V1 remains for legacy tests;
// actual queue CLI uses the locked constructor from Phase 1E.3B onward.
func NewOperatorGPUQueueRunnerWithGlobalLockV1(observer *CloudNodeObserverV1, gpuBinary, expectedSHA256, lockDir string) (LocalGPURunnerV1, error) {
	if observer == nil || gpuBinary == "" || expectedSHA256 == "" || lockDir == "" {
		return nil, errors.New("offline locked queue requires pinned node, worker and lock directory")
	}
	return func(ctx context.Context, body []byte, approvedID string) (CloudLocalBrokerGPUResultV1, error) {
		return RunLocalBrokerGPULabWithGlobalLockV1(ctx, body, observer, approvedID, gpuBinary, expectedSHA256, lockDir)
	}, nil
}

// RunApprovedLocalGPUQueueV1 submits 2..4 distinct, already approved jobs to
// ONE scheduler with ONE runner callback at a time. It produces no partial
// accepted results if any job fails. It does not provide a cross-process GPU
// lock or protection against arbitrary untrusted CUDA kernels.
func RunApprovedLocalGPUQueueV1(ctx context.Context, works []ApprovedLocalGPUWorkV1, runner LocalGPURunnerV1) ([]CloudLocalBrokerGPUResultV1, error) {
	if ctx == nil || runner == nil || len(works) < 2 || len(works) > 4 {
		return nil, errors.New("offline GPU queue requires 2..4 tasks and a runner")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(works))
	snapshot := make([]ApprovedLocalGPUWorkV1, len(works))
	// Validate ALL approvals before creating a scheduler or starting CUDA.
	for i, item := range works {
		task, err := ParseSubmitV1(item.Body)
		if err != nil {
			return nil, fmt.Errorf("task %d rejected before GPU admission: %w", i+1, err)
		}
		decoded, err := hex.DecodeString(item.ApprovedTaskID)
		if err != nil || len(decoded) != 32 || hex.EncodeToString(decoded) != item.ApprovedTaskID ||
			subtle.ConstantTimeCompare([]byte(task.ID), []byte(item.ApprovedTaskID)) != 1 {
			return nil, fmt.Errorf("task %d: operator task-ID approval mismatch", i+1)
		}
		if _, exists := seen[task.ID]; exists {
			return nil, errors.New("duplicate offline GPU task IDs are refused")
		}
		seen[task.ID] = struct{}{}
		snapshot[i] = ApprovedLocalGPUWorkV1{Body: append([]byte(nil), item.Body...), ApprovedTaskID: item.ApprovedTaskID}
	}
	scheduler, err := NewLocalGPUSchedulerV1(len(snapshot), 30*time.Second, runner)
	if err != nil {
		return nil, err
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make([]CloudLocalBrokerGPUResultV1, len(snapshot))
	failures := make([]error, len(snapshot))
	var wg sync.WaitGroup
	for i := range snapshot {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			v, e := scheduler.Submit(runCtx, snapshot[index].Body, snapshot[index].ApprovedTaskID)
			results[index], failures[index] = v, e
			if e != nil {
				cancel() // fail closed: cancel queued and active work
			}
		}(i)
	}
	wg.Wait()
	closeCtx, closeCancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer closeCancel()
	if err := scheduler.Close(closeCtx); err != nil {
		return nil, fmt.Errorf("offline GPU queue failed to shut down: %w", err)
	}
	for i, e := range failures {
		if e != nil {
			return nil, fmt.Errorf("offline GPU queue task %d failed: %w", i+1, e)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return results, nil
}
