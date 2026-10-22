# PRISM v0.65 — 2C.5C / Linux persistent vote journal lab

**EXPERIMENT ONLY. NOT ACTIVE PRISM CONSENSUS OR BFT FINALITY.**

`SignVoteOnceV065(dir, ctx, key)` uses an advisory `flock` (Linux/WSL), verifies previously stored signed votes, rejects conflicting vote contexts for the same `(chain ID, genesis, height, round, step, signer address)`, and atomically replaces a JSON vote journal. The replacement file is synced, renamed and its containing directory synced before the signature is returned.

A retry for the exact same context returns the original signed vote. Different blocks or validator-set hashes for the same signing slot are rejected. The slot does *not* include those values, so modifying the snapshot hash cannot bypass same-round anti-equivocation.

**Trust and threat-model boundaries:**

- All signer processes must use this API, the same **trusted ext4/Linux directory**, and the same lock file; use under the WSL home directory, **not** `/mnt/c/` or network filesystems.
- The journal and private key must not be deleted, rolled back, copied to conflicting forks, shared with untrusted users, or used outside this guarded API. Privileged processes and corrupted local disks are out of scope.
- No BFT round-locking rules have been designed: a signer *can* sign conflicting block hashes at different rounds, so this lab is **not sufficient for finality**.
- `SignVote` from the preceding lab remains available and unguarded. Production would need an exclusive signer service/API, authenticated validator snapshots, validator key protection, durable highest-lock state and consensus integration.
- The journal is JSON and reads all historical votes on every call, with a 4 MiB cap. This is intentionally not a production persistence design.
- The tests cover same-process, multi-process concurrency, retry/restart, tamper rejection and a stale/journal-corruption failure mode. Abrupt-power-loss and filesystem failure injection remain untested. A successful fsync is a precondition, not a formal durability guarantee on every storage device.

Run:

```sh
go test ./internal/finality -v -count=1
go test -race ./internal/finality -count=5
go test ./... -count=1
go vet ./...
```
