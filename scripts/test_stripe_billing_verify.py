"""Contract tests for scripts/stripe-billing-verify.sh."""

from __future__ import annotations

import json
import os
import stat
import subprocess
import tempfile
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
VERIFY = ROOT / "scripts" / "stripe-billing-verify.sh"
TOKEN = "disposable-session-token-must-not-leak"
WORKSPACE = "tea-stripe-verify"


def readiness(*, payment_ready: bool = False) -> dict[str, object]:
    return {
        "workspaceId": WORKSPACE,
        "mode": "test",
        "customerReady": True,
        "subscriptionReady": True,
        "paymentMethodReady": payment_ready,
        "tax": {
            "configured": False,
            "enabled": False,
            "reason": "product_tax_not_configured",
            "productTaxCode": "",
            "taxBehavior": "",
            "registrationCount": 0,
        },
    }


class BillingHandler(BaseHTTPRequestHandler):
    server: "BillingServer"

    def log_message(self, _format: str, *_args: object) -> None:
        pass

    def send_json(self, body: object, status: int = 200) -> None:
        encoded = json.dumps(body).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(encoded)))
        self.end_headers()
        self.wfile.write(encoded)

    def authenticated(self) -> bool:
        if self.headers.get("X-Session-Token") == TOKEN:
            return True
        self.send_json({"error": "unauthorized"}, 401)
        return False

    def do_GET(self) -> None:  # noqa: N802
        if not self.authenticated():
            return
        if self.path == f"/v1/workspaces/{WORKSPACE}/billing":
            self.send_json(self.server.readiness)
            return
        self.send_json({"error": "not found"}, 404)

    def do_POST(self) -> None:  # noqa: N802
        if not self.authenticated():
            return
        length = int(self.headers.get("Content-Length", "0"))
        body = json.loads(self.rfile.read(length))
        if self.path == f"/v1/workspaces/{WORKSPACE}/billing/checkout-session":
            self.send_json(
                {
                    "url": "https://checkout.stripe.com/c/pay/cs_test_verify#fragment",
                    "expiresAt": "2026-07-27T18:00:00Z",
                },
                201,
            )
            return
        if self.path == "/graphql":
            if "workspaceBillingReadiness" in body.get("query", ""):
                self.send_json(
                    {"data": {"workspaceBillingReadiness": self.server.graphql_readiness}}
                )
                return
            if "createBillingPortalSession" in body.get("query", ""):
                self.send_json(
                    {
                        "data": {
                            "createBillingPortalSession": {
                                "url": "https://billing.stripe.com/p/session/graphql"
                            }
                        }
                    }
                )
                return
        if self.path == "/mcp":
            name = body.get("params", {}).get("name")
            if name == "get_billing_readiness":
                self.send_json(
                    {
                        "jsonrpc": "2.0",
                        "id": body.get("id"),
                        "result": {"structuredContent": self.server.mcp_readiness},
                    }
                )
                return
            if name == "create_billing_portal_session":
                self.send_json(
                    {
                        "jsonrpc": "2.0",
                        "id": body.get("id"),
                        "result": {
                            "structuredContent": {
                                "url": "https://billing.stripe.com/p/session/mcp"
                            }
                        },
                    }
                )
                return
        self.send_json({"error": "not found"}, 404)


class BillingServer(ThreadingHTTPServer):
    readiness: dict[str, object]
    graphql_readiness: dict[str, object]
    mcp_readiness: dict[str, object]


class VerifyScriptTest(unittest.TestCase):
    def setUp(self) -> None:
        value = readiness()
        self.server = BillingServer(("127.0.0.1", 0), BillingHandler)
        self.server.readiness = value
        self.server.graphql_readiness = value.copy()
        self.server.mcp_readiness = value.copy()
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()

    def tearDown(self) -> None:
        self.server.shutdown()
        self.thread.join()
        self.server.server_close()

    def run_verify(self, **extra: str) -> subprocess.CompletedProcess[str]:
        env = os.environ.copy()
        env.update(
            {
                "BEX_VERIFY_API_URL": f"http://127.0.0.1:{self.server.server_port}",
                "BEX_VERIFY_DASHBOARD_URL": "https://dashboard.bex.co",
                "BEX_VERIFY_SESSION_TOKEN": TOKEN,
                "BEX_VERIFY_WORKSPACE_ID": WORKSPACE,
            }
        )
        env.update(extra)
        return subprocess.run(
            [str(VERIFY)],
            cwd=ROOT,
            env=env,
            text=True,
            capture_output=True,
            check=False,
        )

    def test_parity_and_hosted_urls_use_private_file(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "hosted.json"
            result = self.run_verify(BEX_VERIFY_HOSTED_URL_FILE=str(output))

            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("mode=test", result.stdout)
            self.assertNotIn(TOKEN, result.stdout + result.stderr)
            self.assertNotIn("checkout.stripe.com", result.stdout + result.stderr)
            self.assertEqual(stat.S_IMODE(output.stat().st_mode), 0o600)
            hosted = json.loads(output.read_text())
            self.assertEqual(
                hosted,
                {
                    "checkoutUrl": "https://checkout.stripe.com/c/pay/cs_test_verify#fragment",
                    "graphqlPortalUrl": "https://billing.stripe.com/p/session/graphql",
                    "mcpPortalUrl": "https://billing.stripe.com/p/session/mcp",
                },
            )

            repeated = self.run_verify(BEX_VERIFY_HOSTED_URL_FILE=str(output))
            self.assertNotEqual(repeated.returncode, 0)
            self.assertIn("must not already exist", repeated.stderr)

    def test_payment_ready_gate_fails_before_hosted_session_creation(self) -> None:
        result = self.run_verify(BEX_VERIFY_REQUIRE_PAYMENT_READY="1")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("has not bound the default payment method", result.stderr)
        self.assertNotIn(TOKEN, result.stdout + result.stderr)

    def test_cross_surface_drift_fails_closed(self) -> None:
        self.server.mcp_readiness = readiness(payment_ready=True)
        result = self.run_verify()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("REST and MCP readiness differ", result.stderr)


if __name__ == "__main__":
    unittest.main()
