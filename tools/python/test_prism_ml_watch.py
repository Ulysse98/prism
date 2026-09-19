"""Fault-injected HTTP and real process-restart tests; this server is not Go.

Go settlement retry behavior is independently covered by the existing
TestSettleComputeJobRetryDoesNotDoublePay test in the Prism repository.
"""
import contextlib
import copy
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import signal
import socket
import subprocess
import sys
import tempfile
import threading
import unittest
from unittest.mock import patch

from check_go_proofs import test_wallet
from prism_ml_protocol import compact, decode_json, job_id, make_proof, make_task, verify_proof
from prism_ml_watch import Journal, JournalError, Watcher, journal_lock, network_identity
from prism_ml_worker import API, demo_payload
from test_prism_ml_worker import new_job, record


class QueueNode:
    def __init__(self, count=3):
        jobs = [new_job(make_task(demo_payload(100 + n)), nonce=100 + n) for n in range(count)]
        self.jobs = {job["id"]: job for job in jobs}
        self.payments = {}
        self.requests = []
        self.identity = {"chainId": "prism-test-v038", "genesisHash": "a" * 64, "chainValid": True}
        self.fault = None
        self.receipt_override = {}
        self.committed = threading.Event()
        self.release = threading.Event()


@contextlib.contextmanager
def queue_server(count=3):
    node = QueueNode(count)

    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *args):
            pass

        def send_json(self, body, status=200):
            raw = compact(body)
            self.send_response(status)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(raw)))
            self.end_headers()
            try:
                self.wfile.write(raw)
            except (BrokenPipeError, ConnectionResetError):
                pass

        def drop(self):
            self.close_connection = True
            self.connection.shutdown(socket.SHUT_RDWR)
            self.connection.close()

        def do_GET(self):
            node.requests.append(("GET", self.path))
            if self.path.endswith("/status"):
                self.send_json(node.identity)
            elif self.path.endswith("/work"):
                self.send_json({"entries": [{"proofId": proof_id} for proof_id in node.payments]})
            else:
                self.send_json({"jobs": list(node.jobs.values())})

        def do_POST(self):
            node.requests.append(("POST", self.path))
            body = decode_json(self.rfile.read(int(self.headers["Content-Length"])))
            if self.path.endswith("/compute/jobs"):
                job = new_job(body["task"], body["nonce"])
                # Demo test rewards are 1, and new_job uses a fixture requester.
                from prism_ml_protocol import job_id
                job.update(requester=body["requester"], reward=body["reward"])
                job["id"] = job_id(job["task"]["id"], job["requester"], job["reward"], job["nonce"])
                if job["id"] in node.jobs:
                    self.send_json({"error": "already exists"}, 400)
                    return
                node.jobs[job["id"]] = job
                self.send_json({"job": job}, 201)
                return
            job = node.jobs[self.path.split("/")[-2]]
            if self.path.endswith("/claim"):
                if node.fault == "race":
                    node.fault = None
                    job.update(status="CLAIMED", worker="prism_" + "2" * 40)
                if job["status"] != "OPEN":
                    self.send_json({"error": "not open"}, 400)
                    return
                job.update(status="CLAIMED", worker=body["worker"])
                if node.fault == "drop_claim":
                    node.fault = None
                    self.drop()
                    return
                self.send_json({"job": job})
                return
            proof = body["proof"]
            verify_proof(proof)
            if proof["worker"] != job.get("worker") or proof["task"]["id"] != job["task"]["id"]:
                self.send_json({"error": "mismatch"}, 400)
                return
            if node.fault in ("503", "400"):
                status = int(node.fault)
                node.fault = None
                self.send_json({"error": "simulated failure"}, status)
                return
            recovered = proof["id"] in node.payments
            if not recovered:
                node.payments[proof["id"]] = {"tx": str(len(node.payments) + 1).zfill(64),
                                                "block": 50 + len(node.payments), "reward": job["reward"]}
            payment = node.payments[proof["id"]]
            if node.fault != "drop_before_market_save":
                job.update(status="VERIFIED", proofId=proof["id"])
            if node.fault in ("drop_complete", "drop_before_market_save"):
                node.fault = None
                self.drop()
                return
            if node.fault == "hang_complete":
                node.fault = None
                node.committed.set()
                node.release.wait(timeout=10)
            response = {"verified": True, "settled": True, "job": copy.deepcopy(job),
                        "bountyReward": payment["reward"], "settlementTxId": payment["tx"],
                        "block": payment["block"], "recovered": recovered}
            response.update(node.receipt_override)
            self.send_json(response)

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    server.daemon_threads = True
    thread = threading.Thread(target=server.serve_forever, kwargs={"poll_interval": 0.01}, daemon=True)
    thread.start()
    try:
        yield node, API(f"http://127.0.0.1:{server.server_port}/api/v1")
    finally:
        node.release.set()
        server.shutdown()
        server.server_close()
        thread.join(timeout=2)


class WatchTests(unittest.TestCase):
    def setup_watcher(self, api, directory):
        wallet = test_wallet()
        journal = Journal(Path(directory) / "watch.json", network_identity(api, wallet.address))
        events = []
        self.now = 1000.0
        watcher = Watcher(api, wallet, journal, clock=lambda: self.now,
                          output=lambda event, **fields: events.append({"event": event, **fields}))
        return watcher, journal, events

    def test_three_distinct_jobs_and_no_repayment_on_second_sweep(self):
        with queue_server() as (node, api), tempfile.TemporaryDirectory() as directory:
            watcher, journal, events = self.setup_watcher(api, directory)
            self.assertEqual(watcher.sweep(), 3)
            self.assertEqual(len(node.payments), 3)
            self.assertEqual(watcher.sweep(), 0)
            self.assertEqual(len(node.payments), 3)
            self.assertEqual(len([e for e in events if e["event"] == "confirmed"]), 3)
            saved = Journal(journal.path, journal.identity)
            self.assertTrue(all(e["phase"] == "confirmed" for e in saved.jobs.values()))
            self.assertEqual(sum(e["receipt"]["bountyReward"] for e in saved.jobs.values()), 3)
            self.assertNotIn(record()["private_key"], journal.path.read_text())

    def check_lost_response(self, fault, expected_status):
        with queue_server(1) as (node, api), tempfile.TemporaryDirectory() as directory:
            node.fault = fault
            watcher, journal, events = self.setup_watcher(api, directory)
            self.assertEqual(watcher.sweep(), 0)
            self.assertEqual(next(iter(node.jobs.values()))["status"], expected_status)
            self.assertEqual(watcher.sweep(), 0)  # persistent backoff
            self.now += 100
            restarted = Watcher(api, test_wallet(), Journal(journal.path, journal.identity),
                                clock=lambda: self.now, output=lambda *a, **k: None)
            self.assertEqual(restarted.sweep(), 1)
            self.assertEqual(len(node.payments), 1)
            self.assertEqual(sum(path.endswith("/claim") for method, path in node.requests if method == "POST"), 1)

    def test_lost_claim_response_resumes_after_restart(self):
        self.check_lost_response("drop_claim", "CLAIMED")

    def test_lost_settlement_response_recovers_verified_job(self):
        self.check_lost_response("drop_complete", "VERIFIED")

    def test_block_saved_but_market_state_not_saved_recovers(self):
        self.check_lost_response("drop_before_market_save", "CLAIMED")

    def test_resume_own_claimed_job_without_a_journal(self):
        with queue_server(1) as (node, api), tempfile.TemporaryDirectory() as directory:
            next(iter(node.jobs.values())).update(status="CLAIMED", worker=test_wallet().address)
            watcher, _, _ = self.setup_watcher(api, directory)
            self.assertEqual(watcher.sweep(), 1)
            self.assertFalse(any(path.endswith("/claim") for _, path in node.requests))

    def test_other_worker_and_unrelated_verified_jobs_are_ignored(self):
        with queue_server(2) as (node, api), tempfile.TemporaryDirectory() as directory:
            jobs = list(node.jobs.values())
            jobs[0].update(status="CLAIMED", worker="prism_" + "2" * 40)
            jobs[1].update(status="VERIFIED", worker=test_wallet().address, proofId="b" * 64)
            watcher, _, _ = self.setup_watcher(api, directory)
            self.assertEqual(watcher.sweep(), 0)
            self.assertFalse(any(method == "POST" for method, _ in node.requests))

    def test_racing_claim_is_not_completed_by_wrong_worker(self):
        with queue_server(1) as (node, api), tempfile.TemporaryDirectory() as directory:
            node.fault = "race"
            watcher, journal, _ = self.setup_watcher(api, directory)
            self.assertEqual(watcher.sweep(), 0)
            self.assertEqual(next(iter(journal.jobs.values()))["phase"], "failed")
            self.assertFalse(any(path.endswith("/complete") for _, path in node.requests))

    def test_http_503_retried_after_reconciliation(self):
        with queue_server(1) as (node, api), tempfile.TemporaryDirectory() as directory:
            node.fault = "503"
            watcher, _, _ = self.setup_watcher(api, directory)
            self.assertEqual(watcher.sweep(), 0)
            self.now += 100
            self.assertEqual(watcher.sweep(), 1)
            self.assertEqual(len(node.payments), 1)

    def test_permanent_failure_requires_explicit_retry(self):
        with queue_server(1) as (node, api), tempfile.TemporaryDirectory() as directory:
            node.fault = "400"
            watcher, journal, _ = self.setup_watcher(api, directory)
            self.assertEqual(watcher.sweep(), 0)
            self.now += 100
            self.assertEqual(watcher.sweep(), 0)
            journal.retry_failed()
            self.assertEqual(watcher.sweep(), 1)

    def test_invalid_receipt_is_not_confirmed_and_can_be_recovered(self):
        with queue_server(1) as (node, api), tempfile.TemporaryDirectory() as directory:
            node.receipt_override["settled"] = "true"
            watcher, journal, _ = self.setup_watcher(api, directory)
            self.assertEqual(watcher.sweep(), 0)
            self.assertNotEqual(next(iter(journal.jobs.values()))["phase"], "confirmed")
            node.receipt_override.clear()
            self.now += 100
            self.assertEqual(watcher.sweep(), 1)
            self.assertEqual(len(node.payments), 1)

    def test_overflow_job_does_not_starve_valid_job(self):
        with queue_server(1) as (node, api), tempfile.TemporaryDirectory() as directory:
            bad = new_job(make_task({"batch_size": 1, "features": 1, "classes": 1,
                                     "inputs": [-(1 << 63)], "weights": [-1], "biases": [0]}))
            node.jobs[bad["id"]] = bad
            watcher, journal, _ = self.setup_watcher(api, directory)
            self.assertEqual(watcher.sweep(), 1)
            self.assertEqual(bad["status"], "OPEN")
            self.assertEqual(journal.jobs[bad["id"]]["phase"], "failed")

    def test_proof_reuse_is_rejected_before_claim(self):
        with queue_server(1) as (node, api), tempfile.TemporaryDirectory() as directory:
            first = next(iter(node.jobs.values()))
            another = new_job(first["task"], first["nonce"] + 1)
            node.jobs[another["id"]] = another
            watcher, _, _ = self.setup_watcher(api, directory)
            self.assertEqual(watcher.sweep(), 1)
            self.assertEqual(len(node.payments), 1)
            self.assertEqual(sum(j["status"] == "OPEN" for j in node.jobs.values()), 1)

    def test_existing_chain_proof_is_rejected_without_prior_journal(self):
        with queue_server(1) as (node, api), tempfile.TemporaryDirectory() as directory:
            job = next(iter(node.jobs.values()))
            proof = make_proof(job["task"], test_wallet())
            node.payments[proof["id"]] = {"tx": "1" * 64, "block": 12, "reward": 1}
            watcher, _, _ = self.setup_watcher(api, directory)
            self.assertEqual(watcher.sweep(), 0)
            self.assertEqual(job["status"], "OPEN")
            self.assertFalse(any(method == "POST" for method, _ in node.requests))

    def test_duplicate_failed_entry_does_not_block_original_recovery(self):
        with queue_server(1) as (node, api), tempfile.TemporaryDirectory() as directory:
            first = next(iter(node.jobs.values()))
            another = new_job(first["task"], first["nonce"] + 1)
            node.jobs[another["id"]] = another
            node.fault = "503"
            watcher, _, _ = self.setup_watcher(api, directory)
            self.assertEqual(watcher.sweep(), 0)
            self.now += 100
            self.assertEqual(watcher.sweep(), 1)
            self.assertEqual(len(node.payments), 1)

    def test_self_requested_jobs_are_not_claimed_by_address_or_name(self):
        for requester in (test_wallet().address, test_wallet().name.lower()):
            with self.subTest(requester=requester), queue_server(1) as (node, api), tempfile.TemporaryDirectory() as directory:
                job = next(iter(node.jobs.values()))
                job["requester"] = requester
                job["id"] = job_id(job["task"]["id"], requester, job["reward"], job["nonce"])
                node.jobs = {job["id"]: job}
                watcher, _, _ = self.setup_watcher(api, directory)
                self.assertEqual(watcher.sweep(), 0)
                self.assertFalse(any(method == "POST" for method, _ in node.requests))

    def test_wrong_chain_fails_before_mutation(self):
        with queue_server(1) as (node, api), tempfile.TemporaryDirectory() as directory:
            watcher, _, _ = self.setup_watcher(api, directory)
            node.identity["genesisHash"] = "c" * 64
            with self.assertRaises(JournalError):
                watcher.sweep()
            self.assertFalse(any(method == "POST" for method, _ in node.requests))

    def test_failed_journal_write_prevents_claim(self):
        with queue_server(1) as (node, api), tempfile.TemporaryDirectory() as directory:
            watcher, journal, _ = self.setup_watcher(api, directory)
            with patch.object(journal, "save", side_effect=JournalError("disk full")):
                with self.assertRaises(JournalError):
                    watcher.sweep()
            self.assertFalse(any(method == "POST" for method, _ in node.requests))

    def test_failed_confirmation_save_recovers_once_from_durable_pending_state(self):
        with queue_server(1) as (node, api), tempfile.TemporaryDirectory() as directory:
            watcher, journal, _ = self.setup_watcher(api, directory)
            original = journal.save
            def save():
                if any(e["phase"] == "confirmed" for e in journal.jobs.values()):
                    raise JournalError("disk full")
                original()
            with patch.object(journal, "save", side_effect=save), self.assertRaises(JournalError):
                watcher.sweep()
            restarted = Watcher(api, test_wallet(), Journal(journal.path, journal.identity), output=lambda *a, **k: None)
            self.assertEqual(restarted.sweep(), 1)
            self.assertEqual(len(node.payments), 1)

    def test_max_jobs_limits_sweep(self):
        with queue_server() as (node, api), tempfile.TemporaryDirectory() as directory:
            watcher, _, _ = self.setup_watcher(api, directory)
            self.assertEqual(watcher.sweep(limit=1), 1)
            self.assertEqual(len(node.payments), 1)


class JournalTests(unittest.TestCase):
    identity = {"api": "http://localhost/api/v1", "chainId": "test", "genesisHash": "a" * 64, "worker": "prism_" + "1" * 40}

    def test_identity_mismatch_does_not_replace_journal(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "state.json"
            journal = Journal(path, self.identity)
            journal.save()
            before = path.read_bytes()
            for field in self.identity:
                with self.subTest(field=field), self.assertRaises(JournalError):
                    Journal(path, dict(self.identity, **{field: "different"}))
            self.assertEqual(path.read_bytes(), before)

    def test_corrupt_journal_is_not_overwritten(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "state.json"
            path.write_bytes(b'{"truncated":')
            with self.assertRaises(JournalError):
                Journal(path, self.identity)
            self.assertEqual(path.read_bytes(), b'{"truncated":')

    def test_atomic_replace_failure_preserves_old_snapshot(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "state.json"
            journal = Journal(path, self.identity)
            journal.save()
            before = path.read_bytes()
            journal.data["extra"] = "new"
            with patch("prism_ml_watch.os.replace", side_effect=OSError("disk full")), self.assertRaises(JournalError):
                journal.save()
            self.assertEqual(path.read_bytes(), before)
            self.assertFalse(list(path.parent.glob("*.tmp")))

    def test_second_writer_is_locked_out(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "state.json"
            with journal_lock(path), self.assertRaises(JournalError):
                with journal_lock(path):
                    pass
            with journal_lock(path):
                pass


class CLIWatchTests(unittest.TestCase):
    def command(self, api, directory, *args):
        return [sys.executable, str(Path(__file__).with_name("prism_ml_worker.py")),
                "--api", api.base, "--data", directory, *args]

    def wallets(self, directory):
        Path(directory, "wallets.json").write_text(json.dumps([record(), record(bytes(reversed(range(32))), "Alice")]))

    def test_queue_three_then_watch_cli(self):
        with queue_server(0) as (node, api), tempfile.TemporaryDirectory() as directory:
            self.wallets(directory)
            queued = subprocess.run(self.command(api, directory, "enqueue-demo", "--requester", "Alice",
                                                 "--reward", "1", "--count", "3", "--nonce", "123"),
                                    capture_output=True, text=True, timeout=15)
            self.assertEqual(queued.returncode, 0, queued.stderr)
            self.assertEqual(len(queued.stdout.splitlines()), 3)
            result = subprocess.run(self.command(api, directory, "watch", "--worker", "Bob", "--max-jobs", "3"),
                                    capture_output=True, text=True, timeout=15)
            self.assertEqual(result.returncode, 0, result.stderr)
            events = [json.loads(line) for line in result.stdout.splitlines()]
            self.assertEqual(len([e for e in events if e["event"] == "confirmed"]), 3)
            self.assertEqual(len(node.payments), 3)
            self.assertNotIn(record()["private_key"], result.stdout + result.stderr)

    def test_hard_process_stop_after_settlement_releases_lock_and_recovers(self):
        with queue_server(1) as (node, api), tempfile.TemporaryDirectory() as directory:
            self.wallets(directory)
            node.fault = "hang_complete"
            process = subprocess.Popen(self.command(api, directory, "watch", "--worker", "Bob"),
                                       stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
            try:
                self.assertTrue(node.committed.wait(timeout=8), "worker never reached settlement")
                process.terminate()
                process.communicate(timeout=5)
            finally:
                if process.poll() is None:
                    process.kill()
                    process.communicate(timeout=5)
                node.release.set()
            result = subprocess.run(self.command(api, directory, "watch", "--worker", "Bob", "--once"),
                                    capture_output=True, text=True, timeout=15)
            self.assertEqual(result.returncode, 0, result.stderr)
            events = [json.loads(line) for line in result.stdout.splitlines()]
            receipt = next(e for e in events if e["event"] == "confirmed")
            self.assertTrue(receipt["recovered"])
            self.assertEqual(len(node.payments), 1)

    def test_invalid_options_fail_before_queue_mutations(self):
        with queue_server(0) as (node, api), tempfile.TemporaryDirectory() as directory:
            self.wallets(directory)
            for flags in (("--poll-interval", "nan"), ("--max-jobs", "-1")):
                result = subprocess.run(self.command(api, directory, "watch", "--worker", "Bob", *flags),
                                        capture_output=True, text=True, timeout=15)
                self.assertEqual(result.returncode, 1, result.stderr)
            self.assertFalse(any(method == "POST" for method, _ in node.requests))

    @unittest.skipIf(os.name == "nt", "Windows Ctrl+C is checked during the local demonstration")
    def test_ctrl_c_exits_and_releases_journal_lock(self):
        with queue_server(0) as (node, api), tempfile.TemporaryDirectory() as directory:
            self.wallets(directory)
            process = subprocess.Popen(self.command(api, directory, "watch", "--worker", "Bob"),
                                       stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
            try:
                # The startup line is flushed only after journal acquisition and identity checks.
                line = process.stdout.readline()
                self.assertEqual(json.loads(line)["event"], "watch_started")
                process.send_signal(signal.SIGINT)
                output, errors = process.communicate(timeout=5)
                self.assertEqual(process.returncode, 130, errors)
                self.assertIn("interrupted", output)
            finally:
                if process.poll() is None:
                    process.kill()
                    process.communicate(timeout=5)
            result = subprocess.run(self.command(api, directory, "watch", "--worker", "Bob", "--once"),
                                    capture_output=True, text=True, timeout=15)
            self.assertEqual(result.returncode, 0, result.stderr)


if __name__ == "__main__":
    unittest.main()
