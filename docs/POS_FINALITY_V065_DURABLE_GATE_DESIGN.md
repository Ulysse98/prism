# PRISM v0.65 — Durable BFT Signing Gate

Status: DESIGN ONLY
Phase: 2C.5F-B
Scope: isolated Linux/WSL laboratory.

## Objective

Prevent a signer from exposing a vote that is inconsistent
with its recovered BFT lock state.

No changes to active PRISM consensus or P2P.

## Existing components

- lab_v065.go: signed votes and weighted quorum
- journal_linux_v065.go: persistent same-slot votes
- locking_machine_v065.go: in-memory BFT transitions

These are not yet atomically integrated.

## Durable state

A versioned record must bind:

- protocol version
- chain ID and genesis hash
- validator-set commitment
- signing validator identity
- height and BFT lock snapshot
- signed vote history
- transition evidence sufficient for recovery validation

Persisting a snapshot alone is not proof that the state
was reached through valid BFT transitions.

Recovery must verify the record's internal consistency
and reject invalid or incomplete state.

## Signing protocol

For each signing request:

1. Acquire one exclusive interprocess lock.
2. Load and validate the durable record.
3. Reconstruct the BFT state without trusting caller state.
4. Verify transition evidence and quorum certificates.
5. Simulate the transition on a private copy.
6. Check existing votes for conflicts.
7. Prepare the signed vote and resulting state.
8. Persist both in a single recoverable record.
9. Synchronize the file and containing directory.
10. Expose the signed vote only after persistence succeeds.

On any failure, return an error without exposing a new vote.

A retry must return only an identical recorded decision.

## State transitions

The durable gate must cover:

- PREVOTE
- PRECOMMIT
- explicit NIL votes
- validated cross-round lock changes
- NIL quorum unlocks
- round advancement
- observed commit certificates

Non-signing transitions must also persist when they affect
future signing authorization.

## Recovery policy

- Missing state after initialization: fail closed.
- Unsupported version: fail closed.
- Corrupt record: fail closed.
- Invalid signature or duplicate slot: fail closed.
- Snapshot inconsistent with evidence: fail closed.
- Unknown validator snapshot: fail closed.
- Invalid lock transition: fail closed.

Do not silently recreate an empty journal after state loss.

## Concurrency

One lock must serialize complete read-transition-persist
operations across cooperating Linux processes.

Never keep independent in-memory state authoritative
after another process can modify the durable record.

The signing key must not be used through an unprotected
API alongside the durable gate.

## Storage

The new laboratory must use a separate test directory
and not overwrite existing votes-v065.json records.

Use a same-directory temporary file, fsync, atomic rename
and directory fsync.

Bound record size and reject unsafe file types and modes.

This design assumes a trusted local Linux filesystem.
Power-loss guarantees require platform-specific evaluation.
A privileged rollback or deletion of all safety state
cannot be prevented by local file locking alone.

## Required tests

T01: First PREVOTE is durable before return.
T02: Identical request returns identical signed vote.
T03: Same-slot conflicting vote is rejected.
T04: Lock is recovered after process restart.
T05: Cross-round conflicting vote without PoLC is rejected.
T06: Valid verified PoLC allows authorized transition.
T07: Signed NIL vote is recovered correctly.
T08: NIL quorum changes lock state according to rules.
T09: Missing or corrupted state fails closed.
T10: Concurrent processes serialize conflicting requests.
T11: Crash before persistence exposes no vote.
T12: Crash after successful persistence restores both.
T13: Crash between rename and directory fsync is handled.
T14: Invalid stored transition evidence is rejected.
T15: Finalized state cannot be replaced by a conflict.
T16: Legacy vote serialization remains compatible.
T17: Wrong chain, height or validator snapshot is rejected.
T18: Storage size and permissions limits are enforced.

Fault injection must distinguish process termination from
actual power loss.

## Non-goals

- No active consensus activation.
- No automatic migration of historical vote journals.
- No formal proof of BFT safety or liveness.
- No protection against privileged storage rollback.
- No remote-signing key isolation yet.
- No production validator release.

## Implementation gate

Review this contract and the existing signing interfaces
before introducing code that can return a BFT signature.

All implementations remain experimental and disabled.
