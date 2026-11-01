# PRISM v0.67 — Phase 1B.3: pinned local source checkpoint (offline)

This phase adds `VerifyAnchorWithPinnedCheckpointV067` as a **read-only**
qualification of a PoUW v2 anchor candidate. It reuses v0.67 Phase 1B.1's
independent CPU, signature and policy checks, plus Phase 1B.2's fully validated
local Prism chain and exact proof/block inclusion check.

The caller supplies a `PinnedSourceCheckpointV067` (chain ID, genesis hash,
checkpoint height/hash and minimum 1..128 successor blocks). The checkpoint
MUST come from an independently authenticated, out-of-band operator policy.
This prototype cannot authenticate checkpoint provenance on its own, and
local state must remain immutable during verification.

Success establishes **only** that an exact candidate proof is present in the
locally validated chain and a *separately pinned* checkpoint is on that same
chain at the required minimum block depth. Depth does not prove canonicality,
quorum, validator-set authenticity, distributed PoS/BFT finality or an absence
of reorganizations. This is NOT a bridge or light client, and must never be
used as permission to anchor, authorize wallet signing, consume a nonce, settle
an escrow, or pay out funds.

The returned error/nil error has **no authorization semantics**. No network
requests, RPC calls, private keys, wallet signing, state mutations, payments,
nonce store or external contracts are introduced by this phase.

Next: independently authenticated finality evidence, then a durable offline
anti-replay reservation design before any cross-chain write path. Treat any
simple local file journal as rollbackable until its storage and trust model
has been separately proven.
