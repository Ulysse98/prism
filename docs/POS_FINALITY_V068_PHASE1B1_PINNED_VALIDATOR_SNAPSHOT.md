# PRISM v0.68 — Phase 1B.1: pinned validator snapshot qualification

**RESEARCH ONLY. NON-SIGNING. NOT ACTIVE CONSENSUS, HISTORICAL EPOCH AUTHENTICATION, OR BFT FINALITY.**

## Goal

Validate a candidate local Prism parent chain and its proposed CURRENT validator snapshot
against a separately authenticated, externally pinned identity tuple:

- chain ID and genesis hash;
- exact parent height and parent block hash;
- validator-set commitment calculated using v0.65 `ValidatorSetHash`.

`VerifyPinnedValidatorSnapshotV068` copies the candidate chain through JSON,
validates the set commitment, checks exact network/parent pin, derives
`Blockchain.ChainID()`, then uses the existing `ValidateChain` with its PoS
snapshot and current locked-stake equivalence. It never signs or creates a vote.

## Trust boundary — essential

The pin MUST be obtained from an authenticated source independent of the
candidate data (e.g., an already trusted, reviewable checkpoint). A malicious
peer may provide both a forged chain and a perfectly matching forged pin.
**This function cannot distinguish that from a truly trusted pin.**

The current Prism model does NOT supply a trust-verified historical validator
set at arbitrary heights/epochs. This lab validates current locked stakes of a
local candidate parent; it does not establish historical voting power or finality.
Nor does it prevent concurrent source mutations, rollback of the external pin,
rollback of local storage, or a privileged signer using another key path.

The genesis parent fixture in tests has height 0. Independent authentication is
represented by the test's controlled pin ONLY; it must never be confused with
a live network checkpoint authorization mechanism.

## Adversarial tests

- Exact pinned context accepted; canonical validator sorting preserved.
- Wrong/missing chain ID, genesis, parent height, parent hash, set hash rejected.
- Noncanonical pin hash rejected.
- Changed/duplicate/empty validator snapshots rejected.
- Even a self-consistent forged set hash is rejected when stake does not match
  `Blockchain.LockedStakes`.
- Mutated local stake state or genesis body rejected.
- Nil parent chain rejected.

## Next blockers (Phase 1B.2+)

1. Authenticating the pin itself, with explicit authority, lifetime and revocation.
2. Deterministically reconstructing historical validator sets from chain state,
   including epoch/activation heights and validator churn.
3. Atomic capture of canonical chain state and signer coordination.
4. Preventing rollback and double-sign through exclusive isolated key usage.
5. Multi-node Byzantine/partition tests and an independently reviewed BFT protocol.

Never wire this lab into `internal/p2p`, wallet signing, escrow settlement,
validator key usage, or active consensus. No change in existing block validity.

## Validation

```sh
go test -count=1 -timeout=5m ./internal/finality ./internal/consensus
go test -race -count=1 -timeout=8m ./internal/finality
go vet ./internal/finality ./internal/consensus
git diff --check
```
