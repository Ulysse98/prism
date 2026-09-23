# Prism v0.41 — cross-chain compute receipts

> **Proof of Stake secures. Proof of Useful Work computes.
> Proof of Useful Participation rewards contribution.**

Prism is an experimental blockchain and decentralized compute protocol
written in Go.

It combines three complementary mechanisms:

- **Proof of Stake (PoS)** — secures the network and selects block proposers.
- **Proof of Useful Work (PoUW)** — rewards verifiable computation.
- **Proof of Useful Participation (PoUP)** — rewards meaningful network participation.

## Current status

**Node software version:** Prism v0.41.0

**Status:** Active development

Prism now includes a blockchain node, P2P networking, validators,
wallets, persistent compute jobs, signed PoUW proofs, REST APIs,
mobile integration, and experimental cross-chain integrations.

Prism v0.38 added a durable sequential Python `watch` worker for quantized
ML jobs, persistent settlement receipts and recovery after interrupted
requests. Prism v0.39 hardened compute proofs by binding every marketplace
proof to its job ID, chain ID and genesis hash before settlement.

Prism v0.40 adds funded compute bounties: OPEN and CLAIMED marketplace jobs
are accounted against the requester's available PRISM balance, overcommitted
jobs are rejected, and concurrent job creation is serialized to prevent
double reservation. This funding guard is not yet an on-chain escrow.

Prism v0.41 adds deterministic cross-chain compute receipts for verified
Proof v2 settlements. Each receipt binds the compute Job ID, Proof ID,
worker identity hash and Prism chain identity to a canonical Keccak-256
Registry ID. The same receipt can be anchored and independently verified
on Arbitrum Sepolia and Solana Devnet.

- [Python setup and numerical parity](tools/python/README.md)
- [Signed HTTP worker](tools/python/WORKER.md)
- [v0.38 watch mode and three-job demonstration](tools/python/WATCH.md)
