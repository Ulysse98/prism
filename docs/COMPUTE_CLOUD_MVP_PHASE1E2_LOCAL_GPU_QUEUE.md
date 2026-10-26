# Prism Compute Cloud — Phase 1E.2 (operator-only GPU queue)

This lab composes the Phase 1D.4 pinned-node offline broker and the Phase
1E.1 bounded single-worker scheduler without changing any existing files or
registering any new HTTP routes. It does not sign claims, write blockchain
state, create marketplace jobs, issue payouts, or access API credentials.

## Safety and limitations

- **Explicit operator approval**: `-enable-local-queue-gpu` AND a separate
  comma-separated exact canonical task ID for each input, in array order.
- **All-or-nothing acceptance**: the CLI prints JSON only if all tasks return
  validated results. This is not a transactional rollback of GPU execution.
- **Local concurrency**: ONE worker callback per scheduler instance, 2..4
  distinct jobs, maximum four waiting slots, 30-second scheduler timeout per
  task and a 20-second CUDA child timeout. The CUDA executable is SHA-256 pinned.
- **Node**: locally pinned chain identity, read-only status before/after each
  CUDA execution. Unchanged observed height does not prove no chain mutations.
- **Verification**: each result is independently recalculated via Prism Go
  `ComputeMLInferenceQuantized`; only matching outputs are accepted.
- **No multi-tenant security**: no global GPU mutex, cross-process quotas,
  process sandbox, fairness, billing, persistence or crash-recovery queue.
- **No server dispatch**: `prism-cloud serve` does not use this package.

## Input

Standard input must be an array of 2 to 4 regular Prism Cloud compute request
objects with exactly 3 features and 3 classes. The CLI verifies canonical
IDs, rejects duplicate tasks and does not accept per-task approvals inside
input. `-confirm-task-ids` is a separate operator-supplied option.

## Example

```sh
cat batch.json | go run ./cmd/prism-cloud-gpu-queue-lab \
  -enable-local-queue-gpu \
  -confirm-task-ids "$TASK_ID_1,$TASK_ID_2" \
  -gpu-bin "$GPU_BIN" -sha256 "$GPU_SHA256" \
  -node-port 18065 -chain-id "$TEST_CHAIN_ID" \
  -genesis-hash "$TEST_GENESIS_HASH"
```

Expected outer response: `mode=operator_local_gpu_queue_lab`,
`serial_per_scheduler=true`, 2 validated `results`, and false for
`http_dispatch_enabled`, `escrow_enabled`, `proofs_available`.

This mode never requests hardware attestation: `gpu_attestation` is false.
