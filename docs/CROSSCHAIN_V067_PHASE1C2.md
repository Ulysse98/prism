# PRISM v0.67 — Phase 1C.2: local process-recovery qualification

Status: experimental OFFLINE tests, Linux trusted local filesystem. No bridge, signer,
RPC transaction, distributed consensus, escrow authorization, or cross-chain payout.

## What the tests establish

- Aborted child process writes an incomplete reservation and exits **without** fsync.
  A subsequent process treats the existing file as occupied but rejects inspection.
- Twelve separate OS processes compete for the same proof identity. Exactly one
  successfully reserves it; the others reject replay. The winning record can be
  inspected from a newly constructed store after the processes have finished.
- A too-permissive replay namespace and a symlinked replay namespace fail initialization.
- Phase 1C.1 regression, race detector, and `go vet` must also pass.

These tests exercise process termination, not a machine-level crash or loss of power.
Process termination leaves page-cache and directory metadata managed by the kernel;
therefore results CANNOT be extrapolated to hard power failure, disk-controller cache
loss, filesystem corruption, kernel panic, or snapshot/backup rollback.

## Durability and recoverability boundaries

The Phase 1C.1 writer uses O_EXCL for local concurrency and attempts file `fsync`
and directory `fsync` before acknowledging success. This is a reasonable local
technique **on an appropriate POSIX filesystem**, not proof of durability across
hardware fault scenarios. If power fails before the directory entry is durable,
the slot may disappear; a later reservation can then appear new.

An existing partial/corrupt slot MUST remain occupied and fail inspection. Do not
automatically remove it or interpret it as an authorized payment. Any recovery
procedure requires human investigation and independently trusted evidence of
whether the proof was registered or settled elsewhere.

A privileged actor able to delete, replace or roll back the storage directory
can defeat this guard. Operators with different storage roots do not share a
replay register. `ReserveProofV067` only checks structural intent validity:
if exposed to untrusted callers, it can be used to occupy a proof slot as a
permanent denial-of-service. Future integration must restrict admission to a
trusted, independently verified candidate pipeline and define recovery policy.
A cross-chain settlement system requires a stronger, globally coordinated
uniqueness policy and authenticated source finality. Merely combining earlier
verification gates with this local reservation MUST NOT authorize registration
or payout.

## Changes in Phase 1C.2

This phase adds only an offline test and this document. It does NOT modify
`local_replay_reservation_v067.go` or any earlier Phase 1A–1C.1 implementation.
