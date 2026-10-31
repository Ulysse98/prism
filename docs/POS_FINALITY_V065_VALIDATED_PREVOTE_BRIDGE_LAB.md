# PRISM v0.65 — Authenticated candidate PREVOTE bridge (4C.2)

STATUS: EXPERIMENTAL / PREVOTE-ONLY / NOT PRODUCTION CONSENSUS.

The parent snapshot is explicitly pinned at initialization. Its chain identity,
validator snapshot, and canonical tip MUST be authenticated by the caller. The
parent is stored alongside safety state, and every stored candidate witness is
revalidated under the signer file lock during replay, before any new signature
is made public. Existing boolean-only signing methods cannot read this binding
(mode + trusted parent tip).

This first bridge intentionally supports ONLY one non-NIL PREVOTE at round 0.
PRECOMMIT, NIL, round advance and commitment are NOT supported by this new gate.
The older APIs remain research-only and MUST NOT be used with the same signing
key. `SignVote` remains publicly callable with raw keys: there is no enforced
key isolation. Run the new bridge with a dedicated throwaway Ed25519 key.

The persisted parent is not connected to live p2p synchronization; a live
reorg/mutation remains outside these guarantees, as do storage rollback,
privileged key misuse, power-loss durability, and block data too large for the
8 MiB research journal. This is NOT an end-to-end BFT validator.

Next: authoritative chain snapshot acquisition and synchronization, validator
key isolation, PRECOMMIT and NIL proof bridging, and fault-injection replay.
