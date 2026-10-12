# Prism v0.60 — Phase 5 security review and CLI/P2P migration

## Migration in this patch

- `prism send` (local CLI), `node-send` and `node-produce` sign V2 transfers using the
  existing `signLocalWalletTransferV2` helper. The signing payload commits to
  the canonical configured `Blockchain.ChainID()`, genesis hash and transfer fields.
- Outbound P2P `Server.SendTransaction` rejects a validly signed V2 transaction
  from a foreign network **before opening a TCP connection**. This is defense
  in depth; the receiving mempool and consensus replay still validate context.
- Historical V1 blocks and signatures are **not rewritten**. Old V1 traffic is
  **not** globally disabled: consensus and incoming P2P still support V1.

## Outstanding security blockers — do not claim production readiness

1. **Plaintext local keys**: `internal/storage/storage.go` writes Ed25519 private
   key hex into `data/**/wallets.json` (file mode `0600` on Unix). This is not
   encrypted key custody, cannot substitute for Android Keystore or secure
   enclave, and file mode alone is not a comprehensive Windows ACL policy.
   Keep real funds out of these development wallets; do not publish, sync or
   upload private wallet files, and restrict access/back up testnet secrets.
2. **V1 replay**: older signed V1 transactions have no cryptographic network
   identity. Compatibility acceptance is still present at the consensus and
   P2P entry points; a network-wide sunset needs a height-based activation
   policy and coordinated node upgrades, not an API-only change.
3. **P2P protocol negotiation**: older peers sharing `ProtocolVersion` may not
   understand V2 signatures. Upgrade all nodes on an isolated testnet before
   producing V2 blocks. Plan explicit version/capability negotiation for a
   public rollout.
4. **No distributed finality for HTTP wallet POST**: the current API persists
   local blocks and returns `locally_included`. This does not prove propagation,
   quorum consensus, finality, or irreversible inclusion.
5. **PoUW network context**: compute proofs and cross-chain receipts retain the
   older genesis-derived identity; do not replace their `chainId` with the V2
   wallet identity without an explicit protocol migration and test vectors.
6. **Nonce, persistence and concurrency**: review replay across restarts,
   reservation accounting, node P2P races, crash-recovery, anti-spam fees and
   invalid-payload limits before a mainnet wallet release.

## Test plan

Run in a clean checkout on Go 1.27 (this patch is *not* validated on a remote
node yet):

```powershell
go test ./cmd/prism -run '^TestWalletV060CLISendProducesV2$' -count=1
go test ./internal/p2p -run '^TestP2PV060OutgoingForeignNetworkRejectedBeforeDial$' -count=1
go test ./... -count=1
go vet ./...
go test -race ./internal/p2p ./internal/blockchain ./internal/mempool ./cmd/prism -count=1
```

The race test needs a platform supporting Go's race runtime; CI runs Ubuntu.

## Deploy policy

Isolated testnet only. Do not merge PR #44 until reviews cover the blockers.
No hard fork, API broadcast mechanism, key encryption or mobile signing is
introduced by this patch.
