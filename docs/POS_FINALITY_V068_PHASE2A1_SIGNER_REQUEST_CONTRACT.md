# PRISM v0.68 — Phase 2A.1: non-signing isolated signer request contract

**RESEARCH ONLY. PURE INPUT CHECK. NOT AN ISOLATED SIGNER, AN ANTI-DOUBLE-SIGN GATE, OR BFT FINALITY.**

## Why this phase exists

Phase 1B qualifies checkpoint policy, signed operator pins, external floor
contracts, and Linux checkpoint-floor storage/recovery. It does not isolate
validator key material. `SignVote` (v0.65) still signs without a global durable
anti-equivocation gate. Other v0.65 signer entrypoints also accept private keys.
No v0.68 tests may be presented as production signing protection.

## Strict 2A.1 contract

`CheckIsolatedSignerRequestV068` validates a proposed `IsolatedSignerRequestV068`
against a separately authenticated floor and witness and the caller-trusted
validator snapshot. It checks:

- valid canonical vote context and exact chain/genesis/set binding;
- exact trusted parent hash and height = floor parent height + 1;
- only a non-NIL round-0 PREVOTE (same restricted scope as the v0.65 bridge);
- staked signer membership in the exact trusted validator snapshot;
- canonical validator snapshot commitment and exact revision/digest witness;
- missing/wrong/stale/malformed trust inputs fail closed.

This preflight does **not** validate the candidate block or a canonical live
node snapshot; those are separate inputs to a future exclusive signer boundary.
It never has a private key, never signs, never writes a file or creates a vote.

## Mandatory negative security regression

Two distinct block hashes in the same round-0 PREVOTE slot can both satisfy
this contract. This is intentional: only durable, serialized anti-equivocation
state in a single exclusive signer authority can prevent both signatures.
The test `TestV068SignerRequestContractDoesNotPreventEquivocation` must remain
as a negative security canary until the actual signer boundary is implemented.

## Still missing before any production signer

1. Exclusive process or hardware-backed custody of validator keys; no parallel
   raw signing access, cloned keystores or alternate signing API.
2. Durable, globally enforced slot locking across crashes, retries, rounds and
   network identity changes, with fail-closed recovery and rollback detection.
3. Authenticate external floor/witness and signer policy independently; do not
   trust a caller-provided digest as a secure monotonic root.
4. Authenticate the actual chain parent and validate each candidate under the
   serialized signer critical section, avoiding time-of-check/time-of-use races.
5. Specify and review IPC transport, caller authentication, key provisioning,
   epoch/history authority, block rounds, NIL/PRECOMMIT behavior and restart.
6. Byzantine fault testing, threat review and independent security audit.

Do not connect this contract to active consensus, P2P, wallet, escrow, public
RPC, validator keys, or v0.65 signing entrypoints. No changes to those paths.

## Local qualification

```sh
go test -count=1 -timeout=5m ./internal/finality ./internal/consensus ./internal/blockchain
go test -race -count=1 -timeout=8m ./internal/finality
go vet ./internal/finality ./internal/consensus ./internal/blockchain
```
