# PRISM v0.69 / Phase 1B.5 — Local Ballot Reconciliation (Closure Readiness)

## Assurance boundary

Offline, Linux-local, read-only inspection only. This phase is **not a ballot closure** and does not add a closing gate to the Phase 1B.2 writer. It is not an admission authority, consensus state, distributed finality, a BFT proof, an authenticated electorate, a source of truth for eligible voters, or a production governance execution certificate. It never signs, sends transactions, uses RPC, or changes Git. A self-selected digest does not authenticate the snapshot or set of ballots. The operator MUST obtain the snapshot and ballot-set SHA-256 pins independently from an appropriately authorized and reviewed process.

## What this phase checks

`AuditLocalAdvisoryReservations` accepts an existing `*AdvisoryVoteReplayStore`, the 1B.4 `AdvisoryTallyReceipt`, trusted roster and ballot-set SHA-256 pins, the proposal, a trusted observation context and the full list of signed ballots. It first **recomputes and verifies** the tally receipt (including signatures, eligibility, weight, quorum, proposal, network, and digest pins). Then it inspects every file in a **dedicated single-proposal reservation directory** and requires:

- Exactly one record for every signed ballot and no additional entries, including other proposal votes, partial/crashed reservations or unknown files.
- Canonical filename derived from the 1B.2 replay key and exact canonical JSON bytes expected from the Phase 1B.2 local writer, including signature and vote SHA-256.
- A real, privately owned Linux directory with mode `0700`, no symlink path components. Files must be regular, owned by the inspecting user, mode `0600`, hardlink count one, no symbolic-link substitution. Linux file handles use `openat` with `O_NOFOLLOW`; read sizes are bounded.
- A canonical, domain-separated `AdvisoryReservationAudit` SHA-256 binding network, chain, genesis, proposal, roster pin, ballot-set pin, tally-receipt digest, observation height, and exact reservation count.
- A verifier can recompute and compare all audit fields only when the original ballots, proposal, independently pinned inputs, context and source store remain available and trusted.

## Crucial operational preconditions

1. The operator **MUST quiesce all Phase 1B.2 vote admissions during the full directory inspection**, including other processes. This module DOES NOT obtain a lock shared with the vote writer; a concurrent writer or a malicious local user may invalidate the observation. No consistent snapshot guarantee is claimed.
2. The inspected directory **MUST be reserved for one proposal**. The current 1B.2 replay store permits multiple proposals per directory; this audit intentionally rejects any extra files rather than silently ignoring them. No automatic deletion or migration occurs.
3. A matching audit proves only that the *inspected local files* matched the *supplied, independently pinned list* at inspection time under these assumptions. It **does not prove completeness of votes never submitted to this host**, correct source of voter eligibility, global cross-host agreement, irrevocable closure, or non-equivocation. The recorded height is trusted caller input, not a chain header or finality proof.
4. A crashing Phase 1B.2 writer can leave a file which blocks a replay but is incomplete. This audit rejects it; repair must follow a separately reviewed operator procedure. No auto-removal.
5. No execution or treasury permission follows from a successful audit. Closing/locking admissions, finality, and distributed cross-node reconciliation are deferred.

## Test coverage

- Stable audit SHA-256 reference vector and independence from ballot input order.
- Full snapshot/ballot/tally verification and receipt tampering, network/context mismatch, invalid signature, duplicate and missing ballots.
- Extra/missing/partial records, modified bytes, unsafe file/directory permissions and symlink redirection.
- Empty roster participation with non-empty eligibility snapshot; non-Linux fail-closed stub compilation.
- Tests: `go test`, `go test -race`, `go vet` in offline Linux lab.

## Execution

The supplied guarded installer adds only four new Go source/test files and this document. It checks the exact expected branch and 1B.4 checkpoint prefix `458bc1b`, validated v0.68 ancestry, clean worktree, SHA-256 pins for the existing 15 governance files, and payload checksums. It does not commit, merge, push, run consensus, use keys or touch an existing production reservation directory.

## Deferred phase 1B.6 / 1C

A genuine, durable closing gate must coordinate with the writer, reject all post-close admissions, handle power-loss recovery and anti-equivocation, and bind the closure to an authenticated external authority and a globally consistent source of ballots. This lab explicitly does not deliver those properties.
