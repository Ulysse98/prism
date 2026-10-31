# Prism v0.66 — Phase 1D.3 real CUDA sealed one-shot qualification

This is an **off-chain laboratory test**, not a consensus change or a production GPU attestation.

- Requires an explicit `PRISM_V066_RUN_REAL_CUDA=1` environment flag and a local Linux/amd64 CUDA worker.
- Uses the v0.66 approved sealed one-shot path: snapshot executable bytes, compare with operator-pinned SHA-256, seal memfd, execute the snapshot, parse the receipt, derive a signed PoUW v2 proof, and verify the proof against a v0.66 task contract and independent CPU computation.
- Negative checks cover a modified result, changed job binding, wrong executable digest, and cancellation. Existing v0.62/v0.63 launch paths are unchanged.
- The sample uses *synthetic* task and chain identities, a newly generated ephemeral worker key, no live node, no marketplace, no escrow, and no settlement.
- The pinned SHA-256 comes from a prior local worker measurement. This is **not** a signature from the build publisher, a reproducible-build certificate, a trusted execution environment attestation, or a guarantee that CUDA dependencies are unchanged.
- `nvidia-smi` process discovery may be incomplete on WSL. The shell `flock` only serializes this specific test, not other Prism Cloud/GPU processes. Run only when no other CUDA jobs are active.
- Successful output verifies the real worker response and mathematical CPU parity, but cannot prove to an external observer that the computation took place on a GPU or that every dependency was untampered.

The test is opt-in and automatically skips during normal `go test` runs.
