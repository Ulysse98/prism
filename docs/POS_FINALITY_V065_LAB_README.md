# PRISM v0.65 — Phase 2C.5B / isolated finality vote laboratory

**RESEARCH ONLY. NOT PART OF ACTIVE CONSENSUS. NOT A FINALITY IMPLEMENTATION.**

This package is an experimental, self-contained weighted vote verifier. It does not change blockchain, mempool, P2P protocol, wallet persistence, or consensus activation.

## Inputs and authenticated data

- The caller supplies a **trusted** PoS validator snapshot (`[]consensus.Validator`), not one chosen by the remote peer. The laboratory sorts, validates, and hashes it. All votes bind to that snapshot hash.
- Ed25519 vote signatures bind: chain ID, genesis hash, block height, round, vote step (`prevote` or `precommit`), candidate block hash, validator set hash, validator address, and validator public key.
- Validator public keys must resolve to the registered PRISM addresses. A repeated or unknown validator is rejected.
- Stake cannot be zero, duplicated, or overflow the `uint64` total.
- Successful quorum requires **strictly more than 2/3 of the configured stake**, computed without multiplication overflow.

## Tests included

- 75/100 stake accepted; 50/100 rejected; exact 2/3 rejected.
- Duplicate, unknown and forged voters rejected; tampered signatures and context rejected.
- Validator set sorting is deterministic; changed stake snapshots, duplicate addresses, zero stake, and overflow rejected.
- Explicit negative-security test: **two conflicting quorum certificates can both verify if validators double-sign**. A certificate alone does not provide BFT finality.

## Missing requirements before consensus use

- Durable signer-side voting journal before emitting a signature (anti-equivocation; crash consistency).
- Protocol for rounds, prevote/precommit and locks, nil votes, timeouts, view changes, and liveness.
- Authenticated historic validator-set selection, activation heights, validator churn and unbonding rules.
- Evidence for double-signing and appropriate punishment; Byzantine and partition tests.
- Deterministic block finalization rules and protection against unsafe syncing/reorgs.

## How to run (after copying these files into the local PRISM repo)

```sh
go test ./internal/finality -v -count=1
go test -race ./internal/finality -count=5
go test ./... -count=1
go vet ./...
```

This package is deliberately not wired into `internal/p2p` or `internal/blockchain`.
