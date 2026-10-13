# Prism v0.61.0 — Release notes and publication gate

**Status:** release candidate preparation; not yet published. Scope is the Prism Go node/governance network. Prism Mobile is a separate repository/build and should receive its own verified release process.

## Highlights

- Local demonstration of timelocked, 2-of-2 approved Team-reserve governance.
- On-chain execution of a 5,000 PRISM Team transfer at canonical block height 12.
- One exact governance block accepted by three keyless, local P2P observer replicas.
- Safe canonical adoption with an external backup and byte-for-byte preservation of validator wallet files.
- Canonical `GET /api/v1/status` and `GET /api/v1/wallets/{address}` confirmed a valid chain and the 5,000 PRISM recipient balance.
- The Prism Mobile development client displayed the exact credited address and balances on a Pixel 8a using a local USB ADB HTTP bridge.

This is a **development network milestone**, not a public token distribution, mainnet launch, or claim of independent cross-organization finality.

## Release gate

- [ ] Review `git status --short` and every relevant `git diff`, including changes in local `cmd/prism-v061-*` helper packages.
- [ ] Stop the canonical API and other canonical writers before running tests.
- [ ] Run `01-Preflight.ps1` and retain the full test output: `go test ./... -count=1`, `go vet ./...`, and `git diff --check` must pass.
- [ ] Validate negative-path governance tests: approvals, timelock, replay, chain identity, spend/balance accounting. If any coverage is missing, add tests first.
- [ ] Install and review `docs/GOVERNANCE_V061.md` and this release document.
- [ ] Change `prismVersion` from `0.59.0` to `0.61.0` (review any other active version values separately; do not alter network `chainId`, genesis, protocol versions without an explicit compatibility decision).
- [ ] Re-run full Go tests, Go vet and diff checks **after** version change.
- [ ] Inspect existing GitHub Actions on the PR (Windows/Linux; Linux race test). Do not declare release ready until required checks are green.
- [ ] Confirm no private keys, wallet databases, canonical node data, backups, faucet secrets, or local `.env` files are staged for commit.
- [ ] Build the Go binary and check `prism api` startup against a **copy of state**, not by recreating or resetting the canonical network.
- [ ] Push the reviewed feature branch; open a PR against `master`; review; merge only when tested.
- [ ] Create annotated `v0.61.0` tag on the merged commit (not on unmerged feature branch); push only after explicit review. Publish GitHub Release notes separately.

## Suggested final validation commands (run locally, after reviewing source)

```powershell
go test ./... -count=1
go vet ./...
git diff --check
git diff --cached --check
git status --short
git diff -- cmd/prism/main.go
```

Release tags and pushes are intentionally **not** automated by this kit.

## Compatibility / limitations

- The API previously reported `version: 0.59.0` despite the new governance behavior; this must be corrected prior to release.
- A static UI balance or testnet faucet action is not proof of a transfer. The successful proof used consensus state, the canonical API and the matching active local Ed25519 mobile address.
- The Pixel 8a app required a dev-client/Metro rebuild and `adb reverse tcp:8080 tcp:8081`, which are **local development conveniences**, not general production distribution paths.
- An unrelated Android PoUW miner error around `ArrayBuffer`/Kotlin was observed and remains outside this Go-node v0.61 release unless its scope is explicitly expanded.
