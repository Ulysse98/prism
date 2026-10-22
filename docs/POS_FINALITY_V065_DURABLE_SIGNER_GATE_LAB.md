# PRISM v0.65 — 2C.5F-B.3B — Experimental durable BFT signing gate

Status: **RESEARCH ONLY — NO ACTIVE CONSENSUS, NO PRODUCTION KEYS**.
Linux/WSL only. Not an audited BFT validator signer.

## What this change does

- Extends the single atomic `bft-safety-v065.json` record with `events` alongside the
  BFT lock snapshot, signed votes and revision number.
- Replays the **entire** recorded BFT transition history against a caller-trusted
  validator snapshot before signing, including verification of signed PoLC/quorum evidence.
- Checks the replayed state equals the stored lock snapshot and that each signed vote
  exactly matches the corresponding PREVOTE / PRECOMMIT decision.
- Serializes read → replay → transition → sign → persist under the existing Linux
  `flock` boundary; does not return a new signature when persistence reports an error.
- Repeats an identical final request idempotently, without another disk revision.
- Exercises same-round equivocation, a lock across rounds, NIL quorum unlock, tampering,
  concurrent processes, fault injection before/after rename, and commit observation.

## Setup and use

`InitDurableGateV065` requires an explicit, fresh, private (0700) directory and a
validator key belonging to the supplied set. No recovery path silently reinitializes
missing records.

Laboratory APIs: `SignDurableGatePrevoteV065`, `SignDurableGatePrecommitV065`,
`AdvanceDurableGateRoundV065`, `ObserveDurableGateCommitV065`.
They take a validator set that the caller **must independently authenticate**.

The gate does **not** use the older separate `votes-v065.json` journal; do not
reuse signing keys across this gate, `SignVote`, `SignVoteOnceV065`, or external tools.
The raw `SignVote` API remains available, so no system-level key isolation exists.

## Critical unsolved security limits

1. **Block validity is a caller-supplied boolean**, not an authenticated consensus
   block validation proof. Similarly, round advancement is invoked by the caller.
   Recorded events replay the *assertions*, not their independent truth. A privileged
   actor can potentially construct plausible-looking histories; do not treat this as
   a production provenance chain or a BFT safety proof.
2. A privileged edit, deletion, backup rollback, or use of the same private key on
   another host/API can bypass this local-only protection.
3. Process fault injection is not equivalent to a hardware power-loss guarantee;
   filesystem/mount assumptions require an explicit durability audit.
4. The in-memory BFT machine is a simplified research model with no network
   pacemaker, proposer rotation, authenticated timeout protocol, validator epochs,
   consensus version activation, slashing, or formal safety/liveness proof.
5. The stored event history may approach the 8 MiB record limit; unbounded history
   archiving and height transitions remain unspecified.
6. A verified PRECOMMIT quorum can be *observed*; it is not a finality guarantee
   without the full distributed protocol and validator honesty assumptions.

## Deployment gate

Never wire this experimental signer to running PRISM nodes or production validator
keys. Require independently verified block validity/evidence, real key isolation,
process/power-loss recovery tests, state snapshot authentication/rollback prevention,
full BFT protocol review, and adversarial independent-node experiments first.
