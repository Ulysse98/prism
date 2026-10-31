# PRISM v0.65 — BFT Locking Specification

Status: DESIGN ONLY — experimental, not active consensus.
Phase: 2C.5D
Reference model: Tendermint/CometBFT round-based BFT.

## 1. Scope

This document specifies a candidate state machine for a future
PRISM BFT finality protocol.

It must not alter existing PRISM block validity, P2P synchronization,
escrow settlement or chain selection until separately reviewed.

Existing labs:
- Weighted signature/quorum verification: 2C.5B
- Durable same-slot vote journal: 2C.5C

Neither lab establishes distributed finality.

## 2. Security assumptions

- Less than one third of voting stake is Byzantine at a height.
- Ed25519 validator keys are correctly authenticated and protected.
- All honest validators use the same authenticated validator snapshot.
- Network identity, genesis, height, round, vote step and block
  identity are bound to every vote.
- Stake changes require explicit epoch/height transition rules.
- Liveness additionally requires suitable network synchrony.

A signed quorum certificate does not independently establish safety.

## 3. State per validator and height

height
round
step: PROPOSE | PREVOTE | PRECOMMIT | COMMIT
lockedBlock: optional canonical block identity
lockedRound: optional round
validBlock: optional canonical block identity
validRound: optional round
signedVotes: durable journal
finalizedBlock: optional immutable block identity

The no-lock/no-valid-round state must be explicit, not confused
with round zero.

A new height must not discard evidence needed for safe recovery.

## 4. Vote format

Vote identity must bind:
- consensus protocol version and domain separator
- chain ID and genesis hash
- authenticated validator-set/epoch commitment
- height and round
- PREVOTE or PRECOMMIT
- canonical block identity OR an explicit NIL vote
- signer public key/address and Ed25519 signature

NIL must have its own unambiguous canonical encoding.

The present 2C.5B lab does not yet support NIL votes.
This is a prerequisite for implementing full BFT transitions.

## 5. Quorum and evidence

For total trusted stake S, a quorum requires:

    signedStake >= floor(2*S/3) + 1

Arithmetic must be overflow-safe.

Each proof must verify:
- correct network, height, round, step and snapshot
- cryptographic validity
- distinct registered signers
- correct voting power
- same canonical block identity, or all NIL votes

A claimed validRound/POLRound is never accepted on its own.
Its supporting signed PREVOTE quorum must be verified.

## 6. PREVOTE decision

At height H and round R:

- Reject an invalid proposal.
- If unlocked, a valid proposal may be prevoted.
- If locked on the proposed block, it may be prevoted.
- If locked on another block, require a valid justification
  from a round Q with lockedRound < Q < R before changing lock
  in the model based on the reference specification.
- Otherwise preserve the lock and follow the specified
  locked-block prevote behavior.
- If there is no valid candidate, vote NIL.

The exact transition must match a reviewed reference protocol;
do not infer authorization from a bare round number.

## 7. PRECOMMIT decision

At height H and round R:

- With >2/3 valid PREVOTEs for block B in round R,
  persist a lock on B at R before exposing PRECOMMIT(B).
- With >2/3 valid PREVOTEs for NIL in round R,
  follow the selected protocol's NIL-unlock rule and
  PRECOMMIT(NIL).
- Without a suitable quorum, preserve the existing lock
  and PRECOMMIT(NIL), according to the selected model.

Persist lock transitions and signed votes as one recoverable
critical operation.

## 8. FINALIZE decision

Finalize B only with >2/3 verified PRECOMMITs for the same
B at height H and a specific round R, from the trusted snapshot.

B must be available and fully validated.

Once finalized, a conflicting block at H is never adopted by
the honest state machine.

Finality evidence must survive crash/restart and be verifiable
by a recovering or newly synchronized node.

## 9. Persistence and signing

The signing journal and lock state must be coordinated
atomically under one serialization mechanism.

Never expose a vote before its required safety state is durable.

Reject corrupted, incomplete or inconsistent recovered state.
Do not silently reset voting locks or signed-vote history.

The v0.65 Linux journal is an experiment; rollback protection,
remote-signing isolation and abrupt power-loss recovery are not
yet established.

## 10. Required adversarial tests

T01: Sign first PREVOTE and recover it after restart.
T02: Reject different hashes in one signing slot.
T03: Preserve a lock across a round increment.
T04: Reject conflicting proposal with no valid unlock proof.
T05: Reject false, forged, duplicate or underweight PoLC.
T06: Reject PoLC from the wrong network/height/snapshot.
T07: Reject a stale or future-round unlock justification.
T08: Accept authorized lock transition with verified evidence.
T09: Precommit only following the specified prevote rules.
T10: Handle NIL votes with unambiguous serialization.
T11: Reject contradictory FINALIZE at the same height.
T12: Recover safely across interruption boundaries.
T13: Serialize simultaneous signer requests across processes.
T14: Test proposer rotation and delayed/conflicting proposals.
T15: Model network partitions, delayed messages and stale rounds.
T16: Model validator-set changes without retroactive vote weight.

Passing unit tests is not a formal BFT safety or liveness proof.

## 11. Activation gates

Before touching active PRISM consensus:

1. Review an exact protocol and prove its safety invariants.
2. Implement a deterministic, isolated transition machine.
3. Validate persistence and signer integration.
4. Add model-based and adversarial multi-validator tests.
5. Specify validator-set transitions and commit certificates.
6. Version consensus messages, blocks and network activation.
7. Test real independent nodes, crashes and partitions.
8. Perform external security review.

Never enable this experimental logic by default.

## References

- https://github.com/cometbft/cometbft/blob/main/spec/consensus/consensus.md
- https://github.com/cometbft/cometbft/blob/main/spec/consensus/signing.md
