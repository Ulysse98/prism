# Prism v0.55 — Monad cross-chain settlement

> **Proof of Stake secures. Proof of Useful Work computes.
> Proof of Useful Participation rewards contribution.**

Prism is an experimental blockchain and decentralized compute protocol
written in Go.

It combines three complementary mechanisms:

- **Proof of Stake (PoS)** — secures the network and selects block proposers.
- **Proof of Useful Work (PoUW)** — rewards verifiable computation.
- **Proof of Useful Participation (PoUP)** — rewards meaningful network participation.

## Current status

**Node software version:** Prism v0.55.0

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

Prism v0.53 introduced experimental **Quantum PoUW** through the
`quantum_simulation` task type. The first workload is a two-qubit Bell
circuit with bounded shot counts. Execution can use the deterministic
built-in backend or an external NVIDIA CUDA-Q backend.

Prism v0.54 connects Quantum PoUW to the complete compute-marketplace
lifecycle. Quantum jobs can be funded, discovered and claimed by an
autonomous worker, executed through CUDA-Q, committed as context-bound
Proof v2 proofs, settled on the Prism chain, rewarded in PRISM and exposed
through canonical cross-chain receipts.

The v0.54 Explorer supports the current `{count, jobs}` response,
Quantum Bell job creation, Proof IDs and verified receipt context.

Prism v0.55 adds **Monad Testnet** as an EVM cross-chain settlement
target for canonical Proof v2 receipts. The Monad integration validates
recorder authorization and deterministic Registry IDs, waits for
transaction confirmation, and publishes the confirmed settlement back
to the Prism receipt API.

The PrismProofRegistry deployment on Monad Testnet uses chain ID `10143`
and address `0xc013BDeb0E20F73d613ed95FD3902802d73626d6`.

The first v0.55 end-to-end Quantum PoUW settlement anchored Registry ID
`0xf367a0f5f7f480f347041dda02da926dd970bef456fc679d06e60ced901fade8`
in Monad Testnet block `68444327` and returned a verified Monad anchor
through `/api/v1/receipts/{jobId}`.

- [Python setup and numerical parity](tools/python/README.md)
- [Signed HTTP worker](tools/python/WORKER.md)
- [v0.38 watch mode and three-job demonstration](tools/python/WATCH.md)
