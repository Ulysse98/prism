# Prism v0.66 — Phase 1B: Quantized ML Task/Proof Binding

## Intent and scope

This is an experimental **off-chain-only** compatibility bridge. It deliberately
adds no fields to `Task`, `Proof`, or the v0.64 verification report, does not alter
consensus or escrow release rules, and does not dispatch or run CUDA workers.

- `NewQuantizedMLComputeTaskContractV066` validates an existing quantized ML
  `Task` and fills the v0.66 commitment with its canonical task ID and input
  hash, along with caller-provided job/network/genesis and artifact digest.
- `VerifyQuantizedMLComputeTaskBindingV066` validates a separately supplied
  contract against a complete signed v2 `Proof`, checks the network/job context,
  matches the task/input commitments and recomputes the output via `VerifyProof`.
  The `WorkloadID` must be `ml-inference-quantized-v1`.
- A trusted artifact SHA-256 is required separately for an off-chain policy
  comparison. **Neither the v2 proof nor this bridge cryptographically proves
  which executable/model bytes the worker actually used.** The trusted digest
  must come from an independent approved allowlist or verified artifact bytes,
  not from the untrusted proof/request itself.

## Explicit limitations

This contract is **unsigned**. Matching it is not proof of requester approval,
worker GPU execution, hardware provenance, correct external artifact retrieval,
verifier quorum, freshness or entitlement to payment. No escrow release is
permitted by this bridge. Prior v0.65 payout validation remains unchanged.

Tests cover successful binding, wrong job/chain/genesis/task/input/workload,
artifact mismatch, proof version, altered output and invalid signature. Race
and static checks also cover the legacy usefulwork and escrow packages.

Next phases may introduce independently authenticated request policy, artifact
byte validation and independent-verifier acceptance before integrating with
escrow. Those require separate threat-model and consensus compatibility reviews.
