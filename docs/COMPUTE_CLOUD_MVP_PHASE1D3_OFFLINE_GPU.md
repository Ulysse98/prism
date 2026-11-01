# Prism Compute Cloud — Phase 1D.3: opt-in offline CUDA executor lab

This is a **local-only lab**. It deliberately adds **no HTTP endpoint** and does not change the `prism-cloud serve` code. It cannot create marketplace jobs, submit a proof, use a wallet, issue rewards, contact the node, or persist jobs.

## Security and limitations

- Requires explicit `-enable-local-gpu` CLI flag, absolute GPU executable path, and trusted SHA-256 pin. Uses the Cloud v1 3×3 workload validator and batch cap 1024.
- Pins executable SHA-256 and denies symlinks/group or world writable binaries. This is NOT a sandbox: use only a locally built, trusted executable, on a trusted host. There is a time-of-check/time-of-use gap between hash verification and executing the pathname. Do not use this code with arbitrary tenant binaries.
- One short-lived worker process per invocation, 20-second timeout; an explicit Linux process group is killed on timeout/cancel. Bounded IPC input 512 KiB, stdout 128 KiB, stderr 16 KiB. Stdout is never used without validating an exact nonce/sequence and nested task/output commitments.
- Limits subprocess environment to HOME, PATH, CUDA_HOME, CUDA_TOOLKIT_PATH, LC_ALL; does not pass cloud API keys or runtime tokens. The trusted CUDA runtime can still use its normal system/GPU facilities.
- Verifies GPU predictions against independent CPU `usefulwork.ComputeMLInferenceQuantized`, including the output hash, before returning results. `gpu_attestation` is always false. A result is NOT a verifiable PoUW proof.
- No tenant isolation, workload scheduling, queuing, service launch, quota accounting, or production dispatch. These require separate designs and validation.
- Actual CUDA qualification must be run explicitly on the target WSL RTX host. Go tests use a Python mock worker for protocol qualification, not CUDA hardware attestation.

## WSL usage after install

```bash
export PATH="/usr/local/go/bin:/usr/local/cuda-13.2/bin:$HOME/.cargo/bin:$PATH"
cd "$HOME/prism-lab/prism-cloud-mvp-phase1"
BIN="$HOME/prism-lab/prism-core-v063/workers/gpu-cutile-v062/target/release/ml_gpu_session_hardened_v063"
PIN="c8d219661ab539569cc04ec712b976632f582184419af8a6eda07db2d0fcaec0"

test "$(sha256sum "$BIN" | cut -d ' ' -f1)" = "$PIN"

printf '%s\n' '{"batch_size":1,"inputs":[1,0,-1],"weights":[1,0,0,0,1,0,0,0,1],"biases":[0,0,0]}' | \
  go run ./cmd/prism-cloud-gpu-lab \
    -enable-local-gpu -gpu-bin "$BIN" -sha256 "$PIN"
```

Expected output is JSON with `mode:"offline_gpu_lab"`, the canonical task ID and input hash, `result_values:[0]`, `cpu_verified:true`, `gpu_attestation:false`, `dispatch_enabled:false`, and `proofs_available:false`. The real CUDA result must be observed before the lab can be marked accepted on this system.

## Changes

New files only:

- `internal/computecloud/gpu_local_lab_v1.go`
- `internal/computecloud/gpu_local_lab_v1_test.go`
- `cmd/prism-cloud-gpu-lab/main.go`
- `cmd/prism-cloud-gpu-lab/main_test.go`
- `docs/COMPUTE_CLOUD_MVP_PHASE1D3_OFFLINE_GPU.md`

The installer checks the pristine Phase 1C.2 commit `454b78e`, then runs Go unit/race tests, vet, and compile. It does not stage, commit, push, connect to a node, or execute the CUDA worker.
