"""Sequential queue worker with network-bound, atomic recovery state.

Recovery relies on the existing Go settleComputeJob idempotency contract.
The journal contains public job/receipt metadata only, never wallet secrets.
"""
from __future__ import annotations

from contextlib import contextmanager
from datetime import datetime, timezone
import json
import math
import os
from pathlib import Path
import sys
import tempfile
import time

from prism_ml_protocol import (
    ProtocolError, TYPE, compact, identifier, load_wallet, make_proof,
    read_json, uint, validate_job,
)
from prism_ml_worker import (
    API, HTTPStatusError, TransportError, WorkerError, checked_receipt, find_job,
)

JOURNAL_LIMIT = 16 << 20
PHASES = {"pending", "submitting", "confirmed", "failed"}


class JournalError(WorkerError):
    pass


class HealthError(WorkerError):
    pass


class HealthFile:
    STATES = {"starting", "healthy", "degraded", "stopped"}
    LIMIT = 64 << 10

    def __init__(self, path: Path, identity: dict, clock=time.time):
        self.path = path.resolve()
        self.identity = identity
        self.clock = clock
        now = self.clock()
        self.started_at = self._timestamp(now)
        self.data = {}

    @staticmethod
    def _timestamp(value: float) -> str:
        if not math.isfinite(value) or value < 0:
            raise HealthError("invalid health timestamp")
        return (
            datetime.fromtimestamp(value, timezone.utc)
            .isoformat()
            .replace("+00:00", "Z")
        )

    def write(
        self,
        status: str,
        *,
        sweeps: int,
        confirmed: int,
        unresolved: int,
        consecutive_errors: int,
        last_error: str | None = None,
    ):
        if status not in self.STATES:
            raise HealthError("invalid worker health state")

        counters = (sweeps, confirmed, unresolved, consecutive_errors)
        if any(type(value) is not int or value < 0 for value in counters):
            raise HealthError("invalid worker health counter")

        now = self.clock()

        self.data = {
            "version": 1,
            "status": status,
            "worker": self.identity["worker"],
            "chainId": self.identity["chainId"],
            "startedAt": self.started_at,
            "updatedAt": self._timestamp(now),
            "sweeps": sweeps,
            "confirmed": confirmed,
            "unresolved": unresolved,
            "consecutiveErrors": consecutive_errors,
            "lastError": None if last_error is None else str(last_error)[:400],
        }

        self.save()

    def save(self):
        raw = compact(self.data) + b"\n"

        if len(raw) > self.LIMIT:
            raise HealthError("worker health snapshot exceeds 64 KiB")

        self.path.parent.mkdir(parents=True, exist_ok=True)
        temporary = None

        try:
            with tempfile.NamedTemporaryFile(
                mode="wb",
                dir=self.path.parent,
                prefix=self.path.name + ".",
                suffix=".tmp",
                delete=False,
            ) as stream:
                temporary = Path(stream.name)
                stream.write(raw)
                stream.flush()
                os.fsync(stream.fileno())

            os.replace(temporary, self.path)
            temporary = None

        except OSError as error:
            raise HealthError(
                "cannot persist worker health snapshot"
            ) from error

        finally:
            if temporary is not None:
                temporary.unlink(missing_ok=True)


def emit(event: str, **fields):
    print(json.dumps({"event": event, **fields}, ensure_ascii=True), flush=True)


def network_identity(api: API, worker: str) -> dict:
    status = api.request("GET", "/status")
    if not isinstance(status, dict) or status.get("chainValid") is not True:
        raise WorkerError("node does not confirm a valid chain")
    chain = status.get("chainId")
    if not isinstance(chain, str) or not chain.strip() or len(chain) > 128:
        raise WorkerError("missing or invalid chain identity")
    return {"api": api.base, "chainId": chain,
            "genesisHash": identifier(status.get("genesisHash"), "genesis hash"), "worker": worker}


@contextmanager
def journal_lock(path: Path):
    """OS releases this lock even after a hard process termination."""
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.with_name(path.name + ".lock").open("a+b") as stream:
        stream.seek(0, os.SEEK_END)
        if stream.tell() == 0:
            stream.write(b"0")
            stream.flush()
        stream.seek(0)
        try:
            if os.name == "nt":
                import msvcrt
                msvcrt.locking(stream.fileno(), msvcrt.LK_NBLCK, 1)
            else:
                import fcntl
                fcntl.flock(stream.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
        except OSError as error:
            raise JournalError("journal is already in use by another worker") from error
        try:
            yield
        finally:
            stream.seek(0)
            if os.name == "nt":
                import msvcrt
                msvcrt.locking(stream.fileno(), msvcrt.LK_UNLCK, 1)
            else:
                import fcntl
                fcntl.flock(stream.fileno(), fcntl.LOCK_UN)


class Journal:
    def __init__(self, path: Path, identity: dict):
        self.path = path.resolve()
        self.identity = identity
        try:
            self.data = read_json(self.path, JOURNAL_LIMIT) if self.path.exists() else {
                "version": 1, "identity": identity, "jobs": {},
            }
            if (not isinstance(self.data, dict) or type(self.data.get("version")) is not int or
                    self.data["version"] != 1 or self.data.get("identity") != identity or
                    not isinstance(self.data.get("jobs"), dict)):
                raise JournalError("journal belongs to another API, chain or wallet, or has an invalid schema")
            for job_id, entry in self.data["jobs"].items():
                identifier(job_id, "journal job ID")
                if (not isinstance(entry, dict) or not isinstance(entry.get("phase"), str) or
                        entry["phase"] not in PHASES):
                    raise JournalError("invalid journal job entry")
                identifier(entry.get("taskId"), "journal task ID")
                if entry.get("proofId") is not None:
                    identifier(entry["proofId"], "journal proof ID")
                uint(entry.get("attempts"), "journal attempts")
                retry_at = entry.get("retryAt")
                if type(retry_at) not in (int, float) or not math.isfinite(retry_at) or retry_at < 0:
                    raise JournalError("invalid journal retry time")
                if entry["phase"] == "confirmed":
                    receipt = entry.get("receipt")
                    if (not isinstance(receipt, dict) or receipt.get("verified") is not True or
                            receipt.get("settled") is not True or receipt.get("jobId") != job_id or
                            receipt.get("worker") != identity["worker"] or
                            receipt.get("proofId") != entry.get("proofId")):
                        raise JournalError("invalid journal confirmation")
        except (OSError, ProtocolError) as error:
            raise JournalError("cannot load worker journal; existing state was not replaced") from error

    @property
    def jobs(self):
        return self.data["jobs"]

    def save(self):
        """Durably replace the complete snapshot, leaving the previous file on failure."""
        raw = compact(self.data) + b"\n"
        if len(raw) > JOURNAL_LIMIT:
            raise JournalError("journal exceeds 16 MiB; archive it before continuing")
        self.path.parent.mkdir(parents=True, exist_ok=True)
        temporary = None
        try:
            with tempfile.NamedTemporaryFile(mode="wb", dir=self.path.parent,
                                             prefix=self.path.name + ".", suffix=".tmp", delete=False) as stream:
                temporary = Path(stream.name)
                stream.write(raw)
                stream.flush()
                os.fsync(stream.fileno())
            os.replace(temporary, self.path)
            temporary = None
            if os.name != "nt":
                directory = os.open(self.path.parent, os.O_RDONLY | os.O_DIRECTORY)
                try:
                    os.fsync(directory)
                finally:
                    os.close(directory)
        except OSError as error:
            raise JournalError("cannot persist worker journal; stopping before further requests") from error
        finally:
            if temporary is not None:
                temporary.unlink(missing_ok=True)

    def retry_failed(self):
        for entry in self.jobs.values():
            if entry["phase"] == "failed":
                entry.update(phase="pending", retryAt=0, attempts=0)
        self.save()


class Watcher:
    def __init__(self, api: API, wallet, journal: Journal, poll_interval=2.0, clock=time.time, output=emit):
        self.api, self.wallet, self.journal = api, wallet, journal
        self.poll_interval, self.clock, self.output = poll_interval, clock, output

    def check_network(self):
        if network_identity(self.api, self.wallet.address) != self.journal.identity:
            raise JournalError("API chain identity changed; no further job requests were sent")

    def defer(self, job_id, error, permanent=False):
        entry = self.journal.jobs[job_id]
        delay = min(60.0, self.poll_interval * 2 ** min(entry["attempts"], 6))
        entry.update(phase="failed" if permanent else entry["phase"],
                     retryAt=self.clock() + delay, error=str(error)[:400])
        self.journal.save()
        self.output("failed" if permanent else "retry_pending", jobId=job_id,
                    error=entry["error"], retryIn=None if permanent else delay)

    def process(self, listed):
        job_id = listed["id"]
        # Refresh before taking action: queue listings can be stale after another worker claims a job.
        self.check_network()
        job = find_job(self.api, job_id)
        previous = self.journal.jobs.get(job_id)
        if job["status"] != "OPEN" and job.get("worker") != self.wallet.address:
            if previous:
                self.defer(job_id, "job belongs to another worker", permanent=True)
            return False
        if job["status"] == "VERIFIED" and previous is None:
            return False
        try:
            identity = self.journal.identity
            proof = make_proof(
                job["task"],
                self.wallet,
                job_id=job["id"],
                chain_id=identity["chainId"],
                genesis_hash=identity["genesisHash"],
            )
            API.encode({"proof": proof})
        except (ProtocolError, WorkerError) as error:
            self.journal.jobs[job_id] = {"taskId": job["task"]["id"], "proofId": None,
                                         "phase": "pending", "attempts": 0, "retryAt": 0}
            self.defer(job_id, error, permanent=True)
            return False
        if previous and (previous["taskId"] != job["task"]["id"] or
                         previous.get("proofId") not in (None, proof["id"])):
            raise JournalError("job no longer matches the pending journal proof")
        if job["status"] == "VERIFIED" and job.get("proofId") != proof["id"]:
            self.defer(job_id, "verified job contains another proof", permanent=True)
            return False
        entry = {"taskId": job["task"]["id"], "proofId": proof["id"], "phase": "pending",
                 "attempts": (previous or {}).get("attempts", 0) + 1, "retryAt": 0}
        reused = any(other_id != job_id and other.get("proofId") == proof["id"]
                     for other_id, other in self.journal.jobs.items())
        if job["status"] == "OPEN":
            history = self.api.request("GET", "/work")
            if not isinstance(history, dict) or not isinstance(history.get("entries"), list):
                raise WorkerError("cannot check existing on-chain proofs")
            reused = reused or any(isinstance(row, dict) and row.get("proofId") == proof["id"]
                                   for row in history["entries"])
        if reused:
            entry["proofId"] = None
            self.journal.jobs[job_id] = entry
            self.defer(job_id, "proof already used by another job or on-chain; a distinct task is required", permanent=True)
            return False
        self.journal.jobs[job_id] = entry
        self.journal.save()  # Intent must be durable before the first mutating request.
        stage = "claim" if job["status"] == "OPEN" else "complete"
        try:
            if stage == "claim":
                response = self.api.request("POST", f"/compute/jobs/{job_id}/claim", {"worker": self.wallet.address})
                claimed = validate_job(response.get("job") if isinstance(response, dict) else None, job_id)
                if claimed["status"] != "CLAIMED" or claimed.get("worker") != self.wallet.address:
                    raise WorkerError("claim not confirmed for this wallet")
            self.check_network()
            stage = "complete"
            entry["phase"] = "submitting"
            self.journal.save()
            # Go recovers an exact existing proof + bounty, including VERIFIED jobs.
            response = self.api.request("POST", f"/compute/jobs/{job_id}/complete", {"proof": proof})
            receipt = checked_receipt(response, job, proof)
        except JournalError:
            raise
        except HTTPStatusError as error:
            # A competing claim can return HTTP 400. Reconcile it before classifying the error.
            permanent = error.status not in (408, 429) and error.status < 500
            if stage == "claim" and error.status in (400, 409):
                fresh = find_job(self.api, job_id)
                permanent = fresh["status"] == "OPEN" or fresh.get("worker") != self.wallet.address
            self.defer(job_id, error, permanent)
            return False
        except (TransportError, WorkerError, ProtocolError) as error:
            # Unknown result or invalid receipt is never recorded as a successful settlement.
            self.defer(job_id, error)
            return False
        entry.update(phase="confirmed", receipt=receipt, retryAt=0)
        self.journal.save()  # If printing is interrupted, the receipt remains in the journal.
        self.output("confirmed", **receipt)
        return True

    def sweep(self, limit=0):
        self.check_network()
        response = self.api.request("GET", "/compute/jobs")
        if not isinstance(response, dict) or not isinstance(response.get("jobs"), list):
            raise WorkerError("invalid job-list response")
        seen, candidates = set(), []
        for raw in response["jobs"]:
            if not isinstance(raw, dict) or not isinstance(raw.get("task"), dict) or raw["task"].get("type") != TYPE:
                continue
            try:
                job = validate_job(raw)
            except ProtocolError:
                self.output("skipped", reason="invalid quantized job")
                continue
            job_id = job["id"]
            if job_id in seen:
                raise WorkerError("duplicate job in queue; no jobs were processed")
            seen.add(job_id)
            entry = self.journal.jobs.get(job_id)
            if entry and (entry["phase"] in ("confirmed", "failed") or entry["retryAt"] > self.clock()):
                continue
            if (job["requester"] == self.wallet.address or
                    job["requester"].casefold() == self.wallet.name.casefold()):
                continue
            if (job["status"] == "OPEN" or
                    (job.get("worker") == self.wallet.address and
                     (job["status"] == "CLAIMED" or entry is not None))):
                candidates.append(job)
        # Resume outstanding claims first; deterministic order otherwise.
        candidates.sort(key=lambda j: (j["status"] == "OPEN", j["id"]))
        confirmed = 0
        for job in candidates:
            if self.process(job):
                confirmed += 1
            if limit and confirmed >= limit:
                break
        return confirmed


def run_watch(args) -> int:
    health = None
    metrics_server = None
    journal = None
    sweeps = 0
    count = 0
    failures = 0

    def journal_counts():
        if journal is None:
            return 0, 0

        confirmed = sum(
            entry["phase"] == "confirmed"
            for entry in journal.jobs.values()
        )

        unresolved = sum(
            entry["phase"] != "confirmed"
            for entry in journal.jobs.values()
        )

        return confirmed, unresolved

    def update_health(status, last_error=None):
        if health is None:
            return

        confirmed, unresolved = journal_counts()

        health.write(
            status,
            sweeps=sweeps,
            confirmed=confirmed,
            unresolved=unresolved,
            consecutive_errors=failures,
            last_error=last_error,
        )

    try:
        if (
            not math.isfinite(args.poll_interval)
            or not 0.1 <= args.poll_interval <= 300
        ):
            raise WorkerError(
                "poll interval must be between 0.1 and 300 seconds"
            )

        if args.max_jobs < 0:
            raise WorkerError("max jobs must be non-negative")

        if not 0 <= args.metrics_port <= 65535:
            raise WorkerError(
                "metrics port must be between 0 and 65535"
            )

        wallet = load_wallet(args.data, args.worker)
        api = API(args.api, args.timeout)
        identity = network_identity(api, wallet.address)

        path = (
            args.journal
            or args.data
            / "python-worker"
            / (wallet.address + ".json")
        ).resolve()

        health_path = (
            args.health
            or args.data
            / "python-worker"
            / (wallet.address + ".health.json")
        ).resolve()

        if health_path == path:
            raise HealthError(
                "health path must differ from journal path"
            )

        with journal_lock(path):
            journal = Journal(path, identity)
            journal.save()

            if args.retry_failed:
                journal.retry_failed()

            health = HealthFile(
                health_path,
                identity,
            )

            update_health("starting")

            if args.metrics_port:
                from prism_ml_metrics import WorkerMetricsServer

                metrics_server = WorkerMetricsServer(
                    health_path,
                    args.metrics_host,
                    args.metrics_port,
                )

                metrics_server.start()

            watcher = Watcher(
                api,
                wallet,
                journal,
                args.poll_interval,
            )

            emit(
                "watch_started",
                worker=wallet.address,
                chainId=identity["chainId"],
                journal=str(path),
                health=str(health_path),
                metricsPort=(
                    metrics_server.port
                    if metrics_server is not None
                    else None
                ),
            )

            while True:
                last_error = None

                try:
                    remaining = (
                        args.max_jobs - count
                        if args.max_jobs
                        else 0
                    )

                    count += watcher.sweep(remaining)
                    failures = 0

                except JournalError:
                    raise

                except HTTPStatusError as error:
                    if (
                        error.status not in (408, 429)
                        and error.status < 500
                    ):
                        raise

                    failures += 1
                    last_error = str(error)

                    emit(
                        "poll_error",
                        error=last_error,
                    )

                except (
                    TransportError,
                    ProtocolError,
                    WorkerError,
                ) as error:
                    failures += 1
                    last_error = str(error)

                    emit(
                        "poll_error",
                        error=last_error,
                    )

                sweeps += 1

                update_health(
                    "degraded" if failures else "healthy",
                    last_error,
                )

                if args.once or (
                    args.max_jobs
                    and count >= args.max_jobs
                ):
                    _, pending = journal_counts()

                    update_health("stopped")

                    emit(
                        "watch_stopped",
                        confirmed=count,
                        unresolved=pending,
                    )

                    return (
                        2
                        if failures
                        or (args.once and pending)
                        else 0
                    )

                delay = (
                    min(
                        60.0,
                        args.poll_interval
                        * 2 ** min(failures, 6),
                    )
                    if failures
                    else args.poll_interval
                )

                time.sleep(delay)

    except KeyboardInterrupt:
        update_health("stopped")

        emit(
            "watch_stopped",
            reason=(
                "interrupted; pending jobs will be "
                "reconciled on restart"
            ),
        )

        return 130

    except (
        OSError,
        ProtocolError,
        WorkerError,
    ) as error:
        if health is not None and not isinstance(
            error,
            HealthError,
        ):
            failures = max(1, failures)

            try:
                update_health(
                    "degraded",
                    str(error),
                )
            except HealthError:
                pass

        print(
            f"ERROR: {error}",
            file=sys.stderr,
        )

        return 1

    finally:
        if metrics_server is not None:
            metrics_server.close()
