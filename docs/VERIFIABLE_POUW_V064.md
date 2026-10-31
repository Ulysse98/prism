# PRISM v0.64 — Verifiable PoUW, Phase 1 (experimental)

## Scope

- Reuse existing context-bound PoUW Proof v2 for quantized ML inference.
- An independent verifier re-executes the **entire** ML result on Go/CPU
  using the existing `VerifyProof` path (not sampling).
- An independent Ed25519 wallet signs a domain-separated, fixed-order
  JSON attestation containing job ID, proof ID, chain ID, genesis hash,
  task/input/output commitments, and deterministic work units.
- The worker cannot sign an attestation with the same wallet identity.
- Altered fields, invalid signatures and cross-job/cross-chain replay fail.

## Security boundaries

This is an **off-chain attestation prototype**, not a ZK/STARK proof,
trusted execution proof, hardware/GPU attestation, decentralized consensus
vote, sybil resistance, or a change to the existing settlement policy.
Distinct wallet addresses alone do not prove different human operators.
The verifier must obtain the real job, canonical genesis and proof context
from a trusted full node before deciding whether an attestation is relevant.

**Do not distribute rewards based on a v0.64 report alone.** Existing
`VerifyProof` and native blockchain verification remain authoritative.
The report is neither persisted nor broadcast by this phase.

## Test on a development clone

```bash
gofmt -w internal/usefulwork/compute_verification_v064*.go
go test ./internal/usefulwork -run '^TestComputeVerificationV064' -count=1
go test ./... -count=1
go vet ./...
git diff --check
```

Do not merge before all tests and code review pass. Follow-up phases:
independent verifier process, candidate report API, persistence/replay
controls, and carefully reviewed policy integration on an isolated testnet.
