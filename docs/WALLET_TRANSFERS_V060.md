# Prism v0.60 — experimental local wallet transfers (phase 1)

This branch introduces a **local testnet prototype**. It does not implement a
network transaction gossip protocol or consensus finality. Its transfer POST
endpoint is **off by default** and requires a loopback-bound API.

## Endpoints

`GET /api/v1/transactions?address=<prism_address>&limit=25`

Returns up to 100 recent non-genesis transactions involving the address,
newest first. A local wallet name such as `Alice` is also accepted. Results
include the block height, hash, timestamp, sender, recipient, amount and nonce.
The `locally_included` status means *included in the locally persisted chain*,
not finalized across all Prism peers.

`POST /api/v1/transactions`

Requires the API to run with `-wallet-transfers-enabled` and
`-host 127.0.0.1`. The POST body contains an existing **client-signed**
transaction. Never send a private key or seed to this API. Example shape:

```json
{
  "chainId": "<value from GET /api/v1/status>",
  "transaction": {
    "id": "<calculated transaction hash>",
    "from": "prism_<sender>",
    "to": "prism_<recipient>",
    "amount": 25,
    "nonce": 0,
    "public_key": "<sender Ed25519 public key in hex>",
    "signature": "<Ed25519 signature of the transaction id in hex>"
  }
}
```

Use the existing Go `transaction.New(...)` and `tx.Sign(privateKey)` for a
trusted *local* client. Read the current nonce with `GET /api/v1/wallets/{address}`.
The server validates signature, sender identity, nonce, available balance,
compute job reservations and the requested network identity. It then adds one
transaction to a locally proposed block and persists the result.

```powershell
go run ./cmd/prism api -host 127.0.0.1 -port 8080 -data data/node-7001 -wallet-transfers-enabled
Invoke-RestMethod http://127.0.0.1:8080/api/v1/status
Invoke-RestMethod 'http://127.0.0.1:8080/api/v1/transactions?address=Alice&limit=10'
```

## Security and operational limitations

- **Do not use with valuable funds.** The v1 signature hashes `from|to|amount|nonce|public_key`
  and is **not bound to chain ID or genesis**. Request `chainId` checks prevent accidental
  routing mistakes, **not cryptographic cross-chain replay**. A properly domain-separated,
  consensus-validated transaction v2 requires a separate protocol migration.
- There is no persistent network-wide mempool and this endpoint does not
  broadcast transactions to peers. It proposes and stores **a local block**.
- A local block is not proof of global finality. Concurrent writers outside
  the API's runtime lock, crashes during state file writes and divergent nodes
  need additional hardening before public operation.
- The endpoint is disabled by default, only accepts loopback when enabled,
  and has a request body cap. No public-internet exposure is recommended.
- The API must run with a compute marketplace; reserved OPEN/CLAIMED compute
  rewards cannot be spent by this endpoint. The existing faucet and other
  transaction producers require their own reservation review.
- No wallet key import/export, mobile signing SDK, QR codes or encrypted key
  vault is added in this phase. Never commit `wallets.json` or secret keys.

## Validation

Run `go test ./... -count=1` and `go vet ./...`, and verify
`git diff --check`. Integration tests cover external wallet ownership,
invalid signatures, nonce replay, chain ID mismatch, reserved compute bounty,
strict body parsing, opt-in and concurrent requests.

## Next phase

Introduce domain-separated transaction v2 signing bound to the canonical
chain ID / genesis with backwards compatibility analysis; then add a durable
submission path and a Prism Mobile secure keystore / signing flow.
