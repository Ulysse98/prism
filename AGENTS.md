\# AGENTS.md



\## Project mission



Prism is an experimental blockchain protocol written in Go combining:



\- Proof of Stake (PoS) for network security

\- Proof of Useful Work (PoUW) for deterministic verifiable computation

\- Proof of Useful Participation (PoUP) for contribution scoring



Preserve existing protocol behavior unless a task explicitly requires a protocol change.



\## Repository map



\- `cmd/prism/main.go` — CLI and local-chain commands

\- `cmd/prism/node.go` — node/P2P commands

\- `internal/blockchain/` — blocks, balances, validation, rewards

\- `internal/consensus/` — PoS and proposer selection

\- `internal/mempool/` — transaction admission

\- `internal/transaction/` — signed transactions and nonces

\- `internal/wallet/` — wallets and addresses

\- `internal/usefulwork/` — PoUW execution and verification

\- `internal/participation/` — PoUP scoring

\- `internal/storage/` — persistence

\- `internal/p2p/` — networking and chain synchronization

\- `docs/` — website

\- `RUNBOOK.md` — reproducible demo workflow



\## Commands



```powershell

go run ./cmd/prism status

go run ./cmd/prism wallets

go run ./cmd/prism balance Alice

go run ./cmd/prism validators

go run ./cmd/prism participation

go run ./cmd/prism send Alice Bob 25

go run ./cmd/prism work Charlie 3 4 5

go run ./cmd/prism node --port 7001

go run ./cmd/prism node --port 7002 --peer 127.0.0.1:7001

go run ./cmd/prism node-produce --port 7001

