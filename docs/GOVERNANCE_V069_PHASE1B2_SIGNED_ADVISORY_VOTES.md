# PRISM v0.69 — Phase 1B.2: Signed Advisory Votes + Local Replay Lab

**Research/offline only. NOT an on-chain governance module, production DAO, wallet
signature API, consensus update, BFT evidence, validator vote, quorum mechanism,
or production-safe multi-node ballot store.**

## Dependency and installation gate

This package *adds* the following files to the Phase 1B.1 worktree:

- `internal/governancev069/vote_intent.go`
- `internal/governancev069/vote_intent_test.go`
- `internal/governancev069/vote_replay_linux.go`
- `internal/governancev069/vote_replay_linux_test.go`
- `internal/governancev069/vote_replay_other.go`
- `docs/GOVERNANCE_V069_PHASE1B2_SIGNED_ADVISORY_VOTES.md`

The installer must run on `feat/v0.69-governance` descended from the approved
v0.68 master baseline. It checks Phase 1B.1 source hash pins and will only
accept the exact Phase 1B.1 untracked files or an otherwise clean branch.
It never overwrites files, stages, commits, merges, pushes, or changes Git refs.
The local WSL repository still needs to pass its own tests.

## Commitments, signatures, and validation

- `AdvisoryVoteIntent` explicitly pins version, network, chain ID, genesis hash,
  proposal commitment, voter Ed25519 public key, nonce, height window, and choice
  (`YES`, `NO`, `ABSTAIN`).
- `ValidateVoteForProposal` takes the *actual proposal* and caller-supplied trusted
  chain context. It rejects mismatched proposal digests, networks, chains, genesis,
  future/expired votes, and votes with windows outside the proposal window.
- `AdvisoryVoteCommitment` uses an independent protocol domain and exact ordered
  length-prefixed binary encoding. SHA-256 is pinned by a canonical test vector.
- `SignAdvisoryVote` uses **standard Ed25519 over the 32-byte vote digest**, not
  an Ed25519ph protocol. The signing key must match the claimed public key;
  key creation and storage are out of scope. No production validator keys.
- `VerifyAdvisoryVote` checks Ed25519 signatures after context/proposal checks;
  signature authenticity is *not voting eligibility*.

## Single-host local replay reservation

`VoteReplayKey` commits to `(network, chain, genesis, proposal, voter public key)`.
Changing a vote choice or nonce cannot bypass the one-vote-per-proposal rule.
On Linux only, `OpenAdvisoryVoteReplayStore(absPath)` opens a private owned
0700 directory on a **trusted, local Linux filesystem with trusted parent
paths**. Its `ReserveVerifiedAdvisoryVote` first verifies the signed vote and
then uses exclusive-create (`O_EXCL`) and file/directory `fsync` to reserve a
single-use key. The rule is **first stored vote wins; no amendments**.

This protects against process-local races, restarts, and competing processes
that all use the **same local directory on a filesystem providing atomic
exclusive creation**. It DOES NOT implement cross-machine atomicity, an
append-only ledger, byzantine agreement, global double-vote protection, or
consensus finality. No NFS/SMB/Windows mounted directories: run the lab in the
WSL ext4 filesystem under `$HOME`, not `/mnt/c`.

A crash during a reservation may leave an incomplete `.vote` file, and the
same replay key then stays blocked. This is intentional fail-closed behavior.
It requires manual forensic examination, not an automatic re-admission.
No production recovery protocol or arbitrary untrusted path support exists.

## Tests and threat cases

- Valid vote + pinned digest, deterministic commitment, identity-bound key.
- Each signed field alteration, forged signature, invalid choice, bad key,
  wrong network, chain, genesis, proposal, window, and out-of-range height.
- A changed nonce/choice cannot evade single-voter single-proposal replay key.
- Restart-resilient local reservation, separate voters, invalid signatures,
  concurrent goroutines, concurrent Linux subprocesses, unsafe directories.
- Go tests with `-race` and `go vet`.

Commands from the worktree:

```bash
export PATH="/usr/local/go/bin:$PATH"
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOWORK=off
go test -count=1 ./internal/governancev069
go test -race -count=1 ./internal/governancev069
go vet ./internal/governancev069
```

## Not yet implemented / mandatory before any real voting

- Membership/eligibility and checkpointed voter snapshot; public key alone
  allows Sybil votes, so **never tally this into decisions**.
- Authority-controlled proposal admission and governance lifecycle.
- Distributed durable deduplication / ordering, and a globally consistent ledger.
- Formal tally, quorum, threshold, abstention handling, and expiry policy.
- Real consensus integration, quorum/timelock revalidation, security review.

Do not import this lab into the existing v0.61 execution path.
