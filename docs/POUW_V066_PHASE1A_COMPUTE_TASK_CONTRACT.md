# Prism v0.66 — Phase 1A: Real Compute Task Contract

## Scope

This experimental, **off-chain** contract defines a deterministic request commitment. It is not part of blockchain consensus, cannot release escrow funds, and does not change existing `Proof`, `Task`, `ComputeVerificationReportV064`, or v0.65 escrow APIs.

`ComputeTaskContractV066` binds the following exact strings: contract version, chain ID, genesis hash, job ID, task ID, input SHA-256, workload ID, and artifact SHA-256. The `ComputeTaskContractIDV066` identifier is SHA-256 over `Prism/PoUW/RealComputeTask/v0.66\x00` concatenated with fixed-order struct JSON. Validation rejects malformed versions, noncanonical SHA-256 strings, empty or unsupported identifier characters, and identifier strings longer than 128 bytes.

## What this does *not* prove

- The input and artifact bytes have **not** been fetched or hashed by this module. Their provided digests are declarations that a later component must verify against actual bytes.
- A matching request digest does not show that a GPU executed any work, nor that an output is correct.
- This unsigned commitment is not authorization from a requester, worker, or verifier. Signature, ownership, replay-protection against chain state, resource budget, and result receipt validation are future phases.
- No mapping from this new contract to existing `usefulwork.Task` / `Proof` semantics is assumed. Inspect and test those definitions before introducing any bridge.
- The independent v0.64 verification report is optional under existing v0.65 escrow rules; this phase does not alter settlement policy.

## Verification

Run `go test -count=1 ./internal/usefulwork ./internal/escrow`, `go test -race -count=1 ./internal/usefulwork ./internal/escrow`, and `go vet ./internal/usefulwork ./internal/escrow`.

Next: inspect `internal/usefulwork/usefulwork.go` task and proof definitions and decide the exact identity/compatibility bridge and threat model before connecting this contract to any dispatch or settlement path.
