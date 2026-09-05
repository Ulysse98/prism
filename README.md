# Prism v0.17

> **Proof of Stake secures. Proof of Useful Work computes. Proof of Useful Participation rewards contribution.**

Prism is an experimental blockchain protocol written in Go for **ETHOnline 2026**.

The v0.17 prototype explores a hybrid architecture where three different signals have different jobs:

- **Proof of Stake (PoS)** selects block proposers and provides the economic security layer.
- **Proof of Useful Work (PoUW)** verifies deterministic computation and rewards useful work.
- **Proof of Useful Participation (PoUP)** measures meaningful contribution and is gated by a human-uniqueness attestation in the current prototype.

Prism v0.17 also runs as a persistent **three-node Docker devnet** with P2P handshakes, Chain IDs, periodic synchronization, fork checks, full remote-chain validation, and automatic catch-up after a node has been offline.

## Why Prism?

Most blockchains concentrate incentives around capital or raw computation.

Prism asks a broader question:

> Can one network reward capital, useful computation, and meaningful human participation without treating them as the same thing?

The protocol separates those roles instead of forcing one consensus mechanism to represent all of them.

## What the ETHOnline prototype demonstrates

### 1. Proof of Stake

Validators lock PRISM and participate in deterministic proposer selection.

The prototype includes:

- validator registration
- locked stake
- deterministic proposer selection
- proposer validation
- block rewards

### 2. Proof of Useful Work

Workers execute deterministic tasks that can be verified before inclusion in a block.

The current PoUW primitive is a **sum-of-squares task**. It is intentionally simple: the goal of v0.17 is to demonstrate the protocol path from task execution to proof verification, block inclusion, scoring, and reward.

### 3. Humanity-gated Proof of Useful Participation

Prism records participation signals such as:

- blocks proposed
- useful work completed
- proposer score
- useful-work score

These signals are combined into a participation score.

For ETHOnline, participation eligibility is gated by a World ID-style humanity proof envelope with:

- required proof data
- action binding
- persistent nullifier anti-replay
- on-chain humanity attestations
- cross-address replay protection

**Prototype limitation:** v0.17 does not yet perform production World ID cryptographic proof verification against the live World ID verification service. The current verifier enforces the prototype proof envelope, action matching, and persistent nullifier uniqueness. No biometric or passport data is stored by Prism.

## Three-node self-synchronizing devnet

Prism v0.17 can run three persistent Dockerized nodes:

```text
                    prism-node-1
                   /            \
                  /              \
          prism-node-2        prism-node-3
                 \              /
                  \____________/

             Prism P2P protocol 0.17
```

Each node tracks:

- Prism Chain ID
- Genesis hash
- current height
- last block hash
- local persistent chain state

Peers perform a validated handshake and compare chain state.

If a node is behind, it requests the remote state, validates the entire received blockchain, checks that the remote chain extends its local history, persists the accepted state, and rejoins the network.

### Recovery scenario demonstrated

The v0.17 devnet has been tested with this sequence:

```text
Node 1: height 2 ---- produce blocks ----> height 4
Node 2: height 2 ---- periodic sync ------> height 4
Node 3: OFFLINE  ----- restart + sync ----> height 4

Final state:
Node 1 == Node 2 == Node 3
Chain state: MATCH
```

## Architecture

```text
                           PRISM
                             |
          +------------------+------------------+
          |                  |                  |
          v                  v                  v
   Proof of Stake     Proof of Useful     Proof of Useful
      Security              Work            Participation
          |             Computation          Contribution
          |                  |                  |
          +------------------+------------------+
                             |
                             v
                         Blockchain
                             |
               +-------------+-------------+
               |                           |
               v                           v
          Transactions                  Blocks
               |                           |
               v                           v
            Mempool                    Storage
                                           |
                                           v
                                          P2P
                                           |
                                           v
                                Validated synchronization
```

## Quick start

Requirements:

- Go 1.27+
- Docker / Docker Compose

Clone and test:

```powershell
git clone https://github.com/Ulysse98/prism.git
cd prism
git switch ethonline-v0.17
go test ./...
```

### Prepare Docker resources

```powershell
docker network inspect prism-net *> $null

if ($LASTEXITCODE -ne 0) {
    docker network create prism-net
}

docker volume create prism-devnet-node1-data
docker volume create prism-devnet-node2-data
docker volume create prism-devnet-node3-data
```

Build the node image:

```powershell
docker build -t prism-node:v0.17 .
```

Start the three-node devnet:

```powershell
docker compose up -d
Start-Sleep -Seconds 8
docker ps --filter "name=prism-node"
```

Inspect the network:

```powershell
docker logs --tail 30 prism-node-1
docker logs --tail 30 prism-node-2
docker logs --tail 30 prism-node-3
```

Expected healthy state:

```text
PRISM NODE v0.17
Protocol: 0.17
Chain state: MATCH
```

For the full offline-node recovery demo, see [`RUNBOOK.md`](RUNBOOK.md).

## CLI examples

Inspect protocol state:

```powershell
go run ./cmd/prism status
go run ./cmd/prism validators
go run ./cmd/prism participation
```

Submit useful work:

```powershell
go run ./cmd/prism work Charlie 3 4 5
go run ./cmd/prism worklog
```

Submit a prototype humanity proof:

```powershell
go run ./cmd/prism human Alice proof_001 nullifier_001
```

Run P2P nodes locally:

```powershell
go run ./cmd/prism node --port 7001
go run ./cmd/prism node --port 7002 --peer 127.0.0.1:7001
```

Produce a block directly against a node state:

```powershell
go run ./cmd/prism node-produce --port 7001
```

## v0.17 capabilities

- Genesis block creation
- persistent blockchain state
- cryptographic wallets and Prism addresses
- signed transactions
- account balances and nonces
- mempool validation
- locked staking
- Proof-of-Stake validators
- deterministic proposer selection
- block rewards
- deterministic Proof of Useful Work
- useful-work verification and rewards
- Proof of Useful Participation scoring
- humanity attestations
- persistent nullifier anti-replay
- cross-address replay protection
- TCP P2P networking
- node identity
- Prism Chain IDs
- protocol v0.17 peer handshakes
- blockchain height comparison
- fork detection
- periodic peer synchronization
- full remote-chain validation before adoption
- persistent synchronized state
- offline-node automatic catch-up
- three-node Docker Compose devnet

## Scope

Prism v0.17 is a **research prototype**, not a production L1.

The ETHOnline build focuses on making the core idea executable and inspectable:

1. separate stake, useful work, and participation;
2. attach those signals to real block production and rewards;
3. gate participation with a human-uniqueness primitive;
4. prove the chain can operate and recover across multiple persistent nodes.

## Project

Repository: https://github.com/Ulysse98/prism

Website: https://ulysse98.github.io/prism/

Demo runbook: [`RUNBOOK.md`](RUNBOOK.md)

Protocol specification: [`Prism v0.17 Protocol Specification.pdf`](Prism%20v0.17%20Protocol%20Specification.pdf)

Whitepaper: [`Prism Blockchain Whitepaper.pdf`](Prism%20Blockchain%20Whitepaper.pdf)

---

**Prism v0.17 — ETHOnline 2026**
