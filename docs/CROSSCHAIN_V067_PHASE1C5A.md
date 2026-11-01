# PRISM v0.67 — Phase 1C.5A: close the raw reservation export

## Scope

Offline Go API-hardening step based on local commit `55c3b17`.
`(*LocalReplayStoreV067).ReserveProofV067` is renamed to
`(*LocalReplayStoreV067).reserveProofV067` and the package's callers/tests
are adjusted. Go code in *other packages* can no longer directly invoke the
low-level write method. `InspectProofV067` remains public and read-only.

The admission helper continues to call `VerifyAnchorWithPinnedCheckpointV067`
before the private one-shot write. The existing atomic exclusive-create,
restart, concurrency and fail-closed behavior remains unchanged.

## Scope limits (important)

- This is **not** a completed privileged admission boundary: code in the
  `crosschain` package can still invoke the private writer.
- The exported `AdmitLocalAnchorCandidateV067` still receives its trusted
  policy, checkpoint, chain snapshot and validator set from its caller. The
  caller must authenticate them; untrusted requests must not choose them.
- This step does not establish BFT finality, distributed replay protection,
  physical power-loss durability, signature authority or any settlement right.
- No CUDA worker, RPC, EVM/Solana/Monad transaction, escrow release or payout.

## Qualification

- Existing `internal/crosschain` tests (including real signed PoUW admission E2E).
- New AST API-surface regression test to detect unintended writer exports.
- `go test -race` and `go vet` under offline mode.
- No commit or push in this script; inspect the diff first.

## Follow-up: Phase 1C.5B

Design an operator-owned entrypoint which pins/authenticates trusted policy,
checkpoint and immutable local chain state, and controls who may submit an
admission request. Keep settlement disabled until network finality and a
multi-node anti-replay design are independently established.
