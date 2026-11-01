# PRISM v0.67 — Windows Go CI, Linux local replay contract

The private local replay store remains **Linux-only** in v0.67. It relies on
private POSIX namespace/file permissions, exclusive local file creation, and
filesystem sync. NTFS ACL semantics on native Windows must not be treated as
POSIX directory mode guarantees. Previously the Windows CI failed with an
`unsafe storage directory or permissions` error, and a negative fixture reported
mode `0777` instead of `0755`.

`NewLocalReplayStoreV067` now refuses all non-Linux platforms with
`ErrLocalReplayInvalidV067` *before creating storage*. The Windows-specific test
requires that refusal and an absent namespace. Tests exercising real Linux-only
replay persistence skip on other operating systems; other cross-platform tests
for proofs, checkpoints, snapshots and API contracts remain enabled.

Run `go test -count=1 ./...`, `go vet ./...`, and
`go test -race -count=1 ./internal/crosschain` on Linux. Cross-compile the
Windows crosschain test binary and verify actual Windows Go CI in a PR.

**This is not a native Windows replay backend.** Do not disable permission
checks, rewrite the namespace modes, authorize payouts or settlements, or
claim that local one-shot reservation proves global replay protection/finality.
No node or network bridge activation is introduced here.
