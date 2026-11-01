# PRISM v0.67 — Phase 1C.5E: Cooperative synchronized source lab

**Scope:** experimental local source-container design. No API exposure, wallet
signing, transfers, settlement, finality or distributed anti-replay.

## Design

- A `LocalSynchronizedOperatorSourceV067` owns private, validated deep copies of
  the blockchain and PoS validator set.
- Readers use `Capture` under `RWMutex.RLock`, returning separate deep copies.
- Trusted, cooperative writers use `ApplyTrustedUpdateV067` under `RWMutex.Lock`.
  Mutations are validated and published copy-on-write; rejected updates leave
  the previous source intact. A second copy prevents post-return mutation using
  a callback-retained pointer.
- `NewLocalOperatorAdmitterFromSynchronizedSourceV067` captures one validated
  source view before delegating to the existing signed-operator admission path.

## Trust assumptions and limits

1. **Initial capture:** The trusted caller must quiesce or synchronize the ORIGINAL
   chain and validator data while constructing the source. A mutex introduced
   after the fact cannot prevent races with writers bypassing it.
2. **Cooperative writers only:** Readers and writers of this private source must
   use its APIs. This does NOT add locks to Prism's blockchain/consensus engine.
3. **Trusted callbacks:** A callback must not start concurrent mutation or
   reenter the source while holding the lock. This is not a hostile-plugin API.
4. **Legacy path:** Existing operator constructors are still callable; no
   mandatory network-facing boundary is established in this phase.
5. **Trust root:** Passing an operator key directly remains possible. Privileged
   provisioning, key rotation/revocation, snapshot authenticity, BFT finality,
   cross-node replay and settlement remain unresolved.
6. **Performance:** JSON snapshots are bounded at 64 MiB and revalidated; this is
   not a production-scale streaming chain snapshot protocol.

## Acceptance checks

The isolated tests cover detachment, defensive captures, rejected updates,
copy-on-write pointer escape, capture blocking under a cooperating writer,
real signed candidate admission and replay rejection. Full crosschain Go
regressions, race detector and vet must pass on the pinned baseline.
