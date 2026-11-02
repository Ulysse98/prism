# PRISM v0.68 — Phase 1B.3C: read-only pending recovery inspection

**RESEARCH ONLY. LINUX/WSL ONLY. NO PRODUCTION ANTI-ROLLBACK, BFT FINALITY OR SIGNER INTEGRATION.**

## Purpose

Provide an advisory inspection of the local 1B.3B durable checkpoint floor
under its cooperating-process `flock` boundary and compare it with a caller-
supplied **independently authenticated** witness. No state mutation, auto-commit,
rewind, silent repair, witness publication, or checkpoint signing is performed.

A successful inspection returns one of the following states:

- `ready`: no pending checkpoint; external witness matches the stored current.
- `awaiting_external`: pending transition exists; external witness still
  matches the committed old floor. Regular reads remain blocked.
- `commit_required`: pending transition exists; external witness matches
  the prepared next floor. The operator may explicitly call the already
  existing `CommitPreparedCheckpointFloorV068` only after independent witness
  authentication; regular reads remain blocked until it completes.

Missing/corrupt state, symlinks, and witnesses matching neither current nor
pending fail closed. Status is **advisory**; it must never be used as vote or
signer authorization and does not expose the floor as a finalized chain height.

## Tests

- New store is ready; prepare produces `awaiting_external` then
  `commit_required` according to the authenticated external witness.
- Inspect never clears pending, even if the external witness already advanced.
- Explicit commit restores ready state; stale external witness then fails.
- Fault injection before/after rename observes the actual disk state on reload;
  injected process errors are not proofs of physical power-loss behavior.
- Corruption, missing record and symlinks fail closed.
- **Negative canary:** coordinated restoration of both local record and the
  purported external witness re-authorizes an older floor. This deliberately
  demonstrates that a local file and caller-provided hash cannot anchor trust.

## Remaining security blockers

1. External witness must be authenticated and guaranteed monotonic through an
   independently protected service or trust anchor. This phase DOES NOT supply it.
2. Crash-safe handshake across local disk and remote witness service needs a
   separately reviewed protocol, timeouts and controlled operator recovery.
3. `flock` works only across cooperating Linux processes/filesystems, not
   untrusted writers or privileged rollback of the entire directory.
4. No historical validator-epoch reconstruction, real distributed BFT finality,
   live P2P/node snapshot binding, key isolation or rollback-safe production signer.
5. This does not test actual power-cut durability of hardware, disks or mounts.

Never wire this helper into active consensus, escrow, wallet, validator signing
or public APIs. Never treat `ready` as proof of chain finality.

## Local qualification

```sh
go test -count=1 -timeout=5m ./internal/finality ./internal/consensus
go test -race -count=1 -timeout=8m ./internal/finality
go vet ./internal/finality ./internal/consensus
```
