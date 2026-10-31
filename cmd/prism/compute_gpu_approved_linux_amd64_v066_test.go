//go:build linux && amd64

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

func approvedGPUFixtureV066(t *testing.T) (usefulwork.Task, *wallet.Wallet) {
	t.Helper()
	task, err := usefulwork.NewMLInferenceQuantizedTask(
		1, 3, 3,
		[]int64{0, 0, 0},
		[]int64{0, 0, 0, 0, 0, 0, 0, 0, 0},
		[]int64{0, 1, 2},
	)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	return task, worker
}

// A tiny offline ELF that prints a deliberately constructed receipt. This
// tests the PROCESS -> RECEIPT -> SIGNED PROOF plumbing, NOT GPU execution.
func fakeApprovedGPUWorkerV066(t *testing.T, task usefulwork.Task, mode string) (string, string) {
	t.Helper()
	predictions, err := usefulwork.ComputeMLInferenceQuantized(task)
	if err != nil {
		t.Fatal(err)
	}
	line := "invalid receipt\n"
	if mode != "invalid-receipt" {
		if mode == "wrong-result" {
			predictions = append([]uint64(nil), predictions...)
			predictions[0] = (predictions[0] + 1) % task.ColsB
		}
		outputJSON, err := json.Marshal(predictions)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(outputJSON)
		receipt, err := json.Marshal(struct {
			Schema         string   `json:"schema"`
			TaskID         string   `json:"task_id"`
			InputHash      string   `json:"input_hash"`
			ResultValues   []uint64 `json:"result_values"`
			OutputHash     string   `json:"output_hash"`
			GPUAttestation bool     `json:"gpu_attestation"`
		}{"prism-gpu-ml-lab-v0", task.ID, task.InputHash, predictions, hex.EncodeToString(sum[:]), false})
		if err != nil {
			t.Fatal(err)
		}
		line = "PRISM_GPU_RECEIPT=" + string(receipt) + "\n"
	}

	dir := t.TempDir()
	src := filepath.Join(dir, "worker.go")
	binary := filepath.Join(dir, "worker")
	program := "package main\nimport \"fmt\"\nfunc main() { fmt.Print(" + strconv.Quote(line) + ") }\n"
	if err := os.WriteFile(src, []byte(program), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "-trimpath", "-o", binary, src)
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile offline ELF fixture: %v: %s", err, out)
	}
	blob, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(blob)
	return binary, hex.EncodeToString(digest[:])
}

func TestGPUApprovedOptInV066ValidBoundProof(t *testing.T) {
	task, worker := approvedGPUFixtureV066(t)
	binary, hash := fakeApprovedGPUWorkerV066(t, task, "valid")
	got, err := executeComputeGPUApprovedV066(
		context.Background(), task, "job-v066", "prism-testnet", strings.Repeat("a", 64), worker, binary, hash,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got.Contract.ArtifactHash != hash || got.Proof.Task.ID != task.ID {
		t.Fatal("invalid result binding")
	}
	if err := usefulwork.VerifyQuantizedMLComputeTaskBindingV066(got.Contract, got.Proof, hash); err != nil {
		t.Fatal(err)
	}
}

func TestGPUApprovedOptInV066RejectsWrongExecutableHash(t *testing.T) {
	task, worker := approvedGPUFixtureV066(t)
	binary, _ := fakeApprovedGPUWorkerV066(t, task, "valid")
	_, err := executeComputeGPUApprovedV066(context.Background(), task, "job-v066", "prism-testnet", strings.Repeat("a", 64), worker, binary, strings.Repeat("0", 64))
	if err == nil {
		t.Fatal("unapproved executable accepted")
	}
}

func TestGPUApprovedOptInV066RejectsWrongModelResult(t *testing.T) {
	task, worker := approvedGPUFixtureV066(t)
	binary, hash := fakeApprovedGPUWorkerV066(t, task, "wrong-result")
	_, err := executeComputeGPUApprovedV066(context.Background(), task, "job-v066", "prism-testnet", strings.Repeat("a", 64), worker, binary, hash)
	if err == nil {
		t.Fatal("incorrect ML inference result accepted")
	}
}

func TestGPUApprovedOptInV066RejectsInvalidReceipt(t *testing.T) {
	task, worker := approvedGPUFixtureV066(t)
	binary, hash := fakeApprovedGPUWorkerV066(t, task, "invalid-receipt")
	_, err := executeComputeGPUApprovedV066(context.Background(), task, "job-v066", "prism-testnet", strings.Repeat("a", 64), worker, binary, hash)
	if err == nil {
		t.Fatal("invalid worker receipt accepted")
	}
}

func TestGPUApprovedOptInV066RejectsCancelledContext(t *testing.T) {
	task, worker := approvedGPUFixtureV066(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := executeComputeGPUApprovedV066(ctx, task, "job-v066", "prism-testnet", strings.Repeat("a", 64), worker, "/nonexistent", strings.Repeat("0", 64)); err == nil {
		t.Fatal("cancelled context accepted")
	}
}

func TestGPUApprovedOptInV066RejectsUnsupportedTask(t *testing.T) {
	task, worker := approvedGPUFixtureV066(t)
	task.ColsA = 2
	if _, err := executeComputeGPUApprovedV066(context.Background(), task, "job-v066", "prism-testnet", strings.Repeat("a", 64), worker, "/nonexistent", strings.Repeat("0", 64)); err == nil {
		t.Fatal("unsupported GPU task accepted")
	}
}
