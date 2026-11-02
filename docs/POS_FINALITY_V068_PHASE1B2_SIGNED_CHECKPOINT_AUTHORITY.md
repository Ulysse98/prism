# PRISM v0.68 — Phase 1B.2: signed checkpoint authority (offline)

**RESEARCH ONLY. NON-SIGNING PRODUCTION INTERFACE. NOT ACTIVE CONSENSUS OR BFT FINALITY.**

## Goal

Add an *offline, read-only* Ed25519 signature verification layer before using
Phase 1B.1's `VerifyPinnedValidatorSnapshotV068` against an independently
trusted Prism chain checkpoint and CURRENT locked-stake validator snapshot.

The caller supplies an out-of-band trusted `CheckpointTrustPolicyV068` containing:

- exact chain ID and canonical genesis hash;
- caller-maintained min/max acceptable parent heights;
- explicit authorized public keys, unique IDs, inclusive activation/expiry
  heights, and revocation flags.

The signed checkpoint binds the operator ID, chain ID, genesis, parent height,
parent block hash and validator-set hash. The cryptographic payload is prefixed
by `prism-finality-v068/signed-checkpoint/v1\x00` and uses the fixed Go JSON
field order from `signedCheckpointPayloadV068` (an experimental v1 contract).

Successful verification then calls the existing `VerifyPinnedValidatorSnapshotV068`
and refuses a candidate with mismatched current locked stakes, chain context,
parent tip, genesis or malformed chain history.

## Trust and safety limits

- The authorized policy **must be provisioned independently of the candidate
  checkpoint and of any untrusted node**. Treating a peer-supplied public key
  as the trust root completely defeats the signature check.
- A policy is supplied in memory, not rooted in protocol governance or a
  historical validator set. A single trusted operator signature is **not** a
  validator quorum, a finalized block or BFT consensus evidence.
- Height windows are **not wall-clock expiration**. The verifier cannot know a
  minimum trusted height without a separately authenticated, durable source.
  Host rollback or an old policy can reauthorize stale signed checkpoints.
- The policy's `Revoked` flag is external; the library cannot discover key
  compromise, revoke keys globally or prevent duplicated authority keys from
  signing outside this process. No certificate-chain trust or rotation protocol.
- The current validator set is checked against the candidate parent state,
  not reconstructed at arbitrary historical epochs. The candidate chain and
  its validator snapshot must remain stable while being copied.
- No checkpoint issuing API, validator signing, voting, wallet signing, network
  RPC, active consensus integration, escrow release or cross-chain settlement.
- Tests use deterministic disposable operator keys **only in test files**.

## Required adversarial tests

- Correct pinned identity, valid signature and current stakes accepted.
- Signature forgery, altered canonical payload, unknown operator and incorrect
  chain/genesis/parent height/hash/validator-set hash rejected.
- Duplicate or alias authority entries, invalid key/ID, revoked authority,
  invalid activation/expiry bounds and stale caller-pinned heights rejected.
- A valid operator signature for a fabricated validator-set commitment does
  not make that fabricated commitment match the parent chain.

## Next: 1B.3+

Authenticated independent policy distribution, monotonically durable trusted
checkpoint height / anti-rollback, historical validator epochs, atomic canonical
node snapshot capture and isolated signer enforcement are still unimplemented.
Never wire this research helper into the active PRISM network.
