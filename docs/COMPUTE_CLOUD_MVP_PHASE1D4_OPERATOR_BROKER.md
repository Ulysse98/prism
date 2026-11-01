# Prism Cloud Phase 1D.4 — Operator-only local GPU broker laboratory

This phase does **not** add any HTTP endpoint, modify existing SaaS routes,
create or claim marketplace jobs, produce PoUW proofs, sign, or spend PRISM.

The operator runs a separate CLI with:
- explicit `-enable-local-broker-gpu` flag;
- `-confirm-task-id` matching the canonical `ParseSubmitV1` task ID;
- exact loopback Prism node port, chain ID and lowercase genesis hash;
- absolute CUDA executable and exact SHA-256 pin;
- a size-limited JSON request on stdin.

The broker validates the workload, checks read-only node status, executes exactly
one short-lived pinned CUDA session through the already-qualified offline
executor, validates its result against the independent CPU calculation, and
checks the pinned node identity / height once more. The output contains
`http_dispatch_enabled:false`, `escrow_enabled:false`,
`proofs_available:false`, `gpu_attestation:false`.

This is NOT multi-user or production isolation. No cross-process scheduling,
rate limits, production GPU sandboxing, hardware attestation, signed proofs,
or settlement are present. A stable node height does not cryptographically
prove the absence of all blockchain mutations. The trusted executable hash
check is not a complete defense against binary substitution races.

Example input: `{"batch_size":1,"inputs":[1,0,-1],"weights":[1,0,0,0,1,0,0,0,1],"biases":[0,0,0]}`.
Expected task ID for this input:
`db09593096d7bf5c364ae10117569108e0d016800b8cfa1a559926f630111d29`.

Qualification: run Go tests and the race detector. Real GPU operation requires
the operator to invoke this command with a qualified CUDA binary and isolated
Prism API; do not use the production node or main wallet.
