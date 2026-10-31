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

var queueTestBodyA = []byte(`{"batch_size":1,"inputs":[1,0,-1],"weights":[1,0,0,0,1,0,0,0,1],"biases":[0,0,0]}`)
var queueTestBodyB = []byte(`{"batch_size":1,"inputs":[-1,1,0],"weights":[1,0,0,0,1,0,0,0,1],"biases":[0,0,0]}`)

func queueTestApproved(t *testing.T, b []byte) ApprovedLocalGPUWorkV1 {
	t.Helper()
	task, err := ParseSubmitV1(b)
	if err != nil {
		t.Fatal(err)
	}
	return ApprovedLocalGPUWorkV1{Body: b, ApprovedTaskID: task.ID}
}

func queueTestVerified(b []byte) (CloudLocalBrokerGPUResultV1, error) {
	task, err := ParseSubmitV1(b)
	if err != nil {
		return CloudLocalBrokerGPUResultV1{}, err
	}
	vals, err := usefulwork.ComputeMLInferenceQuantized(task)
	if err != nil {
		return CloudLocalBrokerGPUResultV1{}, err
	}
	raw, err := json.Marshal(vals)
	if err != nil {
		return CloudLocalBrokerGPUResultV1{}, err
	}
	digest := sha256.Sum256(raw)
	return CloudLocalBrokerGPUResultV1{
		Mode: "operator_local_gpu_lab", Workload: usefulwork.TaskTypeMLInferenceQuantized,
		ChainID: "prism-local-lab", TaskID: task.ID, InputHash: task.InputHash,
		ResultValues: vals, OutputHash: hex.EncodeToString(digest[:]), CPUVerified: true,
	}, nil
}

func TestApprovedLocalGPUQueueV1Serial(t *testing.T) {
	var active, peak, calls atomic.Int32
	runner := func(ctx context.Context, b []byte, id string) (CloudLocalBrokerGPUResultV1, error) {
		n := active.Add(1)
		defer active.Add(-1)
		calls.Add(1)
		if n > peak.Load() {
			peak.Store(n)
		}
		select {
		case <-time.After(3 * time.Millisecond):
		case <-ctx.Done():
			return CloudLocalBrokerGPUResultV1{}, ctx.Err()
		}
		result, err := queueTestVerified(b)
		if err != nil {
			return CloudLocalBrokerGPUResultV1{}, err
		}
		if result.TaskID != id {
			return CloudLocalBrokerGPUResultV1{}, errors.New("wrong approved ID forwarded")
		}
		return result, nil
	}
	works := []ApprovedLocalGPUWorkV1{queueTestApproved(t, queueTestBodyA), queueTestApproved(t, queueTestBodyB)}
	got, err := RunApprovedLocalGPUQueueV1(context.Background(), works, runner)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || calls.Load() != 2 || peak.Load() != 1 {
		t.Fatalf("bad queue result: %d %d %d", len(got), calls.Load(), peak.Load())
	}
	if got[0].TaskID != works[0].ApprovedTaskID || got[1].TaskID != works[1].ApprovedTaskID {
		t.Fatal("result order not preserved")
	}
}

func TestApprovedLocalGPUQueueV1RejectsBeforeRunner(t *testing.T) {
	var calls atomic.Int32
	runner := func(context.Context, []byte, string) (CloudLocalBrokerGPUResultV1, error) {
		calls.Add(1)
		return CloudLocalBrokerGPUResultV1{}, errors.New("should not run")
	}
	a := queueTestApproved(t, queueTestBodyA)
	b := queueTestApproved(t, queueTestBodyB)
	for _, works := range [][]ApprovedLocalGPUWorkV1{
		{a},
		{a, a},
		{a, {Body: b.Body, ApprovedTaskID: a.ApprovedTaskID}},
		{a, {Body: []byte(`{"reward":1}`), ApprovedTaskID: b.ApprovedTaskID}},
		{a, b, a, b, a},
	} {
		if got, err := RunApprovedLocalGPUQueueV1(context.Background(), works, runner); err == nil || got != nil {
			t.Fatalf("accepted invalid queue: %v", got)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("runner called without full approval")
	}
	if _, err := NewOperatorGPUQueueRunnerV1(nil, "x", "y"); err == nil {
		t.Fatal("nil node accepted")
	}
}

func TestApprovedLocalGPUQueueV1FailClosed(t *testing.T) {
	works := []ApprovedLocalGPUWorkV1{queueTestApproved(t, queueTestBodyA), queueTestApproved(t, queueTestBodyB)}
	var calls atomic.Int32
	runner := func(ctx context.Context, b []byte, _ string) (CloudLocalBrokerGPUResultV1, error) {
		calls.Add(1)
		result, err := queueTestVerified(b)
		if err != nil {
			return CloudLocalBrokerGPUResultV1{}, err
		}
		result.CPUVerified = false
		return result, nil
	}
	got, err := RunApprovedLocalGPUQueueV1(context.Background(), works, runner)
	if err == nil || got != nil || calls.Load() < 1 {
		t.Fatalf("unsafe result accepted: %+v %v", got, err)
	}
}

func TestApprovedLocalGPUQueueV1Cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	works := []ApprovedLocalGPUWorkV1{queueTestApproved(t, queueTestBodyA), queueTestApproved(t, queueTestBodyB)}
	_, err := RunApprovedLocalGPUQueueV1(ctx, works, func(context.Context, []byte, string) (CloudLocalBrokerGPUResultV1, error) {
		t.Fatal("runner called")
		return CloudLocalBrokerGPUResultV1{}, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("missing cancellation: %v", err)
	}
}
