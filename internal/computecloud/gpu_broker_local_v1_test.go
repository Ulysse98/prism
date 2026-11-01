//go:build linux

package computecloud

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func localTestObserverV1(t *testing.T, chain string, healthy *atomic.Bool, height *atomic.Uint64) *CloudNodeObserverV1 {
	t.Helper()
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/status" {
			t.Errorf("unexpected node request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"version": "0.63.0", "chainId": chain,
			"genesisHash": strings.Repeat("a", 64), "height": height.Load(), "chainValid": healthy.Load(),
		})
	}))
	// Force the test server to be on IPv4 loopback, matching the production observer.
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_ = server.Listener.Close()
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)
	_, portStr, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(portStr)
	observer, err := NewCloudNodeObserverV1(port, "prism-test", strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	return observer
}

func TestCloudLocalBrokerV1FailsClosedBeforeGPU(t *testing.T) {
	ctx := context.Background()
	task, err := ParseSubmitV1([]byte(cloudGPUSampleV1))
	if err != nil {
		t.Fatal(err)
	}
	healthy := &atomic.Bool{}
	height := &atomic.Uint64{}
	healthy.Store(true)
	observer := localTestObserverV1(t, "prism-test", healthy, height)
	// The binary is deliberately invalid: these checks must reject before execution.
	cases := []struct {
		name, body, approved string
		node                 *CloudNodeObserverV1
	}{
		{"no observer", cloudGPUSampleV1, task.ID, nil},
		{"invalid input", `{"reward":1}`, task.ID, observer},
		{"wrong approval", cloudGPUSampleV1, strings.Repeat("0", 64), observer},
		{"malformed approval", cloudGPUSampleV1, "invalid", observer},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := RunLocalBrokerGPULabV1(ctx, []byte(tc.body), tc.node, tc.approved, "/nonexistent", strings.Repeat("0", 64)); err == nil {
				t.Fatal("unsafe broker operation accepted")
			}
		})
	}
	healthy.Store(false)
	if _, err := RunLocalBrokerGPULabV1(ctx, []byte(cloudGPUSampleV1), observer, task.ID, "/nonexistent", strings.Repeat("0", 64)); err == nil || !strings.Contains(err.Error(), "preflight node refused") {
		t.Fatalf("unhealthy node did not block GPU: %v", err)
	}
}

func TestCloudLocalBrokerV1RejectsNodeMismatchBeforeGPU(t *testing.T) {
	healthy := &atomic.Bool{}
	healthy.Store(true)
	height := &atomic.Uint64{}
	observer := localTestObserverV1(t, "prism-other", healthy, height)
	task, err := ParseSubmitV1([]byte(cloudGPUSampleV1))
	if err != nil {
		t.Fatal(err)
	}
	_, err = RunLocalBrokerGPULabV1(context.Background(), []byte(cloudGPUSampleV1), observer, task.ID, "/nonexistent", strings.Repeat("0", 64))
	if err == nil || !strings.Contains(err.Error(), "preflight node refused") {
		t.Fatalf("identity mismatch accepted: %v", err)
	}
}

func TestCloudLocalBrokerV1MockExecutionAndPostflight(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 required for mock CUDA receipt")
	}
	_ = python
	task, err := ParseSubmitV1([]byte(cloudGPUSampleV1))
	if err != nil {
		t.Fatal(err)
	}
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
	binary := filepath.Join(t.TempDir(), "mock-worker")
	if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(script))
	hash := hex.EncodeToString(digest[:])
	healthy := &atomic.Bool{}
	healthy.Store(true)
	height := &atomic.Uint64{}
	height.Store(12)
	observer := localTestObserverV1(t, "prism-test", healthy, height)
	got, err := RunLocalBrokerGPULabV1(context.Background(), []byte(cloudGPUSampleV1), observer, task.ID, binary, hash)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != "operator_local_gpu_lab" || got.ChainID != "prism-test" ||
		got.NodeHeight != 12 || got.TaskID != task.ID || !got.CPUVerified ||
		got.HTTPDispatchEnabled || got.EscrowEnabled || got.ProofsAvailable || got.GPUAttestation ||
		len(got.ResultValues) != 1 || got.ResultValues[0] != 0 {
		t.Fatalf("unexpected offline broker result: %+v", got)
	}
}

func TestCloudLocalBrokerV1RejectsNodeChangeAfterGPU(t *testing.T) {
	// Confirm that the postflight guard rejects a result after a node-height
	// change even though the short-lived mock GPU executable returns valid data.
	task, err := ParseSubmitV1([]byte(cloudGPUSampleV1))
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "mock-worker")
	script := `#!/usr/bin/env python3
import hashlib,json,sys
req=json.loads(sys.stdin.readline()); task=req["task"]
values=[0]
receipt={"schema":"prism-gpu-ml-lab-v0","task_id":task["id"],"input_hash":task["input_hash"],"result_values":values,"output_hash":hashlib.sha256(b"[0]").hexdigest(),"gpu_attestation":False}
frame={"schema":"prism-gpu-session-frame-v063","seq":req["seq"],"nonce":req["nonce"],"receipt":receipt}
print("PRISM_GPU_SESSION="+json.dumps(frame,separators=(",",":")))
`
	if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte(script))
	healthy := &atomic.Bool{}
	healthy.Store(true)
	counter := &atomic.Int64{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := counter.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"version": "0.63.0", "chainId": "prism-test", "genesisHash": strings.Repeat("a", 64),
			"height": n - 1, "chainValid": healthy.Load(),
		})
	}))
	t.Cleanup(server.Close)
	_, portRaw, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(portRaw)
	observer, err := NewCloudNodeObserverV1(port, "prism-test", strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	got, err := RunLocalBrokerGPULabV1(context.Background(), []byte(cloudGPUSampleV1), observer, task.ID, binary, hex.EncodeToString(hash[:]))
	if err == nil || got.TaskID != "" || !strings.Contains(err.Error(), "node changed") {
		t.Fatalf("changed node accepted: %+v / %v", got, err)
	}
}
