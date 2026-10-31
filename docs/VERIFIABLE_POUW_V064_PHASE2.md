# PRISM v0.64 — Phase 2A: read-only proof discovery (experimental)

**Status:** prototype API for independently auditing **already-settled** quantized ML PoUW proofs. This does not make verification a prerequisite for settlement or modify consensus/rewards.

## Endpoint

`GET /api/v1/compute/proofs/{jobId}` returns a JSON object with `job`, `proof`, `block`, `blockHash`, `chainId`, and `genesisHash`. Works only for a `VERIFIED` marketplace job with a context-bound PoUW Proof v2 (`ml_inference_quantized`) present in a validated, locally stored blockchain.

- 200: a matching native-verified, on-chain proof.
- 400: malformed identifier.
- 404: no such job.
- 405: non-GET request.
- 409: incomplete settlement, unsupported proof, missing or mismatched evidence.
- 500/503: state or marketplace unavailable.

The node rechecks worker, exact task, work units, proof signature, CPU recomputation and job/chain/genesis context. On-chain discovery is **local inclusion, not global network finality**. The endpoint never exposes wallet private keys, stores no attestations, and accepts no worker-supplied proof or signed report.

## Follow-on Phase 2B

Build a standalone verifier which fetches `/status`, eligible `VERIFIED` jobs, then `/compute/proofs/{jobId}`, pins the expected chain/genesis, independently recomputes `VerifyProof`, and signs `ComputeVerificationReportV064` with an **independent verifier key**. Persist reports separately with duplicate/replay safeguards; do not publish verifier secrets or accept jobs based solely on remote HTTP responses.

**Safety:** use a loopback-only development API and isolated testnet. No network consensus/governance/rewards changes; no real funds. Do not describe node-side verification and signatures as ZK proofs or hardware attestations.

## Tests

```bash
go test ./cmd/prism -run '^TestComputeProof(Snapshot|Endpoint)V064' -count=1
go test ./internal/usefulwork -run '^TestComputeVerificationV064' -count=1
go test ./... -count=1
go vet ./...
git diff --check
```
