# Prism Compute Cloud — Phase 1E.1: offline bounded GPU scheduler

## Purpose

Provide a *non-networked* scheduler for the existing operator-authorized
GPU broker. No new HTTP routes, no wallet accesses, no marketplace creation,
no proof signing, no escrow or settlement.

## Contract

- **At most one active callback** per scheduler instance. The worker is
  a single goroutine and calls the GPU runner synchronously.
- **Bounded backlog:** configure 1–4 *waiting* tasks; an additional single
  task may be active. Full queue rejects immediately (`ErrLocalGPUQueueFullV1`).
- **Operator binding:** every submission must supply the exact canonical
  64-hex `task_id`; malformed or mismatched confirmations are refused before
  enqueueing any work.
- **Input validation and copy:** strict SaaS v1 JSON, max 64 KiB, 3x3 and
  1024 samples; request bytes copied to prevent caller mutation.
- **Cancellation:** caller cancellations propagate to an active runner and
  expired queued work is discarded. Server close cancels active work and
  refuses new submissions. Per-callback timeout must be between 1ns and 30s.
- **Defense-in-depth output validation:** expected task ID/input hash,
  no dispatch/escrow/proofs/attestation claims, correct result dimensions,
  exact independent Go CPU computation, and SHA256 of output vector.
- **No GPU starts merely from installing or constructing the scheduler:**
  the runner is injected separately. This phase only validates fake-runner
  tests. Actual CUDA authorization and pinning remain in 1D.4.

## Limitations — NOT a public job scheduler

This is an **in-memory, one-process experimental primitive**.
It does not implement per-tenant isolation, billing, authentication, quotas,
crash recovery, durable queues, GPU memory limits, fair scheduling or a
cross-process lease. Cancellation requires the injected runner to honor ctx.
A malicious or hung callback can delay shutdown; Close(ctx) bounds the
caller's wait but cannot kill a non-cooperative process by itself.
No HTTP endpoint calls the scheduler. No GPU execution occurs during install.

The future operator CLI integration must wrap `RunLocalBrokerGPULabV1` in a
context-aware runner, use the existing pinned binary and node observer,
and keep the operator confirmation for every submitted task.

## Qualification

```
go test -count=1 ./internal/computecloud ./cmd/prism-cloud ./cmd/prism-cloud-gpu-broker-lab ./internal/usefulwork
go test -race -count=1 ./internal/computecloud
go vet ./internal/computecloud ./cmd/prism-cloud
```

Test focus: unauthorized input, strict queue bound, single active callback,
canceled queue items, deadline, close and rejection of a forged CPU result.
