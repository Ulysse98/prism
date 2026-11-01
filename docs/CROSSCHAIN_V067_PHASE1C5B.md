# PRISM v0.67 — Phase 1C.5B: signed operator trust, offline admission

## Design and threat boundary

- The operator must provision an Ed25519 **public key from a trusted local
  channel**, separate from any untrusted candidate data. Accepting a key chosen
  by an applicant defeats authentication. This phase does NOT implement that
  provisioning service or a network-facing authenticated boundary.
- An operator-signed bundle binds all fields of `AnchorProofPolicyV067` and
  `PinnedSourceCheckpointV067` in a domain-separated, versioned Go JSON payload.
  Tampering with any field causes signature verification to fail.
- `NewLocalOperatorAdmitterV067` verifies the signer and captures the policy
  and checkpoint **by value**. `Admit` only takes untrusted candidate material.
- The older exported `AdmitLocalAnchorCandidateV067` is renamed to
  `admitLocalAnchorCandidateV067`, preventing *other Go packages* from passing
  arbitrary policies into the previously public helper. The lower-level writer
  remains package-private as in Phase 1C.5A.

## Residual risks — DO NOT DEPLOY OR PAY

- Any privileged caller that lets an untrusted request choose `pinnedOperatorKey`,
  store root, chain pointer, validator set, or signed trust bundle still has a
  trust-boundary vulnerability. Go exported constructors are not authentication.
- The local blockchain and validator-set pointers are NOT deep-copied or locked;
  the operator must provide an immutable, independently trusted snapshot and
  prevent mutation throughout admission and the context lifetime.
- This does NOT establish network finality, global anti-replay, power-loss
  durability, multi-node settlement uniqueness, or authorization to sign, pay,
  submit to an external registry, or release escrow.
- Signer identity and trust-root provisioning, signature rotation/revocation,
  and trustworthy process/file ownership still require separate design.
- No CUDA, RPC, chain writes, transactions, settlement, commit or push.

## Qualification

- Positive signed operator context + independently signed V2 PoUW,
  independently signed verification report, local inclusion and checkpoint.
- Rejected tampered signatures, policies, checkpoints, wrong/unpinned roots,
  candidate-controlled destination and falsified worker signature.
- Same-process and reopened-store replay rejection; failed admissions never
  occupy a legitimate proof slot.
- Regression, race detector, static analysis, and API-surface AST test.
