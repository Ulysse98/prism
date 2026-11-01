# Prism Compute Cloud — Phase 1C.2 / Safe compute preflight

## Purpose

Adds an authenticated, **non-dispatching** compute preflight endpoint to the local Prism Cloud gateway.
This is not a live compute broker. It does not submit marketplace jobs, reserve rewards, sign transactions, run GPUs, or issue PoUW receipts.

## API

`POST /cloud/v1/compute/plan`

Authorization: same `Bearer` key as Phase 1B. Content-Type: `application/json`.
Body: same Phase 1A `SubmitV1` contract, strictly capped to 64 KiB and quantized 3x3 / batch size 1..1024.

The handler:

1. Authenticates the SaaS client **before** processing the body or contacting Prism Core.
2. Requires POST, rejects query strings and non-JSON media types.
3. Validates client inputs using `ParseSubmitV1` and derives canonical `task_id` / `input_hash` on the server.
4. Requires the already-configured, pinned loopback node observer. It makes at most one `GET /api/v1/status` and validates chain identity and health.
5. Returns a `dry_run` response. **No job is recorded and no GPU or escrow operation is possible through this endpoint.**

Example response:

```json
{
  "mode": "dry_run",
  "task_id": "<server-derived>",
  "input_hash": "<server-derived>",
  "workload": "ml_inference_quantized",
  "chain_id": "<pinned-node-chain-id>",
  "node_height": 0,
  "dispatch_enabled": false,
  "compute_enabled": false,
  "escrow_enabled": false,
  "proofs_available": false
}
```

All existing `/cloud/v1/jobs`, `/cloud/v1/usage`, `/cloud/v1/node`, and `/cloud/v1/capabilities` handlers remain delegated unchanged. Existing simulated jobs and usage persist.

## Fail-closed behavior

- 401 for missing, invalid, or duplicate bearer credentials.
- 405 for unsupported method, 400 for unsupported query or malformed/forbidden workload.
- 413 oversized payload; 415 non-JSON payload.
- 503 when no node observer is configured.
- 502 when the pinned upstream node is unavailable, unhealthy, or a different chain.

The Cloud API is restricted to `127.0.0.1:18066`. Never expose this development setup publicly.

## Not included

No true worker discovery, GPU resource guarantee, proof correctness guarantee, pricing, wallet funding, signature, escrow, dispatch, job processing, billing, or production assurance. A `dry_run` plan is advisory and never an authorization to spend or promise of capacity.

## Local checks

```bash
go test -count=1 ./internal/computecloud ./cmd/prism-cloud ./internal/compute ./internal/usefulwork
go test -race -count=1 ./internal/computecloud ./cmd/prism-cloud
go vet ./internal/computecloud ./cmd/prism-cloud
go build ./cmd/prism-cloud
```

No commit or push is performed by the installer. Review exact additions and patch before committing.
