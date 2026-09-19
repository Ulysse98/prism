# Prism v0.37 — v0.38 worker development

> **Proof of Stake secures. Proof of Useful Work computes.
> Proof of Useful Participation rewards contribution.**

Prism is an experimental blockchain and decentralized compute protocol
written in Go.

It combines three complementary mechanisms:

- **Proof of Stake (PoS)** — secures the network and selects block proposers.
- **Proof of Useful Work (PoUW)** — rewards verifiable computation.
- **Proof of Useful Participation (PoUP)** — rewards meaningful network participation.

## Current status

**Node software version:** Prism v0.37.0

**Status:** Active development

Prism now includes a blockchain node, P2P networking, validators,
wallets, persistent compute jobs, signed PoUW proofs, REST APIs,
mobile integration, and experimental cross-chain integrations.

The v0.38 development work adds a sequential Python `watch` worker for
quantized ML jobs, persistent settlement receipts and recovery after an
interrupted request. The Go node and proof format remain those of v0.37.0
while the worker is validated on a running node.

- [Python setup and numerical parity](tools/python/README.md)
- [Signed HTTP worker](tools/python/WORKER.md)
- [v0.38 watch mode and three-job demonstration](tools/python/WATCH.md)
