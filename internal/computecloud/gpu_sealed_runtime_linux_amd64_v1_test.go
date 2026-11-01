//go:build linux && amd64

package computecloud

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The mocked worker is Python, never CUDA. It checks that the running process
// sees the sealed memfd on FD 3 and the locked file description on FD 4.
// The test also verifies end-to-end CPU matching and that the GPU lock
// becomes available again after the worker exits.
func TestCloudSealedRuntimeV1LockedExecutesApprovedSnapshotWithFD4(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 unavailable for offline IPC fixture")
	}
	dir := t.TempDir()
	worker := filepath.Join(dir, "mock-cloud-worker")
	lockDir := filepath.Join(dir, "gpu-lock")
	body := []byte(`#!/usr/bin/env python3
import fcntl, hashlib, json, os, sys
assert (fcntl.fcntl(3, 1034) & 15) == 15, "worker FD 3 was not fully sealed"
lock_path = os.readlink("/proc/self/fd/4")
probe = os.open(lock_path, os.O_RDWR)
try:
    try:
        fcntl.flock(probe, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except BlockingIOError:
        pass
    else:
        sys.exit(23)
finally:
    os.close(probe)
req = json.loads(sys.stdin.readline())
task = req["task"]
values = [0]
output_hash = hashlib.sha256(json.dumps(values, separators=(",", ":")).encode()).hexdigest()
receipt = {"schema": "prism-gpu-ml-lab-v0", "task_id": task["id"], "input_hash": task["input_hash"], "result_values": values, "output_hash": output_hash, "gpu_attestation": False}
frame = {"schema": "prism-gpu-session-frame-v063", "seq": req["seq"], "nonce": req["nonce"], "receipt": receipt}
print("PRISM_GPU_SESSION=" + json.dumps(frame, separators=(",", ":")))
`)
	if err := os.WriteFile(worker, body, 0700); err != nil {
		t.Fatal(err)
	}
	result, err := RunLocalGPULabWithGlobalLockV1(context.Background(), []byte(cloudGPUSampleV1), worker, cloudSealedHashV1(body), lockDir)
	if err != nil {
		t.Fatal(err)
	}
	if !result.CPUVerified || result.GPUAttestation || result.DispatchEnabled || result.ProofsAvailable || len(result.ResultValues) != 1 || result.ResultValues[0] != 0 {
		t.Fatalf("unexpected Cloud result after sealed+locked worker: %+v", result)
	}
	lock, err := TryAcquireGlobalGPULockV1(lockDir)
	if err != nil {
		t.Fatalf("cooperative GPU lock not released after worker exit: %v", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
}
