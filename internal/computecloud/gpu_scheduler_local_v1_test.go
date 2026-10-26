//go:build linux

package computecloud

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"prism/internal/usefulwork"
)

var schedulerTaskOneV1 = []byte(`{"batch_size":1,"inputs":[1,0,-1],"weights":[1,0,0,0,1,0,0,0,1],"biases":[0,0,0]}`)
var schedulerTaskTwoV1 = []byte(`{"batch_size":1,"inputs":[-1,1,0],"weights":[1,0,0,0,1,0,0,0,1],"biases":[0,0,0]}`)

func taskIDForSchedulerTestV1(t *testing.T, body []byte) string {
	t.Helper()
	task, err := ParseSubmitV1(body)
	if err != nil {
		t.Fatal(err)
	}
	return task.ID
}

func correctSchedulerResultV1(t *testing.T, body []byte) CloudLocalBrokerGPUResultV1 {
	t.Helper()
	task, err := ParseSubmitV1(body)
	if err != nil {
		t.Fatal(err)
	}
	values, err := usefulwork.ComputeMLInferenceQuantized(task)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	return CloudLocalBrokerGPUResultV1{
		Mode: "operator_local_gpu_lab", Workload: task.Type, ChainID: "prism-isolated",
		TaskID: task.ID, InputHash: task.InputHash, ResultValues: values,
		OutputHash: hex.EncodeToString(digest[:]), CPUVerified: true,
	}
}

func closeSchedulerV1(t *testing.T, s *LocalGPUSchedulerV1) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.Close(ctx); err != nil {
		t.Fatal(err)
	}
}

func waitQueuedV1(t *testing.T, s *LocalGPUSchedulerV1, expected int) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		if len(s.pending) == expected {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("queue length did not reach %d", expected)
		case <-tick.C:
		}
	}
}

func TestLocalGPUSchedulerV1ConfigAndAuthorization(t *testing.T) {
	fake := func(context.Context, []byte, string) (CloudLocalBrokerGPUResultV1, error) {
		return CloudLocalBrokerGPUResultV1{}, nil
	}
	for _, tc := range []struct {
		cap int
		d   time.Duration
		run LocalGPURunnerV1
	}{
		{0, time.Second, fake}, {5, time.Second, fake}, {1, 0, fake}, {1, 31 * time.Second, fake}, {1, time.Second, nil},
	} {
		if s, err := NewLocalGPUSchedulerV1(tc.cap, tc.d, tc.run); err == nil {
			_ = s.Close(context.Background())
			t.Fatalf("accepted invalid config %+v", tc)
		}
	}
	var calls atomic.Int32
	runner := func(_ context.Context, body []byte, _ string) (CloudLocalBrokerGPUResultV1, error) {
		calls.Add(1)
		return correctSchedulerResultV1(t, body), nil
	}
	s, err := NewLocalGPUSchedulerV1(1, time.Second, runner)
	if err != nil {
		t.Fatal(err)
	}
	defer closeSchedulerV1(t, s)
	if _, err := s.Submit(context.Background(), schedulerTaskOneV1, "bad"); err == nil {
		t.Fatal("missing approval accepted")
	}
	if _, err := s.Submit(context.Background(), []byte(`{"reward":123}`), taskIDForSchedulerTestV1(t, schedulerTaskOneV1)); err == nil {
		t.Fatal("untrusted input accepted")
	}
	if calls.Load() != 0 {
		t.Fatal("executed unauthorized job")
	}
	v, err := s.Submit(context.Background(), schedulerTaskOneV1, taskIDForSchedulerTestV1(t, schedulerTaskOneV1))
	if err != nil || !v.CPUVerified || v.HTTPDispatchEnabled {
		t.Fatalf("expected verified result: %+v %v", v, err)
	}
	if calls.Load() != 1 {
		t.Fatal("missing one execution")
	}
}

func TestLocalGPUSchedulerV1SerialAndBounded(t *testing.T) {
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	var active, peak atomic.Int32
	runner := func(ctx context.Context, body []byte, _ string) (CloudLocalBrokerGPUResultV1, error) {
		n := active.Add(1)
		defer active.Add(-1)
		if n > peak.Load() {
			peak.Store(n)
		}
		started <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
			return CloudLocalBrokerGPUResultV1{}, ctx.Err()
		}
		return correctSchedulerResultV1(t, body), nil
	}
	s, err := NewLocalGPUSchedulerV1(1, 2*time.Second, runner)
	if err != nil {
		t.Fatal(err)
	}
	defer closeSchedulerV1(t, s)
	type answer struct {
		v   CloudLocalBrokerGPUResultV1
		err error
	}
	first := make(chan answer, 1)
	second := make(chan answer, 1)
	go func() {
		v, e := s.Submit(context.Background(), schedulerTaskOneV1, taskIDForSchedulerTestV1(t, schedulerTaskOneV1))
		first <- answer{v, e}
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("runner not started")
	}
	go func() {
		v, e := s.Submit(context.Background(), schedulerTaskTwoV1, taskIDForSchedulerTestV1(t, schedulerTaskTwoV1))
		second <- answer{v, e}
	}()
	waitQueuedV1(t, s, 1)
	if _, err := s.Submit(context.Background(), schedulerTaskOneV1, taskIDForSchedulerTestV1(t, schedulerTaskOneV1)); !errors.Is(err, ErrLocalGPUQueueFullV1) {
		t.Fatalf("expected saturation: %v", err)
	}
	select {
	case <-started:
		t.Fatal("overlapping callback")
	default:
	}
	close(release)
	a, b := <-first, <-second
	if a.err != nil || b.err != nil {
		t.Fatalf("job error %v %v", a.err, b.err)
	}
	if a.v.TaskID == b.v.TaskID {
		t.Fatal("responses mixed")
	}
	if peak.Load() != 1 {
		t.Fatalf("overlapping GPU executions: %d", peak.Load())
	}
}

func TestLocalGPUSchedulerV1QueuedCancellation(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	runner := func(ctx context.Context, b []byte, _ string) (CloudLocalBrokerGPUResultV1, error) {
		calls.Add(1)
		select {
		case started <- struct{}{}:
		default:
		}
		select {
		case <-release:
		case <-ctx.Done():
			return CloudLocalBrokerGPUResultV1{}, ctx.Err()
		}
		return correctSchedulerResultV1(t, b), nil
	}
	s, err := NewLocalGPUSchedulerV1(1, time.Second, runner)
	if err != nil {
		t.Fatal(err)
	}
	defer closeSchedulerV1(t, s)
	first := make(chan error, 1)
	go func() {
		_, e := s.Submit(context.Background(), schedulerTaskOneV1, taskIDForSchedulerTestV1(t, schedulerTaskOneV1))
		first <- e
	}()
	<-started
	ctx, cancel := context.WithCancel(context.Background())
	second := make(chan error, 1)
	go func() {
		_, e := s.Submit(ctx, schedulerTaskTwoV1, taskIDForSchedulerTestV1(t, schedulerTaskTwoV1))
		second <- e
	}()
	waitQueuedV1(t, s, 1)
	cancel()
	if err := <-second; !errors.Is(err, context.Canceled) {
		t.Fatalf("waiting cancel: %v", err)
	}
	close(release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	closeSchedulerV1(t, s)
	if calls.Load() != 1 {
		t.Fatalf("canceled waiting request executed: %d", calls.Load())
	}
}

func TestLocalGPUSchedulerV1TimeoutAndClose(t *testing.T) {
	runner := func(ctx context.Context, _ []byte, _ string) (CloudLocalBrokerGPUResultV1, error) {
		<-ctx.Done()
		return CloudLocalBrokerGPUResultV1{}, ctx.Err()
	}
	s, err := NewLocalGPUSchedulerV1(1, 30*time.Millisecond, runner)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Submit(context.Background(), schedulerTaskOneV1, taskIDForSchedulerTestV1(t, schedulerTaskOneV1))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected timeout: %v", err)
	}
	closeSchedulerV1(t, s)
	_, err = s.Submit(context.Background(), schedulerTaskOneV1, taskIDForSchedulerTestV1(t, schedulerTaskOneV1))
	if !errors.Is(err, ErrLocalGPUClosedV1) {
		t.Fatalf("expected closed: %v", err)
	}
}

func TestLocalGPUSchedulerV1RejectsUnverifiedResult(t *testing.T) {
	wrong := correctSchedulerResultV1(t, schedulerTaskOneV1)
	wrong.ResultValues = []uint64{2}
	runner := func(context.Context, []byte, string) (CloudLocalBrokerGPUResultV1, error) { return wrong, nil }
	s, err := NewLocalGPUSchedulerV1(1, time.Second, runner)
	if err != nil {
		t.Fatal(err)
	}
	defer closeSchedulerV1(t, s)
	got, err := s.Submit(context.Background(), schedulerTaskOneV1, taskIDForSchedulerTestV1(t, schedulerTaskOneV1))
	if err == nil || got.CPUVerified {
		t.Fatal("accepted false CUDA result")
	}
}

func TestLocalGPUSchedulerV1CloseCancelsActiveRunner(t *testing.T) {
	active := make(chan struct{})
	stopped := make(chan struct{})
	runner := func(ctx context.Context, _ []byte, _ string) (CloudLocalBrokerGPUResultV1, error) {
		close(active)
		<-ctx.Done()
		close(stopped)
		return CloudLocalBrokerGPUResultV1{}, ctx.Err()
	}
	s, err := NewLocalGPUSchedulerV1(1, time.Second, runner)
	if err != nil {
		t.Fatal(err)
	}
	answer := make(chan error, 1)
	approved := taskIDForSchedulerTestV1(t, schedulerTaskOneV1)
	go func() { _, e := s.Submit(context.Background(), schedulerTaskOneV1, approved); answer <- e }()
	select {
	case <-active:
	case <-time.After(time.Second):
		t.Fatal("GPU callback not active")
	}
	closeSchedulerV1(t, s)
	select {
	case <-stopped:
	default:
		t.Fatal("shutdown did not cancel callback")
	}
	if err := <-answer; !errors.Is(err, ErrLocalGPUClosedV1) && !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected shutdown response: %v", err)
	}
	if _, err := s.Submit(context.Background(), schedulerTaskOneV1, approved); !errors.Is(err, ErrLocalGPUClosedV1) {
		t.Fatalf("accepted work after close: %v", err)
	}
}

func TestLocalGPUSchedulerV1RejectsUnsafeFlagsAndBadHash(t *testing.T) {
	approved := taskIDForSchedulerTestV1(t, schedulerTaskOneV1)
	for _, tc := range []struct {
		name   string
		change func(*CloudLocalBrokerGPUResultV1)
	}{
		{"http_dispatch", func(v *CloudLocalBrokerGPUResultV1) { v.HTTPDispatchEnabled = true }},
		{"escrow", func(v *CloudLocalBrokerGPUResultV1) { v.EscrowEnabled = true }},
		{"proofs", func(v *CloudLocalBrokerGPUResultV1) { v.ProofsAvailable = true }},
		{"attestation", func(v *CloudLocalBrokerGPUResultV1) { v.GPUAttestation = true }},
		{"wrong_hash", func(v *CloudLocalBrokerGPUResultV1) { v.OutputHash = "bad" }},
		{"wrong_id", func(v *CloudLocalBrokerGPUResultV1) { v.TaskID = "bad" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value := correctSchedulerResultV1(t, schedulerTaskOneV1)
			tc.change(&value)
			runner := func(context.Context, []byte, string) (CloudLocalBrokerGPUResultV1, error) { return value, nil }
			s, err := NewLocalGPUSchedulerV1(1, time.Second, runner)
			if err != nil {
				t.Fatal(err)
			}
			defer closeSchedulerV1(t, s)
			v, err := s.Submit(context.Background(), schedulerTaskOneV1, approved)
			if err == nil || v.CPUVerified {
				t.Fatalf("unsafe result accepted: %+v %v", v, err)
			}
		})
	}
}
