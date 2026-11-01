# PRISM v0.67 — Phase 1A: Offline anchor intent

Status: experimental, offline only. Does not certify a proof or cause settlement.

## Goal

Create a stable commitment to a **candidate** Prism PoUW job/proof, a source
block, and exactly one destination registry and chain. The commitment is usable
as an intent identifier by future integration code, not as an authorization.

## Encoding

`AnchorIntentV067.Digest()` computes SHA-256 over:

- Domain `PRISM/CROSSCHAIN/ANCHOR-INTENT/V067\x00` (literal NUL byte at the end)
- `version`: 4-byte unsigned big-endian
- `source_chain_id`: 4-byte length + raw UTF-8/ASCII bytes
- `source_genesis_hash`: length + raw lowercase hexadecimal bytes
- `source_block_height`: 8-byte unsigned big-endian
- `source_block_hash`, `job_id`, `proof_hash`: each length + raw bytes
- `destination_chain_id`, `destination_registry`: each length + raw bytes
- `nonce`: 8-byte unsigned big-endian

The digest is stable **only** for identical input values. Registry identifiers
are treated as case-sensitive opaque IDs; target-specific canonicalization and
address validation remain unimplemented. No implicit chain ID is trusted.

## Strict boundaries

- No proof verification, source-block finality verification, or signature validation.
- No live network request, RPC, wallet, API dispatch, blockchain transaction or payout.
- A nonce is committed but **not consumed** or tracked in Phase 1A. This phase
  does not prevent replay on its own.
- A consumer MUST NOT interpret a valid intent or digest as proof of completed
  computation, custody, payment authorization, or bridge finality.
- Any real anchoring or settlement needs an independent Prism proof verifier,
  finalized source-block identity, authenticated authorization, persistent
  idempotency/replay state, destination-chain confirmation and failure recovery.
- Existing Cloud 2I.1 work must remain untouched.

## Offline validation

`go test -count=1 ./internal/crosschain`

`go test -race -count=1 ./internal/crosschain`

`go vet ./internal/crosschain`

## Planned next phase (1B)

Verify the source proof against Prism's independent proof verifier and source
chain finality, then define persistent replay state and a destination adapter
contract for one testnet first. Never derive a `verified=true` claim merely
from this intent or accept an off-chain boolean flag as authority.
