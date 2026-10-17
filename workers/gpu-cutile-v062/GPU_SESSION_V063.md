# Prism v0.63 — Persistent CUDA PoUW worker (experimental)

The `gpu-session` compute backend reuses one NVIDIA CUDA process across
quantized ML PoUW tasks, avoiding a new CUDA initialization for each job.
It is **opt-in**. The default remains CPU, and the v0.62 `gpu` one-shot
backend is not changed. Do not use this experimental backend with real funds
or a production Prism node without an additional operational review.

## Supported computation and requirements

- `ml_inference_quantized`, exactly 3 features and 3 classes.
- Batch size **1–1365** (Go limits signed input values to 4096 total).
- Signed `int64` inputs, weights and biases with overflow checks.
- Linux/WSL2, NVIDIA GPU, CUDA toolkit, Rust/Cargo and Prism Go CLI.
- No implicit CPU fallback if CUDA fails.

Build the separate v0.63 worker from the repository root:

```bash
export CUDA_TOOLKIT_PATH=/usr/local/cuda-13.2
export CUDA_HOME="$CUDA_TOOLKIT_PATH"
export PATH="$CUDA_TOOLKIT_PATH/bin:$PATH"
cargo build --locked --release \
  --manifest-path workers/gpu-cutile-v062/Cargo.toml \
  --bin ml_gpu_session_hardened_v063
```

## Using the backend on an isolated local testnet

Substitute the absolute binary path, a locally isolated testnet data directory,
and a job that is already present in the test marketplace. Do not point these
examples at a live wallet or funded production node.

```bash
GPU_BIN="$PWD/workers/gpu-cutile-v062/target/release/ml_gpu_session_hardened_v063"

# Explicit single job (session closes at the end of the CLI invocation).
./prism compute-worker \
  -api http://127.0.0.1:8080/api/v1 -data ./isolated-test-data \
  -compute-backend gpu-session -gpu-bin "$GPU_BIN" Bob "$JOB_ID"

# Continuous opt-in autonomous mode: CUDA is reused between eligible jobs.
./prism compute-worker \
  -api http://127.0.0.1:8080/api/v1 -data ./isolated-test-data \
  -compute-backend gpu-session -gpu-bin "$GPU_BIN" \
  -auto -poll 5s Bob
```

`-gpu-bin` must be an **absolute executable path**. Unsupported tasks are
rejected before claiming. On GPU failure the CLI creates no proof and does not
submit a completion; a previously claimed job may be retried in a new session.
The node, not the CUDA worker, verifies the PoUW proof before settlement.

## Session protocol and containment

- One newline-delimited request per task, bounded to **512 KiB**.
- Each response is a `PRISM_GPU_SESSION=` envelope containing `seq`, a fresh
  random 64-character hexadecimal `nonce`, and the original six-field receipt.
- The Go supervisor validates sequence, nonce, task commitment and output
  before constructing a signed PoUW V2 proof. Stale/replayed responses fail.
- A session processes at most **64 tasks** and has a **45-second maximum life**.
  After either limit, the owner creates a fresh CUDA process for later work.
- Go applies per-request timeouts and bounded stdout/stderr collection. An
  invalid response, timeout or crash poisons the session.
- `gpu_attestation: false` remains explicit. A matching nonce is **not**
  hardware attestation and is not a consensus proof.

## CI and local qualification

The Rust workflow checks pinned dependencies and formatting, and performs a
CUDA 13.2 **host compilation** of both Rust bins. This does not execute CUDA
on a real GPU. Go CI runs Go tests without a GPU; GPU-specific E2E tests are
run separately on suitable hardware.

On a dedicated local test environment:

```bash
export PRISM_V063_E2E_GPU_BIN="$PWD/workers/gpu-cutile-v062/target/release/ml_gpu_session_hardened_v063"
go test ./cmd/prism -run '^TestGPUV063(ActualCLI|AutoCLI|MarketSession)' -count=1
go test -race ./cmd/prism -run '^TestGPUV063(ActualCLI|AutoCLI)' -count=1
```

## Performance boundaries (RTX 3070 Laptop, offline lab)

| Quantized ML 3×3 batch | CPU Go median | Persistent GPU warm median |
|---:|---:|---:|
| 256 | 0.085 ms | 1.137 ms |
| 1024 | 0.184 ms | 1.880 ms |
| 1365 | 0.282 ms | 1.900 ms |

Measured wall-clock timings include Go proof construction and IPC, but not
HTTP or blockchain settlement. The GPU session is dramatically quicker than
relaunching CUDA for each task, **but CPU remains faster on all three small
3×3 workloads**. Do not claim GPU throughput superiority from these tests.

This v0.63 backend is experimental; testing on an isolated chain does not
constitute approval for real-network production deployment.
