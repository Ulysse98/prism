# PRISM v0.67 — Phase 1C.3: verified LOCAL candidate admission (offline)

## Scope and implementation

Adds `local_anchor_admission_v067.go`, its isolated unit/adversarial tests,
and this document.  The exported `AdmitLocalAnchorCandidateV067` function
always invokes the existing `VerifyAnchorWithPinnedCheckpointV067` before
calling `ReserveProofV067`.  This composes previously tested PoUW V2 proof,
independent verifier report, v0.66 contract, local-chain inclusion and
operator-pinned checkpoint validation with the local one-shot replay store.

On verification failure, the gate must create **no** reservation. After a
successful preflight, duplicate proof identity still fails closed. The local
store's exclusive-create mechanism determines the single winner. This is
an experimental local admission and audit step, NOT a consensus or payout API.

## Tests and their meaning

- Invalid candidate and unavailable chain/validator view: real preflight rejects
  before any disk reservation.
- Invalid intent: rejects before verifier invocation.
- Preflight errors: do not occupy slots.
- Preflight-success *test seam*: verifies only ordering, stored inspection and
  one-shot reservation; this mocks verifier success and is **not** an end-to-end
  positive cryptographic proof test.
- Parallel local calls: at most one success, others reject as replays.
- Corrupted/partial state and multiprocess tests remain in Phases 1C.1/1C.2.

## Explicit non-guarantees and trust boundaries

- External caller MUST NOT supply source policy, checkpoint, validator set or
  local chain view. These are independently authenticated operator-controlled
  inputs and must remain immutable across verification. Otherwise admission is
  subject to spoofing and time-of-check/time-of-use vulnerabilities.
- Checkpoint and successor depth DO NOT establish BFT/global finality.
- Successful admission is NOT authority for on-chain anchoring, escrow release,
  nonce consumption, signing, a PRSM transfer, or cross-chain settlement.
- `ReserveProofV067` remains callable directly. This helper is **not** an
  enforced security perimeter until callers and storage permissions are
  isolated so untrusted requests cannot invoke that lower-level function.
- Local anti-replay is NOT distributed anti-replay across nodes or storage roots.
- This work does not simulate physical power loss or prove disk/FS durability.
- No network calls, private keys, live-chain operation, or git commit/push.

## Next gates before network operation

Add a privileged admission boundary and a separately authenticated
network-finality/authorization policy; test positive E2E proof fixtures; design
shared/distributed anti-replay and recovery semantics before any payout logic.
