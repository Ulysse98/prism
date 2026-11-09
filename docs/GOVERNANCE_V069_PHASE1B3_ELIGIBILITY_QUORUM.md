# PRISM v0.69 / Phase 1B.3 — Advisory Eligibility & Quorum Lab

## Scope and non-goals

This is **local, offline, advisory computation only**. It grants **no** voting rights, token or stake ownership, validator status, execution authorization, production governance, on-chain consensus, BFT quorum, cross-node finality, or treasury control. It does **not** implement real validator or token snapshot proofs. It does not modify existing v0.61 timelock or proposal admission.

This module adds `EligibilitySnapshotCommitment` and `TallyAdvisoryVotes` to the existing `internal/governancev069` package. Pure functions perform no disk writes, RPC, key generation, networking, wallet operations, signing, or Git changes.

## Trust boundary

The caller must independently acquire and approve the canonical eligibility roster and its exact SHA-256 commitment before accepting tally inputs. A digest pin detects roster mutation but **does not authenticate** who chose the roster, prove assets/stake/balances, establish distribution fairness, or supply distributed consensus. An adversary allowed to choose both roster and pin can choose the electorate.

- The roster is bound to `network`, `chain_id`, `genesis_sha256`, `proposal_sha256` and a `snapshot_height` equal to the proposal's `created_at_height`.
- The trusted context must match the proposal; the proposal must be active at `CurrentHeight`. Advisory votes must be valid at that same height.
- Member IDs are lowercase 32-byte Ed25519 public-key encodings (64 hex characters), sorted lexicographically, without duplicates. Weights are nonzero `uint64`; total must not overflow `uint64`. Maximum 10,000 voters.
- A separate exact lowercase digest pin is required. No fallback to an unpinned roster.
- Every ballot signature is verified using the already-existing Phase 1B.2 signature and proposal/context verification. Ineligible or duplicate voters, signature errors, malformed or context-mismatched votes cause the **whole** tally to fail. Duplicate detection ignores nonce and choice.
- Inputs and results are memory-only. This does not inspect or reconcile the persisted Phase 1B.2 Linux reservation store; a production intake process would need reliable, auditable reconciliation first.

## Local arithmetic rules

- `QuorumBps` is 1 to 10,000; `ApprovalBps` is 5,001 to 10,000 (strict majority or more). 10,000 basis points = 100%.
- Quorum = `YES + NO + ABSTAIN` weights compared against all eligible weight.
- Approval = `YES / (YES + NO)` (abstain excluded). If `YES + NO == 0`, approval is false.
- `AdvisoryPassed = QuorumMet && ApprovalMet`. This is **not** execution approval and may change while new votes arrive; there is no finalization/closing/quorum certificate.
- Comparisons use exact integer arithmetic with `math/big`, not floats; equality meets threshold. Overflow and empty or unbounded rosters fail closed.
- The proposal and roster are pinned in output for audit.

## Adversarial coverage

Unit/race/vet checks cover deterministic snapshot commitment, tampering of network, chain, genesis, proposal, height, policy, member weight; invalid version, unsorted/duplicate identities, zero or overflowing weights, oversized rosters; wrong digest pin; wrong trusted context; unknown voter; invalid signature; duplicate voter using different nonce/choice; zero ballots; abstain-only ballots; exact thresholds; very large `uint64` sums and no partial tally on error.

## Command (Linux WSL)

```bash
export PATH="/usr/local/go/bin:$PATH"
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOWORK=off
cd "$HOME/prism-v069-governance"
umask 077
go test -count=1 ./internal/governancev069
go test -race -count=1 ./internal/governancev069
go vet ./internal/governancev069
```

## Proposed later phases (not delivered here)

1. Independently verifiable eligibility source and governance policy review.
2. Auditable roster publication, chain and height proof, data availability and ballot collection/reconciliation.
3. Close/finalize semantics, deterministic certificates, adversarial multi-node simulations and independent review.

The existence of a passing local test does not imply the protocol is fit for real-value production governance.
