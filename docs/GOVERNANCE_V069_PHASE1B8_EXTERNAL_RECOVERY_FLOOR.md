# PRISM v0.69 / Phase 1B.8 — Externally Pinned Recovery Floor & Offline Rotation Candidate

Status: offline, research-only, Linux single-host closure inspection. No consensus, chain writes, wallet signing, RPC, or key installation.

## Threat model and trust boundary

1B.7 authenticates an immutable local closure record. An administrator who can restore the *entire* voting directory to an earlier copy can remove that marker. A local SHA-256 checksum or a floor stored in that same directory cannot distinguish a restored old state from the original old state.

Phase 1B.8 introduces a **read-only comparison against an independently retained exact closure checkpoint**. The `AdvisoryClosureRecoveryFloor` is bound to the network/chain/genesis/proposal, closure digest, closure height, Ed25519 signer, operator-assigned positive epoch and the SHA-256 of the complete canonical signed marker bytes (including the authorization signature and nonce). The floor's own digest is domain separated and canonical; it is *not* an authentication mechanism.

### Trusted floor lifecycle (operator procedure)

- First verify the complete sealed vote store using `VerifySealedLocalAdvisoryVoteStore` and independently trusted proposal, snapshot, ballots and tally pins; never treat marker inspection as a vote-completeness proof.
- Supply an **independently approved** closure authority Ed25519 public key and an authority epoch (positive integer, tracked outside the store) to `BuildAdvisoryClosureRecoveryFloor`.
- Export and retain the resulting floor on a separately protected, rollback-resistant operator medium or independent system; do not reconstruct it from the local voting directory when recovering from an incident. Treat recovery floors as immutable audit anchors.
- During recovery, call `InspectLocalAdvisoryClosureAgainstFloor` with that external, trusted floor. A missing or different local signed marker, including a different valid authorization of the same closure body, fails closed. The procedure makes **no write** and does not create or remove a marker.
- The status `EXTERNAL_FLOOR_MATCH_LOCAL_ONLY` means exact local marker agreement under that external pin. It is **not** proof that all ballot files are correct, or that the filesystem and the separate pin cannot both be rolled back.

### Offline authority rotation candidate

`SignAdvisoryAuthorityRotationCandidate` produces a proposal for a +1 authority epoch bound to the exact externally pinned floor. It requires an Ed25519 signature from the old pinned operator and a proof-of-possession signature from a different new public key. `VerifyAdvisoryAuthorityRotationCandidate` rejects wrong network, genesis, proposal, closure, floor, epoch, nonce, signer or signature. No runtime rotation occurs: `SealLocalAdvisoryVoteStore`, `VerifySealedLocalAdvisoryVoteStore`, and the persisted closure marker retain their original semantics.

Activation of rotated keys, durable anti-rollback floors, trust-store synchronization, migration of older records, key revocation and distributed authority remain **out of scope**. Do not trust a new key solely because this proposed transition verifies; a separate explicit trust-policy decision and protected update path are required.

## Sources and tests

New paths: `closure_floor.go`, `closure_floor_linux.go`, `closure_floor_other.go`, `closure_floor_linux_test.go`, `authority_rotation.go`, `authority_rotation_test.go`, and this document. No modifications to existing Go source files. Expected parent checkpoint `37dc70f` on branch `feat/v0.69-governance`.

Tests: deterministic hash vectors, exact-byte marker pin, reopened store, parallel inspectors, missing marker, replaced valid signature, wrong proposal/key/epoch/height, malformed floor, dual-signature rotation, stale replay, altered signatures, epoch skipping/overflow. Run offline:

```
umask 077
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOWORK=off go test ./internal/governancev069 -count=1
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOWORK=off go test -race ./internal/governancev069 -count=1
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOWORK=off go vet ./internal/governancev069
```

This implementation does not prevent a privileged administrator from changing the local directory or an operator from failing to call the new read-only verifier. Linux `flock` only coordinates cooperating processes on a trusted local filesystem. This does not establish BFT, trusted time, quorum, distributed finality, or production voting rights.
