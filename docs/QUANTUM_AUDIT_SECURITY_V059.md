# Prism v0.59 — Quantum audit operations

## Scope

The quantum audit API is observational. Its results do not
independently prove CUDA-Q execution, GPU hardware identity,
or consensus finality.

The configured policy epoch and canonical policy fingerprint
are recorded in a local append-only JSONL journal named:

`quantum-audit-epochs.jsonl`

## Normal startup

- A policy-enabled node requires a positive policy epoch.
- Repeating the same policy and epoch is permitted.
- Changing policy requires a strictly greater epoch.
- Moving to a lower recorded epoch is refused.
- A malformed or incomplete journal prevents startup.
- The journal is scoped to the configured node data directory.

## Storage and trust boundaries

The journal is protected against accidental configuration
rollback, not malicious deletion or replacement by an actor
with write permission to the data directory.

Use a private local filesystem and restrict its access.
On Windows, inspect the effective NTFS ACLs of both the
directory and the journal. Go's `0600` creation mode does
not replace NTFS ACL verification.

The short-lived journal lock serializes epoch updates.
A separate `prism-api-active.lock` directory excludes
cooperating API instances sharing the same data directory.

The runtime lock is held until successful graceful HTTP
shutdown. If shutdown fails or the process crashes, the
lock remains and requires manual recovery.

Older Prism versions do not honor this runtime lock.
Operators must stop older instances before starting v0.59.

Durability during sudden power loss depends on filesystem
and storage guarantees. Synchronizing the journal file
alone is not a full crash-consistency guarantee for every
filesystem, especially at initial file creation.

## Stale-lock recovery

Two independent lock directories may remain after a crash:

- `prism-api-active.lock`: exclusive API runtime lock
- `quantum-audit-epochs.jsonl.lock`: epoch journal lock

Neither lock may be automatically deleted on startup.

Safe recovery procedure:

1. Identify the exact Prism node data directory.
2. Stop all API processes using it, including Docker or WSL.
3. Verify that no relevant process remains active.
4. Back up the entire data directory and journal.
5. Inspect the journal for corruption or incomplete records.
6. Confirm the last trusted policy epoch and fingerprint.
7. Identify which lock directories are stale.
8. Remove only confirmed stale lock directories, manually.
9. Never delete or reset the journal during lock recovery.
10. Restart with the same approved policy and epoch, or
    advance the epoch when changing policy.
11. Verify successful startup and journal consistency.

If lock ownership or journal integrity cannot be established,
stop recovery and investigate. Never guess that a lock is stale.

Older Prism versions do not honor the API runtime lock.

## API exit status

The `prism api` CLI exits with status 0 only after a
successful graceful shutdown and resource cleanup.

Failed startup validation, journal rejection, listener
failure, incomplete HTTP shutdown, SQLite close failure,
or runtime lock release failure results in nonzero status.

An error exit does not authorize automatic lock deletion.
## Corrupt or truncated journal

Do not automatically truncate or rewrite journal history.

Keep a forensic copy of the damaged file. Investigate the
interruption and restore a trusted backup only after
confirming its epoch and fingerprint history.

An unavailable trusted history is an operational/security
incident requiring explicit recovery decisions. Deleting
the journal and restarting from epoch 1 is not an
authenticated restoration of the earlier history.

## Testing limitations

Unit tests cover monotonic epochs, corruption handling,
interprocess contention, and a simulated abandoned lock.

Interrupted-append tests construct damaged files
deterministically. They are not physical power-loss tests.

Linux symlink and filesystem behavior require CI coverage.
Local Windows symlink tests may be skipped when the account
lacks privileges to create symbolic links.
