# PRISM v0.67 / 1C.5G — Privileged admission API boundary (Linux offline lab)

This phase restricts *exported constructor access* to one public operator
factory: `NewLocalOperatorAdmitterFromPinnedSynchronizedSourceV067`.
Its trusted caller supplies a separately provisioned operator Ed25519 **public**
key file, an independent pinned SHA-256 fingerprint, a signed local policy and
checkpoint, a replay store and an owned synchronized source container.

The former exported constructors (`NewLocalOperatorAdmitterV067`,
`NewLocalOperatorAdmitterFromPinnedRootV067`, and
`NewLocalOperatorAdmitterFromSynchronizedSourceV067`) become **unexported**.
Package-internal tests exercise them unchanged. All publicly constructible
admitters therefore pass both the root-loader check and synchronized capture
on Linux. A new AST test prevents re-exporting another admission constructor;
negative tests cover missing root, wrong fingerprint, wrong signer, nil store
and nil synchronized source. Signed real-compute admission and local replay
remain part of the regression suite.

## Scope and residual risks

- This is **compile-time package API hardening**, not OS-enforced privileges.
  Caller-provided path and pin MUST come from a trusted independent operator
  configuration, not from a candidate or remote request.
- It does NOT make the **initial** `NewLocalSynchronizedOperatorSourceV067`
  snapshot safe against original chain/PoS writers. Writers outside this
  cooperative container remain out of scope. In-package code may still call
  unexported helpers, so code review is needed for future internal call sites.
- `LoadPinnedOperatorRootV067` still has documented Linux filesystem TOCTOU
  and parent-directory ownership assumptions. No authenticated key rotation.
- Local anti-replay is not global replay prevention; PoS checkpoint depth is
  not network BFT finality. No network hooks, wallets, transfers, settlement,
  escrow release or cross-chain registry transactions are enabled.
- Linux build target only for the privileged public constructor. This is
  experimental offline admission and not production authorization.

Qualification: package regression, negative tests, full Go tests, race checks,
`go vet`, package API audit and exact file-set check. No commit, merge or push.
