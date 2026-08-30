# Prism

> **Proof of Stake secures. Proof of Useful Work computes. Proof of Useful Participation grows.**

Prism is an experimental blockchain protocol written in Go exploring a hybrid architecture combining:

- **Proof of Stake (PoS)** for network security and block proposer selection
- **Proof of Useful Work (PoUW)** for verifiable computation
- **Proof of Useful Participation (PoUP)** for measuring and rewarding meaningful network contribution

Prism is currently an early-stage research and engineering project.

**Current version: v0.16**

Repository: https://github.com/Ulysse98/prism

---

## Vision

Traditional blockchain consensus mechanisms primarily answer one question:

> Who is allowed to produce the next block?

Prism explores a broader question:

> Can a blockchain secure itself, execute useful computation, and reward meaningful participation at the same time?

The project separates these responsibilities into three complementary mechanisms.

### Proof of Stake — Secures

Validators lock PRISM tokens and participate in block production.

Stake provides the economic security layer of the network.

### Proof of Useful Work — Computes

Participants can execute deterministic computational tasks and submit verifiable proofs of their work.

The current prototype implements a deterministic **sum-of-squares task** as the first Proof of Useful Work primitive.

Useful work is verified before being included in the blockchain.

### Proof of Useful Participation — Grows

Prism tracks participation beyond stake ownership.

The current participation model measures contributions including:

- blocks proposed
- useful work completed
- proposer activity
- useful-work score

These signals are combined into a participation score.

The long-term goal is to explore reputation and incentive mechanisms based on actual network contribution.

---

# Current State

Prism v0.16 already implements a functional experimental blockchain node.

Current capabilities include:

- Genesis block creation
- Persistent blockchain state
- Cryptographic wallets and Prism addresses
- Signed transactions
- Account balances
- Transaction nonces
- Stake locking
- Proof-of-Stake validator registration
- Deterministic proposer selection
- Block rewards
- Mempool validation
- Proof of Useful Work tasks
- Useful-work verification
- Useful-work rewards
- Proof of Useful Participation scoring
- Blockchain validation
- Local state persistence
- TCP peer-to-peer networking
- Node identity
- Prism Chain IDs
- Peer handshake protocol
- Blockchain height comparison
- Validated blockchain synchronization
- Fork detection
- Full remote-chain validation before adoption

---

# Architecture

```text
                         PRISM
                           │
            ┌──────────────┼──────────────┐
            │              │              │
            ▼              ▼              ▼
     Proof of Stake   Useful Work    Participation
        Security      Computation       Scoring
            │              │              │
            └──────────────┼──────────────┘
                           │
                           ▼
                      Blockchain
                           │
                ┌──────────┴──────────┐
                │                     │
                ▼                     ▼
           Transactions             Blocks
                │                     │
                ▼                     ▼
             Mempool              Storage
                                      │
                                      ▼
                                    P2P
                                      │
                                      ▼
                              Chain Synchronization
