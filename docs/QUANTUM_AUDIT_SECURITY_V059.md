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

The directory lock serializes journal updates, but does not
hold a lifetime lock for the entire running API process.
Operators must independently prevent overlapping old and
new audit instances when rotating an epoch.

Durability during sudden power loss depends on filesystem
and storage guarantees. Synchronizing the journal file
alone is not a full crash-consistency guarantee for every
filesystem, especially at initial file creation.

## Stale-lock recovery

A crash can leave the directory:

`quantum-audit-epochs.jsonl.lock`

Never remove it while an audit process may still be using
the same data directory.

Recovery procedure:

1. Stop all Prism API instances using the affected directory.
2. Confirm no such process remains active.
3. Back up the complete data directory, including the journal.
4. Inspect the journal and record its last verified epoch.
5. If the journal is valid and only the stale lock remains,
   remove that lock directory manually.
6. Restart with the same policy/epoch, or advance the epoch
   when changing policy.
7. Confirm startup and inspect the new journal state.

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
