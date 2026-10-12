# Prism v0.60 — Phase 6: live P2P V2 admission policy

## Change (isolated experimental testnet only)

- Outbound `Server.SendTransaction` refuses all new V1 P2P transactions before network dialing.
- Inbound `Server.acceptTransaction` refuses V1 before mempool insertion and gossip.
- V2 submissions continue through the signature, configured network-context, nonce and balance checks.
- Historical V1 chain blocks remain valid, including received historic blocks and existing ledger state.

## Important NON-guarantees

- **Not a consensus V1 sunset.** The chain still accepts valid V1 in block validation; a malicious, legacy or custom producer could include V1 in a fresh block. Only live P2P submission is V2-only.
- **No automatic network upgrade**. The P2P protocol identifier and consensus activation policy are unchanged. This branch must not run against mixed-version peers; plan coordinated deployment and a formal activation height if consensus V1 retirement is desired.
- V2 local API inclusion is NOT gossip, multi-node finality or irreversible settlement.
- `wallets.json` stores private keys as plaintext development data (written with mode 0600 on POSIX; this is not secure Android/Windows hardware-backed custody). Never use real funds and never upload wallet files to GitHub.
- Existing PoUW receipt/proof chainId derives from genesis on legacy flows; wallet ChainID may include configuration. Do not silently unify these contexts.

## Local validation

```powershell
go test ./internal/p2p -run '^TestP2PV060' -count=1
go test ./... -count=1
go vet ./...
git diff --check
```

## Release gate / later engineering work

1. Decide and test V1 consensus activation height on an isolated fresh testnet, with a coordinated upgrade or migration path for all participants.
2. Audit remaining hand-built P2P and local producer paths; verify mempool nonce/balance semantics during concurrent submissions and replay attempts.
3. Implement encrypted/hardware-backed mobile key custody, backup/recovery flows and secure signing. Do NOT reuse testnet development private keys.
4. Implement transaction broadcast/confirmation semantics and communicate local-vs-network inclusion clearly in the UI.
5. Complete security review before merging Draft PR #44 or advertising a secure wallet.
