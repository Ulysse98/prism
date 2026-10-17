package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"prism/internal/usefulwork"
	"prism/internal/wallet"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

type gpuManagerCaseV063 struct {
	Seq       int             `json:"seq"`
	Batch     int             `json:"batch"`
	Task      usefulwork.Task `json:"task"`
	Reference []uint64        `json:"reference"`
}

func gpuManagerCasesV063(t *testing.T) []gpuManagerCaseV063 {
	t.Helper()
	// Standard CI runs without local CUDA hardware or the frozen lab dataset.
	if os.Getenv("PRISM_V063_GO_MGR_CASES") == "" || os.Getenv("PRISM_V063_GO_MGR_BIN") == "" {
		t.Skip("set PRISM_V063_GO_MGR_CASES and PRISM_V063_GO_MGR_BIN for CUDA lab tests")
	}
	buf, err := os.ReadFile(os.Getenv("PRISM_V063_GO_MGR_CASES"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []gpuManagerCaseV063
	if err = json.Unmarshal(buf, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 39 {
		t.Fatalf("wrong dataset case count %d", len(cases))
	}
	return cases
}
func gpuManagerRunV063(t *testing.T, bin string, timeout time.Duration) *gpuSessionSupervisorV063 {
	t.Helper()
	s, err := startGPUV063Supervisor(bin, timeout)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
func gpuManagerFakeV063(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-gpu")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nIFS= read -r request || exit 0\n"+body+"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestGPUV063GoSessionManager39Cases(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux/WSL2 only")
	}
	cases := gpuManagerCasesV063(t)
	s := gpuManagerRunV063(t, os.Getenv("PRISM_V063_GO_MGR_BIN"), 10*time.Second)
	count := 0
	seen := map[string]bool{}
	for i, c := range cases {
		if c.Seq != i || c.Batch != int(c.Task.RowsA) || seen[c.Task.ID] {
			t.Fatalf("dataset mismatch at %d", i)
		}
		seen[c.Task.ID] = true
		values, err := s.Execute(context.Background(), c.Task, c.Seq)
		if err != nil {
			t.Fatalf("GPU request %d: %v (stderr=%s)", i, err, s.stderr.Detail())
		}
		if len(values) != len(c.Reference) {
			t.Fatalf("predictions length: %d", i)
		}
		for j, v := range values {
			if v != c.Reference[j] {
				t.Fatalf("GPU request %d prediction %d mismatch", i, j)
			}
		}
		count += len(values)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if count != 34112 {
		t.Fatalf("expected 34112 predictions, got %d", count)
	}
	t.Logf("PASS: Go supervisor validated 39 receipts / %d predictions, one process, strict EOF", count)
}
func TestGPUV063GoSessionManagerRejectStaleReceiptAndRecover(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux/WSL2 only")
	}
	cases := gpuManagerCasesV063(t)
	payload, err := json.Marshal(cases[0].Reference)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(payload)
	receipt := gpuMLReceiptV062{
		Schema: "prism-gpu-ml-lab-v0", TaskID: cases[0].Task.ID, InputHash: cases[0].Task.InputHash,
		ResultValues: cases[0].Reference, OutputHash: hex.EncodeToString(h[:]), GPUAttestation: false,
	}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	// Echo the correct session seq/nonce, but insert a receipt committed to
	// another task. This isolates nested task-commitment validation, rather
	// than merely rejecting the wrong outer response prefix.
	fakeProgram := fmt.Sprintf(`#!/usr/bin/env python3
import json,sys,time
req=json.loads(sys.stdin.readline())
receipt=json.loads(%q)
frame={"schema":"prism-gpu-session-frame-v063", "seq":req["seq"],
       "nonce":req["nonce"], "receipt":receipt}
sys.stdout.write("PRISM_GPU_SESSION="+json.dumps(frame,separators=(',',':'))+"\n")
sys.stdout.flush()
time.sleep(30)
`, string(encoded))
	fake := filepath.Join(t.TempDir(), "fake-other-task-gpu")
	if err := os.WriteFile(fake, []byte(fakeProgram), 0700); err != nil {
		t.Fatal(err)
	}
	s := gpuManagerRunV063(t, fake, 2*time.Second)
	if _, err := s.Execute(context.Background(), cases[1].Task, 1); err == nil {
		t.Fatal("stale receipt was accepted")
	}
	if !s.IsDead() {
		t.Fatal("mismatched receipt did not poison session")
	}
	if _, err := s.Execute(context.Background(), cases[2].Task, 2); err == nil {
		t.Fatal("poisoned session reused")
	}
	recovered := gpuManagerRunV063(t, os.Getenv("PRISM_V063_GO_MGR_BIN"), 10*time.Second)
	values, err := recovered.Execute(context.Background(), cases[1].Task, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != len(cases[1].Reference) {
		t.Fatal("failed GPU restart parity")
	}
	for i, v := range values {
		if v != cases[1].Reference[i] {
			t.Fatalf("GPU restart parity index %d", i)
		}
	}
	if err := recovered.Close(); err != nil {
		t.Fatal(err)
	}
	t.Log("PASS: wrong-task receipt rejected, old session poisoned, new CUDA process recovers")
}
func TestGPUV063GoSessionManagerTimeoutAndStdoutBound(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux/WSL2 only")
	}
	task := gpuManagerCasesV063(t)[0].Task
	t.Run("silent_worker_timeout", func(t *testing.T) {
		fake := gpuManagerFakeV063(t, "sleep 30 &\nwait")
		s := gpuManagerRunV063(t, fake, 350*time.Millisecond)
		started := time.Now()
		if _, err := s.Execute(context.Background(), task, 0); err == nil {
			t.Fatal("silent worker returned success")
		}
		if time.Since(started) > 5*time.Second {
			t.Fatal("timeout did not terminate process promptly")
		}
		if !s.IsDead() {
			t.Fatal("timeout did not poison session")
		}
		t.Log("PASS: per-request timeout terminates isolated GPU process group")
	})
	t.Run("oversized_stdout", func(t *testing.T) {
		// >128KiB without newline. ReadSlice must reject the line before completion.
		fake := gpuManagerFakeV063(t, "printf '%s\\n' '"+strings.Repeat("X", 128*1024+64)+"'\nsleep 30 &\nwait")
		s := gpuManagerRunV063(t, fake, 2*time.Second)
		if _, err := s.Execute(context.Background(), task, 0); err == nil {
			t.Fatal("oversized stdout accepted")
		}
		if !s.IsDead() {
			t.Fatal("oversized stdout did not poison session")
		}
		t.Log("PASS: oversized receipt rejected and worker terminated")
	})
	t.Run("stderr_aggregate_overflow", func(t *testing.T) {
		fake := gpuManagerFakeV063(t, "printf '%s' '"+strings.Repeat("E", 16*1024+64)+"' >&2\nsleep 30 &\nwait")
		s := gpuManagerRunV063(t, fake, 2*time.Second)
		if _, err := s.Execute(context.Background(), task, 0); err == nil {
			t.Fatal("oversized stderr accepted")
		}
		if !s.IsDead() {
			t.Fatal("stderr overflow did not poison session")
		}
		if !s.stderr.Overflow() {
			t.Fatal("stderr writer did not detect overflow")
		}
		t.Log("PASS: aggregate stderr overflow cancels the GPU session")
	})
}

// Real GPU bridge: 39 fresh nonce-bound frames, actual V2 signatures,
// CPU reference parity and independent context verification.
func TestGPUV063NonceBound39SignedProofs(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux/WSL2")
	}
	cases := gpuManagerCasesV063(t)
	s := gpuManagerRunV063(t, os.Getenv("PRISM_V063_GO_MGR_BIN"), 10*time.Second)
	worker, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	const chain = "v063-nonce-offline-chain"
	const genesis = "v063-nonce-offline-genesis"
	for i, c := range cases {
		values, err := s.Execute(context.Background(), c.Task, i)
		if err != nil {
			t.Fatalf("nonce session %d: %v", i, err)
		}
		if !reflect.DeepEqual(values, c.Reference) {
			t.Fatalf("GPU reference mismatch %d", i)
		}
		job := fmt.Sprintf("v063-nonce-offline-job-%03d", i)
		proof, err := usefulwork.BuildComputeMLProofV2FromResult(c.Task, job, chain, genesis, worker, values)
		if err != nil {
			t.Fatal(err)
		}
		if err := usefulwork.VerifyProof(proof); err != nil {
			t.Fatal(err)
		}
		if err := usefulwork.VerifyComputeProofContext(proof, job, chain, genesis); err != nil {
			t.Fatal(err)
		}
		cpu, err := usefulwork.ExecuteCompute(c.Task, job, chain, genesis, worker)
		if err != nil || !reflect.DeepEqual(proof, cpu) {
			t.Fatalf("GPU/CPU proof mismatch %d: %v", i, err)
		}
		n += len(values)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if n != 34112 {
		t.Fatalf("wrong prediction count %d", n)
	}
	t.Log("PASS: 39 nonce-bound GPU frames, 39 signed V2 proofs and 34112 predictions")
}

// A fake server echoes a valid nonce-bound frame and pre-sends its duplicate,
// before seeing the request for the SAME task at a new sequence/nonce.
// The first must pass; the second MUST be rejected and kill the session.
func TestGPUV063NonceBoundSameTaskReplayRejected(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux/WSL2")
	}
	c := gpuManagerCasesV063(t)[0]
	raw, err := json.Marshal(c.Reference)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(raw)
	receipt := gpuMLReceiptV062{
		Schema: "prism-gpu-ml-lab-v0", TaskID: c.Task.ID,
		InputHash: c.Task.InputHash, ResultValues: c.Reference,
		OutputHash: hex.EncodeToString(hash[:]), GPUAttestation: false,
	}
	inner, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	pythonSource := fmt.Sprintf(`#!/usr/bin/env python3
import sys,json,time
req=json.loads(sys.stdin.readline())
# JSON boolean 'false' must be decoded, not pasted as Python syntax.
receipt=json.loads(%q)
frame={"schema":"prism-gpu-session-frame-v063", "seq":req["seq"],
       "nonce":req["nonce"],"receipt":receipt}
line="PRISM_GPU_SESSION="+json.dumps(frame,separators=(',',':'))+"\n"
sys.stdout.write(line+line)
sys.stdout.flush()
time.sleep(30)
`, string(inner))
	fake := filepath.Join(t.TempDir(), "fake-stale-nonce-gpu")
	if err := os.WriteFile(fake, []byte(pythonSource), 0700); err != nil {
		t.Fatal(err)
	}
	s := gpuManagerRunV063(t, fake, 3*time.Second)
	first, err := s.Execute(context.Background(), c.Task, 0)
	if err != nil || !reflect.DeepEqual(first, c.Reference) {
		t.Fatalf("first reply invalid: %v", err)
	}
	_, err = s.Execute(context.Background(), c.Task, 1)
	if err == nil {
		t.Fatal("CRITICAL: stale same-task reply was accepted")
	}
	if !s.IsDead() {
		t.Fatal("stale reply failed to poison session")
	}
	if _, err := s.Execute(context.Background(), c.Task, 2); err == nil {
		t.Fatal("dead session reused")
	}
	t.Log("PASS: duplicate same-task stale nonce rejected; session poisoned")
	recovered := gpuManagerRunV063(t, os.Getenv("PRISM_V063_GO_MGR_BIN"), 10*time.Second)
	fresh, err := recovered.Execute(context.Background(), c.Task, 1)
	if err != nil || !reflect.DeepEqual(fresh, c.Reference) {
		t.Fatalf("new CUDA session failed: %v", err)
	}
	if err := recovered.Close(); err != nil {
		t.Fatal(err)
	}
	t.Log("PASS: fresh CUDA session recovered after stale-reply attack")
}

func TestGPUV063NonceSessionEnvelopeRejectsMutation(t *testing.T) {
	// Pure parser regression: runs in normal CI without CUDA or lab JSON files.
	c := gpuManagerCaseV063{Task: gpuTaskForTestV062(t), Reference: []uint64{1, 2, 1}}
	raw, err := json.Marshal(c.Reference)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	r := gpuMLReceiptV062{Schema: "prism-gpu-ml-lab-v0", TaskID: c.Task.ID, InputHash: c.Task.InputHash,
		ResultValues: c.Reference, OutputHash: hex.EncodeToString(sum[:]), GPUAttestation: false}
	nonce := strings.Repeat("a", 64)
	base := map[string]any{"schema": "prism-gpu-session-frame-v063", "seq": 7, "nonce": nonce, "receipt": r}
	encoded, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	success := append([]byte("PRISM_GPU_SESSION="), encoded...)
	if v, err := parseGPUV063SessionNonceReceipt(c.Task, 7, nonce, success); err != nil || !reflect.DeepEqual(v, c.Reference) {
		t.Fatalf("valid frame rejected: %v", err)
	}
	for _, name := range []string{"seq", "nonce", "extra", "schema"} {
		changed := make(map[string]any)
		for k, v := range base {
			changed[k] = v
		}
		switch name {
		case "seq":
			changed["seq"] = 8
		case "nonce":
			changed["nonce"] = strings.Repeat("b", 64)
		case "extra":
			changed["extra"] = true
		case "schema":
			changed["schema"] = "unexpected"
		}
		encoded, _ = json.Marshal(changed)
		if _, err := parseGPUV063SessionNonceReceipt(c.Task, 7, nonce, append([]byte("PRISM_GPU_SESSION="), encoded...)); err == nil {
			t.Fatalf("mutated %s session frame accepted", name)
		}
	}
	t.Log("PASS: session seq/nonce/schema/extra-field mutations rejected")
}

// Boundary tests exercise the full 3-feature range accepted by Prism Go,
// not merely the three benchmark batch sizes.
func TestGPUV063SessionBoundaryBatches(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux/WSL2 only")
	}
	if os.Getenv("PRISM_V063_GO_MGR_BIN") == "" {
		t.Skip("set PRISM_V063_GO_MGR_BIN to run CUDA boundary tests")
	}
	weights := []int64{2, 1, -1, -1, -2, 2, 1, 1, 1}
	biases := []int64{1, -1, 0}
	s := gpuManagerRunV063(t, os.Getenv("PRISM_V063_GO_MGR_BIN"), 10*time.Second)
	for seq, batch := range []int{1, 3, 31, 32, 33, 255, 1365} {
		inputs := make([]int64, batch*3)
		for j := range inputs {
			inputs[j] = int64((j*11)%17 - 8)
		}
		task, err := usefulwork.NewMLInferenceQuantizedTask(
			uint64(batch), 3, 3, inputs, weights, biases,
		)
		if err != nil {
			t.Fatalf("batch %d rejected by Go: %v", batch, err)
		}
		reference, err := usefulwork.ComputeMLInferenceQuantized(task)
		if err != nil {
			t.Fatal(err)
		}
		got, err := s.Execute(context.Background(), task, seq)
		if err != nil || !reflect.DeepEqual(got, reference) {
			t.Fatalf("batch %d failed parity: %v", batch, err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	tooLarge := make([]int64, 1366*3)
	if _, err := usefulwork.NewMLInferenceQuantizedTask(
		1366, 3, 3, tooLarge, weights, biases,
	); err == nil {
		t.Fatal("Go unexpectedly accepted batch 1366")
	}
}

// A 65th request must be rejected before any result is accepted.
func TestGPUV063SessionRequestLimit(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux/WSL2 only")
	}
	task := gpuManagerCasesV063(t)[0].Task
	s := gpuManagerRunV063(t, os.Getenv("PRISM_V063_GO_MGR_BIN"), 10*time.Second)
	for seq := 0; seq < 64; seq++ {
		if _, err := s.Execute(context.Background(), task, seq); err != nil {
			t.Fatalf("valid session request %d failed: %v", seq, err)
		}
	}
	if _, err := s.Execute(context.Background(), task, 64); err == nil {
		t.Fatal("65th request was accepted")
	}
	if !s.IsDead() {
		t.Fatal("over-budget request did not terminate the session")
	}
}

// Opening an idle worker must not make clean shutdown fail.
func TestGPUV063SessionCloseWithoutRequests(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux/WSL2 only")
	}
	bin := os.Getenv("PRISM_V063_GO_MGR_BIN")
	if bin == "" {
		t.Skip("set PRISM_V063_GO_MGR_BIN to run CUDA lifecycle tests")
	}
	s := gpuManagerRunV063(t, bin, 10*time.Second)
	if err := s.Close(); err != nil {
		t.Fatalf("idle GPU session failed graceful close: %v", err)
	}
}
