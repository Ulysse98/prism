import json
from pathlib import Path
import tempfile
import unittest
import urllib.error
import urllib.request

from prism_ml_metrics import (
    WorkerMetricsServer,
    render_worker_metrics,
)


class WorkerMetricsTests(unittest.TestCase):
    def snapshot(self):
        return {
            "version": 1,
            "status": "healthy",
            "worker": "prism_" + "1" * 40,
            "chainId": "prism-test",
            "startedAt": "2026-09-28T08:00:00Z",
            "updatedAt": "2026-09-28T08:00:05Z",
            "sweeps": 12,
            "confirmed": 3,
            "unresolved": 1,
            "consecutiveErrors": 0,
            "lastError": None,
        }

    def test_render_worker_metrics(self):
        output = render_worker_metrics(
            self.snapshot()
        )

        expected = (
            "prism_worker_healthy 1",
            "prism_worker_sweeps_total 12",
            "prism_worker_confirmed_total 3",
            "prism_worker_unresolved_jobs 1",
            "prism_worker_consecutive_errors 0",
        )

        for metric in expected:
            self.assertIn(metric, output)

        self.assertIn(
            'status="healthy"',
            output,
        )

    def test_degraded_worker_is_not_healthy(self):
        snapshot = self.snapshot()
        snapshot["status"] = "degraded"
        snapshot["consecutiveErrors"] = 2

        output = render_worker_metrics(snapshot)

        self.assertIn(
            "prism_worker_healthy 0",
            output,
        )

        self.assertIn(
            "prism_worker_consecutive_errors 2",
            output,
        )

    def test_http_server_serves_metrics(self):
        with tempfile.TemporaryDirectory() as directory:
            health = Path(directory) / "health.json"

            health.write_text(
                json.dumps(self.snapshot()),
                encoding="utf-8",
            )

            server = WorkerMetricsServer(
                health,
                "127.0.0.1",
                0,
            )

            server.start()

            try:
                with urllib.request.urlopen(
                    f"http://127.0.0.1:{server.port}/metrics",
                    timeout=5,
                ) as response:
                    body = response.read().decode("utf-8")

                    self.assertEqual(
                        response.status,
                        200,
                    )

                self.assertIn(
                    "prism_worker_sweeps_total 12",
                    body,
                )

            finally:
                server.close()

    def test_http_server_returns_503_for_bad_snapshot(self):
        with tempfile.TemporaryDirectory() as directory:
            health = Path(directory) / "health.json"
            health.write_text(
                "{broken",
                encoding="utf-8",
            )

            server = WorkerMetricsServer(
                health,
                "127.0.0.1",
                0,
            )

            server.start()

            try:
                with self.assertRaises(
                    urllib.error.HTTPError,
                ) as caught:
                    urllib.request.urlopen(
                        f"http://127.0.0.1:{server.port}/metrics",
                        timeout=5,
                    )

                try:
                    self.assertEqual(
                        caught.exception.code,
                        503,
                    )
                finally:
                    caught.exception.close()

            finally:
                server.close()


if __name__ == "__main__":
    unittest.main()
