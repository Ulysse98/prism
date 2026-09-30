#!/usr/bin/env python3

import json
import sys

import cudaq


@cudaq.kernel
def bell():
    q = cudaq.qvector(2)
    h(q[0])
    x.ctrl(q[0], q[1])
    mz(q)


def fail(message: str, code: int = 1) -> None:
    print(
        json.dumps(
            {
                "ok": False,
                "error": message,
            },
            separators=(",", ":"),
        )
    )
    raise SystemExit(code)


def main() -> None:
    try:
        request = json.load(sys.stdin)
    except Exception as exc:
        fail(f"invalid request JSON: {exc}")

    circuit = request.get("circuit")
    qubits = request.get("qubits")
    shots = request.get("shots")
    target = request.get("target", "nvidia")

    if circuit != "bell":
        fail(f"unsupported circuit: {circuit}")

    if qubits != 2:
        fail("bell circuit requires exactly 2 qubits")

    if not isinstance(shots, int) or shots <= 0:
        fail("shots must be a positive integer")

    if shots > 1_000_000:
        fail("shots exceed maximum")

    try:
        cudaq.set_target(target)
    except Exception as exc:
        fail(f"unable to select CUDA-Q target {target}: {exc}")

    try:
        result = cudaq.sample(
            bell,
            shots_count=shots,
        )
    except Exception as exc:
        fail(f"CUDA-Q execution failed: {exc}")

    counts = {
        "00": int(result.count("00")),
        "01": int(result.count("01")),
        "10": int(result.count("10")),
        "11": int(result.count("11")),
    }

    total = sum(counts.values())

    if total != shots:
        fail(
            f"CUDA-Q returned {total} measurements, expected {shots}"
        )

    response = {
        "ok": True,
        "backend": f"cudaq:{target}",
        "workload": "bell",
        "shots": shots,
        "counts": counts,
    }

    print(
        json.dumps(
            response,
            separators=(",", ":"),
            sort_keys=True,
        )
    )


if __name__ == "__main__":
    main()
