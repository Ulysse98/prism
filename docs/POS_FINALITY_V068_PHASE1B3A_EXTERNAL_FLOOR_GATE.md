# PRISM v0.68 — Phase 1B.3A: external monotonic checkpoint-floor gate

**RESEARCH ONLY. PURE/IN-MEMORY. NO DURABLE ANTI-ROLLBACK CLAIM, NO ACTIVE CONSENSUS.**

## Purpose

A valid old signed checkpoint can still be replayed if the verifier accepts an old
policy or an old trusted floor. Phase 1B.3A adds a pure, fail-closed, non-signing
admission gate layered on 1B.2. The operator supplies a **separately authenticated**
checkpoint floor (revision + full network/parent/set anchor) and a separately
trusted exact revision/digest witness. The gate compares them, refuses lower
heights and same-height conflicts, then re-verifies the full operator signature,
authority policy, candidate parent chain and current locked stake.

Successful equal-height requests are idempotent; a higher verified height returns
an incremented proposed floor revision and new digest. There is no automatic
bootstrap from a peer, no mutation of caller state, and no write to disk.

## Explicit trust assumptions

- The external witness and floor must be authenticated and provisioned **outside**
  the candidate checkpoint, its peer and any local state vulnerable to rollback.
- SHA-256 binds bytes but **does not authenticate** a floor or its witness.
  An attacker replacing both with old consistent values bypasses this layer.
- A persistent, monotonic external witness is **not implemented here**. No
  crash atomicity, power-loss durability, rollback-resistant storage, live node
  snapshot isolation, historical epoch validator derivation, or key custody.
- A new higher floor is only a *proposal*. The caller must atomically persist
  and independently anchor its new witness before trusting it. This API does
  not know whether that happened. Fail-closed recovery remains future work.
- The 1B.2 operator signature is not a validator quorum nor BFT finality.

## Tests

- Equal-height, exact anchored checkpoint accepted idempotently.
- A real Prism one-block extension with valid signed transaction, proposer,
  parent chain and operator signature advances revision by one.
- Lower-height replay and same-height conflicting block/set rejected.
- Rollback/mismatch between floor and witness rejected (if witness truly trusted).
- Corrupted floor, malformed digest, forged checkpoint and revoked authority
  rejected without mutating caller state.

## Next — Phase 1B.3B (not implemented here)

A Linux-only, separately reviewed durable/recoverable checkpoint-floor store
with crash-boundary fault injection, process concurrency, external witness
coordination and explicit rollback limitations. Do **not** call a local JSON
file or checksum a secure anti-rollback root. Full host/filesystem rollback or
restoration of the external witness still bypasses local-only storage.

Never wire this lab into a production signer, finality, P2P, wallet or escrow.
