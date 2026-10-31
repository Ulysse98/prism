# PRISM v0.65 — Atomic BFT State Storage Lab (2C.5F-B.3A)

**EXPERIMENTAL STORAGE ONLY. NOT A SIGNING GATE. NOT BFT FINALITY.**

This Linux/WSL laboratory stores an entire BFT snapshot and its signed-vote history in a single versioned safety record. It serializes cooperating processes with `flock`, validates file permissions and bindings, creates a temporary file in the same directory, syncs it, renames it, and syncs the directory.

All storage functions are package-private, and the new code does not sign votes or change consensus, blockchain, or P2P paths. Explicit initialization requires a newly created 0700 directory. A normal read/update cannot initialize a missing or corrupted record.

**Crucial missing invariant:** structural validation of a lock snapshot and Ed25519 signature verification do not prove an *authorized transition*. The test-only storage mutator can produce internally consistent but unjustified state. Before any private key can use this layer, the gate must replay or independently validate full transition evidence, bind vote generation to the resulting lock state, handle NIL and cross-round proofs, and control all uses of the validator key.

A privileged user can delete or roll back the entire directory and explicitly initialize it again, or use the key elsewhere. This layer does not prevent that. The tests for before/after rename faults are process-level fault injections, **not power-loss or filesystem-crash proofs**. In particular, a post-rename failure can leave the new record visible despite the caller receiving an error. Recovery must reload, not assume rollback.

Tested cases: initialization, restart, lock snapshot storage, wrong identity, invalid permission, corruption, symlink denial, signed vote binding, duplicate slot, invalid signature, injected failure before and after rename, and 12 subprocesses updating one record under one exclusive lock.

Do not enable this code in active PRISM consensus or treat it as a production signing API.
