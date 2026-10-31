# PRISM v0.65 — Phase 2B.2 escrow lifecycle replay (isolated testnet)

This experimental patch adds **standalone** consensus-committed `CLAIM`, `RELEASE`, and `REFUND` blocks alongside the existing v0.65 task-bound `FUND` operation.

- `FUND`: signed requester funding, locks spendable balance without moving or creating tokens.
- `CLAIM`: verifies worker's signed job/network claim and marks the funded escrow claimed.
- `RELEASE`: independently verifies the signed task-bound PoUW proof, removes the requester lock, **debits requester balance**, and **credits the verified worker** in the same canonical `GetState` replay operation. There is no second requester signature.
- `REFUND`: after the deadline (block height), any proposer may include a refund, but the only effect is to remove the lock of the **original** requester. The gross requester balance is unchanged.

`GetEscrowLockedBalancesV065` and `GetEscrowRecordV065` reconstruct state from blocks, not SQLite. The new operations use the preexisting v0.65 escrow hash domain; legacy blocks retain their prior hash functions. A standalone block can contain one escrow operation only, not ordinary transfers, PoUW rewards, or governance actions. Release proofs do not receive protocol PoUW emission by inclusion in an escrow block.

**Not ready for shared networks or funds of value.** No activation height or hard-fork coordination has been specified. The old v0.65 unbound funding prototype is incompatible with task-bound funding. No wallet/marketplace API, mempool transaction type, P2P broadcast path, rollback/migration procedure, checkpoint audit, resource limits, or external security audit is included. Do not run against existing production data or conflate an API marketplace reservation with native escrow locks. Keep the isolated testnet disposable.

Tests cover honest fund→claim→release, expiry refund, forged proof rejection, replay/double release/refund, proposer-only reward accounting, supply conservation outside normal rewards, and hash/proof tamper rejection. Run full `go test ./...`, `go vet ./...`, race tests and peer/restart tests before deploying to any multi-node network. Post-release wallet accounting and an independent replay audit remain necessary next steps.
