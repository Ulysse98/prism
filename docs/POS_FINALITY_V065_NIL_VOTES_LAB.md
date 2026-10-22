# PRISM v0.65 — Phase 2C.5F-A: explicit NIL votes

Status: ISOLATED LAB ONLY. NOT ACTIVE CONSENSUS OR DISTRIBUTED FINALITY.

This patch adds an explicit `Nil bool` flag (JSON `nil,omitempty`) to
`VoteContext`. Signed NIL votes have `Nil=true` and an empty `BlockHash`;
normal block votes require `Nil=false` and a canonical 32-byte hex block hash.
The NIL choice is signed together with the chain/genesis, validator snapshot,
height, round, and PREVOTE or PRECOMMIT step. The `omitempty` encoding is
intended to preserve signatures already made for non-NIL votes in 2C.5B.

The existing Linux vote journal records and verifies NIL signatures under the
same signing slot as ordinary block votes. It rejects an attempt to sign a
block after NIL or NIL after a block at the same height/round/step.

The isolated BFT state machine learns one additional transition:
- verified >2/3 NIL PREVOTE quorum in the current round: clear the current
  lock, produce PRECOMMIT(NIL).
- no quorum: retain the lock, produce PRECOMMIT(NIL), as before.
- verified non-NIL PREVOTE quorum: lock the validated block, as before.
- a NIL PRECOMMIT quorum may **not** finalize a block.

Reference: https://github.com/cometbft/cometbft/blob/main/spec/consensus/consensus.md

## Deliberate limitations

1. The lock machine **does not persist** any state transition. It must not be
   connected directly to `SignVoteOnceV065` yet. Journal writes and lock writes
   are not atomic together; crash-recovery and cross-round enforcement are not
   solved by this patch.
2. The trusted validator set, block validity, networking, and leader rotation
   are external. Lab signatures do not imply distributed finality.
3. The journal only prevents conflicting signing *within the same slot*.
   Restore-from-backup, off-path signing, and multi-machine key use can bypass it.
4. `validBlock` and `blockValidated` remain caller-provided lab flags.
5. NIL is represented by `Nil=true`; ambiguous NIL/block encodings are rejected.

## Validation

Run `go test ./internal/finality -run '^TestNilV065' -v -count=1`, then
`go test -race ./internal/finality -count=5`, `go test ./... -count=1`,
`go vet ./...` on Linux/WSL. No network activation or commit is automatic.

Next subphase: persist cross-round lock state atomically with vote decisions
before any signer integration. Never assume the current journal suffices.
