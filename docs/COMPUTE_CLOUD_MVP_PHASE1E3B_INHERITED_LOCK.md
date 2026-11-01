# PRISM Cloud — Phase 1E.3B, inherited cooperative GPU lock

The operator-only offline GPU queue now acquires the shared Linux `flock`
from `$HOME/.prism-cloud-gpu-lock-v1` just before CUDA process launch,
after validating the workload, binary SHA-256 pin, and broker preflight.
The GPU child inherits the same open-file description as fd 3 via
`exec.Cmd.ExtraFiles`. No separate file lock may be acquired by another
cooperating process using that same directory until all holders close.

**Critical:** the supervisor must close its own descriptor without calling
`LOCK_UN`. Explicit unlock on a shared open-file description would revoke
ownership from a still-running child. The stand-alone lock lab keeps its
original `Release` API for non-inherited use.

Existing direct `RunLocalGPULabV1` and `RunLocalBrokerGPULabV1` are
legacy unlocked offline APIs for compatibility. The queue CLI now uses
the locked broker runner; all GPU users must cooperate with the same lock
for cross-process exclusion to have meaning. Other local GPU entrypoints
are not yet migrated. Public dispatch is not enabled.

The pinned GPU executable is trusted/cooperative: it must keep fd 3 open
through its GPU work. If it forks descendants, they must also retain the
inherited fd for as long as they can use the GPU. A worker that explicitly
closes fd 3, calls `LOCK_UN`, or moves work to an unrelated process defeats
this advisory protection. The existing process-group `SIGKILL` cancellation
and 2-second `WaitDelay` remain in place, but there is no cgroup tracking,
GPU hardware isolation, or proof that every descendant has been reaped.
Killing the supervisor may leave an orphaned worker running, intentionally
holding the lock until it exits. Do not delete the lock file to recover it.

New offline tests simulate a worker inheriting fd 3 and demonstrate that
SIGKILL of its supervisor does not release the shared flock, whereas killing
the last holder does. These tests do not execute CUDA. Before promotion,
verify the *actual pinned CUDA executable* preserves fd 3 during its entire
lifetime and run isolated GPU integration tests. This phase does not enable
Cloud HTTP dispatch, PoUW proofs, settlement, escrow, wallets, or rewards.
