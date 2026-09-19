"""Compare Python with the actual internal/usefulwork package in a Prism repo.

Uses small numeric cases, not a full test of network Task/Proof validation.
Creates one temporary Go program in the repo and removes it afterwards.
"""

from __future__ import annotations

import argparse
import json
from pathlib import Path
import random
import re
import shutil
import subprocess
import sys
import tempfile

from prism_ml_runtime import InferenceError, INT64_MAX, INT64_MIN, infer


def task(inputs, weights, biases, batch=1, features=1, classes=1):
    return dict(batch_size=batch, features=features, classes=classes,
                inputs=inputs, weights=weights, biases=biases)


def cases():
    example = json.loads(Path(__file__).with_name("example_quantized.json").read_text())
    directed = [
        ("signed-batch", example),
        ("class-major-layout", task([3, 4], [1, 0, 0, 2, -1, -1], [0, 0, 0], features=2, classes=3)),
        ("negative-scores", task([2], [-10, -2], [-1, -3], classes=2)),
        ("negative-tie", task([5], [-2, -2, -2], [1, 1, 1], classes=3)),
        ("bias-decides", task([0], [1, 1], [-1, 1], classes=2)),
        ("positive-bias-boundary", task([0], [1], [INT64_MAX])),
        ("negative-bias-boundary", task([0], [1], [INT64_MIN])),
        ("mul-positive-overflow", task([INT64_MAX], [2], [0])),
        ("mul-negative-overflow", task([INT64_MIN], [2], [0])),
        ("mul-min-neg-one", task([INT64_MIN], [-1], [0])),
        ("mul-neg-one-min", task([-1], [INT64_MIN], [0])),
        ("add-positive-overflow", task([1], [1], [INT64_MAX])),
        ("add-negative-overflow", task([-1], [1], [INT64_MIN])),
        ("intermediate-positive-overflow", task([1, -1], [1, 1], [INT64_MAX], features=2)),
        ("intermediate-negative-overflow", task([-1, 1], [1, 1], [INT64_MIN], features=2)),
        ("losing-class-overflow", task([INT64_MIN], [0, -1], [10, 0], classes=2)),
        ("missing-input", task([], [1], [0])),
        ("missing-weight", task([1], [], [0])),
        ("missing-bias", task([1], [1], [])),
        ("zero-batch", task([], [1], [0], batch=0)),
        ("zero-features", task([], [], [0], features=0)),
        ("zero-classes", task([1], [], [], classes=0)),
    ]
    for value_name, value in (("min", INT64_MIN), ("max", INT64_MAX)):
        for weight in (0, 1):
            directed.append((f"boundary-{value_name}-times-{weight}", task([value], [weight], [0])))
    rng = random.Random(37)
    for index in range(100):
        batch, features, classes = (rng.randint(1, 5) for _ in range(3))
        directed.append((f"seeded-{index:03d}", task(
            [rng.randint(-100, 100) for _ in range(batch * features)],
            [rng.randint(-100, 100) for _ in range(classes * features)],
            [rng.randint(-100, 100) for _ in range(classes)],
            batch=batch, features=features, classes=classes,
        )))
    return directed


def error_kind(message: str) -> str:
    for kind in ("multiplication overflow", "addition overflow"):
        if kind in message:
            return kind
    return "invalid input"


def run(repo: Path) -> int:
    module_file = repo / "go.mod"
    if not module_file.is_file():
        raise ValueError("go.mod absent: run from C:\\Users\\ulyss\\prism or pass --repo")
    if not re.search(r'^module\s+(?:prism|"prism")\s*$', module_file.read_text(), re.MULTILINE):
        raise ValueError("this adapter expects the Go module named prism")
    go = shutil.which("go")
    if go is None:
        raise ValueError("Go was not found on PATH")
    vectors = cases()
    # Internal imports are allowed because the temporary file lives in Prism.
    with tempfile.TemporaryDirectory(prefix=".prism-python-parity-", dir=repo) as scratch:
        source_path = Path(scratch) / "main.go"
        source_path.write_bytes(Path(__file__).with_name("go_reference.go.txt").read_bytes())
        process = subprocess.run(
            [go, "run", "-mod=readonly", str(source_path)], cwd=repo,
            input=json.dumps([value for _, value in vectors]),
            capture_output=True, text=True, encoding="utf-8", timeout=180,
        )
    if process.returncode:
        raise ValueError("Go adapter failed:\n" + process.stderr.strip())
    if process.stderr.strip():
        print(process.stderr.strip(), file=sys.stderr)
    try:
        references = json.loads(process.stdout)
    except json.JSONDecodeError as error:
        raise ValueError("Go adapter did not return valid JSON: " + process.stdout[:500]) from error
    if not isinstance(references, list) or len(references) != len(vectors):
        raise ValueError("unexpected number of Go results")
    failed, accepted, rejected = [], 0, 0
    for (name, value), reference in zip(vectors, references):
        if not isinstance(reference, dict):
            raise ValueError(f"invalid Go response for {name}")
        try:
            result = infer(value)
            python_error = None
        except InferenceError as error:
            result, python_error = None, str(error)
        go_error = reference.get("error")
        if python_error is not None:
            if not go_error or error_kind(go_error) != error_kind(python_error):
                failed.append(f"{name}: Python={python_error!r}; Go={reference!r}")
            else:
                rejected += 1
        elif go_error or result != reference:
            failed.append(f"{name}: Python={result!r}; Go={reference!r}")
        else:
            accepted += 1
    if failed:
        for failure in failed:
            print("FAIL " + failure, file=sys.stderr)
        print(f"FAILED: {len(failed)}/{len(vectors)} comparisons", file=sys.stderr)
        return 1
    print(f"PASS: {len(vectors)} Python/Go cases ({accepted} predictions+hashes, {rejected} matching rejections).")
    print("Numeric/hash parity only; HTTP claims, proofs, signatures and settlement remain to be integrated.")
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
