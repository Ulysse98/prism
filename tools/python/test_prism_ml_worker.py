"""Protocol tests and HTTP integration with an in-process simulated node.

The separate check_go_proofs.py is the independent Go verification gate.
"""
import contextlib
import copy
from http.server import BaseHTTPRequestHandler, HTTPServer
import io
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import threading
import unittest

from cryptography.exceptions import InvalidSignature
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

from check_go_proofs import proof_cases, test_wallet
from prism_ml_protocol import (
    ProtocolError, address, compact, decode_json, job_id, load_wallet, make_claim,
    make_proof, make_task, task_payload, uint, validate_job, verify_claim, verify_proof,
    wallet_from_record, work_units,
)
from prism_ml_worker import API, WorkerError, create_job, demo_payload, run_job


def record(seed=bytes(range(32)), name="Bob"):
    key = Ed25519PrivateKey.from_private_bytes(seed)
    public = key.public_key().public_bytes_raw()
    return {"name": name, "address": address(public), "public_key": public.hex(),
            "private_key": (seed + public).hex()}


def new_job(task=None, nonce=37):
    task = make_task(demo_payload(37)) if task is None else task
    requester = "prism_" + "1" * 40
    return {"id": job_id(task["id"], requester, 1, nonce), "task": task,
            "requester": requester, "reward": 1, "workUnits": work_units(task),
            "nonce": nonce, "status": "OPEN"}


class ProtocolTests(unittest.TestCase):
    def test_task_roundtrip_and_limits(self):
        p = demo_payload(37)
        self.assertEqual(task_payload(make_task(p)), p)
        self.assertEqual(work_units(make_task(p)), 12)
        p = {"batch_size": 4097, "features": 1, "classes": 1,
             "inputs": [0] * 4097, "weights": [0], "biases": [0]}
        with self.assertRaisesRegex(ProtocolError, "element limit"):
            make_task(p)

    def test_work_limit(self):
        p = {"batch_size": 4096, "features": 1, "classes": 257,
             "inputs": [0] * 4096, "weights": [0] * 257, "biases": [0] * 257}
        with self.assertRaisesRegex(ProtocolError, "work limit"):
            make_task(p)

    def test_exact_work_limit_is_allowed(self):
        p = {"batch_size": 4096, "features": 1, "classes": 256,
             "inputs": [0] * 4096, "weights": [0] * 256, "biases": [0] * 256}
        self.assertEqual(work_units(make_task(p)), 1 << 20)

    def test_task_tampering(self):
        for key, value in (("id", "0" * 64), ("input_hash", "0" * 64),
                           ("type", "sum_squares"), ("values", [1]), ("rows_a", True)):
            task = make_task(demo_payload(37))
            task[key] = value
            with self.subTest(key=key), self.assertRaises(ProtocolError):
                task_payload(task)

    def test_proofs_and_forgeries(self):
        for name, proof, valid in proof_cases():
            with self.subTest(name=name):
                if valid:
                    verify_proof(proof)
                else:
                    with self.assertRaises(ProtocolError):
                        verify_proof(proof)

    def test_signature_is_of_ascii_hex_id(self):
        wallet = test_wallet()
        proof = make_proof(make_task(demo_payload(37)), wallet)
        public = wallet.private_key.public_key()
        signature = bytes.fromhex(proof["signature"])
        public.verify(signature, proof["id"].encode("ascii"))
        with self.assertRaises(InvalidSignature):
            public.verify(signature, bytes.fromhex(proof["id"]))

    def test_deterministic_signature(self):
        wallet, task = test_wallet(), make_task(demo_payload(37))
        self.assertEqual(make_proof(task, wallet), make_proof(task, wallet))

    def test_signed_claim_roundtrip(self):
        wallet = test_wallet()
        job = "a" * 64
        chain = "prism-test-v045"
        genesis = "b" * 64

        claim = make_claim(
            job,
            chain,
            genesis,
            wallet,
        )

        verify_claim(
            claim,
            job_id_value=job,
            chain_id=chain,
            genesis_hash=genesis,
        )

        with self.assertRaises(ProtocolError):
            verify_claim(
                claim,
                job_id_value=job,
                chain_id="prism-other-chain",
                genesis_hash=genesis,
            )

    def test_wallet_seed_suffix_and_address_checks(self):
        original = record()
        self.assertEqual(wallet_from_record(original).address, original["address"])
        for key, value in (("private_key", "0" * 64 + original["public_key"]),
                           ("private_key", original["private_key"][:64] + "0" * 64),
                           ("public_key", "0" * 64), ("address", "prism_" + "0" * 40)):
            bad = dict(original, **{key: value})
            with self.subTest(key=key), self.assertRaises(ProtocolError):
                wallet_from_record(bad)

    def test_wallet_file_and_duplicate_names(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "wallets.json"
            path.write_text(json.dumps([record()]), encoding="utf-8-sig")
            self.assertEqual(load_wallet(Path(directory), "Bob").name, "Bob")
            with self.assertRaises(ProtocolError):
                load_wallet(Path(directory), "Absent")
            path.write_text(json.dumps([record(), record()]))
            with self.assertRaises(ProtocolError):
                load_wallet(Path(directory), "Bob")

    def test_private_key_not_in_repr_or_error(self):
        original = record()
        self.assertNotIn(original["private_key"], repr(wallet_from_record(original)))
        secret_marker = "PRIVATE_MATERIAL_NOT_TO_LOG"
        with self.assertRaises(ProtocolError) as caught:
            wallet_from_record(dict(original, private_key=secret_marker))
        self.assertNotIn(secret_marker, str(caught.exception))

    def test_strict_json_and_uint(self):
        for raw in (b'{"a":1,"a":2}', b'{"a":NaN}', b'{}{}'):
            with self.subTest(raw=raw), self.assertRaises(ProtocolError):
                decode_json(raw)
        for value in (True, False, 1.0, -1, 1 << 64):
            with self.subTest(value=value), self.assertRaises(ProtocolError):
                uint(value, "value")

    def test_job_identity_and_score(self):
        for key, value in (("id", "0" * 64), ("reward", 2), ("nonce", 38),
                           ("workUnits", 13), ("status", "UNKNOWN")):
            bad = dict(new_job(), **{key: value})
            with self.subTest(key=key), self.assertRaises(ProtocolError):
                validate_job(bad)

    def test_demo_uses_new_task_id_without_changing_predictions(self):
        wallet = test_wallet()
        a = make_proof(make_task(demo_payload(37)), wallet)
        b = make_proof(make_task(demo_payload(38)), wallet)
        self.assertNotEqual(a["id"], b["id"])
        self.assertEqual(a["result_values"], [0, 1, 0])
        self.assertEqual(a["result_values"], b["result_values"])


class Node:
    def __init__(self):
        self.job = new_job()
        self.requests = []
        self.override = {}
        self.claim_worker = None
        self.complete_error = False
        self.redirect = False
        self.large_response = False
        self.received_proof = None
        self.identity = {
            "chainId": "prism-test-v045",
            "genesisHash": "a" * 64,
            "chainValid": True,
        }


@contextlib.contextmanager
def node_server():
    node = Node()

    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *args):
            pass

        def send_json(self, payload, status=200):
            body = compact(payload)
            self.send_response(status)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def do_GET(self):
            node.requests.append(("GET", self.path))
            if node.redirect:
                self.send_response(302)
                self.send_header("Location", "/redirect-target")
                self.send_header("Content-Length", "0")
                self.end_headers()
                return
            if self.path.endswith("/status"):
                self.send_json(node.identity)
            else:
                self.send_json({
                    "jobs": [node.job],
                    "padding": "x" * (1 << 20) if node.large_response else "",
                })

        def do_POST(self):
            node.requests.append(("POST", self.path))
            body = decode_json(self.rfile.read(int(self.headers["Content-Length"])))
            if self.path.endswith("/claim"):
                try:
                    verify_claim(
                        body,
                        job_id_value=node.job["id"],
                        chain_id=node.identity["chainId"],
                        genesis_hash=node.identity["genesisHash"],
                    )
                except ProtocolError:
                    self.send_json(
                        {"error": "invalid signed claim"},
                        401,
                    )
                    return

                if node.job["status"] != "OPEN":
                    self.send_json({"error": "not open"}, 400)
                    return

                node.job.update(
                    status="CLAIMED",
                    worker=body["worker"],
                )
                returned = copy.deepcopy(node.job)
                if node.claim_worker:
                    returned["worker"] = node.claim_worker
                self.send_json({"job": returned})
            elif self.path.endswith("/complete"):
                if node.complete_error:
                    self.send_json({"error": "simulated failure"}, 400)
                    return
                proof = body["proof"]
                try:
                    verify_proof(proof)
                    if node.job["status"] != "CLAIMED" or proof["worker"] != node.job["worker"] or proof["task"]["id"] != node.job["task"]["id"]:
                        raise ProtocolError("job mismatch")
                except ProtocolError:
                    self.send_json({"error": "invalid proof"}, 400)
                    return
                node.received_proof = proof
                node.job.update(status="VERIFIED", proofId=proof["id"])
                response = {"verified": True, "settled": True, "job": copy.deepcopy(node.job),
                            "bountyReward": node.job["reward"], "settlementTxId": "test-settlement",
                            "block": 43, "recovered": False}
                response.update(node.override)
                for key in list(response):
                    if response[key] == "REMOVE_FIELD":
                        del response[key]
                self.send_json(response)
            elif self.path.endswith("/compute/jobs"):
                node.job = {"task": body["task"], "requester": body["requester"], "reward": body["reward"],
                            "nonce": body["nonce"], "workUnits": work_units(body["task"]), "status": "OPEN"}
                node.job["id"] = job_id(body["task"]["id"], body["requester"], body["reward"], body["nonce"])
                self.send_json({"job": node.job}, 201)
            else:
                self.send_json({"error": "not found"}, 404)

    server = HTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, kwargs={"poll_interval": 0.01}, daemon=True)
    thread.start()
    try:
        yield node, API(f"http://127.0.0.1:{server.server_port}/api/v1")
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=2)


class HTTPWorkerTests(unittest.TestCase):
    def test_demo_cli_with_local_wallet_file(self):
        with node_server() as (node, api), tempfile.TemporaryDirectory() as directory:
            alice = record(bytes(reversed(range(32))), "Alice")
            bob = record()
            Path(directory, "wallets.json").write_text(json.dumps([alice, bob]))
            result = subprocess.run(
                [sys.executable, str(Path(__file__).with_name("prism_ml_worker.py")),
                 "--api", api.base, "--data", directory, "demo", "--requester", "Alice",
                 "--worker", "Bob", "--reward", "1", "--nonce", "123"],
                capture_output=True, text=True, timeout=15,
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            receipt = json.loads(result.stdout)
            self.assertEqual(receipt["predictions"], [0, 1, 0])
            self.assertIs(receipt["settled"], True)
            self.assertEqual(receipt["worker"], bob["address"])
            self.assertNotIn(bob["private_key"], result.stdout + result.stderr)
            self.assertNotIn(alice["private_key"], result.stdout + result.stderr)

    def test_open_claim_complete(self):
        with node_server() as (node, api):
            receipt = run_job(api, test_wallet(), node.job["id"])
            self.assertIs(receipt["verified"], True)
            self.assertIs(receipt["settled"], True)
            self.assertEqual(receipt["predictions"], [0, 1, 0])
            self.assertEqual(receipt["bountyReward"], 1)
            self.assertEqual([method for method, _ in node.requests], ["GET", "GET", "POST", "POST"])
            self.assertNotIn("private_key", compact(node.received_proof).decode())

    def test_create_and_process(self):
        with node_server() as (node, api), contextlib.redirect_stderr(io.StringIO()):
            created = create_job(api, make_task(demo_payload(123)), "prism_" + "2" * 40, 1, 123)
            receipt = run_job(api, test_wallet(), created["id"])
            self.assertEqual(receipt["jobId"], created["id"])
            self.assertEqual(receipt["score"], 12)

    def test_resume_own_claimed_job(self):
        with node_server() as (node, api):
            node.job.update(status="CLAIMED", worker=test_wallet().address)
            run_job(api, test_wallet(), node.job["id"])
            self.assertFalse(any(path.endswith("/claim") for _, path in node.requests))

    def test_other_worker_refused_before_post(self):
        with node_server() as (node, api):
            node.job.update(status="CLAIMED", worker="prism_" + "2" * 40)
            with self.assertRaises(WorkerError):
                run_job(api, test_wallet(), node.job["id"])
            self.assertEqual(len(node.requests), 1)

    def test_already_verified_refused_before_post(self):
        with node_server() as (node, api):
            node.job.update(status="VERIFIED", worker=test_wallet().address, proofId="0" * 64)
            with self.assertRaises(WorkerError):
                run_job(api, test_wallet(), node.job["id"])
            self.assertEqual(len(node.requests), 1)

    def test_overflow_refused_before_claim(self):
        with node_server() as (node, api):
            task = make_task({"batch_size": 1, "features": 1, "classes": 1,
                              "inputs": [-(1 << 63)], "weights": [-1], "biases": [0]})
            node.job = new_job(task)
            with self.assertRaisesRegex(ProtocolError, "overflow"):
                run_job(api, test_wallet(), node.job["id"])
            self.assertEqual(node.job["status"], "OPEN")
            self.assertEqual(len(node.requests), 1)

    def test_tampered_task_refused_before_post(self):
        with node_server() as (node, api):
            node.job["task"]["biases"][0] += 1
            with self.assertRaises(ProtocolError):
                run_job(api, test_wallet(), node.job["id"])
            self.assertEqual(len(node.requests), 1)

    def test_claim_response_worker_checked(self):
        with node_server() as (node, api):
            node.claim_worker = "prism_" + "2" * 40
            with self.assertRaises(WorkerError):
                run_job(api, test_wallet(), node.job["id"])
            self.assertFalse(any(path.endswith("/complete") for _, path in node.requests))

    def test_verified_and_settled_must_be_literal_true(self):
        for key in ("verified", "settled"):
            for value in (False, 1, "true", "REMOVE_FIELD"):
                with self.subTest(key=key, value=value), node_server() as (node, api):
                    node.override[key] = value
                    with self.assertRaises(WorkerError):
                        run_job(api, test_wallet(), node.job["id"])

    def test_receipt_fields_checked(self):
        for key, value in (("bountyReward", 2), ("bountyReward", True), ("settlementTxId", ""),
                           ("block", True), ("recovered", "REMOVE_FIELD")):
            with self.subTest(key=key), node_server() as (node, api):
                node.override[key] = value
                with self.assertRaises((WorkerError, ProtocolError)):
                    run_job(api, test_wallet(), node.job["id"])

    def test_receipt_proof_checked(self):
        with node_server() as (node, api):
            bad = copy.deepcopy(node.job)
            bad.update(status="VERIFIED", worker=test_wallet().address, proofId="0" * 64)
            node.override["job"] = bad
            with self.assertRaises(WorkerError):
                run_job(api, test_wallet(), node.job["id"])

    def test_http_error_is_not_retried(self):
        with node_server() as (node, api):
            node.complete_error = True
            with self.assertRaisesRegex(WorkerError, "HTTP 400"):
                run_job(api, test_wallet(), node.job["id"])
            self.assertEqual(sum(path.endswith("/complete") for _, path in node.requests), 1)

    def test_redirect_not_followed(self):
        with node_server() as (node, api):
            node.redirect = True
            with self.assertRaisesRegex(WorkerError, "HTTP 302"):
                run_job(api, test_wallet(), node.job["id"])
            self.assertEqual(len(node.requests), 1)

    def test_response_size_limit(self):
        with node_server() as (node, api):
            node.large_response = True
            with self.assertRaisesRegex(WorkerError, "1 MiB"):
                run_job(api, test_wallet(), node.job["id"])

    def test_request_size_limit(self):
        with self.assertRaisesRegex(WorkerError, "body size"):
            API.encode({"value": "x" * 65_536})


if __name__ == "__main__":
    unittest.main()
