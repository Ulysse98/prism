# PRISM v0.65 — Phase 2B prerequisite: task-bound escrow funding

Before the chain can validate RELEASE against a PoUW proof, FUND must commit to the exact marketplace task. The previous v0.65 FUND payload committed to `jobID` but did not commit to the corresponding `taskID` or marketplace job nonce. A proof for a different task could otherwise be paired with the same job ID by an attacker controlling the worker account.

`Funding` now carries `TaskID` and `JobNonce`. Consensus and signature validation check:

```
JobID = SHA256(TaskID + "|" + Requester + "|" + Reward + "|" + JobNonce)
```

This matches `compute.CalculateJobID` **only when Requester is the canonical `prism_...` address**, not a local alias such as `Alice`. The funding authorization signature binds both new fields and uses an updated domain separator.

This is **not** the RELEASE/REFUND implementation. The current funding locks are still irreversible and should only be used on isolated disposable testnets, without real-value funds.

**Compatibility:** This experimental hardening intentionally rejects FUND blocks written using the earlier unbound v0.65 prototype. Never mix the two versions on the same network or assume a live chain can be upgraded without an explicit migration/activation strategy. Preserve any existing chain data before resetting an isolated testnet.

Next: implement consensus-committed CLAIM, RELEASE and REFUND, verify the worker claim and job-bound signed PoUW proof during replay, and audit replay/double-spend protections. The independent verifier introduced in v0.64 is not in itself consensus finality.
