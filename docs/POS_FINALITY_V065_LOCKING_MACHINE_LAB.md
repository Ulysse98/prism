# PRISM v0.65 — Phase 2C.5E: Isolated BFT Lock Transition Lab

**Status: RESEARCH LAB ONLY — no active consensus, signing, persistence, P2P, or block-finalization integration.**

Implements an in-memory deterministic skeleton inspired by the Tendermint/CometBFT PROPOSE → PREVOTE → PRECOMMIT → COMMIT steps. It reuses the **cryptographically verified, stake-weighted non-NIL quorum** from `internal/finality/lab_v065.go`. This is not an implementation of the full reference protocol, and passing tests is not a BFT safety/liveness proof.

## Added files

- `internal/finality/locking_machine_v065.go`: isolated transitions and read-only snapshot.
- `internal/finality/locking_machine_v065_test.go`: signed-quorum adversarial tests.
- `docs/POS_FINALITY_V065_LOCKING_MACHINE_LAB.md`: this document.

No modifications to the existing 2C.5B or 2C.5C labs. Does not call the journal signer or alter the existing blockchain/network.

## Model decisions

- Initialized for **one fixed height** with an externally authenticated `chainID`, `genesisHash` and validator snapshot. Validator snapshot is cloned.
- A locked validator normally PREVOTEs its locked block, even when a different valid block is proposed. An explicit NIL choice uses an empty string **only inside this in-memory machine**. It cannot be converted to a signed 2C.5B `VoteContext`, which currently rejects NIL hashes.
- To change from lock A to proposal B, a **fully verified PREVOTE quorum** for B in prior round Q is required, with `lockedRound < Q < currentRound`. An asserted round number alone is rejected.
- At the current round, a **fully verified PREVOTE quorum** for an externally validated, available block B lets the machine set `lockedBlock=B`, `lockedRound=currentRound`, `validBlock=B`, then recommend PRECOMMIT(B).
- Without a quorum, the model recommends PRECOMMIT(NIL) and preserves any lock. **PREVOTE(NIL) quorum and its unlock effects are not implemented.**
- A verified non-NIL PRECOMMIT quorum may enter the local **observed** COMMIT state only when a caller asserts that the block is present and fully validated. Later contradictory commits at the same height are locally rejected; identical fully verified retries are idempotent.
- Any incoming certificate is verified against a trusted validator snapshot: signatures, distinct addresses, voting power, network, height, step, round and block hash. Stake threshold is >2/3.

## Explicit security boundaries and limitations

1. **No real votes are signed** by the machine, no journal is consulted, and no lock is persisted. A process restart resets this machine. A durable atomic integration of lock state + signed votes is still required.
2. **No signed NIL votes or NIL quorum proofs.** Full BFT liveness and correct NIL unlocking are therefore absent.
3. **No block-validity function.** `validBlock` and `blockValidated` are trust-sensitive boolean test inputs. Before integration, replace them with real block validation and availability checks.
4. **No proposal signature, proposer-per-round selection, pacemaker, timeout, gossip, evidence retention, validator transitions, or certified cross-node finality.**
5. The local `ObserveCommitV065` state is **not a distributed finality guarantee**. The adversarial test intentionally demonstrates that two independent machines can observe contradictory signed supermajorities if validators double-sign. Byzantine stake assumptions and signer anti-equivocation must hold independently.
6. An independently supplied validator snapshot must be authenticated; hash commitment alone is insufficient. Snapshot/epoch binding and reconfiguration remain design work.
7. No safety guarantee against replay from stale disk, key misuse elsewhere, process crash, power loss or partition. This module is intentionally outside active PRISM consensus.

## Tests

Run on Linux/WSL inside the PRISM repository:

```bash
go test ./internal/finality -run '^TestBFTLockV065' -v -count=1
go test -race ./internal/finality -count=5
go test ./... -count=1
go vet ./...
```

The suite covers absent/invalid proposals, one decision per step, lock preservation, proper verified prior-round PoLC, stale or future proof, wrong block, network, height, validator snapshot, vote step, invalid signatures, duplicated/underweight votes, observed precommit certificates, conflicting local observed commits and snapshot-copy isolation.

## Next gate (2C.5F)

Specify and test signed NIL votes and full NIL-PoLC transitions; integrate signer journal and machine state with a single durable, crash-recoverable critical operation (using rollback defenses appropriate to production). Then add model-based safety checks over multiple independent validators and rounds **before** any P2P or consensus activation.

## Conceptual reference

- https://github.com/cometbft/cometbft/blob/main/spec/consensus/consensus.md
- https://github.com/cometbft/cometbft/blob/main/spec/consensus/signing.md
