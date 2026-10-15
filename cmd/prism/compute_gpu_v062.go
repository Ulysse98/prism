package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

// boundedGPUOutputV062 retains only a fixed prefix of subprocess output.
// The overflow bit forces the caller to reject oversized output.
// exec.Cmd owns separate readers for stdout and stderr.
type boundedGPUOutputV062 struct {
	data     []byte
	limit    int
	exceeded bool
}

func (b *boundedGPUOutputV062) Write(p []byte) (int, error) {
	n := len(p)
	space := b.limit - len(b.data)
	if space < 0 {
		space = 0
	}
	if space > n {
		space = n
	}
	b.data = append(b.data, p[:space]...)
	if n > space {
		b.exceeded = true
	}
	return n, nil
}

// gpuMLReceiptV062 is an experimental worker-to-client protocol.
// It is not a hardware attestation or a consensus proof.
type gpuMLReceiptV062 struct {
	Schema         string   `json:"schema"`
	TaskID         string   `json:"task_id"`
	InputHash      string   `json:"input_hash"`
	ResultValues   []uint64 `json:"result_values"`
	OutputHash     string   `json:"output_hash"`
	GPUAttestation bool     `json:"gpu_attestation"`
}

func validateGPUComputeTaskV062(task usefulwork.Task) error {
	if err := usefulwork.ValidateTask(task); err != nil {
		return err
	}
	if task.Type != usefulwork.TaskTypeMLInferenceQuantized ||
		task.RowsA == 0 || task.RowsA > 4096 ||
		task.ColsA != 3 || task.ColsB != 3 {
		return fmt.Errorf("GPU v0.62 supports only ML quantized 3x3, 1..4096 samples")
	}
	return nil
}

func validateGPUWorkerBinaryV062(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("-gpu-bin requires an absolute path")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("GPU binary unavailable: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("GPU binary is not a regular file")
	}
	if info.Mode().Perm()&0111 == 0 {
		return fmt.Errorf("GPU binary is not executable")
	}
	return nil
}

func parseGPUReceiptV062(task usefulwork.Task, stdout []byte) ([]uint64, error) {
	const prefix = "PRISM_GPU_RECEIPT="
	if len(stdout) > 128*1024 {
		return nil, fmt.Errorf("GPU output exceeds limit")
	}
	line := strings.TrimSpace(string(stdout))
	if strings.ContainsAny(line, "\r\n") || !strings.HasPrefix(line, prefix) {
		return nil, fmt.Errorf("GPU stdout must contain exactly one receipt line")
	}
	raw := []byte(strings.TrimPrefix(line, prefix))
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fmt.Errorf("invalid GPU receipt JSON: %w", err)
	}
	required := []string{
		"schema", "task_id", "input_hash", "result_values",
		"output_hash", "gpu_attestation",
	}
	if len(fields) != len(required) {
		return nil, fmt.Errorf("unexpected GPU receipt fields")
	}
	for _, name := range required {
		if _, ok := fields[name]; !ok {
			return nil, fmt.Errorf("missing GPU receipt field: %s", name)
		}
	}
	var r gpuMLReceiptV062
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&r); err != nil {
		return nil, fmt.Errorf("invalid GPU receipt structure: %w", err)
	}
	if r.Schema != "prism-gpu-ml-lab-v0" || r.GPUAttestation {
		return nil, fmt.Errorf("unsupported GPU receipt schema or false attestation")
	}
	if r.TaskID != task.ID || r.InputHash != task.InputHash {
		return nil, fmt.Errorf("GPU receipt task commitment mismatch")
	}
	if uint64(len(r.ResultValues)) != task.RowsA {
		return nil, fmt.Errorf("GPU prediction count mismatch")
	}
	for _, class := range r.ResultValues {
		if class >= task.ColsB {
			return nil, fmt.Errorf("GPU prediction class out of range")
		}
	}
	encoded, err := json.Marshal(r.ResultValues)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(encoded)
	if r.OutputHash != hex.EncodeToString(digest[:]) {
		return nil, fmt.Errorf("GPU result SHA-256 mismatch")
	}
	return r.ResultValues, nil
}

// executeComputeGPUV062 runs the actual CUDA process. No CPU inference
// is used on this path. Nodes still independently VerifyProof.
func executeComputeGPUV062(
	task usefulwork.Task, jobID, chainID, genesisHash string,
	worker *wallet.Wallet, binary string,
) (usefulwork.Proof, error) {
	if err := validateGPUComputeTaskV062(task); err != nil {
		return usefulwork.Proof{}, err
	}
	if err := validateGPUWorkerBinaryV062(binary); err != nil {
		return usefulwork.Proof{}, err
	}
	request, err := json.Marshal(struct {
		Task usefulwork.Task `json:"task"`
	}{Task: task})
	if err != nil {
		return usefulwork.Proof{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	stdoutData, err := runGPUWorkerProcessV062(ctx, binary, request)
	if err != nil {
		return usefulwork.Proof{}, err
	}
	predictions, err := parseGPUReceiptV062(task, stdoutData)
	if err != nil {
		return usefulwork.Proof{}, err
	}
	return usefulwork.BuildComputeMLProofV2FromResult(
		task, jobID, chainID, genesisHash, worker, predictions,
	)
}
