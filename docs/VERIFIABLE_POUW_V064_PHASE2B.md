# PRISM v0.64 — Phase 2B: local autonomous verifier (experimental)

This standalone Go process **never** submits a settlement transaction, modifies
chain state, calls a POST route, broadcasts proofs, or distributes rewards.
It reads only the loopback PRISM API and a **separately chosen isolated-testnet**
local chain/wallet directory. Do **not** use production keys or real funds.

## Behavior

- Fetch local node status and filtered `VERIFIED` quantized ML jobs.
- Fetch `GET /api/v1/compute/proofs/{jobId}` for each candidate.
- Load and validate the **entire local chain** via `storage.Load` on each scan.
- Match proof identity, full task, input, worker, signed proof context, proof ID,
  proof contents, and block hash/height against the locally validated chain.
- Independently recompute the entire signed quantized ML prediction batch in Go.
- Sign an off-chain `ComputeVerificationReportV064` using a **distinct**
  verifier wallet, check the attestation, then print one JSON line per report.
- Optional `-poll 5s` repeats scanning, suppressing duplicate reports within
  the running process. No durable cursor/report store yet.
- Fail closed on malformed, missing, inconsistent, or unavailable data.
  Responses are bounded to 2 MiB and redirected HTTP requests are rejected.

## Run (isolated local testnet only)

From the repository root, **with a separate, isolated testnet data directory**
containing the node's fully validated blockchain and development wallets:

```bash
# Compile without contacting any remote chain.
go build -o /tmp/prism-pouw-verifier-v064 ./cmd/prism-pouw-verifier

# One read-only scan. 'Charlie' is only an example wallet name; choose an
# independent verifier address present in your isolated testnet wallets.
/tmp/prism-pouw-verifier-v064 \
  -api http://127.0.0.1:8080/api/v1 \
  -data "$HOME/prism-lab/isolated-v064-testnet" \
  -verifier Charlie

# Optional, once you have checked the single-scan behavior:
# ... -poll 5s
```

If there are no *settled verified* quantized ML jobs, the verifier prints no
report and exits normally. This does not mean the code failed.

## Security boundaries

This is **not an independent full-node consensus implementation**. The
verifier trusts the local `storage.Load` validation code and local node data.
Shared filesystem/private-key access does **not** provide operator
independence, and distinct wallet addresses are not sybil resistance.
The API is restricted to an explicitly provided loopback HTTP endpoint.
Verification reports are **not** incorporated into consensus or settlement,
and no blockchain protocol/hash or emission policy is changed.

The development `wallets.json` holds raw private keys. Never copy it to
source control or send it to a public service. Use an isolated testnet wallet,
not a real wallet. The standalone process prints no private key material.

## Gate

```bash
go test ./cmd/prism-pouw-verifier -count=1 -v
go test ./... -count=1
go vet ./...
git diff --check
```

Phase 2C can add a persisted verifier cursor and independently managed
signer key before any broader deployment. Do not merge without review.
