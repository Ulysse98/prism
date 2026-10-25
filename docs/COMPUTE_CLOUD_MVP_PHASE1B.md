# Prism Compute Cloud — Phase 1B (local simulated HTTP API)

**Scope:** Linux/WSL2 only; development sandbox, loopback address `127.0.0.1:18066` only.
**Base:** `feat/compute-cloud-mvp-phase1` at `8f9b129`.
**No on-chain actions, no wallet access, no GPU execution and NO proof generation.**

## Design

- `cmd/prism-cloud`: independent HTTP binary (not linked to Prism core HTTP server).
- `internal/computecloud/gateway_v1.go`: API key validation, per-tenant access and strict HTTP boundary.
- `internal/computecloud/local_store_v1.go`: single-process local JSON snapshot store, atomic renames and Linux `flock` for exclusive writer; synchronous durability attempts (`fsync`). This is a local MVP store, not a multi-node transactional database.
- Existing `internal/computecloud/request_v1.go`: canonical task builder for `ml_inference_quantized` 3x3 (up to 1024 samples).

## API

- `POST /cloud/v1/jobs` requires Bearer token, `Idempotency-Key` (1..128 allowed ASCII characters), JSON with the already-defined task contract, returns `201` or `200` on replay. Status is always `SIMULATED`.
- `GET /cloud/v1/jobs/{id}` requires same tenant; missing / cross-tenant IDs both return `404`.
- `GET /cloud/v1/jobs/{id}/proof` returns `409` for an owned simulated job, `404` otherwise. **Never returns an invented proof.**
- `GET /cloud/v1/usage` returns persistent lifetime job count and limit.

Tokens: 32 random bytes encoded in an unguessable `pc_dev_...` key. Only their SHA-256 digests are saved in `~/.config/prism-cloud/keys.json` (mode 0600). This file is not a password database for human-chosen secrets: don't use weak keys.

## Manual startup

Run from the new worktree in WSL2, with Go available in PATH:

```bash
export PATH="/usr/local/go/bin:$PATH"
cd "$HOME/prism-lab/prism-cloud-mvp-phase1"
go run ./cmd/prism-cloud init-dev
# IMPORTANT: copy the displayed token somewhere safe; it is shown only once.
go run ./cmd/prism-cloud serve
```

`init-dev` fails closed if the key config exists; it never overwrites keys. Only one developer tenant is bootstrapped. Stop the server with Ctrl-C. The endpoint accepts HTTP **only on loopback**; no remote access. Using a browser across origins is unsupported (no CORS).

For smoke testing in a separate WSL terminal, **do not paste the token into shell commands recorded in history**:

```bash
read -r -s -p 'API key: ' PRISM_CLOUD_TOKEN; echo
curl --fail-with-body -sS -X POST http://127.0.0.1:18066/cloud/v1/jobs \
  -H "Authorization: Bearer $PRISM_CLOUD_TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: smoke-001' \
  --data '{"batch_size":1,"inputs":[1,0,-1],"weights":[1,0,0,0,1,0,0,0,1],"biases":[0,0,0]}'
unset PRISM_CLOUD_TOKEN
```

## Security boundary / limits

- Reject unauthenticated requests, multiple authorization headers, missing/duplicate idempotency headers, unknown JSON fields, oversized payloads (>64 KiB), oversized matrices and unpermitted workload types.
- Tenant IDs are internal, never derived from user fields. A job or proof request must match the authenticated tenant.
- Idempotency is persisted by `(tenant, idempotency-key)` and canonical task ID, with conflict on changed payload. Retry does not spend quota.
- First 32 accepted jobs **per tenant over its entire lifetime**; this is **not** a rate limit and is **not** billing.
- No request reaches a blockchain API. `SIMULATED` means no actual compute has been run. No fees are charged.
- JSON snapshot store relies on a single local filesystem/process; do not use NFS or move it to a shared disk. Production would need a transactional database, migration handling, real quotas/rate limiting, observability, timeouts across downstream operations, secure provisioning, TLS/authenticated reverse proxy, key rotation, and a worker/escrow adapter.
- No HTTP response includes the stored tenant ID, token, token hash or idempotency key.
- For worker GPU later, do not exceed the worker-specific input cap (potentially more conservative than the task contract); validate again before dispatch.

## Checks

```bash
go test -count=1 ./internal/computecloud
go test -race -count=1 ./internal/computecloud
go test -count=1 ./cmd/prism-cloud
go vet ./internal/computecloud ./cmd/prism-cloud
git diff --check
git status --short
```

## Next phase

1. Introduce an explicit broker interface plus a no-op / simulation adapter and integration tests.
2. Move persistent quotas/idempotency from JSON to a transactional database before multi-instance deployment.
3. Bind authorized SaaS tenant to server-owned billing account and escrow, with no client-provided wallet/reward/nonce.
4. Keep proof status separate from GPU receipt: a receipt isn't hardware attestation.
5. Build a minimal UI only after job lifecycle and security invariants pass.
