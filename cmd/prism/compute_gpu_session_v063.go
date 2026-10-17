package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"prism/internal/usefulwork"
	"sync"
	"time"
)

// Separate mutex prevents racing Go's exec stderr capture with the request path.
// On overflow cancel() kills the ENTIRE Linux process group.
type gpuManagerStderrV063 struct {
	mu       sync.Mutex
	data     []byte
	exceeded bool
	limit    int
	cancel   context.CancelFunc
}

func (w *gpuManagerStderrV063) Write(p []byte) (int, error) {
	w.mu.Lock()
	room := w.limit - len(w.data)
	if room < 0 {
		room = 0
	}
	if room > len(p) {
		room = len(p)
	}
	w.data = append(w.data, p[:room]...)
	over := len(p) > room
	if over {
		w.exceeded = true
	}
	w.mu.Unlock()
	if over {
		w.cancel()
	}
	return len(p), nil
}
func (w *gpuManagerStderrV063) Overflow() bool { w.mu.Lock(); defer w.mu.Unlock(); return w.exceeded }
func (w *gpuManagerStderrV063) Detail() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return string(w.data)
}

// Prototype only: serial requests, one live CUDA process, a receipt per request.
// There is no network, signing, marketplace, or persistent job state here.
type gpuSessionSupervisorV063 struct {
	mu         sync.Mutex
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     *bufio.Reader
	stderr     *gpuManagerStderrV063
	life       context.Context
	cancel     context.CancelFunc
	perRequest time.Duration
	dead       bool
	waited     bool
	waitErr    error
	served     int
}

func startGPUV063Supervisor(bin string, perRequest time.Duration) (*gpuSessionSupervisorV063, error) {
	if !filepath.IsAbs(bin) || perRequest <= 0 {
		return nil, fmt.Errorf("invalid supervisor binary or timeout")
	}
	if err := validateGPUWorkerBinaryV062(bin); err != nil {
		return nil, err
	}
	life, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	cmd := exec.CommandContext(life, bin)
	if err := configureGPUWorkerProcessV062(cmd); err != nil {
		cancel()
		return nil, err
	}
	cmd.WaitDelay = 2 * time.Second
	w := &gpuManagerStderrV063{limit: 16 * 1024, cancel: cancel}
	cmd.Stderr = w
	in, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		_ = in.Close()
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		cancel()
		_ = in.Close()
		return nil, err
	}
	return &gpuSessionSupervisorV063{
		cmd: cmd, stdin: in, stdout: bufio.NewReaderSize(out, 128*1024+1),
		stderr: w, life: life, cancel: cancel, perRequest: perRequest,
	}, nil
}
func (s *gpuSessionSupervisorV063) waitLocked() error {
	if !s.waited {
		s.waitErr = s.cmd.Wait()
		s.waited = true
	}
	return s.waitErr
}
func (s *gpuSessionSupervisorV063) abortLocked(err error) error {
	if !s.dead {
		s.dead = true
		s.cancel()
		_ = s.stdin.Close()
		_ = s.waitLocked()
	}
	return err
}
func (s *gpuSessionSupervisorV063) Execute(caller context.Context, task usefulwork.Task, seq int) ([]uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dead {
		return nil, fmt.Errorf("GPU session was terminated")
	}
	// Keep the supervisor's request budget aligned with the Rust worker.
	// Refuse the 65th request without allowing another receipt to escape.
	if s.served >= 64 {
		return nil, s.abortLocked(fmt.Errorf("GPU session reached 64-task limit"))
	}
	if err := caller.Err(); err != nil {
		return nil, s.abortLocked(err)
	}
	if err := validateGPUComputeTaskV062(task); err != nil {
		return nil, s.abortLocked(err)
	}
	if seq < 0 {
		return nil, s.abortLocked(fmt.Errorf("invalid sequence"))
	}
	nonceBytes := make([]byte, 32)
	if _, err := rand.Read(nonceBytes); err != nil {
		return nil, s.abortLocked(fmt.Errorf("GPU nonce generation failed: %w", err))
	}
	nonce := hex.EncodeToString(nonceBytes)
	request, err := json.Marshal(struct {
		Seq   int             `json:"seq"`
		Nonce string          `json:"nonce"`
		Task  usefulwork.Task `json:"task"`
	}{Seq: seq, Nonce: nonce, Task: task})
	if err != nil {
		return nil, s.abortLocked(err)
	}
	if len(request)+1 > 512*1024 {
		return nil, s.abortLocked(fmt.Errorf("GPU request exceeds 512 KiB"))
	}
	request = append(request, '\n')

	// Covers BOTH pipe write and receipt read; any expiry poisons the session.
	timer := time.AfterFunc(s.perRequest, s.cancel)
	stopCaller := context.AfterFunc(caller, s.cancel)
	defer stopCaller()
	defer timer.Stop()
	_, err = s.stdin.Write(request)
	if err != nil {
		return nil, s.abortLocked(fmt.Errorf("GPU IPC request write: %w", err))
	}
	line, readErr := s.stdout.ReadSlice('\n')
	if readErr != nil || len(line) > 128*1024 {
		return nil, s.abortLocked(fmt.Errorf("GPU IPC receipt absent/oversized: %v (bytes=%d)", readErr, len(line)))
	}
	if caller.Err() != nil || s.life.Err() != nil || s.stderr.Overflow() {
		return nil, s.abortLocked(fmt.Errorf("GPU session cancelled or stderr exceeded limit"))
	}
	// Exactly the v0.62 production parser: commits to this task and its result.
	values, err := parseGPUV063SessionNonceReceipt(task, seq, nonce, line)
	if err != nil {
		return nil, s.abortLocked(fmt.Errorf("invalid GPU receipt: %w", err))
	}
	if caller.Err() != nil || s.life.Err() != nil || s.stderr.Overflow() {
		return nil, s.abortLocked(fmt.Errorf("GPU request cancelled after receipt"))
	}
	s.served++
	return values, nil
}
func (s *gpuSessionSupervisorV063) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dead {
		return nil
	}
	_ = s.stdin.Close()
	// The session must exit after EOF; reject unsolicited stdout after last receipt.
	timer := time.AfterFunc(s.perRequest, s.cancel)
	defer timer.Stop()
	_, err := s.stdout.ReadByte()
	if !errors.Is(err, io.EOF) {
		return s.abortLocked(fmt.Errorf("GPU session trailing stdout or close read: %v", err))
	}
	if err := s.waitLocked(); err != nil {
		s.dead = true
		s.cancel()
		return fmt.Errorf("GPU session failed on close: %w", err)
	}
	s.dead = true
	s.cancel()
	if s.stderr.Overflow() {
		return fmt.Errorf("GPU session stderr exceeds 16 KiB")
	}
	return nil
}
func (s *gpuSessionSupervisorV063) IsDead() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dead
}

// The session-only frame binds a per-request random nonce to the sequence.
// It does not alter the existing six-field PoUW receipt or its validator.
func parseGPUV063SessionNonceReceipt(task usefulwork.Task, seq int, nonce string, line []byte) ([]uint64, error) {
	const prefix = "PRISM_GPU_SESSION="
	if len(line) > 128*1024 || !bytes.HasPrefix(line, []byte(prefix)) {
		return nil, fmt.Errorf("invalid session frame prefix/size")
	}
	line = bytes.TrimSuffix(line, []byte("\n"))
	if bytes.ContainsAny(line, "\r\n") {
		return nil, fmt.Errorf("multiple session response lines")
	}
	raw := line[len(prefix):]
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fmt.Errorf("invalid session JSON: %w", err)
	}
	if len(fields) != 4 {
		return nil, fmt.Errorf("unexpected session envelope fields")
	}
	for _, k := range []string{"schema", "seq", "nonce", "receipt"} {
		if _, ok := fields[k]; !ok {
			return nil, fmt.Errorf("session envelope missing %s", k)
		}
	}
	var frame struct {
		Schema  string          `json:"schema"`
		Seq     int             `json:"seq"`
		Nonce   string          `json:"nonce"`
		Receipt json.RawMessage `json:"receipt"`
	}
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, fmt.Errorf("invalid session envelope types: %w", err)
	}
	if frame.Schema != "prism-gpu-session-frame-v063" || frame.Seq != seq ||
		len(frame.Nonce) != 64 || len(nonce) != 64 || subtle.ConstantTimeCompare([]byte(frame.Nonce), []byte(nonce)) != 1 {
		return nil, fmt.Errorf("session sequence or nonce mismatch")
	}
	if len(frame.Receipt) == 0 || frame.Receipt[0] != '{' {
		return nil, fmt.Errorf("invalid nested PoUW receipt")
	}
	inner := append([]byte("PRISM_GPU_RECEIPT="), frame.Receipt...)
	return parseGPUReceiptV062(task, inner)
}
