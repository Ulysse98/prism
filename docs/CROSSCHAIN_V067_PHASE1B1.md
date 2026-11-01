# PRISM v0.67 — Phase 1B.1: Independent PoUW candidate binding (offline)

## Scope

Reuses production Prism `usefulwork.VerifyIndependentQuantizedMLComputeV066` to
verify the worker's context-bound signed V2 PoUW proof, full independent CPU
recomputation, the independently signed V0.64 report, approved verifier ID,
and a trusted externally pinned artifact SHA-256. Also ties the marketplace
job ID/status/proof ID, Prism chain and genesis, and one destination registry
to the Phase 1A candidate anchor intent.

The caller MUST supply trusted policy separately; copying expected policy from
untrusted intent/proof/report would defeat the trust boundary.

`ProofHash` in the Phase 1A intent means `usefulwork.Proof.ID` (the Prism
context-bound proof identifier), not a generic SHA-256 of serialized proof.

## Absolutely NOT established

- No source block inclusion verification: `source_block_height` and
  `source_block_hash` remain **untrusted candidate metadata**.
- No validated local blockchain snapshot, no network-distributed consensus
  finality, fork choice, checkpoint, or BFT finality proof.
- No durable nonce consumption, replay protection, or per-registry uniqueness.
- No signed authority to anchor, transfer funds, release escrow, or settle.
- No real GPU hardware execution attestation; CPU checks correctness only.
- No HTTP handler, RPC, network, chain action, wallet dispatch, transaction,
  private-key operation, or payout. Test wallets exist in memory only.

**A nil error MUST NOT authorize registration or settlement.**

## Next: Phase 1B.2

On a separately loaded and fully validated local Prism chain, verify exactly
one matching proof at the claimed height and block hash; bind job and genesis;
independently check source inclusion against pinned chain identity. Prove
finality only if/when a genuine authenticated consensus-finality mechanism
exists. Add persistent replay protection in a later phase before any writes.
