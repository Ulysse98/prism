"""Check Python Ed25519 proofs against real Go VerifyProof/Execute/Job.Complete.

Uses only a public deterministic test key, never data/wallets.json.
"""
from __future__ import annotations

import argparse
import copy
import json
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile

from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from prism_ml_protocol import (
    Wallet, address, compact, digest, job_id, make_proof, make_task, proof_id, validate_job,
)
from prism_ml_worker import demo_payload


def test_wallet() -> Wallet:
    key = Ed25519PrivateKey.from_private_bytes(bytes(range(32)))
    public = key.public_key().public_bytes_raw()
    return Wallet("PUBLIC-TEST-KEY", address(public), public.hex(), key)


def resign(proof: dict, wallet: Wallet) -> dict:
    proof["id"] = proof_id(proof)
    proof["signature"] = wallet.private_key.sign(proof["id"].encode("ascii")).hex()
    return proof


def proof_cases():
    wallet = test_wallet()
    payloads = [
        demo_payload(0), demo_payload(37), demo_payload(1_789_834_800_000_000_000),
        {"batch_size": 1, "features": 1, "classes": 2,
         "inputs": [2], "weights": [-10, -2], "biases": [-1, -3]},
        {"batch_size": 1, "features": 1, "classes": 2,
         "inputs": [5], "weights": [-2, -2], "biases": [1, 1]},
        {"batch_size": 1, "features": 1, "classes": 1,
         "inputs": [-(1 << 63)], "weights": [1], "biases": [0]},
    ]
    cases = [(f"valid-{index}", make_proof(make_task(payload), wallet), True)
             for index, payload in enumerate(payloads)]
    original = cases[0][1]
    bad = copy.deepcopy(original)
    bad["result_values"][0] = 1
    bad["output_hash"] = digest(compact(bad["result_values"]))
    cases.append(("false-result-valid-signature", resign(bad, wallet), False))
    for name, key, value in (
        ("wrong-score", "score", original["score"] + 1),
        ("scalar-result", "result", 1),
        ("wrong-output-hash", "output_hash", "0" * 64),
        ("wrong-worker", "worker", "prism_" + "0" * 40),
    ):
        bad = copy.deepcopy(original)
        bad[key] = value
        cases.append((name, resign(bad, wallet), False))
    bad = copy.deepcopy(original)
    bad["signature"] = wallet.private_key.sign(bytes.fromhex(bad["id"])).hex()
    cases.append(("signed-decoded-id-instead-of-text", bad, False))
    bad = copy.deepcopy(original)
    bad["signature"] = "0" * 128
    cases.append(("wrong-signature", bad, False))
    bad = copy.deepcopy(original)
    bad["task"]["biases"][0] += 1
    cases.append(("tampered-task", bad, False))
    bad = copy.deepcopy(original)
    bad["id"] = "0" * 64
    cases.append(("wrong-proof-id", bad, False))
    bad = copy.deepcopy(original)
    bad["public_key"] = "0" * 64
    cases.append(("wrong-public-key", resign(bad, wallet), False))
    return cases


def run(repo: Path) -> int:
    if not (repo / "go.mod").is_file():
        raise ValueError("go.mod absent; run from the Prism repo or pass --repo")
    if not re.search(r'^module\s+(?:prism|"prism")\s*$', (repo / "go.mod").read_text(), re.MULTILINE):
        raise ValueError("expected Go module prism")
    go = shutil.which("go")
    if go is None:
        raise ValueError("Go was not found on PATH")
    cases = proof_cases()
    with tempfile.TemporaryDirectory(prefix=".prism-proof-parity-", dir=repo) as scratch:
        source = Path(scratch) / "main.go"
        source.write_bytes(Path(__file__).with_name("go_proof_reference.go.txt").read_bytes())
        process = subprocess.run(
            [go, "run", "-mod=readonly", str(source)], cwd=repo,
            input=json.dumps([proof for _, proof, _ in cases]), capture_output=True,
            text=True, encoding="utf-8", timeout=180,
        )
    if process.returncode:
        raise ValueError("Go adapter failed:\n" + process.stderr.strip())
    results = json.loads(process.stdout)
    if not isinstance(results, list) or len(results) != len(cases):
        raise ValueError("unexpected Go response count")
    failures = []
    for (name, proof, expected_valid), result in zip(cases, results):
        if not isinstance(result, dict) or result.get("verified") is not expected_valid:
            failures.append(f"{name}: unexpected Go verdict {result}")
            continue
        if expected_valid:
            if result.get("proof") != proof:
                failures.append(f"{name}: Python proof differs from Go Execute (including signature)")
            expected_job_id = job_id(proof["task"]["id"], "test-requester", 1, 37)
            try:
                job = validate_job(result.get("job"), expected_job_id)
                if job["status"] != "VERIFIED" or job.get("proofId") != proof["id"] or job.get("worker") != proof["worker"]:
                    raise ValueError("Go Job.Complete output mismatch")
            except ValueError as error:
                failures.append(f"{name}: {error}")
    if failures:
        for failure in failures:
            print("FAIL " + failure, file=sys.stderr)
        return 1
    valid_count = sum(valid for _, _, valid in cases)
    print(f"PASS: {len(cases)} Python/Go proof cases ({valid_count} identical signed proofs + completed Go jobs, {len(cases)-valid_count} rejected forgeries).")
    print("Next gate: HTTP demo and settlement against your running local node.")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", type=Path, default=Path.cwd())
    args = parser.parse_args()
    try:
        return run(args.repo.resolve())
    except (OSError, ValueError, subprocess.TimeoutExpired) as error:
        print(f"ERROR: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
