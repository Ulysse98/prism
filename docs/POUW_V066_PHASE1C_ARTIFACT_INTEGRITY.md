# Prism v0.66 — Phase 1C: Off-chain Artifact Byte Integrity

## Scope

Adds a bounded, streaming SHA-256 verifier for a local regular file; initial
artifact semantics: **GPU worker executable binary**. The independent approved
SHA-256 is a policy input controlled by the verifier/operator, **not** an
untrusted request/contract/proof value. Quantized model input/weights are
already encoded in `usefulwork.Task` and validated by the v2 proof path.

- `ComputeArtifactFileSHA256V066`: hashes actual local file bytes (max 512 MiB),
  rejects empty/non-regular files, final-component symlinks, relative paths,
  changed file identity, and size/mtime differences detected during hashing.
- `VerifyComputeArtifactFileV066`: compares actual bytes against a separately
  approved, lowercase canonical SHA-256 using fail-closed comparison.
- `VerifyQuantizedMLComputeTaskBindingWithArtifactV066`: uses the real byte
  check, then delegates to the Phase 1B verification of signed v2 proof and
  contract fields, including the trusted digest.

## Security boundaries

This is **preflight-only**, off-chain. It does not launch GPU code, verify which
binary was executed, attest a GPU, validate requester authority, guarantee
artifact immutability after hashing, or authorize settlement. There is a
TOCTOU window between hashing and execution. Deployments must secure parent
paths and replace path-based execution with safely pinned/immutable artifacts
before relying on this for execution security. File size/mtime comparisons are
best-effort mutation detection, not an immutable snapshot guarantee.

Do not derive the approved digest from the same untrusted contract or file
being checked; instead use a separately authenticated, reviewed allowlist or
manifest. Artifact digest inventory via `sha256sum` is not an approval step.

No changes to `Task`, `Proof`, v0.64 verifier, v0.65 escrow, blockchain consensus,
API, or CUDA workers. All prior payout rules remain unchanged.
