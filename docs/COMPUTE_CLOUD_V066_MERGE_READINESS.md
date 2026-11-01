# Prism Cloud MVP v0.66 — experimental merge readiness

## Purpose and scope

This is a **self-assessment for an experimental code merge**, not an independent
security audit, production deployment approval or hardware attestation.

- Target: `master` from `integration/prism-cloud-v066`, PR #50.
- Base before Cloud PR: Core v0.66 `c657d9f`.
- GPU-hardening baseline: `d2410e0` (Cloud phases 2F.1–2F.3).
- This review document and its accompanying source-comment correction change
  **no executable behavior**. The final commit still needs CI on its own SHA.

## Confirmed experimental boundaries

- `cmd/prism-cloud` is Linux-only, bound to `127.0.0.1:18066`, and implements
  a **loopback simulation gateway**, not a public SaaS compute dispatcher.
- The gateway requires Bearer authentication, derives task identities server-side,
  applies bounded requests and per-tenant simulation quotas, and stores only
  simulated jobs. The node observer is read-only with pinned chain identity.
- No existing Cloud HTTP route invokes CUDA, wallet signing, live marketplace
  job submission, escrow settlement or consensus operations.
- Linux/amd64 offline GPU execution runs a SHA-256-approved immutable `memfd`
  snapshot via `/proc/self/fd/3`. The GPU lock, when active, is passed as FD 4.
- Standalone operator GPU, broker and queue CLIs use one shared advisory `flock`
  directory, by convention also used by cooperating Prism Core v0.66 workers.
- Parsed GPU responses are checked against an independent CPU reference.
  Neither this check nor a pinned hash proves physical GPU execution.

## Important limitations — not production guarantees

- `flock` coordinates cooperating processes that use the **same user, directory,
  filesystem and file inode**. It is not hardware isolation. Other GPU processes
  can ignore it; exported internal unlocked lab entrypoints still exist.
- The approved SHA-256 is supplied by an operator CLI, not yet governed by a
  centrally enforced, independently trusted artifact approval/revocation policy.
- A sealed executable prevents source-path replacement affecting the bytes
  executed; it does not sandbox the subprocess, attest GPU hardware or guarantee
  that linked libraries, runtime dependencies and operating system are trusted.
- The new hardening was qualified using offline tests and mock workers;
  recent GitHub CI does not establish a new end-to-end real-CUDA production run.
- There is no authenticated Cloud-to-Core job dispatch, billing, payout, escrow
  settlement, or signed PoUW proof lifecycle from the HTTP gateway.
- This document does not replace independent security review of all 51+ PR files.

## Verification evidence and gate

Before the documentation-only follow-up commit:

- Local offline Go suite, selected `-race` packages, `go vet`, and compatible
  cross-platform builds passed on the `d2410e0` patch.
- PR checks for `d2410e0` passed on GitHub: Go CI, Python worker CI,
  and Rust cuTile GPU worker CI.
- No file conflicts with the then-current Core `master` were reported.

For the **final PR head**, verify again:

1. Branch head and target base match the reviewed commits, with no unexpected
   code or workflow changes.
2. All GitHub checks on the new head commit complete successfully.
3. Any repository-required approvals and policy gates are satisfied; the
   absence of merge conflicts or CI failures is not an independent code review.
4. The merge is explicitly accepted as **experimental / not deployed**.

## Before any production Cloud GPU or PoUW activation

Require an independently trusted and revocable GPU artifact policy; explicit
Cloud-to-Core authenticated and scoped dispatch; process isolation; verification
of job identities and signed proofs; replay/idempotency defense; cancellations
and crash recovery; escrow lifecycle safety; operational telemetry and limits;
and adversarial end-to-end tests with live GPU under a separate approved gate.

**This merge must not start Cloud services, enable public ingress, activate GPU
execution from HTTP, create on-chain proofs, or settle escrow.**
