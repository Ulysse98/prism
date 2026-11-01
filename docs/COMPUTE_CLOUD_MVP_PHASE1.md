# Prism Compute Cloud — MVP Phase 1A

## Status

**Internal validation module only.** Not a running SaaS, not a deployed API, and not a paid compute service. No network calls, authentication, balances, worker scheduling, payments, marketplace mutations or consensus changes in this phase.

## Input contract

A single JSON object with exactly four fields:

```json
{
  "batch_size": 1,
  "inputs": [1, 0, -1],
  "weights": [1, 0, 0, 0, 1, 0, 0, 0, 1],
  "biases": [0, 0, 0]
}
```

The *server* constructs `usefulwork.Task` with type `ml_inference_quantized`, fixed dimensions 3×3, and derived `id` and `input_hash`. The client cannot select its payer, wallet, reward, nonce, task ID, hash, worker or proof.

## Phase 1A security contracts

- Reject unknown JSON properties and duplicate top-level properties.
- Reject multiple JSON values, non-object roots, malformed JSON, null roots, oversized payloads (>64 KiB), and incorrect tensor lengths.
- Batch size 1..1024 (conservative vs. worker documented limit).
- Signed quantized input/weight/bias values bounded to [-32767, 32767].
- Delegate canonical Prism task validation to `usefulwork.ValidateTask`.
- No HTTP handler is mounted, no untrusted token is accepted, no job is created, no financial operation occurs.
- The HTTP boundary in 1B MUST enforce request size *before* reading, e.g. `http.MaxBytesReader`, and must authenticate before calling this parser.

## Next phase 1B (not implemented)

- Separate HTTP service `cmd/prism-cloud`: TLS at trusted reverse proxy; API key hashed at rest; credential rotation and revoke; rate limiting per tenant, concurrent jobs cap, cost cap.
- Server-owned tenant -> billing identity -> Prism requester wallet mapping. Never accept wallet/reward/nonce from the user request.
- SQL store scoped by tenant with durable unique `(tenant_id, idempotency_key)` and conflict detection for different payloads; transactions to avoid double job creation.
- POST /cloud/v1/jobs (202 + polling URL), GET /cloud/v1/jobs/{id}, GET /cloud/v1/jobs/{id}/proof, GET /cloud/v1/usage.
- Every GET uses tenant-filtered lookup (not just unguessable IDs), and proof retrieval confirms verified chain-linked results. Never trust an unauthenticated GPU receipt as proof of correct computation or hardware origin.
- Adapter validates using existing `usefulwork.ValidateTask`, approved gateway policy, and existing Prism compute marketplace path; no raw shell/process execution and no consensus hooks.
- Meter usage only on verified/finalized successful work, with idempotent accounting; no real charging in Phase 1B.
- Tests: unauthorized/forbidden, cross-tenant access, duplicate idempotency keys, concurrent races, 429/413, context cancellation, workflow failures, proof rejection, `-race`, Go vet, and existing Prism regression tests.

## How to validate

From the isolated Prism Cloud worktree:

```bash
go test ./internal/computecloud -count=1
go test -race ./internal/computecloud -count=1
go vet ./internal/computecloud
git diff --check
```

Do not merge this branch until the full Prism tests and the working-tree baseline have been reviewed.
