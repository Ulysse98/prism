# PRISM v0.69 — Phase 1B.7: Pinned Closure Authority & Local Recovery Inspection

**Status:** offline, single-host Linux research lab only. No RPC, production keys, on-chain governance, quorum authority, validator signing, distributed consensus, or BFT finality.

## Objective and threat model

Phase 1B.6 allowed *any cooperating caller* with write access to run `SealLocalAdvisoryVoteStore` with an unsigned receipt. Phase 1B.7 intentionally BREAKS that old API signature: the exported seal now requires an Ed25519 operator authorization and a **separately supplied pinned public key**, under the same directory-level `flock` already shared by seal and vote admission. This stops the unmodified 1B.6 closure entry point from continuing to seal without a signature, within cooperating recompiled callers.

The operator must approve the proposal, pre-pinned eligibility snapshot, pre-pinned ballots, local audit, observation height, key identity, and policy OUTSIDE this package. A public key read from the seal itself is **not** an authority. Anyone controlling the process or filesystem as the store owner can still modify local state; pinned key distribution, key revocation/rotation, HSM protection, operator authentication and global agreement are NOT supplied.

## Two-stage offline closure

1. `PrepareLocalAdvisoryClosureDraft(store, receipt, snapshot, snapshotPin, ballotPin, proposal, trusted, ballots)` takes the store's directory lock, checks no closure marker exists, fully re-audits 1B.4/1B.5 inputs and returns a *draft only*. **It makes no disk writes**. Independently approved snapshots/pins are a mandatory operator prerequisite.
2. Offline signer calls `SignAdvisoryClosureAuthorization(draft, nonzeroNonce, operatorPrivateKey)`; only a domain-separated SHA-256 of the canonical closure body is signed. No keys are loaded or persisted by this package.
3. Caller invokes `SealLocalAdvisoryVoteStore(..., authorization, independentlyPinnedOperatorPublicKeyHex)`. Seal takes the same directory lock, re-audits from disk (preventing stale drafts), verifies the signed authority and the independently pinned key, then creates the original marker path `.prism-advisory-closed.v1` without overwrite, fsyncs it and its directory. A partial/ambiguous marker remains fail-closed.
4. `VerifySealedLocalAdvisoryVoteStore(..., pinnedOperatorPublicKeyHex)` re-verifies the signed marker against its independent key pin **and** recomputes the full local reservation audit/receipt under the same lock.
5. `InspectLocalAdvisoryClosureRecovery(store, pinnedOperatorPublicKeyHex)` inspects the local marker after a restart, without writing anything. Its three states are `NO_LOCAL_MARKER_NOT_AUTHORIZATION`, `SIGNED_LOCAL_MARKER_VERIFIED_NOT_GLOBAL_FINALITY`, and `MARKER_PRESENT_UNVERIFIABLE_FAIL_CLOSED`. An absent marker is not proof that admissions are authorized. An invalid marker must NOT be deleted automatically. A valid signature alone does **not** prove local ballot completeness; use step 4.

The signed authorization is embedded in the closure JSON, with versioned fields `version`, `signer_public_key_hex`, `closure_sha256`, `nonce` and `signature_hex`. The signer must match the **caller-supplied pin**, and the signature is over a canonical hash including the signature domain, version, signer, exact closure digest, and nonce. This operator nonce is a signed field, **not** a globally enforced non-equivocation or replay ledger. Cloned local stores can still accept the same authorization unless an independent operator policy stops it.

## Intentional V2 marker format

`AdvisoryBallotClosureVersion` advances to `PRISM_GOV_V069_LOCAL_BALLOT_CLOSURE_V2` and the closure commitment domain advances to `/v2`. The marker **filename stays unchanged** to preserve the Phase 1B.6 admission fail-closed check and avoid reopening a previously sealed store. A 1B.6 V1 unsigned marker is rejected by the V2 reader; it continues blocking votes. There is **no automatic V1→V2 migration or marker replacement**.

## What this does *not* guarantee

- This does not bind an operator to an on-chain identity, stake, validator set, authenticated head, chain finality, or distributed election.
- This does not protect against an unmodified 1B.6 binary operating on the same files, direct filesystem modifications by the owning UID, compromised private keys, lying storage or filesystem rollback.
- It does not distribute the independent public-key pin or its trust policy. Caller must obtain and safeguard the pin out of band.
- It does not make a time-based or height-based closure automatic or authoritative. `ClosedAtHeight` remains an externally supplied observation height.
- There is no automated recovery of corrupt/partial markers: manual forensic response is required and admission must remain stopped.

## Tests and release discipline

Unit tests cover rejection of absent/mismatched pins, invalid signature, nonce and closure digest, signer substitution, a ballot admitted between draft and seal, re-audit after reopen, wrong pinned key on recovery, corrupt marker, and continued post-close rejection. The original 1B.1–1B.6 tests and multiprocess lock tests must also remain green. `go test`, `go test -race`, `go vet`, and a Windows compile-only check are run offline.

The guarded installer requires a clean `feat/v0.69-governance` worktree at **HEAD `09c8f43`**, matches all **25 tracked governance files** by SHA-256, introduces five files, and changes exactly five existing files. It saves rollback copies and restores them on installation/test failure. It never creates a Git commit, push or merge, and never opens a real ballot store.
