# PRISM v0.66 — Phase 1E.2B: shared advisory GPU lock (offline)

This is an opt-in Linux/amd64 Core wrapper. Legacy GPU v0.62/v0.63 paths remain unchanged. There is no CLI activation, no network dispatch, no blockchain mutation and no escrow settlement.

## Compatibility protocol

Both Prism Cloud v1 and Core v0.66 use an exclusive nonblocking Linux `flock` on **the same inode**: `$HOME/.prism-cloud-gpu-lock-v1/gpu-owner.lock` when the caller selects that absolute directory. Paths must be clean and absolute; the directory must be private and owned by the current user. The file is created with mode 0600, rejects symlinks, hardlinks and unsafe ownership/permissions, and is revalidated after acquiring `flock`.

The sealed worker executable is copied and SHA-256 checked against an independently approved operator policy before acquiring the lock. At launch the executable's sealed memfd is inherited as child FD 3; the shared flock descriptor is inherited as child FD 4. The parent **only closes** its FD, never explicitly issues `LOCK_UN`, because the child shares the same open-file description. Kernel cleanup frees the lock when the final inheriting process exits. Do not delete the lock file as a recovery procedure.

API: `executeComputeGPUApprovedWithSharedLockV066(ctx, task, jobID, chainID, genesisHash, worker, bin, approvedSHA256, lockDir)`; pass the exact same trusted lock directory used by Prism Cloud. This implementation still validates the receipt and PoUW v2 proof, but it is **not** GPU hardware attestation or settlement authorization.

## Threat model and limits

- Advisory only: non-cooperating CUDA programs can still run; `nvidia-smi` is not a reliable exclusion mechanism under WSL.
- The safety of the parent path and directory containing the lock directory is an operator deployment assumption.
- It protects the approved main executable's bytes, not CUDA drivers, loaded dynamic libraries or all runtime dependencies.
- The inherited lock may remain busy if an orphan child or descendant still holds FD 4. Do not unlink the file to force recovery.
- This stage validates the Linux lock semantics with isolated subprocesses. **Direct simultaneous Prism Cloud/Core execution on the real GPU remains a separate qualification.**

## Tests

`go test -count=1 -run '^TestGPUSharedLockV066' ./cmd/prism`

`go test -race -count=1 -run '^TestGPUSharedLockV066' ./cmd/prism`

No CUDA is launched by these tests.
