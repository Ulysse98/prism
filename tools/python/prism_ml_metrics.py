"""Prometheus exporter for Prism Python worker health snapshots."""

from __future__ import annotations

from datetime import datetime
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
from pathlib import Path
import threading


HEALTH_LIMIT = 64 << 10
HEALTH_STATES = {"starting", "healthy", "degraded", "stopped"}


class MetricsError(ValueError):
    pass


def _counter(value: object, name: str) -> int:
    if type(value) is not int or value < 0:
        raise MetricsError(f"invalid {name}")
    return value


def _label(value: str) -> str:
    return (
        value
        .replace("\\", "\\\\")
        .replace("\n", "\\n")
        .replace('"', '\\"')
    )


def _timestamp(value: object, name: str) -> float:
    if not isinstance(value, str):
        raise MetricsError(f"invalid {name}")

    try:
        parsed = datetime.fromisoformat(
            value.replace("Z", "+00:00")
        )
    except ValueError as error:
        raise MetricsError(f"invalid {name}") from error

    if parsed.tzinfo is None:
        raise MetricsError(f"invalid {name}")

    return parsed.timestamp()


def validate_health_snapshot(value: object) -> dict:
    if not isinstance(value, dict):
        raise MetricsError("invalid worker health snapshot")

    if value.get("version") != 1:
        raise MetricsError("unsupported worker health version")

    status = value.get("status")
    if status not in HEALTH_STATES:
        raise MetricsError("invalid worker health status")

    worker = value.get("worker")
    chain_id = value.get("chainId")

    if not isinstance(worker, str) or not worker:
        raise MetricsError("invalid worker identity")

    if not isinstance(chain_id, str) or not chain_id:
        raise MetricsError("invalid chain identity")

    return {
        "status": status,
        "worker": worker,
        "chainId": chain_id,
        "startedAt": _timestamp(
            value.get("startedAt"),
            "worker start timestamp",
        ),
        "updatedAt": _timestamp(
            value.get("updatedAt"),
            "worker update timestamp",
        ),
        "sweeps": _counter(
            value.get("sweeps"),
            "worker sweep count",
        ),
        "confirmed": _counter(
            value.get("confirmed"),
            "worker confirmation count",
        ),
        "unresolved": _counter(
            value.get("unresolved"),
            "worker unresolved count",
        ),
        "consecutiveErrors": _counter(
            value.get("consecutiveErrors"),
            "worker consecutive error count",
        ),
    }


def load_health_snapshot(path: Path) -> dict:
    raw = path.read_bytes()

    if len(raw) > HEALTH_LIMIT:
        raise MetricsError(
            "worker health snapshot exceeds 64 KiB"
        )

    try:
        value = json.loads(raw.decode("utf-8"))
    except (
        UnicodeDecodeError,
        json.JSONDecodeError,
    ) as error:
        raise MetricsError(
            "invalid worker health JSON"
        ) from error

    validate_health_snapshot(value)
    return value


def render_worker_metrics(snapshot: object) -> str:
    health = validate_health_snapshot(snapshot)

    healthy = 1 if health["status"] == "healthy" else 0

    worker = _label(health["worker"])
    chain_id = _label(health["chainId"])
    status = _label(health["status"])

    return (
        "# HELP prism_worker_info Static Prism worker identity and state.\n"
        "# TYPE prism_worker_info gauge\n"
        f'prism_worker_info{{worker="{worker}",chain_id="{chain_id}",status="{status}"}} 1\n'
        "# HELP prism_worker_healthy Whether the Prism worker is currently healthy.\n"
        "# TYPE prism_worker_healthy gauge\n"
        f"prism_worker_healthy {healthy}\n"
        "# HELP prism_worker_sweeps_total Total queue sweeps performed by this worker process.\n"
        "# TYPE prism_worker_sweeps_total counter\n"
        f"prism_worker_sweeps_total {health['sweeps']}\n"
        "# HELP prism_worker_confirmed_total Confirmed jobs recorded in the worker journal.\n"
        "# TYPE prism_worker_confirmed_total counter\n"
        f"prism_worker_confirmed_total {health['confirmed']}\n"
        "# HELP prism_worker_unresolved_jobs Jobs in the worker journal not yet confirmed.\n"
        "# TYPE prism_worker_unresolved_jobs gauge\n"
        f"prism_worker_unresolved_jobs {health['unresolved']}\n"
        "# HELP prism_worker_consecutive_errors Consecutive worker polling errors.\n"
        "# TYPE prism_worker_consecutive_errors gauge\n"
        f"prism_worker_consecutive_errors {health['consecutiveErrors']}\n"
        "# HELP prism_worker_started_timestamp_seconds Worker process start time as Unix seconds.\n"
        "# TYPE prism_worker_started_timestamp_seconds gauge\n"
        f"prism_worker_started_timestamp_seconds {health['startedAt']:.3f}\n"
        "# HELP prism_worker_last_update_timestamp_seconds Last worker health update as Unix seconds.\n"
        "# TYPE prism_worker_last_update_timestamp_seconds gauge\n"
        f"prism_worker_last_update_timestamp_seconds {health['updatedAt']:.3f}\n"
    )


class WorkerMetricsServer:
    def __init__(
        self,
        health_path: Path,
        host: str,
        port: int,
    ):
        self.health_path = health_path.resolve()

        exporter = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_GET(self):
                if self.path != "/metrics":
                    self.send_response(404)
                    self.end_headers()
                    return

                try:
                    snapshot = load_health_snapshot(
                        exporter.health_path
                    )

                    body = render_worker_metrics(
                        snapshot
                    ).encode("utf-8")

                    status = 200

                except (
                    OSError,
                    MetricsError,
                ):
                    body = (
                        "# Prism worker metrics unavailable\n"
                    ).encode("utf-8")

                    status = 503

                self.send_response(status)
                self.send_header(
                    "Content-Type",
                    "text/plain; version=0.0.4; charset=utf-8",
                )
                self.send_header(
                    "Content-Length",
                    str(len(body)),
                )
                self.end_headers()
                self.wfile.write(body)

        self.server = ThreadingHTTPServer(
            (host, port),
            Handler,
        )

        self.server.daemon_threads = True

        self.thread = threading.Thread(
            target=self.server.serve_forever,
            name="prism-worker-metrics",
            daemon=True,
        )

    @property
    def port(self) -> int:
        return self.server.server_address[1]

    def start(self):
        self.thread.start()

    def close(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=5)
