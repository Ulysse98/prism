# Prism v0.67 — Phase 1C.5C: operator-owned local source snapshot

Experimental, offline-only admission hardening. No transaction or settlement authority.

## Change

`NewLocalOperatorAdmitterV067` verifies the pinned operator Ed25519 signature,
then captures private JSON round-trip copies of the source `Blockchain` and
`ProofOfStake` validators, checks their encoded round trip and validates the
copied chain. The admission method reads only these retained copies. The
constructor rejects invalid snapshots without reserving any proof.

## Assumptions / known limits

- The trusted operator MUST synchronize access to source chain/validators
  **during snapshot construction**; concurrent mutation while encoding is a
  data race and is not protected here.
- This is a bounded (64 MiB per encoded structure) local lab snapshot, not an
  efficient production archival implementation.
- Every consensus-relevant field and custom JSON encoding must be audited
  before deployment; JSON round-trip and chain validation do not prove the
  absence of omitted state.
- The operator public key is still supplied by the privileged caller. This
  phase does **not** establish root-key provenance, persistence, rotation,
  revocation or access control of the constructor.
- Caller-controlled source snapshots may be self-consistent yet noncanonical.
  Local chain validation and a signed checkpoint do **not** prove BFT/network
  finality or distributed replay uniqueness.
- The `crosschain` package's private raw reservation helper is not a network
  security boundary. No funds transfer, escrow release, signing or external
  registry registration is authorized by these checks.

## Qualification

The new tests check deep-detachment of nested proof and validator data,
positive signed admission after the original inputs are mutated, persistent
one-shot replay rejection, fail-closed corrupt-source initialization, and
nil/empty input rejection. Regressions and Go race detector must pass on the
actual Prism worktree before Phase 1C.5C is accepted.
