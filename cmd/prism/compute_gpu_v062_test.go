package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

func gpuTaskForTestV062(t *testing.T) usefulwork.Task {
	t.Helper()
	task, err := usefulwork.NewMLInferenceQuantizedTask(
		3, 3, 3,
		[]int64{2, -3, 1, -1, 4, 2, 0, -2, 5},
		[]int64{2, 1, -1, -1, -2, 2, 1, 1, 1},
		[]int64{1, -1, 0},
	)
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func TestGPUReceiptValidationV062(t *testing.T) {
	task := gpuTaskForTestV062(t)
	predictions := []uint64{1, 2, 1}
	encoded, _ := json.Marshal(predictions)
	sum := sha256.Sum256(encoded)
	good := gpuMLReceiptV062{
		Schema:         "prism-gpu-ml-lab-v0",
		TaskID:         task.ID,
		InputHash:      task.InputHash,
		ResultValues:   predictions,
		OutputHash:     hex.EncodeToString(sum[:]),
		GPUAttestation: false,
	}
	format := func(r gpuMLReceiptV062) []byte {
		t.Helper()
		data, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		return append([]byte("PRISM_GPU_RECEIPT="), data...)
	}
	got, err := parseGPUReceiptV062(task, format(good))
	if err != nil || !reflect.DeepEqual(got, predictions) {
		t.Fatalf("valid receipt rejected: %v (%v)", err, got)
	}
	tests := []struct {
		name   string
		change func(*gpuMLReceiptV062)
	}{
		{"wrong task", func(r *gpuMLReceiptV062) { r.TaskID = strings.Repeat("0", 64) }},
		{"wrong input hash", func(r *gpuMLReceiptV062) { r.InputHash = strings.Repeat("0", 64) }},
		{"wrong result hash", func(r *gpuMLReceiptV062) { r.OutputHash = strings.Repeat("0", 64) }},
		{"false attestation", func(r *gpuMLReceiptV062) { r.GPUAttestation = true }},
		{"out of range", func(r *gpuMLReceiptV062) { r.ResultValues = []uint64{3, 2, 1} }},
		{"short predictions", func(r *gpuMLReceiptV062) { r.ResultValues = []uint64{1} }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bad := good
			tc.change(&bad)
			if _, err := parseGPUReceiptV062(task, format(bad)); err == nil {
				t.Fatal("invalid receipt accepted")
			}
		})
	}
	if _, err := parseGPUReceiptV062(task, append(format(good), []byte("\nextra")...)); err == nil {
		t.Fatal("extra stdout accepted")
	}
	if err := validateGPUWorkerBinaryV062("relative/path"); err == nil {
		t.Fatal("relative executable path accepted")
	}
}

func TestGPUWorkerLiveV062(t *testing.T) {
	binary := os.Getenv("PRISM_GPU_TASK_BIN")
	if binary == "" {
		t.Skip("set PRISM_GPU_TASK_BIN to run real CUDA integration")
	}
	task := gpuTaskForTestV062(t)
	worker, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	const job, chain, genesis = "lab-job-4f", "lab-chain-4f", "lab-genesis-4f"
	proof, err := executeComputeGPUV062(task, job, chain, genesis, worker, binary)
	if err != nil {
		t.Fatal(err)
	}
	if err := usefulwork.VerifyProof(proof); err != nil {
		t.Fatal(err)
	}
	if err := usefulwork.VerifyComputeProofContext(proof, job, chain, genesis); err != nil {
		t.Fatal(err)
	}
	cpu, err := usefulwork.ExecuteCompute(task, job, chain, genesis, worker)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cpu, proof) {
		t.Fatal("GPU proof differs from CPU reference")
	}
	t.Logf("REAL CUDA GPU -> PRISM SIGNED V2 PASS: %s", proof.ID)
}
