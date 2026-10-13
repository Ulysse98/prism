# Prism v0.61 - Governance Bootstrap

Status: Design / not activated.

## Problem

A legacy Prism chain may have an empty Team authority set.
The reserved allocation exists, but cannot be authorized.

## Proposed solution

- Preserve existing genesis and historical blocks.
- Require an explicit consensus protocol upgrade.
- Initialize an empty authority pool only once.
- Require signatures from validators representing
  more than two-thirds of active stake.
- Bind approvals to chain ID, genesis hash, pool,
  authority set, threshold, nonce and activation height.
- Record initialization in a consensus-validated block.
- Preserve existing governance delays and supply limits.
- Reject unauthorized, duplicate and replayed requests.

## Security requirements

- Reject insufficient stake approval.
- Reject invalid signatures.
- Reject wrong network identity.
- Reject repeated bootstrap.
- Reject invalid authority thresholds.
- Reject unauthorized reserved allocations.
- Verify historical block compatibility.
- Verify chain ID compatibility.
- Test consensus replay and accounting.

## Intended follow-up

After legitimate governance initialization, separately
propose a 5000 PRISM Team allocation to:

prism_297ee45259340851cf341ece082b160e23eac731

No funds are credited by this design document.
