# Prism v0.60 — Phase 4: testnet producer migration to transaction V2

## What changed

- Faucet payouts now use network-bound transaction V2 signatures. This is not an
  HTTP signing API: the node's existing local faucet wallet signs internally.
- Marketplace PoUW bounties also use transaction V2, independently of the
  PoUW proof's existing network context and cross-chain receipt domain.
- The faucet now accounts for OPEN/CLAIMED compute bounties when its funding
  wallet is also a compute requester. It does not spend those reserved funds.
- Both producers share `signLocalWalletTransferV2`, which derives the configured
  `Blockchain.ChainID()` and genesis hash and validates the locally signed tx.

## Compatibility

- Historical v1 transactions remain valid; previous blocks are not changed or
  re-signed. Duplicate faucet claims remain rejected even if the first claim
  was an old v1 transfer.
- This is **NOT** a global v1 sunset. The command-line `send` path, P2P local
  send paths, and other legacy producers still require a migration plan.
- This code introduces V2 blocks on faucet/compute issuance. **Do not deploy to
  an existing mixed-version P2P network**: all nodes must support V2 consensus
  before new V2 transactions appear, otherwise older nodes reject the chain.
- PoUW compute proofs and cross-chain receipts still use the existing
  genesis-derived proof context, where applicable. Do not silently reinterpret
  historical proof `chainId` as the wallet transaction `chainId`.
- The HTTP wallet submission route stays loopback-only and disabled by default.
  Transaction inclusion remains local; it is not distributed finality.

## What still blocks a public release

1. Audit all remaining `transaction.New` calls in production paths.
2. Decide an explicit activation and compatibility strategy for disabling **new**
   V1 transactions in consensus after older nodes are upgraded.
3. Audit lock/account balance interactions and fee/nonce policies.
4. Implement network transaction gossip and durable propagation semantics.
5. Add secure mobile key custody, backup and signing (not in this phase).

## Tests to run

```powershell
go test ./cmd/prism -run 'Test(Faucet|ComputeSettlement)' -count=1
go test ./... -count=1
go vet ./...
git diff --check
```

Testnet only; never use real funds.
