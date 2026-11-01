# PRISM v0.67 — Phase 1C.1: immutable local replay reservation

**Scope: OFFLINE, Linux local filesystem, one trusted storage root.** This is an
experimental one-shot source-proof reservation mechanism, **not** a bridge,
settlement mechanism, verification result, finality oracle, or authorization.

## Integration boundary

The earlier v0.67 stages independently inspect candidate PoUW proof,
local source-chain inclusion and an operator-pinned checkpoint. None establishes
global BFT finality. This local reservation also establishes no finality.
A future phase must design an independently authenticated finality and bridge
policy; these phase results **MUST NOT** be composed into payout authority.

Unlike `SettlementStore.Upsert` (mutable settlement *status*), the new store
creates a separate, immutable source-proof occupancy record. The proof key is
domain-separated SHA-256 of length-prefixed source chain ID, source genesis,
job ID, and proof hash. Nonce, destination chain/registry and source block
are intentionally **not** part of this key. Reusing the same source proof with
these modified fields still collides. Independent proof identities get separate
slots. A different local storage root has no knowledge of these reservations.

## Durable, fail-closed local behavior

- One file per source proof in `crosschain-v067-local-replay/`.
- Atomic `O_CREATE|O_EXCL` provides a single winner across concurrent writers
  on an appropriate local POSIX filesystem.
- File contents are synced and closed, then containing directory is synced,
  before successful acknowledgment. The storage namespace parent is synced.
- A crash/interrupted write may leave a partial file. **Never** delete or
  overwrite it automatically. A partial file occupies the slot but cannot
  pass inspection. Manual remediation requires separate incident handling.
- `InspectProofV067` rejects missing, corrupted, mismatched, symlinked, or
  trailing-data records. JSON checks provide consistency, *not* tamper-proofing.
- Failures never allow signature, registration, transfer, or payout.

## Security limits

This is **not** multi-node distributed consensus, cross-chain exactly-once
execution, tamper-resistant storage, a finality guarantee, or a payment ledger.
An operator with authority to remove/replace files can bypass this local guard.
Do not use on NFS or unverified shared/virtual filesystems without a separate
analysis of exclusive creation and durability. The storage parent and process
are trusted; the implementation has no hardened directory-FD sandbox against
an adversary with local directory replacement privileges. Preserve backups and
reconcile incidents before future bridge integration.

## Offline qualification

`go test -count=1 ./internal/crosschain`
`go test -race -count=1 -run '^TestLocalReplayV067' ./internal/crosschain`
`go vet ./internal/crosschain`

The tests cover identical and cross-destination replay, restart/new store,
concurrent independent instances, cross-process duplicate detection, corrupt
and partial tombstones, symlinks, malformed JSON, and storage failure.
