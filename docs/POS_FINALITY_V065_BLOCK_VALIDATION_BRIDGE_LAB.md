# PRISM 2C.5F-B.4A — Non-signing block validation bridge

**EXPERIMENTAL ONLY. No active consensus changes. No private keys used.**

`ValidatePrismProposalV065` creates detached copies of the supplied parent
chain, validator snapshot and candidate block, binds the expected vote context,
then runs the real PRISM parent-chain validation and `AppendValidatedBlock`
against the copied chain. It never changes the caller's chain and cannot sign.

Its `trustedChainID`, `expectedParentHash` and validator snapshot **MUST** be
authenticated from a trusted, immutable local source. The function compares the
context against the trusted chain ID argument but does **not** independently
derive the chain ID from PRISM configuration. This is a known trust boundary.

The local P2P test uses the real v0.65 escrow fixture and verifies a valid
RELEASE and multiple adversarial alterations. It also checks that the parent
is unchanged after validation.

## Security boundaries still open

- This is **not** a sign authorization, finality certificate, or live consensus.
- Do not pass this function's success as a cached `blockValidated=true` into
  `SignDurableGatePrevoteV065` or `SignDurableGatePrecommitV065`. The current
  gate's replay would still trust that flag instead of independently replaying
  the same authenticated parent and candidate.
- The parent and PoS snapshot must not be mutated concurrently during cloning.
- A production gate must authenticate and freeze parent tip, network config,
  validator epoch and block data *inside* its durable serialization boundary.
- Recovery must reverify saved evidence and parent provenance before signing.
- This lab does not address private key isolation, file rollback, live partitions,
  power-loss durability, or BFT safety and liveness proofs.

## Tests

`go test ./internal/p2p -run '^TestEscrowV065CandidateForFinalityVote$' -v -count=1`

`go test -race ./internal/p2p -run '^TestEscrowV065CandidateForFinalityVote$' -count=5`

These tests run against the local PRISM v0.65 code, not the reduced stand-in lab.
