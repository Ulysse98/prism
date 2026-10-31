# Prism Compute Cloud — Phase 1E.3A, cooperative Linux GPU lock

## Purpose

Provide an **offline, operator-only** process exclusion primitive for the
single WSL machine running the experimental NVIDIA CUDA worker. No HTTP routes,
marketplace calls, blockchain writes, signatures, CUDA launches, or GPU dispatch
are added. **This primitive is not yet wired to Phase 1E.2's queue.**

## Contract

- Linux `flock(LOCK_EX | LOCK_NB)` on a private regular file.
- Shared default directory for a single operator:
  `$HOME/.prism-cloud-gpu-lock-v1`.
- The lock directory must have a trusted parent and be owned by the current
  user, private (no group/other permission), and not itself a symlink.
- Lock file must be a regular file owned by current user, private, and without
  multiple hardlinks. Symlink lock files fail closed.
- A second cooperative process using the **same lock directory** gets
  `ErrGlobalGPUBusyV1` immediately.
- Releasing the descriptor drops the lock; the kernel also releases on process
  termination. The lock *file remains*. **Never delete the lock file to recover
  a stale lock**, or two competing lock inodes could undermine exclusion.
- O_CLOEXEC means subprocesses do not inherit the lock file descriptor.

## Local demonstration (does not use CUDA)

```bash
export PATH="/usr/local/go/bin:$PATH"
go run ./cmd/prism-cloud-gpu-lock-lab \
  -enable-global-gpu-lock -hold-for 5s
```

The output contains `mode: offline_gpu_global_lock_lab`, `status:
lock_acquired`, and `cuda_executed: false`. A concurrent process using the
same directory is refused with `GPU_BUSY` (exit status 3). `-hold-for` ranges
from >0 to 30 seconds.

## Tests

```bash
go test -count=1 ./internal/computecloud ./cmd/prism-cloud-gpu-lock-lab
go test -race -count=5 -timeout=3m \
  ./internal/computecloud ./cmd/prism-cloud-gpu-lock-lab
go vet ./internal/computecloud ./cmd/prism-cloud-gpu-lock-lab
```

The test suite includes actual child-process exclusion, process SIGKILL and
kernel lock recovery, double-release, path symlink and hardlink rejection,
unsafe directory rejection, and explicit CLI opt-in. Tests run in tempdirs.

## Limits / mandatory next work

This is an **advisory cooperative process lock, not GPU resource isolation**.
It cannot prevent external CUDA clients, other users with different lock
paths, malicious same-UID software, or another host from using the GPU.

**Most importantly:** a supervising process can crash while its CUDA child
continues to run. The kernel would then release the supervisor's file lock
before the orphaned CUDA process stops. For Phase 1E.3B, integrate the lock
with the **entire** worker subprocess lifetime and ensure its process group
is terminated and reaped before releasing ownership. A lock alone is not
sufficient to make remote customer dispatch safe.

Do not enable public HTTP execution, escrow, proofs, rewards, or autonomous
marketplace work based only on these tests.
