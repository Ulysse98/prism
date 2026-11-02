# Prism — Experimental Blockchain & Verifiable Compute

> **Proof of Stake secures. Proof of Useful Work computes.
> Proof of Useful Participation rewards contribution.**

Prism is an experimental blockchain and decentralized compute protocol
written in Go.

It combines three complementary mechanisms:

- **Proof of Stake (PoS)** — secures the network and selects block proposers.
- **Proof of Useful Work (PoUW)** — rewards verifiable computation.
- **Proof of Useful Participation (PoUP)** — rewards meaningful network participation.

## Current status

| Component | Status |
| --- | --- |
| Latest published GitHub release | [v0.63.0 — Persistent GPU PoUW Worker](https://github.com/Ulysse98/prism/releases/tag/v0.63.0) |
| Node-reported software version on `master` | `0.63.0` (see `cmd/prism/main.go`) |
| Development in `master` | Experimental work spanning v0.64–v0.67; **v0.67.0 is not yet a published release** |
| Production readiness | **Research / testnet only.** Not audited for production funds, public cloud services or live cross-chain settlement. |

Prism includes a Go blockchain node, P2P networking, validators, wallets, persistent compute jobs, signed Proof v2 receipts, REST APIs, experimental mobile integration and cross-chain research. Some newer components are isolated laboratories or opt-in code paths; their presence in `master` does not mean they are activated on a live network.

## Recent milestones

### v0.60–v0.63 — Wallet V2 and persistent GPU workers

- **v0.60–v0.61:** Network-bound Wallet V2 signing and governance bootstrap experiments.
- **v0.62:** Experimental GPU Proof of Useful Work for bounded, quantized ML workloads, with CPU/GPU parity tests.
- **v0.63:** Opt-in persistent CUDA worker sessions, bounded task processing, signed receipts, replay rejection and failure recovery. CPU remains the default backend.

### v0.64 — Independent PoUW verification (experimental)

A separate verifier can read local verified jobs and compute proofs, validate their chain context, recompute supported quantized ML outputs on CPU and sign an off-chain Ed25519 verification report. Reports are **not** consensus votes, hardware attestations, zero-knowledge proofs or independent authorization to pay rewards.

See [verifiable PoUW design](docs/VERIFIABLE_POUW_V064.md) and [read-only verifier](docs/VERIFIABLE_POUW_V064_PHASE2B.md).

### v0.65 — Escrow and finality research (experimental)

A task-bound escrow lifecycle supports `FUND`, `CLAIM`, `RELEASE` and `REFUND` in isolated consensus replay tests. Separate validator-vote and signer-safety laboratories explore finality-related protections; they are **not** an activated BFT finality protocol.

**Do not use this escrow prototype with production funds or existing production chain data.**

See [escrow lifecycle](docs/ESCROW_V065_PHASE2B2_CONSENSUS.md) and [finality lab scope](docs/POS_FINALITY_V065_LAB_README.md).

### v0.66 — Approved compute and cooperative GPU locking (experimental)

Linux/amd64 GPU experiments cover locally approved executable hashes, sealed worker execution, independently recomputed CPU outputs and advisory GPU-lock coordination between cooperating Prism Core and Prism Cloud processes. These checks do **not** attest the physical GPU, sandbox arbitrary subprocesses, prove binary provenance or authorize on-chain payouts.

See [independent verifier qualification](docs/POUW_V066_PHASE1E1_INDEPENDENT_VERIFIER.md) and [real-CUDA shared-lock lab](docs/POUW_V066_PHASE1E2D_SHARED_LOCK_REAL_CUDA.md).

### v0.67 — Cross-chain admission hardening (experimental)

Local offline admission experiments bind anchor intents and proofs to a specific Prism chain and checkpoint; use signed operator policies, pinned Ed25519 trust roots, synchronized local snapshots and local anti-replay storage. The privileged admission constructor is Linux-specific. Unsupported Windows replay storage fails closed.

This **does not** activate cross-chain registry transactions, global replay protection, network finality, escrow release or production authorization.

See [privileged admission boundary](docs/CROSSCHAIN_V067_PHASE1C5G.md) and [Windows/Linux replay contract](docs/CROSSCHAIN_V067_WINDOWS_CI.md).

### Prism Compute Cloud — Local MVP research

`cmd/prism-cloud` is a **loopback-only, authenticated simulation gateway**, not a deployed SaaS. It supports bounded task planning, simulated per-tenant operations and read-only node observation. Separate offline GPU broker/scheduler labs exist, but **the Cloud HTTP gateway does not dispatch CUDA jobs, sign wallets, settle escrow, bill customers or submit live marketplace jobs**.

See [Cloud MVP scope](docs/COMPUTE_CLOUD_MVP_PHASE1.md) and [merge-readiness/security limits](docs/COMPUTE_CLOUD_V066_MERGE_READINESS.md).

## Development checks

Use the Go version declared by [`go.mod`](go.mod) (currently Go 1.27.0). From the repository root:

```sh
go test ./... -count=1
go vet ./...
git diff --check
```

On Linux, for an additional race-detector check of the v0.67 cross-chain package:

```sh
go test -race -count=1 ./internal/crosschain
```

Hardware-dependent CUDA tests require an explicitly configured local NVIDIA/CUDA environment; standard CI passing does not independently establish hardware qualification or a production security audit. See [GitHub Actions](https://github.com/Ulysse98/prism/actions).

## Earlier development history

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

## Worker documentation

- [Python setup and numerical parity](tools/python/README.md)
- [Signed HTTP worker](tools/python/WORKER.md)
- [v0.38 watch mode and three-job demonstration](tools/python/WATCH.md)
