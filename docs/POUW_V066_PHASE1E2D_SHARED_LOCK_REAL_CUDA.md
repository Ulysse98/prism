# PRISM v0.66 Phase 1E.2D — Core/Cloud shared-lock CUDA qualification

- Opt-in, Linux amd64, off-chain integration test with a locally pinned `ml_gpu_task_3x3` worker.
- Core uses the same private-directory `gpu-owner.lock` advisory `flock` protocol as Prism Cloud.
- Case A: Cloud owns the lock; Core's approved sealed CUDA path returns GPU_BUSY and no proof.
- Case B: Cloud releases; Core acquires the lock, executes the real CUDA worker from a sealed memfd, verifies CPU/GPU parity, then signs and verifies an independent CPU report.
- Case C: after Core exits, Cloud reacquires the shared lock.
- This tests cooperating processes, not noncooperating CUDA workloads, device isolation or hardware attestation.
- The observed SHA-256 is a local operator pin, not an externally certified binary provenance.
- The opt-in test does not call consensus, marketplace settlement or blockchain APIs; no payout is authorized.
- Lock directory is private to the qualification run; the production default lock path is not modified.
- No code from Prism Cloud is modified.
