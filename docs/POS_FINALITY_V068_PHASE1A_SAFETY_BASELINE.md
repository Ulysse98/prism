# PRISM v0.68 — Phase 1A: Validator/PoS safety baseline

**STATUS: research-only and non-activating. This is not a deployed consensus protocol or a security audit.**

## Pinned starting point

- GitHub `master`: `32f4e5eef2447b7625875588348aafcef6a32ab4`, after v0.67 PR #52.
- Existing `v0.67` tag: `7a06f757328932b998c485c05eff3313f5e67f5a` (never rewrite).
- The active PoS selector remains in `internal/consensus/pos.go`.
- The finality laboratory is in `internal/finality`, starting with the v0.65 work.
- This phase adds **tests and documentation only**. No active blockchain/P2P/validator/signer changes.

## Existing evidence and non-evidence

Existing v0.65 laboratories provide weighted Ed25519 vote certificates, a simplified
locking machine, NIL votes, Linux-only durable records and a round-0 validated
PREVOTE bridge. These are separate, incomplete research components. See:

- `docs/POS_FINALITY_V065_LAB_README.md`
- `docs/POS_FINALITY_V065_BFT_LOCKING_SPEC.md`
- `docs/POS_FINALITY_V065_DURABLE_SIGNER_GATE_LAB.md`
- `docs/POS_FINALITY_V065_VALIDATED_PREVOTE_BRIDGE_LAB.md`

A correctly verified quorum is **not** a finality guarantee. The raw `SignVote`
API is not an anti-equivocation gate. The trusted validator snapshot and
chain/height/epoch context must be authenticated independently. The v0.65
durable gate does not prevent filesystem rollback, private-key duplication,
or a privileged actor invoking other signing APIs.

## Phase 1A acceptance / tested invariants

1. Strict **more than** 2/3 stake; exactly 2/3 is insufficient.
2. Rejected signatures, duplicate voters or mismatched validator snapshot.
3. An independently signed quorum for each of two conflicting blocks can verify
   if a Byzantine/unguarded signer double-signs: **documented negative-security canary**.
4. An invalid cross-network PoLC cannot release an existing lock, and the rejected
   transition leaves the in-memory state unchanged.
5. With no valid unlock evidence, the simulated validator remains on its locked block.
6. All Phase 1A tests run only under `internal/finality`; live consensus is untouched.

## v0.68 follow-on gates — NOT implemented by Phase 1A

- **1B:** authenticated height/epoch validator snapshots; verified chain-source provenance;
  rollback and stale parent rejection at the signing boundary.
- **2A:** signer API isolation; a single authorized key-use path; identity separation,
  durable anti-equivocation across processes and restarts.
- **2B:** atomic vote+lock journal recovery; process crash, state truncation and rollback
  threat model; real filesystem/power-failure qualification remains separate.
- **3A:** reviewed reference BFT transition specification, NIL, round advance,
  authenticated timeouts and verifiable precommit certificates.
- **3B:** multi-node partitions, Byzantine validators, churn, proposer scheduling,
  liveness testing and external protocol/security review.

**Activation gate:** Never connect experimental signing/finality paths to live
`internal/p2p` or `internal/blockchain`, reuse production keys, release escrow,
or claim BFT safety until all gates are independently reviewed.

## Offline validation

```sh
go test -count=1 -timeout=5m ./internal/finality ./internal/consensus
go test -race -count=1 -timeout=8m ./internal/finality
go vet ./internal/finality ./internal/consensus
git diff --check
```

Passing the commands does not establish distributed safety, liveness, or a
production security guarantee.
