"""Prism v0.37: local, deterministic quantized linear inference.

This module accepts the six-field *hash payload*, not a network Task or Proof.
Go remains responsible for task IDs, protocol limits and proof validation.
No dependencies beyond Python's standard library (Python >= 3.10).
"""

from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path
import sys

INT64_MIN = -(1 << 63)
INT64_MAX = (1 << 63) - 1
UINT64_MAX = (1 << 64) - 1
FIELDS = ("batch_size", "features", "classes", "inputs", "weights", "biases")
# Local CLI budgets, NOT Prism consensus constants.
CLI_MAX_BYTES = 65_536
CLI_MAX_MULTIPLIES = 1_000_000


class InferenceError(ValueError):
    """Malformed input or arithmetic outside the signed int64 domain."""


def _integer(value: object, lower: int, upper: int, label: str) -> int:
    # bool is an int subclass in Python, but is not a Go JSON integer.
    if type(value) is not int or not lower <= value <= upper:
        raise InferenceError(f"{label}: expected an integer in [{lower}, {upper}]")
    return value


def normalize_payload(payload: object) -> dict:
    if not isinstance(payload, dict) or set(payload) != set(FIELDS):
        raise InferenceError("expected exactly these fields: " + ", ".join(FIELDS))
    batch, features, classes = (
        _integer(payload[name], 1, UINT64_MAX, name) for name in FIELDS[:3]
    )
    expected_lengths = (batch * features, classes * features, classes)
    if max(expected_lengths) > UINT64_MAX:
        raise InferenceError("dimension product exceeds uint64")
    arrays = []
    for name, length in zip(FIELDS[3:], expected_lengths):
        values = payload[name]
        if not isinstance(values, list) or len(values) != length:
            raise InferenceError(f"{name}: expected an array of {length} integers")
        arrays.append([
            _integer(value, INT64_MIN, INT64_MAX, f"{name}[{index}]")
            for index, value in enumerate(values)
        ])
    # Order matches the struct fields in calculateMLInferenceQuantizedInputHash.
    return dict(zip(FIELDS, (batch, features, classes, *arrays)))


def canonical_input_bytes(payload: object) -> bytes:
    normalized = normalize_payload(payload)
    # All strings are fixed ASCII keys and all values are integers/arrays.
    # No spaces, key sorting, floats or trailing newline: match json.Marshal.
    return json.dumps(normalized, separators=(",", ":"), allow_nan=False).encode("ascii")


def input_hash(payload: object) -> str:
    return hashlib.sha256(canonical_input_bytes(payload)).hexdigest()


def infer(payload: object) -> dict:
    task = normalize_payload(payload)
    batch, features, classes = (task[name] for name in FIELDS[:3])
    predictions = []
    for sample in range(batch):
        best_class = 0
        best_score = None
        for class_index in range(classes):
            score = task["biases"][class_index]
            for feature in range(features):
                product = (
                    task["inputs"][sample * features + feature]
                    * task["weights"][class_index * features + feature]
                )
                if not INT64_MIN <= product <= INT64_MAX:
                    raise InferenceError("quantized ML inference multiplication overflow")
                score += product
                if not INT64_MIN <= score <= INT64_MAX:
                    raise InferenceError("quantized ML inference addition overflow")
            # Strict > preserves the first (lowest) class on ties.
            if best_score is None or score > best_score:
                best_class, best_score = class_index, score
        predictions.append(best_class)
    return {"inputHash": input_hash(task), "predictions": predictions}


def _unique_object(pairs: list) -> dict:
    result = {}
    for key, value in pairs:
        if key in result:
            raise InferenceError(f"duplicate JSON key: {key}")
        result[key] = value
    return result


def load_payload(raw: bytes) -> dict:
    try:
        payload = json.loads(raw.decode("utf-8-sig"), object_pairs_hook=_unique_object)
    except (UnicodeError, json.JSONDecodeError) as error:
        raise InferenceError(f"invalid JSON: {error}") from error
    return normalize_payload(payload)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("input", help="JSON hash-payload file, or - for stdin")
    args = parser.parse_args()
    try:
        if args.input == "-":
            raw = sys.stdin.buffer.read(CLI_MAX_BYTES + 1)
        else:
            with Path(args.input).open("rb") as stream:
                raw = stream.read(CLI_MAX_BYTES + 1)
        if len(raw) > CLI_MAX_BYTES:
            raise InferenceError("local CLI input budget exceeded (65536 bytes)")
        task = load_payload(raw)
        operations = task["batch_size"] * task["features"] * task["classes"]
        if operations > CLI_MAX_MULTIPLIES:
            raise InferenceError("local CLI compute budget exceeded (1000000 multiplies)")
        print(json.dumps(infer(task), separators=(",", ":")))
        return 0
    except (OSError, InferenceError) as error:
        print(f"ERROR: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
