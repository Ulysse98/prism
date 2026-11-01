# PRISM v0.67 — Phase 1C.5D: Pinned public operator root (offline Linux lab)

An **additive, experimental** operator initialization path. No network RPC,
wallet signing, registry submission, escrow release, or cross-chain settlement.

## Mechanism

`LoadPinnedOperatorRootV067` reads a local regular file containing **only**
64 lowercase hexadecimal characters encoding an Ed25519 public key, with an
optional final LF. It rejects file/parent symlinks, a nonregular file,
non-owner-readable-only permissions, wrong file owner, malformed keys, and a
public-key SHA-256 fingerprint different from the independently pinned digest.
It uses `O_NOFOLLOW` and compares file identities before and after opening.

`NewLocalOperatorAdmitterFromPinnedRootV067` then passes the loaded root into
the existing signature verifier and independently captured source snapshot.
Tests cover valid admission, replay, unexpected key replacement, incorrect
fingerprint, tampering and unsafe paths.

## Trust and deployment boundary

The trusted operator MUST fix both the absolute key-file path and the SHA-256
fingerprint outside all untrusted requests; ideally build/configuration pinned
and independently reviewed. This helper does not authenticate where that
fingerprint came from. The existing constructor accepting a key directly is
still exported, so root-file provisioning is NOT YET mandatory for all callers.

This phase does not establish parent-directory immutability or protect against
malicious privileged processes and concurrent path rewriting. It is not a
certificate authority, revocation protocol, durable key rotation process,
network finality proof or global replay-prevention system. Never store a
private signing key in the public-root file.

Before any network exposure: isolate the provisioning code behind privileged
process boundaries, explicitly define root rotation/revocation and persistent
identity, pin trusted source snapshot production under synchronization, and
perform dedicated security review.
