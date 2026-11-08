# PRISM v0.69 / Phase 1B.6 — Guarded Local Advisory Ballot Closure

## Scope: SINGLE-HOST RESEARCH, NOT PRODUCTION FINALITY

This phase adds a **durable local admission gate** to the existing v0.69 advisory vote replay store. It is an offline, Linux-only, single-host lab. No consensus, validator signing, real token voting, on-chain admission, treasury transaction, timelock execution, RPC, or cross-host agreement is enabled. The closure is **unsigned**, with no independently authenticated close authority. Caller-provided heights and SHA-256 pins are NOT proof of consensus or snapshot trust. The operator must authenticate and approve the proposed closure and both pins outside this module.

## Architecture

- The existing `(*AdvisoryVoteReplayStore).ReserveVerifiedAdvisoryVote` is **modified** to acquire an exclusive Linux `flock` on the trusted **directory file descriptor** before checking the closure marker or creating a vote reservation. Writes now use `openat` relative to that same locked descriptor and `O_EXCL|O_NOFOLLOW`. Vote files and directory metadata are `fsync`ed before unlocking.
- `SealLocalAdvisoryVoteStore` acquires that **same directory lock**; refuses an existing or unreadable marker; independently verifies the 1B.4 tally receipt, 1B.3 snapshot/ballot pins and signatures, and 1B.5 exact vote-file reconciliation; then creates `.prism-advisory-closed.v1` via `openat(O_CREAT|O_EXCL|O_NOFOLLOW, 0600)`, writes canonical JSON and calls `fsync` on the file and directory before unlocking.
- An incomplete/corrupt/replaced/symlink closure marker **does not reopen admission**. Any entry at the marker path (including an inaccessible or invalid one) causes the writer to fail closed; no auto-delete or recovery path is provided.
- `VerifySealedLocalAdvisoryVoteStore` reopens and validates the canonical closure record (regular file, owned by caller, 0600, hardlink count one, bounded size, exact canonical serialization and domain-separated SHA-256), then recomputes the full vote-store reconciliation against the externally pinned inputs while holding the admission lock.
- 1B.5 `AuditLocalAdvisoryReservations` is updated to permit exactly one recognized and **fully verified** closure marker. All unrelated entries still fail closed. An ordinary standalone audit still requires operator-controlled quiescence for all noncooperating writers.

## Public APIs

- `SealLocalAdvisoryVoteStore(store, receipt, snapshot, snapshotPin, ballotPin, proposal, trusted, ballots) (AdvisoryBallotClosure, error)`
- `VerifySealedLocalAdvisoryVoteStore(store, receipt, snapshot, snapshotPin, ballotPin, proposal, trusted, ballots) (AdvisoryBallotClosure, error)`
- `ErrAdvisoryBallotClosed`: indicates sealed/ambiguous marker. A second seal fails; reopening the store does not re-enable admission.

## Safety preconditions and limitations

1. The store must be dedicated to exactly one proposal, with trusted parent directories and exclusive filesystem ownership. It must be a private `0700` local Linux filesystem directory; no NFS, cloud-share, cross-host volume or untrusted same-UID writer. Local `flock` coordinates **only upgraded cooperating callers**. Old/unpatched executables and direct filesystem writes can bypass the gate.
2. All inputs/pins and the operator's authority to close must be independently approved. Self-generated SHA-256 pins and unsigned receipts do not authenticate policy, the voter roster, global ballot completeness, real chain height or the close decision.
3. Once a closure marker has been created, **never auto-remove** it even on failed write/sync, crash or mismatched audit. Incident response is a separate operator-controlled forensic process. An ambiguous crash is intentionally fail-closed. Filesystem hardware that lies about `fsync` or can roll back metadata is outside the threat model.
4. The recorded height is the trusted caller's observation height, not a finalized chain header or consensus timestamp. It does not imply that the proposal expired or that the network authorized closing.
5. The operator must retain an independent copy of closure evidence/pins, and must prevent same-user edits or directory relocation. The marker is not tamper-proof against the filesystem owner or root and is not a cryptographic signature.
6. There is no on-chain execution, BFT finality, distributed non-equivocation, cross-host uniqueness or data availability guarantee. This gate is a narrow local-admission property, not a production vote-finalization protocol.

## Test coverage

Existing 1B.1–1B.5 tests continue passing. New tests cover canonical closure digest, post-restart verification, changed nonce/choice, double sealing, corrupt/partial/symlink/untrusted markers, invalid pinned inputs, omitted and extra reservations, concurrent seal-vs-vote ordering, and six re-executed processes that must reject post-close admission. Linux tests run with `go test`, `go test -race`, `go vet`; Windows gets compile-only fail-closed stubs.

## Installation discipline

Guarded installer requires clean `feat/v0.69-governance` HEAD `85e7c97` and the exact SHA-256 hashes of all **20 previously committed governance files**. It explicitly **updates two existing tracked files** (`vote_replay_linux.go`, `reservation_audit_linux.go`) and creates four Go files plus this document; no other files are changed. It first backs up both tracked sources outside Git and restores the originals/removes its new files on a failed check. No automatic commit, push, merge, network access, production keys or real admission store operations.
