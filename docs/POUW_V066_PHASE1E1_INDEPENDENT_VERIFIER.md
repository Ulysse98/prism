# Prism v0.66 — Phase 1E.1 independent verifier qualification

This phase is **off-chain only** and leaves v0.64 proof verification, v0.65 escrow, blockchain consensus, and legacy GPU launch paths unchanged.

## Trust boundaries

- `VerifyIndependentQuantizedMLComputeV066` checks an externally approved artifact digest, a v0.66 ML task contract, a signed context-bound PoUW v2 proof, a v0.64 independent verification report, and an externally approved verifier address.
- The source of `approvedArtifactSHA256` and `approvedVerifierAddress` must be an operator-maintained trusted policy, not an untrusted request or report. A distinct key is not by itself evidence of an independent organization or trustworthy verifier.
- The v0.64 report reflects independent CPU recomputation, NOT hardware GPU attestation, authenticated CUDA dependencies, a ZK proof, or permission to pay.
- Cross-job, cross-chain, modified-output, and unapproved-verifier combinations are rejected. Replaying the exact same report with the exact same proof is not prevented (this function has no durable anti-replay state); escrow consensus must independently prevent double settlement.
- The unsigned v0.66 compute contract is not inside the signed v2 proof. Binding its fields and the external artifact/verifier policy happens at the verification boundary, not cryptographically at the worker signature level.

## Test modes

Run without parameters for offline Go unit tests, race tests, and non-CUDA regression. Run with `--run-real-cuda` to additionally run one explicit real CUDA sealed-worker test and independent verification, with the locally pinned v0.63 binary hash.

The lab's `flock` cooperates only with other v0.66 smoke tests. `nvidia-smi` can have incomplete WSL process visibility. Ensure no Prism Cloud worker or other CUDA workload is active. The test does **not** submit a transaction, touch balances, or run a real network node.
