# Prism v0.61 — Timelocked Team reserve governance

Status: **validated end-to-end on the user's local Prism development network on 2026-10-13**. This document does **not** assert public-mainnet deployment, independent validator finality, or token market value.

## Overview

Prism v0.61 introduces an on-chain, proposal-driven path for spending from the Team reserve. A queued proposal requires recorded approvals and a block-height timelock. Execution is committed through a normal consensus-validated block, then reflected in the canonical wallet balance. No direct JSON balance edits are part of the demonstrated transfer.

## Demonstrated local-network execution

| Field | Observed value |
| --- | --- |
| Chain ID | `prism-ceea53aa87bcae01` |
| Genesis hash | `807567a62284fb7990ab6a3d75c0a8255928a5d3b8681ec7fb86eac8615c5f85` |
| Team authority | Alice + Bob, 2 of 2 approvals |
| Approved proposal | `f5acea92fdd2cef7a5ef377024f653755ce37115599e087b497bd5f3d795f584` |
| Proposal inclusion | Block 7 |
| Execution delay | 5 block heights; earliest block 12 |
| Canonical execution block | Height 12 |
| Block 12 hash | `ba417238f787fde4926737c0dd733d8fb028604dbd0ad5c79a752f7320d40362` |
| Recipient | `prism_297ee45259340851cf341ece082b160e23eac731` |
| Transfer amount | 5,000 PRISM |
| Recipient confirmed by API | `totalBalance=5000`, `availableBalance=5000`, `lockedStake=0`, `nonce=0` |
| Canonical chain.json SHA-256 after commit | `11F649FBC54647C485C98BAB07889352E907610A2899595CD25465DA5B9CCDB1` |
| Validator wallets.json SHA-256, unchanged | `CC6CD32FC3DFF4D309370CD4EE5F7CAD89D9EB5E755060578640657C77B8A473` |
| Exact block12 candidate SHA-256 | `3EB29978D0A0F8F0C65ED29F58412A588AF2BB3511B0B8E05687016BDEE3192D` |

The latest observed canonical API reported `chainValid=true`, `height=12`, `chainId=walletChainId=prism-ceea53aa87bcae01`, `totalSupply=7578`, and `lastHash` equal to the block-12 hash.

## Local sequence

1. Block 6: Team-reserve authority bootstrapped with Alice and Bob, 2/2.
2. Block 7: a queued, approved 5,000 PRISM proposal recorded with the recipient address. The transfer was not executed at proposal time.
3. Blocks 8–11: heights advanced while preserving the timelock.
4. Block 12: a single `reserved_transfer_executions` entry executed the proposal; the in-memory consensus replay verified reserve use and recipient credit.
5. Exactly that block-12 candidate was propagated to the **three local keyless public observer replicas** Alice, Bob and Charlie; each reported identical height, hash and proposal execution.
6. An external byte-verified backup preceded canonical adoption. The original `wallets.json` remained byte-for-byte unchanged.
7. The canonical API returned the wallet balances above, and the Pixel 8a development-client Wallet screen displayed the matching Ed25519 address and balances.

## Operational safeguards

- Never copy private `wallets.json` or signing keys to public observer data directories.
- Bind proposal authorization, timelock, execution and ledger accounting to consensus replay. Do not issue fake balances through JSON edits or mobile display overrides.
- Do not regenerate or re-adopt the demonstrated block-12 candidate. Its exact hash and file checksum are immutable for this historical run.
- A failed or interrupted commit requires investigation before any retry or restore.
- Use `GET /api/v1/status` and `GET /api/v1/wallets/{address}` against the verified **same chain ID**. The observed local API was bound to `127.0.0.1:8081` with faucet and experimental wallet transfers disabled.
- The three observed peers were on one machine with public, keyless state; the experiment does not prove geographically independent decentralization.
- The wallet's `humanityVerified=false` is expected in this run; the Team governance transfer did not imply a humanity attestation.

## Recommended regression coverage before release

**Existing demonstrations are not a substitute for an automated negative-path suite.** Confirm automated tests cover: insufficient approvals, incorrect proposer or signature context, altered recipient/amount/nonce, attempts before the timelock, repeat execution, chain/genesis mismatch, corrupted historical block or state, wrong observer chain, and no private key leakage. Treat any missing test as an open release item, not a passed check.
