# Prism Cloud MVP + Prism Core v0.66

## Integration baseline

This integration combines:
- Prism Core master c657d9f (v0.60-v0.66).
- Prism Cloud feature commit ad48045.
- Integration merge commit fc57111.

The merge is local and does not itself deploy services.

## Platform policy

- Prism Core: Windows and Linux compatible build paths.
- Prism Cloud command: Linux-only by explicit Go build tag.
- Cloud GPU scheduler, broker, worker and locking labs:
  evaluated under Linux.
- WSL2 may be used for Linux development and GPU qualification.
- Internal computecloud tests compile for Windows.
  This does not establish native Windows Cloud runtime support.

The Windows build of cmd/prism-cloud is deliberately excluded.
Do not remove Linux constraints just to make it compile.

## Security boundaries

The cmd/prism-cloud gateway is loopback-only and simulation-only.
Its current entrypoint does not dispatch wallet, GPU,
escrow or consensus operations.

GPU execution laboratories require explicit authorization.
The Linux flock protects cooperating processes only.
It is not GPU hardware isolation or remote attestation.

Prism v0.66 sealed GPU execution and independent verification
remain off-chain and opt-in. This integration does not enable
production proof dispatch or escrow payout.

## Qualification evidence

Local integration:
- Full offline Go test suite passed.
- Race tests passed for computecloud, usefulwork,
  escrow and cmd/prism.
- go vet ./... passed.
- Linux Core and Cloud builds passed.
- Windows-compatible Go package builds passed.
- Windows internal/computecloud test compilation passed.
- Linux Cloud package tests and race detector passed.

Previously completed standalone qualifications also include
cooperative Core/Cloud GPU lock interoperability and
approved sealed CUDA execution.

These checks do not establish a complete operational
Cloud request-to-PoUW-to-settlement workflow.

## Remaining integration work

- Define authorized Cloud-to-Core compute dispatch.
- Enforce trusted artifact and verifier policies.
- Verify complete job lifecycle and signed result handling.
- Test failures, cancellation, replay and recovery across
  the real broker and PoUW interfaces.
- Perform security review before production activation.
