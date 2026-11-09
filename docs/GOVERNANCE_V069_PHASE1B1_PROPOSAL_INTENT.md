# PRISM v0.69 — Phase 1B.1: Advisory Proposal Intent

**Scope:** Offline/isolated research primitive; no mainnet/devnet activation; no changes to `internal/reserved` consensus paths. This does not implement a DAO.

## Why

The existing v0.61 authority governance has authorized proposals, delayed execution and consensus guards. A signer threshold is **not** a community voting quorum. v0.69 begins by specifying a non-executable advisory intent with stable binary commitment.

## Contract

- `ProposalIntent` is versioned and bound to explicit `network`, `chain_id`, `genesis_sha256`, `proposer`, `nonce`, creation/expiry heights, advisory topic, title, rationale and payload digest.
- `ValidateShape` fails closed for invalid identifiers, malformed hashes, invalid UTF-8 and unsupported topics.
- `ValidateForContext` requires an independently supplied trusted network/chain/genesis and height; endpoints are inclusive.
- `Commitment` is domain-separated SHA-256 over explicit, ordered, 4-byte-big-endian length-prefixed strings and 8-byte-big-endian integers. It does not sign or authenticate the proposer. Title/rationale are hashed as raw UTF-8 without normalization.
- `ReplayKey` only provides a stable identity for a proposer nonce within a chain context. **It does not enforce anti-replay**; durable atomic replay admission belongs to a later phase.

## Hard safety boundaries

This package never executes, stores or broadcasts on-chain changes, does not cast or verify signatures or votes, does not establish token snapshots, quorum or timelock, and does not confer proposal authority. In particular, avoid connecting any result of `Validate*` directly to existing governance execution routines.

## Gate to 1B.2

Inspect the current v0.61 structures, define the eligible voter population and snapshot point, independent voting/signature domain, voting duration, quorum and threshold semantics, and durable single-use tally/admission. All require adversarial tests and an explicit consensus change review before activation.

## Local tests

From the v0.69 worktree after applying the isolated package:

```bash
export PATH="/usr/local/go/bin:$PATH"
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOWORK=off

go test -count=1 ./internal/governancev069

go test -race -count=1 ./internal/governancev069

go vet ./internal/governancev069
```

No push, merge or commit is performed by the installer.
