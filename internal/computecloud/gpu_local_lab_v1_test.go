//go:build linux

package computecloud

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"prism/internal/usefulwork"
)

const cloudGPUSampleV1 = `{"batch_size":1,"inputs":[1,0,-1],"weights":[1,0,0,0,1,0,0,0,1],"biases":[0,0,0]}`

func testGPUFrameV1(t *testing.T) (usefulwork.Task, string, []uint64, cloudGPUFrameV1) {
	t.Helper()
	task, err := ParseSubmitV1([]byte(cloudGPUSampleV1))
	if err != nil {
		t.Fatal(err)
	}
	values, err := usefulwork.ComputeMLInferenceQuantized(task)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(values)
	hash := sha256.Sum256(raw)
	receipt := cloudGPUReceiptV1{
		Schema: "prism-gpu-ml-lab-v0", TaskID: task.ID, InputHash: task.InputHash,
		ResultValues: values, OutputHash: hex.EncodeToString(hash[:]), GPUAttestation: false,
	}
	nested, _ := json.Marshal(receipt)
	nonce := strings.Repeat("a", 64)
	return task, nonce, values, cloudGPUFrameV1{
		Schema: "prism-gpu-session-frame-v063", Seq: 0, Nonce: nonce, Receipt: nested,
	}
}

func frameBytesV1(t *testing.T, frame cloudGPUFrameV1) []byte {
	t.Helper()
	raw, err := json.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	return append(append([]byte("PRISM_GPU_SESSION="), raw...), '\n')
}

func TestCloudGPULabV1ValidFrameAndCPUReference(t *testing.T) {
	task, nonce, reference, frame := testGPUFrameV1(t)
	got, err := parseCloudGPUFrameV1(task, nonce, frameBytesV1(t, frame), reference)
	if err != nil {
		t.Fatal(err)
	}
	if got.TaskID != task.ID || got.InputHash != task.InputHash || !got.CPUVerified || got.GPUAttestation ||
		got.ProofsAvailable || got.DispatchEnabled || !reflect.DeepEqual(got.ResultValues, reference) {
		t.Fatalf("unsafe or invalid lab result: %+v", got)
	}
}

func TestCloudGPULabV1RejectMaliciousFrames(t *testing.T) {
	task, nonce, reference, frame := testGPUFrameV1(t)
	wrongReceipt := func(change func(*cloudGPUReceiptV1)) cloudGPUFrameV1 {
		f := frame
		var receipt cloudGPUReceiptV1
		if err := json.Unmarshal(f.Receipt, &receipt); err != nil {
			t.Fatal(err)
		}
		change(&receipt)
		f.Receipt, _ = json.Marshal(receipt)
		return f
	}
	tests := []struct {
		name string
		data []byte
	}{
		{"wrong nonce", frameBytesV1(t, func() cloudGPUFrameV1 { f := frame; f.Nonce = strings.Repeat("b", 64); return f }())},
		{"wrong sequence", frameBytesV1(t, func() cloudGPUFrameV1 { f := frame; f.Seq = 1; return f }())},
		{"incorrect prediction", frameBytesV1(t, wrongReceipt(func(r *cloudGPUReceiptV1) {
			r.ResultValues = []uint64{(reference[0] + 1) % 3}
			raw, _ := json.Marshal(r.ResultValues)
			h := sha256.Sum256(raw)
			r.OutputHash = hex.EncodeToString(h[:])
		}))},
		{"faked attestation", frameBytesV1(t, wrongReceipt(func(r *cloudGPUReceiptV1) { r.GPUAttestation = true }))},
		{"bad output hash", frameBytesV1(t, wrongReceipt(func(r *cloudGPUReceiptV1) { r.OutputHash = strings.Repeat("0", 64) }))},
		{"wrong input hash", frameBytesV1(t, wrongReceipt(func(r *cloudGPUReceiptV1) { r.InputHash = strings.Repeat("0", 64) }))},
		{"two lines", append(frameBytesV1(t, frame), []byte("surprise\n")...)},
		{"missing newline", bytes.TrimSuffix(frameBytesV1(t, frame), []byte{'\n'})},
		{"unexpected top-level key", append(append([]byte("PRISM_GPU_SESSION={\"extra\":1,"), frameBytesV1(t, frame)[len("PRISM_GPU_SESSION={"):]...), []byte{}...)},
		{"duplicate frame key", []byte("PRISM_GPU_SESSION={\"schema\":\"junk\",\"schema\":\"prism-gpu-session-frame-v063\",\"seq\":0,\"nonce\":\"" + nonce + "\",\"receipt\":" + string(frame.Receipt) + "}\n")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseCloudGPUFrameV1(task, nonce, tc.data, reference); err == nil {
				t.Fatal("malicious frame accepted")
			}
		})
	}
}

func TestCloudGPULabV1RefusesUntrustedBinaryAndBadRequests(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "fake")
	content := []byte("#!/bin/sh\nexit 17\n")
	if err := os.WriteFile(bin, content, 0700); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(content)
	hash := hex.EncodeToString(h[:])
	if err := validateCloudGPUBinaryV1(bin, hash); err != nil {
		t.Fatal(err)
	}
	if err := validateCloudGPUBinaryV1(bin, strings.Repeat("0", 64)); err == nil {
		t.Fatal("wrong binary SHA accepted")
	}
	link := filepath.Join(dir, "symlink")
	if err := os.Symlink(bin, link); err != nil {
		t.Fatal(err)
	}
	if err := validateCloudGPUBinaryV1(link, hash); err == nil {
		t.Fatal("symlink binary accepted")
	}
	if _, err := RunLocalGPULabV1(context.Background(), []byte(`{"reward":1}`), bin, hash); err == nil {
		t.Fatal("invalid cloud workload accepted")
	}
	if _, err := RunLocalGPULabV1(context.Background(), []byte(cloudGPUSampleV1), bin, hash); err == nil {
		t.Fatal("crashing worker accepted")
	}
}

// The fake worker qualifies IPC and supervision only. The actual RTX/CUDA
// device is qualified separately by the explicit operator CLI invocation.
func TestCloudGPULabV1MockProcessEndToEnd(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 needed for offline mock process")
	}
	_ = python
	bin := filepath.Join(t.TempDir(), "fake-gpu-worker")
	script := `#!/usr/bin/env python3
import hashlib, json, sys
req=json.loads(sys.stdin.readline())
task=req["task"]
inputs=task["signed_values"]
weights=task["signed_values_b"]
biases=task["biases"]
values=[]
for sample in range(task["rows_a"]):
    x=inputs[3*sample:3*sample+3]
    scores=[sum(x[k]*weights[3*c+k] for k in range(3))+biases[c] for c in range(3)]
    values.append(max(range(3), key=lambda c: scores[c]))
out_hash=hashlib.sha256(json.dumps(values,separators=(",",":")).encode()).hexdigest()
receipt={"schema":"prism-gpu-ml-lab-v0","task_id":task["id"],"input_hash":task["input_hash"],"result_values":values,"output_hash":out_hash,"gpu_attestation":False}
frame={"schema":"prism-gpu-session-frame-v063","seq":req["seq"],"nonce":req["nonce"],"receipt":receipt}
print("PRISM_GPU_SESSION="+json.dumps(frame,separators=(",",":")))
`
	if err := os.WriteFile(bin, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(script))
	result, err := RunLocalGPULabV1(context.Background(), []byte(cloudGPUSampleV1), bin, hex.EncodeToString(digest[:]))
	if err != nil {
		t.Fatal(err)
	}
	if !result.CPUVerified || len(result.ResultValues) != 1 || result.ResultValues[0] != 0 || result.ProofsAvailable || result.GPUAttestation {
		t.Fatalf("unexpected fake GPU result: %+v", result)
	}
}
