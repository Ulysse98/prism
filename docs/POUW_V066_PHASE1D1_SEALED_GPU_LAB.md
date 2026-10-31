# Prism v0.66 — Phase 1D.1: Linux AMD64 sealed worker prototype

## Scope

Off-chain isolated execution primitive in `cmd/prism`, *not* wired into the
existing v0.62 one-shot or v0.63 session launchers. No consensus, escrow,
marketplace or signing changes. Tests use `/bin/echo` and `/bin/false` and do
not require CUDA hardware.

## Trust boundary

The SHA-256 supplied to `runApprovedGPUWorkerV066` MUST come from a trusted,
independently maintained executable allowlist, never from an untrusted client,
task, proof, receipt, request parameter or filesystem metadata.

1. Open absolute worker pathname with `O_NOFOLLOW | O_RDONLY | O_CLOEXEC`.
2. Require a regular, nonempty, executable file not exceeding 512 MiB.
3. Copy the actual bytes to a Linux `memfd_create` file in bounded fashion.
4. Hash those copied bytes and compare to the independently approved SHA-256.
5. Apply and verify `F_SEAL_WRITE`, `F_SEAL_GROW`, `F_SEAL_SHRINK`, `F_SEAL_SEAL`.
6. Execute `/proc/self/fd/3` using `ExtraFiles`, inheriting *only the sealed
   executable snapshot* as fd 3. The source pathname is never used for exec.
7. Preserve legacy Linux process-group cancellation, 60-second execution
   ceiling, and stdout/stderr bounds.

If `memfd_create`, file sealing or `/proc/self/fd` execution is unavailable,
execution must fail closed. Linux **amd64 only**; the syscall number is pinned
to the Linux amd64 ABI. No other operating system is enabled by this file.

## Covered by tests

- Approved ELF runs from sealed memory.
- Wrong, noncanonical or malformed SHA-256 rejected.
- Relative path, symlink, directory, empty/non-executable file rejected.
- Size restrictions enforced.
- In-place corruption rejected.
- Atomic replacement of the original path **after sealing** cannot redirect
  execution (new attempts from the replaced path fail).
- Writes to sealed memfd fail; unsealed descriptors cannot launch.
- Cancellation and nonzero exit fail closed.

## Explicit limits

This prevents pathname replacement between byte verification and launching the
**main executable**. It does NOT attest that a CUDA GPU performed the task;
measure GPU execution; pin dynamically loaded libraries, model files or
plugins; guarantee host, GPU driver, kernel, environment variables or library
integrity; or grant permission to settle escrow.

Parent-directory symlinks and filesystem namespace changes are not generally
rejected; copied executable bytes remain subject to the trusted digest.
Calling code must still use a controlled environment and trust policy for
runtime dependencies. The new primitive is intentionally opt-in and is not
connected to the existing GPU execution paths; a separate design review and
integration tests are required before production use.
