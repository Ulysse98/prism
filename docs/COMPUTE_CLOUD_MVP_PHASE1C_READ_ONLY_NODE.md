# Prism Compute Cloud MVP — Phase 1C.1: Read-only node observer

Base: branch `feat/compute-cloud-mvp-phase1`, commit `72fd7c9` (after Phase 1B).

**Design invariant:** no SaaS request can create, claim, complete, settle, or fund a Prism job. The only new upstream operation is `GET http://127.0.0.1:<port>/api/v1/status`.

## Files

- `internal/computecloud/node_observer_v1.go`: status-only upstream observer with identity pinning.
- `internal/computecloud/observe_gateway_v1.go`: opt-in read-only HTTP routes, same Bearer authentication as Phase 1B.
- `*_test.go` for tests and `cmd/prism-cloud/main_test.go` for opt-in configuration.
- A narrow patch to `cmd/prism-cloud/main.go` only, leaving every earlier route unchanged.

## New authenticated endpoints

- `GET /cloud/v1/capabilities`: static, explicit workload contract (`ml_inference_quantized`, 3x3, max_batch 1024). Returns `compute_enabled:false`, `proofs_available:false` and `mode:"simulation"` even if a node is reachable.
- `GET /cloud/v1/node`: returns an allowlisted subset (`mode`, `node_version`, `chain_id`, `height`, `chain_valid`, flags showing no compute/proof). With no observer config, returns HTTP 503 `node_probe_disabled`. Unreachable upstream returns 502. A chain ID or genesis mismatch returns 502, never the mismatched values.

No API keys, wallet identifiers, last hashes, full node replies, genesis hash or upstream errors are returned. No SaaS bearer header is forwarded to the node.

## Hardening

- Config is opt-in: all three variables are required if any is present:
  - `PRISM_CLOUD_NODE_PORT` (1..65535)
  - `PRISM_CLOUD_NODE_CHAIN_ID` (expected network identity)
  - `PRISM_CLOUD_NODE_GENESIS_HASH` (exact 64-character lowercase hex)
- Fixed upstream scheme, host and path. Port-only choice. Redirects disabled. Proxy bypassed. Dialer rejects non-loopback or alternate destinations.
- 1.5-second downstream timeout and maximum 8 KiB JSON response; status validation checks canonical identity and `chainValid=true`.
- Observer cannot invoke `/api/v1/compute/jobs`, wallet API, escrow, work submissions, GPU execution or proof synthesis.
- The observer's successful status is *not* evidence of an available GPU, successful execution, valid proofs or production readiness.

## Running without a Prism node (safe default)

```bash
export PATH="/usr/local/go/bin:$PATH"
cd "$HOME/prism-lab/prism-cloud-mvp-phase1"
# In a terminal where the Phase 1B server has been stopped:
go run ./cmd/prism-cloud serve
```

Then in a separate terminal:

```bash
read -r -s -p 'Cloud API key: ' PRISM_CLOUD_TOKEN; echo
curl --fail-with-body -sS -H "Authorization: Bearer $PRISM_CLOUD_TOKEN" \
  http://127.0.0.1:18066/cloud/v1/capabilities | python3 -m json.tool
# Without configured node: /cloud/v1/node returns 503 (by design).
unset PRISM_CLOUD_TOKEN
```

## Optional: observe an isolated Prism node

Start a **separate isolated Prism testnet** by your existing lab procedure; do not point this at your valuable/mainnet node. The example lab port 18064 is not assumed to be running. Check its `/api/v1/status` before trusting it:

```bash
curl --max-time 2 --fail-with-body -sS http://127.0.0.1:18064/api/v1/status | python3 -m json.tool
```

Using independently verified chain ID and genesis hash values from your isolated lab, set all three variables **in the terminal running the cloud gateway** and restart it:

```bash
export PRISM_CLOUD_NODE_PORT=18064
export PRISM_CLOUD_NODE_CHAIN_ID='prism-YOUR-ISOLATED-CHAIN-ID'
export PRISM_CLOUD_NODE_GENESIS_HASH='YOUR-64-CHARACTER-LOWERCASE-GENESIS-HEX'
go run ./cmd/prism-cloud serve
```

The example identity placeholders must be replaced. Do not automatically trust identity values returned by an unknown process occupying port 18064. You should observe `GET /cloud/v1/node` returning 200 only with the expected identity and healthy status. Otherwise, failure is expected, not a reason to weaken pinning.

## Tests

```bash
go test -count=1 ./internal/computecloud ./cmd/prism-cloud
go test -race -count=1 ./internal/computecloud ./cmd/prism-cloud
go vet ./internal/computecloud ./cmd/prism-cloud
git diff --check
```

Tests use local `httptest` servers. They check: auth before network access; no bearer forwarding; status allowlist; redirects and non-loopback rejection; invalid config; timeouts / large replies; wrong identity; unhealthy node; disabled default; no regression of earlier gateway routes.

## Next Phase 1C.2

Introduce a separate, server-owned broker interface with a **no-op adapter by default**. Only after explicit authorization, task restrictions and budget enforcement are audited should a future adapter submit to a genuinely isolated Prism marketplace. Keep receipt vs PoUW proof vs hardware attestation distinct.
