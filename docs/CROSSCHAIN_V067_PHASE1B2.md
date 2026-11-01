# PRISM v0.67 — Phase 1B.2: read-only local inclusion on validated Prism chain

This lab combines the v0.67 Phase 1B.1 independently verified off-chain
candidate with a complete *locally validated* Prism blockchain.

`VerifyAnchorLocalInclusionV067` checks:

1. A pinned source chain ID and genesis hash derived from the local genesis.
2. `Blockchain.ValidateChain(pos)` using the separately supplied local PoS snapshot.
3. Exactly one block at the claimed height and matching block hash.
4. Exactly one full proof equal to the independently verified worker-signed
   `usefulwork.Proof`, in precisely that block.
5. Existing pinned destination registry/chain, approved artifact, approved
   independent verifier, job status and V2 signatures (Phase 1B.1).

Caller obligations: both chain and validator-set arguments must come from an
independently trusted, validated local source; keep them immutable through the
call. Do not derive pinned policy from untrusted snapshots or API payloads.
The function revalidates local state but cannot prove the local validator set
is globally canonical, nor prove finality after a possible chain reorg.

## Strict limits

- Local inclusion is **not** distributed consensus finality, quorum certificate,
  trustless light-client validation, fork-choice agreement or a bridge.
- Does NOT record used nonces, enforce destination registry uniqueness, or
  authorize registering a proof on Arbitrum, Monad, Sui or Solana.
- Does NOT sign or send transactions, call RPC, control wallets, unlock
  escrow, pay workers, or assert GPU hardware attestation.
- Validating the stored PoS set does not independently authenticate that set;
  a trusted checkpoint / validator-set provenance is needed for stronger claims.
- A `nil` return means only locally validated proof inclusion, not a settlement permit.

Next phase: establish a separately authenticated source checkpoint/finality
policy and durable anti-replay authorization *before* any cross-chain writes.
