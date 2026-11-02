# PRISM v0.68 — Phase 1B.3D: multi-process recovery qualification

**RESEARCH-ONLY, TESTS + DOCUMENTATION, LINUX-ONLY. NO CONSENSUS OR SIGNER CHANGE.**

## Scope

Exercise existing v0.68 Phase 1B.3B/1B.3C checkpoint-floor APIs through
**separate Linux test-binary processes**, rather than only concurrent goroutines:

- Multiple concurrent competing `CommitPreparedCheckpointFloorV068` calls:
  exactly one commits and every losing process must receive the known
  `no prepared checkpoint floor` rejection. The committed floor must match
  the test's proposed next witness; old witnesses must be rejected.
- An independent process attempting a read-only inspection must remain
  blocked on the same Linux `flock` while a cooperating process holds it.
- Absent/unavailable external witness is modeled by a zero witness. Inspect,
  read and commit must reject it; the pending state must remain unchanged.
- Test fixture highlights that merely passing a matching witness from an
  untrusted process is **not authentication**.

The test uses throwaway fixture keys and isolated temporary owner-controlled
Linux directories. No network or checkpoint publisher is contacted.

## Non-goals and security boundaries

- This is a process-concurrency regression lab, not a remote authenticated
  witness protocol or a Byzantine safety proof.
- Linux `flock` requires all cooperating processes to share the lock path;
  it does not stop privileged/non-cooperating writers or full-host rollback.
- Process tests do not establish power-loss durability, filesystem integrity,
  atomic distributed transactions or availability under partitions.
- Unavailable external trust is fail-closed for tested calls. No automated
  recovery, operator authorization or production signing may be inferred.
- No changes to live Prism consensus, wallets, escrow, API, or P2P.

## Qualification

```sh
go test -count=1 -timeout=5m ./internal/finality ./internal/consensus
go test -race -count=1 -timeout=8m ./internal/finality
go vet ./internal/finality ./internal/consensus
```
