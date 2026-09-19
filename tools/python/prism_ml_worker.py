"""Python worker for Prism's quantized marketplace: direct jobs and watch mode."""
from __future__ import annotations

import argparse
import http.client
import json
from pathlib import Path
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

from prism_ml_protocol import (
    ProtocolError, compact, decode_json, identifier, job_id, load_wallet,
    make_proof, make_task, read_json, uint, validate_job, work_units,
)


class WorkerError(ValueError):
    pass


class HTTPStatusError(WorkerError):
    def __init__(self, status: int, message: str):
        super().__init__(message)
        self.status = status


class TransportError(WorkerError):
    pass


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


class API:
    def __init__(self, base: str, timeout: float = 10.0):
        parsed = urllib.parse.urlsplit(base)
        if (parsed.scheme not in ("http", "https") or not parsed.netloc or
                parsed.username or parsed.password or parsed.query or parsed.fragment):
            raise WorkerError("API URL must be http(s), without credentials, query or fragment")
        if not 0 < timeout <= 300:
            raise WorkerError("timeout must be between 0 and 300 seconds")
        self.base, self.timeout = base.rstrip("/"), timeout
        self.opener = urllib.request.build_opener(NoRedirect())

    @staticmethod
    def encode(payload: dict, limit: int = 65_536) -> bytes:
        encoded = compact(payload)
        if len(encoded) > limit:
            raise WorkerError("request exceeds server body size limit")
        return encoded

    def request(self, method: str, path: str, payload: dict | None = None):
        data = None if payload is None else self.encode(payload, 4096 if path.endswith("/claim") else 65_536)
        request = urllib.request.Request(
            self.base + path, data=data, method=method,
            headers={"Content-Type": "application/json", "Accept": "application/json"},
        )
        try:
            with self.opener.open(request, timeout=self.timeout) as response:
                raw = response.read((1 << 20) + 1)
        except urllib.error.HTTPError as error:
            # Surface a short structured API error for local-node diagnostics.
            # Never dump a whole response or any local wallet content.
            detail = ""
            try:
                error_body = decode_json(error.read(4096))
                if isinstance(error_body, dict) and isinstance(error_body.get("error"), str):
                    detail = ": " + " ".join(error_body["error"].split())[:400]
            except (ProtocolError, OSError):
                pass
            finally:
                error.close()
            raise HTTPStatusError(error.code, f"{method} {path}: HTTP {error.code}{detail}; no automatic retry") from error
        except (urllib.error.URLError, OSError, http.client.HTTPException) as error:
            raise TransportError(f"{method} {path}: connection failed; server outcome may be unknown, no automatic retry") from error
        if len(raw) > 1 << 20:
            raise WorkerError("HTTP response exceeds 1 MiB")
        return decode_json(raw)


def find_job(api: API, selected_id: str) -> dict:
    identifier(selected_id, "job ID")
    response = api.request("GET", "/compute/jobs")
    if not isinstance(response, dict) or not isinstance(response.get("jobs"), list):
        raise WorkerError("invalid job-list response")
    matching = [job for job in response["jobs"] if isinstance(job, dict) and job.get("id") == selected_id]
    if len(matching) != 1:
        raise WorkerError("job absent or duplicated in API response")
    return validate_job(matching[0], selected_id)



def compute_proof_context(api: API, job_id_value: str) -> dict | None:
    identifier(job_id_value, "job ID")
    status = api.request("GET", "/status")
    if not isinstance(status, dict):
        raise WorkerError("invalid node status response")
    if "chainValid" in status and status.get("chainValid") is not True:
        raise WorkerError("node does not confirm a valid chain")
    chain_id = status.get("chainId")
    genesis_value = status.get("genesisHash")
    # Legacy/local test nodes may expose only the old status shape.
    # Keep v1 for those nodes; a current node supplies both fields.
    if chain_id is None or genesis_value is None:
        return None
    if (not isinstance(chain_id, str) or not chain_id.strip() or
            len(chain_id) > 128):
        raise WorkerError("missing or invalid chain identity")
    genesis_hash = identifier(genesis_value, "genesis hash")
    return {
        "job_id": job_id_value,
        "chain_id": chain_id,
        "genesis_hash": genesis_hash,
    }

def checked_receipt(response: object, original: dict, proof: dict) -> dict:
    if not isinstance(response, dict) or response.get("verified") is not True or response.get("settled") is not True:
        raise WorkerError("completion not confirmed: verified and settled must both be true")
    job = validate_job(response.get("job"), original["id"])
    if (job["status"] != "VERIFIED" or job.get("worker") != proof["worker"] or
            job.get("proofId") != proof["id"]):
        raise WorkerError("completion job, worker or proof mismatch")
    bounty = uint(response.get("bountyReward"), "bounty reward")
    if bounty != original["reward"]:
        raise WorkerError("completion bounty differs from the job reward")
    txid = response.get("settlementTxId")
    if not isinstance(txid, str) or not txid.strip():
        raise WorkerError("missing settlement transaction ID")
    block = uint(response.get("block"), "settlement block")
    recovered = response.get("recovered")
    if type(recovered) is not bool:
        raise WorkerError("missing or invalid recovered flag")
    return {
        "verified": True, "settled": True, "jobId": job["id"], "worker": proof["worker"],
        "predictions": proof["result_values"], "score": proof["score"], "proofId": proof["id"],
        "bountyReward": bounty, "settlementTxId": txid, "block": block, "recovered": recovered,
    }


def run_job(api: API, wallet, selected_id: str) -> dict:
    job = find_job(api, selected_id)
    if job["status"] == "VERIFIED":
        raise WorkerError("job is already VERIFIED; no proof was resubmitted")
    if job["status"] == "CLAIMED" and job.get("worker") != wallet.address:
        raise WorkerError("job is claimed by a different worker")
    # Preflight before claiming: bad arithmetic, wallet or oversized proof must
    # not strand an otherwise OPEN job in CLAIMED state.
    # Preflight locally before making the extra status request.
    proof = make_proof(job["task"], wallet)
    body = {"proof": proof}
    API.encode(body)

    context = compute_proof_context(api, job["id"])
    if context is not None:
        proof = make_proof(job["task"], wallet, **context)
        body = {"proof": proof}
        API.encode(body)
    if job["status"] == "OPEN":
        response = api.request("POST", f"/compute/jobs/{selected_id}/claim", {"worker": wallet.address})
        claimed = validate_job(response.get("job") if isinstance(response, dict) else None, selected_id)
        if claimed["status"] != "CLAIMED" or claimed.get("worker") != wallet.address:
            raise WorkerError("claim not confirmed for this wallet")
    response = api.request("POST", f"/compute/jobs/{selected_id}/complete", body)
    return checked_receipt(response, job, proof)


def create_job(api: API, task: dict, requester: str, reward: int, nonce: int) -> dict:
    reward, nonce = uint(reward, "reward", 1), uint(nonce, "nonce")
    expected = job_id(task["id"], requester, reward, nonce)
    body = {"task": task, "requester": requester, "reward": reward, "nonce": nonce}
    API.encode(body)
    print(f"Job ID: {expected}", file=sys.stderr, flush=True)
    response = api.request("POST", "/compute/jobs", body)
    job = validate_job(response.get("job") if isinstance(response, dict) else None, expected)
    if job["status"] != "OPEN":
        raise WorkerError("new job is not OPEN")
    return job


def demo_payload(nonce: int) -> dict:
    uint(nonce, "nonce")
    if nonce > (1 << 63) - 1024:
        raise WorkerError("demo nonce too large for signed bias offset")
    # Different runs need different task/proof IDs, not only different job IDs.
    # A common bias offset preserves the three predicted classes.
    return {
        "batch_size": 3, "features": 2, "classes": 2,
        "inputs": [2, -1, -2, 3, 0, 0], "weights": [2, -1, -1, 2], "biases": [nonce + 1, nonce],
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--api", default="http://127.0.0.1:8080/api/v1")
    parser.add_argument("--data", type=Path, default=Path("data"))
    parser.add_argument("--timeout", type=float, default=10.0)
    sub = parser.add_subparsers(dest="command", required=True)
    identity = sub.add_parser("identity", help="show public wallet identity only")
    identity.add_argument("--wallet", required=True)
    run = sub.add_parser("run", help="process one OPEN job or resume own CLAIMED job")
    run.add_argument("--worker", required=True)
    run.add_argument("--job", required=True)
    create = sub.add_parser("create", help="create a job from a six-field inference payload")
    create.add_argument("--requester", required=True, help="local wallet name")
    create.add_argument("--input", type=Path, required=True)
    create.add_argument("--reward", type=int, required=True)
    create.add_argument("--nonce", type=int)
    demo = sub.add_parser("demo", help="create, claim and complete a small fresh task")
    demo.add_argument("--requester", required=True, help="local wallet name")
    demo.add_argument("--worker", required=True, help="local wallet name")
    demo.add_argument("--reward", type=int, required=True)
    demo.add_argument("--nonce", type=int)
    watch = sub.add_parser("watch", help="process queued quantized jobs with a durable journal")
    watch.add_argument("--worker", required=True)
    watch.add_argument("--poll-interval", type=float, default=2.0)
    watch.add_argument("--journal", type=Path)
    watch.add_argument("--max-jobs", type=int, default=0, help="stop after N new confirmations; 0 means unlimited")
    watch.add_argument("--once", action="store_true", help="perform one queue sweep and exit")
    watch.add_argument("--retry-failed", action="store_true", help="reconsider jobs previously stopped by a permanent error")
    queue = sub.add_parser("enqueue-demo", help="create distinct demo jobs without processing them")
    queue.add_argument("--requester", required=True)
    queue.add_argument("--reward", type=int, required=True)
    queue.add_argument("--count", type=int, default=3)
    queue.add_argument("--nonce", type=int)
    args = parser.parse_args()
    if args.command == "watch":
        from prism_ml_watch import run_watch
        return run_watch(args)
    try:
        if args.command == "identity":
            wallet = load_wallet(args.data, args.wallet)
            output = {"name": wallet.name, "address": wallet.address, "public_key": wallet.public_key}
        elif args.command == "run":
            wallet = load_wallet(args.data, args.worker)
            print(f"Job ID: {args.job}", file=sys.stderr, flush=True)
            output = run_job(API(args.api, args.timeout), wallet, args.job)
        else:
            requester = load_wallet(args.data, args.requester)
            nonce = time.time_ns() if args.nonce is None else args.nonce
            uint(args.reward, "reward", 1)
            uint(nonce, "nonce")
            if args.command == "enqueue-demo":
                if not 1 <= args.count <= 20:
                    raise WorkerError("demo count must be between 1 and 20")
                tasks = [make_task(demo_payload(nonce + index)) for index in range(args.count)]
                api = API(args.api, args.timeout)
                for index, task in enumerate(tasks):
                    job = create_job(api, task, requester.address, args.reward, nonce + index)
                    print(json.dumps({"event": "queued", "jobId": job["id"], "reward": job["reward"]}), flush=True)
                return 0
            if args.command == "demo":
                worker = load_wallet(args.data, args.worker)
                if requester.address == worker.address:
                    raise WorkerError("demo requester and worker must be different wallets")
                task = make_task(demo_payload(nonce))
                # Check calculation and completion-body size before creating a job.
                API.encode({"proof": make_proof(task, worker)})
            else:
                task = make_task(read_json(args.input, 65_536))
            api = API(args.api, args.timeout)
            created = create_job(api, task, requester.address, args.reward, nonce)
            output = run_job(api, worker, created["id"]) if args.command == "demo" else {"job": created}
        print(json.dumps(output, indent=2, ensure_ascii=True))
        return 0
    except (WorkerError, ProtocolError, OSError) as error:
        print(f"ERROR: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
