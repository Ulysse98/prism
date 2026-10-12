# Prism v0.60 — Phase 3: explicit network identity and V1 migration audit

**Scope:** read-only HTTP metadata and tests. No consensus activation, ledger rewrite,
wallet key changes, transaction gossip, or new transfer capabilities in this patch.
Keep PR #44 in Draft.

## Why `/status.chainId` cannot simply be replaced

The existing PoUW workers use `/api/v1/status` `chainId` to construct proof
contexts. The compute, mine, receipts and quantum audit API code still has
several paths based on `p2p.MakeChainID(genesisHash)` (legacy, genesis-only).
On configured networks the P2P handshake and V2 wallet signatures use
`Blockchain.ChainID()` instead, which commits to the chain configuration.
Changing `/status.chainId` alone would risk making fresh proofs incompatible
with their verifiers and historic receipts on configured networks.

**Phase 3 preserves both semantics explicitly:**

| HTTP source | Field | Meaning |
| --- | --- | --- |
| `GET /api/v1/status` | `chainId` | **Legacy** genesis-only proof chain ID; retained for compatibility |
| `GET /api/v1/health` | `chainId` | Same legacy proof chain ID |
| `GET /api/v1/status` | `walletChainId` | **Canonical** configured ID for V2 wallet signing and P2P |
| `GET /api/v1/health` | `walletChainId` | Same canonical configured ID |
| `GET /api/v1/transactions/context` | `chainId` | Authoritative canonical configured ID for V2 wallet signing |
| `GET /api/v1/transactions/context` | `genesisHash` | Genesis committed into every V2 signature |

On an unconfigured legacy chain the two IDs are identical. On a configured
chain they **may differ**. Wallet clients should ALWAYS use
`/api/v1/transactions/context` as the source of truth, validate that its ID
matches `walletChainId`, and sign locally; they must **not** use
`/status.chainId` for a configured network.

## V1 producer migration inventory

| Current code path | V1 producer/use | Migration requirement |
| --- | --- | --- |
| Historical chain replay | V1 blocks validated today | Keep V1 historical hashes and signatures valid after activation |
| `cmd/prism/main.go` | Local CLI `send` | Change new transactions to V2 with the active configured context |
| `cmd/prism/node.go` | P2P CLI / node transaction sending | Upgrade generation to V2 and coordinate peer protocol compatibility |
| `cmd/prism/api_faucet.go` | Testnet faucet | Generate V2 with correct nonce and network context |
| `cmd/prism/api_compute_settlement.go` | Funded PoUW bounty settlement | Generate V2 while preserving historic proof and receipt semantics |
| `internal/p2p/node.go` and mempool | Inbound V1 remains accepted | Require coordinated network activation and peer admission rules |
| `POST /api/v1/transactions` | Already requires V2 | Keep disabled by default / loopback testnet only |

**Important:** V2 protects newly signed V2 transfers against cross-network
replay, but **V1 remains replayable on compatible networks** wherever it is
still admitted by the consensus. This patch does not claim network-wide V1
replay elimination.

## Safe future activation plan (NOT implemented)

1. Enumerate and upgrade every V1 transaction producer. Agree on a protocol
   version and activation height (or network-wide consensus feature gate).
2. Ensure all validators agree on a deterministic rule: allow V1 at historical
   heights below activation; reject V1 at and after activation. Apply the same
   rule in the mempool and received-block validation; account for reorgs.
3. Test mixed old/new nodes, funded compute settlements, faucet, old JSON
   snapshots, chain replay, proof context, receipts and cross-chain anchoring.
4. Stage a separate fork/upgrade review, record rollback and recovery steps,
   and migrate only a disposable testnet first.

## Validation

- New API tests check that legacy/configured fields and V2 context agree on
  both default and configured networks, without breaking old clients.
- A signed V2 transaction for a *different* chain config with the same genesis
  must be rejected, while the correctly signed configured transfer succeeds.
- Run `go test ./... -count=1`, `go vet ./...`, and `git diff --check`.
- Run GitHub Windows/Linux CI and the race detector before review.

**Release posture:** experimental testnet, no valuable funds and no claimed
P2P broadcast/finality or production security audit.
