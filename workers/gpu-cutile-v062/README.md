# Prism v0.62 — Rust/cuTile GPU Worker

Experimental NVIDIA GPU backend for Prism PoUW
quantized ML inference.

## Supported tasks

- Task type: `ml_inference_quantized`
- Features: 3
- Classes: 3
- Batch size: 1 to 4096 samples
- Signed 64-bit integer inputs, weights and biases
- Conservative integer-overflow rejection
- No implicit CPU fallback

## Requirements

- Linux or WSL2
- Compatible NVIDIA GPU and driver
- NVIDIA CUDA toolkit (validated with CUDA 13.2)
- Rust/Cargo
- Go toolchain for Prism integration tests

## Build

```bash
export CUDA_TOOLKIT_PATH=/usr/local/cuda-13.2
export CUDA_HOME="$CUDA_TOOLKIT_PATH"
export PATH="$CUDA_TOOLKIT_PATH/bin:$PATH"

cargo build --locked --release \
  --manifest-path workers/gpu-cutile-v062/Cargo.toml \
  --bin ml_gpu_task_3x3
```

The cuTile dependency is pinned to an exact NVIDIA
cuda-rust Git revision in Cargo.toml.

## Protocol

Input: one JSON object on stdin containing a Prism
task under the `task` key.

Output: one stdout line prefixed with:

`PRISM_GPU_RECEIPT=`

The JSON receipt includes:
- schema
- task_id
- input_hash
- result_values
- output_hash
- gpu_attestation: false

The receipt is NOT a cryptographic hardware attestation.

The Go adapter validates the receipt and constructs
a signed PoUW V2 proof. The Prism node independently
verifies the useful-work result before settlement.

## Limitations

This is an experimental GPU backend. It is not a
general-purpose GPU scheduler and supports only the
task dimensions documented above.

Testing on an isolated blockchain does not imply
deployment to the main Prism network.
