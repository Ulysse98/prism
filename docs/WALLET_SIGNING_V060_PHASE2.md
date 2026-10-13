# Prism v0.60 — Wallet signing V2 (phase 2, experimental)

## What changes

Wallet transfers submitted via `POST /api/v1/transactions` now require a **V2**
Ed25519 signature. The client signs a transaction ID derived from an explicit
`prism-transaction-v2` domain and these fields:

`chain_id | genesis_hash | from | to | amount | nonce | public_key`

The format also includes the `version` field (value **2**). Field syntax is
restricted to canonical lowercase hexadecimal addresses and network hashes,
which prevents ambiguity in the delimited payload. Never transmit a seed or
private key to the node: signatures are generated **client-side**.

`GET /api/v1/transactions/context` (read-only) returns:

```json
{
  "chainId": "prism-0123456789abcdef",
  "genesisHash": "<64 lowercase hex characters>",
  "transactionVersion": 2
}
```

The chain ID above is a **format example**, not a real testnet identifier.
The signing context uses `Blockchain.ChainID()` (including chain configuration),
which is authoritative for V2, even if the legacy `/api/v1/status` chain ID
for a configured chain differs. Before exposing public-wallet signing, unify
all API/P2P chain ID displays and their expectations.

Example body shape, **not signed or ready for submission**:

```json
{
  "chainId": "<from GET /api/v1/transactions/context>",
  "transaction": {
    "id": "<SHA256 domain-separated transaction ID>",
    "from": "prism_<40 lowercase hex chars>",
    "to": "prism_<40 lowercase hex chars>",
    "amount": 25,
    "nonce": 0,
    "public_key": "<sender Ed25519 public key hex>",
    "signature": "<sender Ed25519 signature over transaction ID hex>",
    "version": 2,
    "chain_id": "<context chainId>",
    "genesis_hash": "<context genesisHash>"
  }
}
```

In Go, construct with `transaction.NewV2(...)` and sign using
`tx.Sign(privateKey)` **locally**. The node verifies the V2 signature, network
context and nonce, available balance, plus compute bounty reservations.
Consensus replay (`Blockchain.GetState`) and the mempool both check V2 network
identity, not only the API request's `chainId` field.

## Compatibility and limitations

- Legacy transactions (implicit version 0, V1) keep their **identical hash and
  JSON serialization**, so historical blocks remain verifiable. V1 is still
  accepted by the underlying consensus and older transaction producers such as
  the faucet and compute settlement; **global replay resistance for V1 is NOT
  achieved**. Disabling new V1 transactions would require an explicit,
  coordinated consensus activation rule and migrations for all producers.
- Wallet HTTP POST rejects new V1 submissions; wallet HTTP POST is **disabled
  by default** and **loopback-only** when explicitly enabled.
- The API persists **local blocks** only. No transaction broadcast,
  network-wide mempool, distributed finality, key vault or mobile wallet signing
  is included. **Use only on isolated development networks; never real funds.**
- Network upgrades require homogeneous protocol support before running V2
  transactions: older nodes will not recognize the V2 signature domain.

## Validation before review

On Windows:

```powershell
go test ./... -count=1
go vet ./...
git diff --check
```

Check dedicated v2 unit tests, cross-network mempool tests, existing historical
chain fixtures, legacy faucet settlement flows, and concurrent wallet tests.
Only mark the PR ready for review when **both** GitHub OS workflows and race
checks pass.
