# Prism v0.66 Phase 1D.2 — Explicit opt-in, sealed one-shot GPU proof path

## What changed

New Linux/amd64-only off-chain function `executeComputeGPUApprovedV066`
connects the 1D.1 `runApprovedGPUWorkerV066` sealed executable runner to
existing one-shot GPU task validation, strict v0.62 receipt parsing, and the
v0.66 contract/proof binding. The old `executeComputeGPUV062` and v0.63 session
launchers remain unchanged and active by default. **No CLI wiring in 1D.2**:
calling this explicit function is the only opt-in; a future operator-facing
integration requires a separate review and secure allowlist configuration.

The pipeline is:

1. Require active context and valid quantized 3x3 ML task (1..4096 samples).
2. Construct an off-chain v0.66 contract with job, chain, genesis and
   independent approved executable SHA-256.
3. Serialize the existing one-shot task request (bounded to 512 KiB).
4. Copy the executable into memfd, verify real bytes against the digest,
   seal, and run with bounded stdout/stderr and a 60-second ceiling.
5. Parse a strictly scoped receipt (the legacy schema has gpu_attestation=false).
6. Build a signed context-bound PoUW v2 proof from the reported predictions.
7. Verify the entire contract/proof binding (includes CPU recomputation) before
   returning the off-chain contract/proof bundle.

## Security boundary

The approved digest MUST come from an operator-controlled independently
maintained allowlist, not from the job/request/receipt/proof. The SHA-256
asserts only which main executable bytes were launched. It does not pin
libraries, drivers, models loaded at runtime, or prove a GPU performed work.
The v0.66 contract is unsigned and NOT part of the existing signed v2 proof;
this bundle is not independently authenticated as an artifact-execution
attestation, does not modify consensus, and is not settlement authorization.

The offline fixture test binary prints a synthetic valid GPU receipt, so
passing tests demonstrate routing, digest checking, parsing and CPU
verification, NOT a real CUDA run. Phase 1D.3 should use a real approved CUDA
worker and explicit operator policy with adversarial regression tests.
