# PRISM v0.67 — Phase 1C.4: positive cryptographic admission E2E (offline)

## Scope

This phase adds **only one Go test file** and this documentation. It exercises
`AdmitLocalAnchorCandidateV067` directly (not the mock preflight seam), reusing
`newCheckpointFixtureV067` and its signed PoUW V2 / independently signed verifier
report, locally validated source chain, and pinned checkpoint. The new test
does **not** modify production code or send any blockchain transactions.

## Positive and adversarial coverage

- Valid signed PoUW V2 job/contract/proof/report, local inclusion, and pinned
  checkpoint leads to exactly one local reservation.
- Reservation inspection and same-store / reopened-store replay rejection.
- Rebinding the same valid proof to a different destination registry/chain and
  nonce does not bypass source-proof replay uniqueness.
- Invalid proof identity, wrong source block, wrong pinned checkpoint, unapproved
  verifier, and insufficient depth must reject *before* consuming a valid slot.
- Full package tests, targeted race detector tests, and static analysis run
  offline (`GOPROXY=off`, `GOSUMDB=off`).

## Security boundaries (unchanged)

This is an offline test using real **cryptographic signatures and a generated
local test chain**, NOT a real CUDA GPU job, a public-chain notarization, or
proof of network-wide finality. Checkpoints are externally provided fixture
values, not independently authenticated consensus certificates.

The lower-level exported `ReserveProofV067` remains directly callable: the
verified admission helper is **not yet an access-control boundary**. A later
phase must design trusted admission ownership and ensure untrusted requests
cannot call the raw reservation API or control trusted policy/chain inputs.

No authenticated distributed replay consensus, power-loss durability claim,
settlement authorization, transaction signing, token transfer, registry
submission, or escrow release is provided or permitted by this phase.
