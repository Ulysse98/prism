# PRISM v0.68 — Phase 1B.3B: local durable checkpoint-floor store

**RESEARCH ONLY. LINUX/WSL ONLY. NO ACTIVE BFT CONSENSUS OR PRODUCTION SIGNER.**

## Purpose

Build a small, separately named local storage laboratory on top of the pure
Phase 1B.3A floor gate. Exact checkpoint identity, operator Ed25519 signature,
policy limits, current parent chain and locked stakes are rechecked by the
existing 1B.3A gate before a higher floor is *prepared*.

The lifecycle is deliberately split:

1. Initialize a new, externally authenticated floor/witness in a pre-created
   owner-controlled private 0700 directory. A compromised or untrusted source
   cannot bootstrap a trustworthy floor merely by matching a digest.
2. **Prepare** a verified higher checkpoint: atomically save a pending floor
   and its proposed next witness under Linux `flock`. The stored current floor
   remains unchanged. Any pending transition freezes normal reads/new prepares.
3. A trusted operator or separate witness service must durably, monotonically
   publish the proposed witness **outside** this local state. This service is
   NOT implemented by this lab. Never invent or calculate witness trust from
   the same candidate being verified.
4. **Commit** only after an independently authenticated next witness has been
   received. Reload under lock, require exact match with pending, and persist
   the new current floor; then pending is cleared.

## Crash handling and deliberate fail-closed behavior

- Crash before preparing: previous floor may remain readable with the old witness.
- Crash during write before rename: old file remains; a leftover temp file must
  not be mistaken for a committed record.
- Crash after rename but before fsync: outcome may be ambiguous. Reload and
  check externally authenticated witness; do not assume a failed call rolled back.
- Crash while pending or after external witness update: `Read` refuses to use
  the state until an explicit matching `Commit` succeeds.
- Missing/corrupt/truncated state, symlinks, incorrect permissions, malformed
  witness and wrong external revision: reject; never auto-reset or reinitialize.

## Tested scope

- Explicit fresh initialization, repeat initialization refusal and restart read.
- Forward signed checkpoint produces one pending witness; stale external witness
  cannot commit; matching independent witness can commit and later read.
- Invalid checkpoint signature fails before durable mutation.
- Fault injection around rename; 12 concurrent cooperating prepare requests
  result in one winner; corrupted state and symlink are rejected.

## Security limitations

- This is a LOCAL disk store, not a hardware monotonic counter, trusted
  checkpoint publisher, or authenticated external witness protocol.
- The caller can lie about external-witness authentication. Same-user,
  privileged, disk-image, backup or full-host rollback may replace both the
  record and witness. Deletion followed by a new explicit initialization also
  bypasses continuity unless separately prevented.
- `flock` serializes cooperating processes using the same lock path on a
  suitable local Linux filesystem. It does not protect against a different
  code path reusing validator keys, non-cooperating writers, or malicious admins.
- Process-level fault injection and sync calls do NOT prove power-loss
  durability on all filesystem/mount/hardware configurations.
- No autonomous pending abort/reconciliation is supplied: ambiguous recovery
  must be resolved through a separately reviewed operator procedure.
- Not an authenticated historic validator epoch, BFT quorum/finality protocol,
  slashing, network synchronizer, or production signing gate.

Never wire this package into active `internal/p2p`, `cmd/prism`, wallet,
consensus, escrow, cross-chain settlements or production validator keys.

## Local qualification

```sh
go test -count=1 -timeout=5m ./internal/finality ./internal/consensus
go test -race -count=1 -timeout=8m ./internal/finality
go vet ./internal/finality ./internal/consensus
```
