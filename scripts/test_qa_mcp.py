#!/usr/bin/env python3
"""Exercise scripts/qa-mcp.py against a loopback stand-in for bex-api."""

import http.server
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import threading
import time
import unittest


SCRIPT = Path(__file__).with_name("qa-mcp.py")
KEY = "bex_7f3c1a52-0d1e-4c55-9b0e-2b7d3f1e9a10_s3cr3t.v4lue~x"
OTHER_KEY = "bex_11111111-2222-3333-4444-555555555555_another.secret"
SESSION = "MTcwMDAwMDAwMHxzZXNzaW9uLWNvb2tpZS12YWx1ZQ=="
JAR = (
    "# Netscape HTTP Cookie File\n"
    "auth.example.test\tFALSE\t/\tTRUE\t0\tcsrf_token\tnot-the-session\n"
    f"#HttpOnly_.example.test\tTRUE\t/\tTRUE\t0\tory_kratos_session\t{SESSION}\n"
)


class Recorder(http.server.BaseHTTPRequestHandler):
    """Answers from the server's `routes`; records every request."""

    def log_message(self, *args):
        pass

    def handle_any(self):
        length = int(self.headers.get("Content-Length") or 0)
        body = self.rfile.read(length).decode() if length else ""
        self.server.requests.append(
            {"method": self.command, "path": self.path, "headers": {k.lower(): v for k, v in self.headers.items()}, "body": body}
        )
        route = self.server.routes.get((self.command, self.path.split("?")[0]))
        if route is None:
            status, headers, payload = 404, {"Content-Type": "text/plain"}, "no route"
        else:
            status, headers, payload = route(self, body)
        data = payload.encode()
        try:
            self.send_response(status)
            for name, value in headers.items():
                self.send_header(name, value)
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)
        except (BrokenPipeError, ConnectionResetError):
            pass  # the client gave up first, as the hang test means it to

    do_GET = do_POST = do_DELETE = do_PUT = do_PATCH = handle_any


def sse(message):
    return 200, {"Content-Type": "text/event-stream"}, f"event: message\ndata: {json.dumps(message)}\n\n"


def as_json(status, message):
    return status, {"Content-Type": "application/json"}, json.dumps(message)


class QAMCPTest(unittest.TestCase):
    def setUp(self):
        self.server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Recorder)
        self.server.requests, self.server.routes = [], {}
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        self.base = f"http://127.0.0.1:{self.server.server_address[1]}"
        self.dir = tempfile.TemporaryDirectory()
        self.tmp = Path(self.dir.name)
        os.chmod(self.tmp, 0o700)
        self.key_file = self.tmp / "key.json"
        self.key_file.write_text(json.dumps({"id": "7f3c1a52", "name": "qa", "ownerId": "tea-qa", "key": KEY}))
        os.chmod(self.key_file, 0o600)
        self.jar = self.tmp / "session.jar"
        self.jar.write_text(JAR)
        os.chmod(self.jar, 0o600)

    def tearDown(self):
        self.server.shutdown()
        self.server.server_close()
        self.dir.cleanup()

    def run_helper(self, *args, url=None, key=True):
        argv = [sys.executable, "-I", str(SCRIPT), "--url", url or self.base + "/mcp"]
        if key:
            argv += ["--key-file", str(self.key_file)]
        result = subprocess.run(argv + list(args), capture_output=True, text=True, timeout=30)
        for secret in (KEY, OTHER_KEY, SESSION, "s3cr3t.v4lue~x"):
            self.assertNotIn(secret, result.stdout + result.stderr, "a credential was printed")
        return result

    def test_initialize_sends_the_bearer_and_parses_sse_without_printing_the_key(self):
        def initialize(handler, body):
            request = json.loads(body)
            return sse({
                "jsonrpc": "2.0",
                "id": request["id"],
                "result": {"serverInfo": {"name": "bex", "version": "0.1.0"}, "echo": handler.headers["Authorization"]},
            })

        self.server.routes[("POST", "/mcp")] = initialize
        result = self.run_helper("initialize")
        self.assertEqual(result.returncode, 0, result.stderr)
        sent = self.server.requests[0]
        self.assertEqual(sent["headers"]["authorization"], "Bearer " + KEY)
        self.assertEqual(sent["headers"]["accept"], "application/json, text/event-stream")
        self.assertEqual(sent["headers"]["mcp-protocol-version"], "2025-06-18")
        self.assertEqual(json.loads(sent["body"])["method"], "initialize")
        self.assertIn('"name": "bex"', result.stdout)
        self.assertIn("Bearer [REDACTED]", result.stdout)
        self.assertIn("-> 200 in", result.stdout)

    def test_tools_list_follows_the_cursor_and_names_duplicates(self):
        def tools(handler, body):
            request = json.loads(body)
            cursor = (request.get("params") or {}).get("cursor")
            page = {"tools": [{"name": "list_services"}, {"name": "get_service"}], "nextCursor": "page-2"}
            if cursor == "page-2":
                page = {"tools": [{"name": "list_services"}]}
            return as_json(200, {"jsonrpc": "2.0", "id": request["id"], "result": page})

        self.server.routes[("POST", "/mcp")] = tools
        result = self.run_helper("tools-list")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("tools/list: 3 tools over 2 page(s)", result.stdout)
        self.assertIn("duplicate names: list_services", result.stdout)
        self.assertEqual(len(self.server.requests), 2)

    def test_anonymous_sends_no_credential_and_shows_the_challenge(self):
        self.server.routes[("POST", "/mcp")] = lambda h, b: (
            401,
            {"Content-Type": "application/json", "WWW-Authenticate": 'Bearer resource_metadata="https://x/.well-known/oauth-protected-resource"'},
            '{"error":"unauthorized"}',
        )
        result = self.run_helper("--anonymous", "tools-list", key=False)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertNotIn("authorization", self.server.requests[0]["headers"])
        self.assertIn("-> 401", result.stdout)
        self.assertIn("www-authenticate: Bearer resource_metadata=", result.stdout)
        self.assertIn("tools/list: no result", result.stdout)

    def test_call_redacts_any_bex_key_in_the_answer(self):
        self.server.routes[("POST", "/mcp")] = lambda h, b: as_json(
            200, {"jsonrpc": "2.0", "id": json.loads(b)["id"], "result": {"content": [{"type": "text", "text": "leaked " + OTHER_KEY}]}}
        )
        result = self.run_helper("call", "list_services", '{"limit": 1}')
        self.assertEqual(result.returncode, 0, result.stderr)
        params = json.loads(self.server.requests[0]["body"])["params"]
        self.assertEqual(params, {"name": "list_services", "arguments": {"limit": 1}})
        self.assertIn("leaked [REDACTED]", result.stdout)

    def test_notification_has_no_id(self):
        self.server.routes[("POST", "/mcp")] = lambda h, b: (202, {}, "")
        result = self.run_helper("notify", "notifications/initialized")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertNotIn("id", json.loads(self.server.requests[0]["body"]))
        self.assertIn("-> 202", result.stdout)

    def test_a_key_file_others_can_read_is_refused_before_any_request(self):
        os.chmod(self.key_file, 0o644)
        result = self.run_helper("tools-list")
        self.assertEqual(result.returncode, 2)
        self.assertIn("readable by others", result.stderr)
        self.assertEqual(self.server.requests, [])

    def test_plain_http_off_loopback_is_refused(self):
        result = self.run_helper("tools-list", url="http://api.example.test/mcp")
        self.assertEqual(result.returncode, 2)
        self.assertIn("only on loopback", result.stderr)

    def test_a_hang_is_a_network_failure(self):
        def slow(handler, body):
            time.sleep(3)
            return as_json(200, {})

        self.server.routes[("POST", "/mcp")] = slow
        result = self.run_helper("--timeout", "0.5", "tools-list")
        self.assertEqual(result.returncode, 3)
        self.assertIn("within 0.5s", result.stderr)

    def test_rest_control_uses_the_same_bearer_on_the_api_origin(self):
        self.server.routes[("GET", "/v1/services")] = lambda h, b: as_json(200, [])
        result = self.run_helper("rest", "GET", "/v1/services?limit=5")
        self.assertEqual(result.returncode, 0, result.stderr)
        sent = self.server.requests[0]
        self.assertEqual(sent["path"], "/v1/services?limit=5")
        self.assertEqual(sent["headers"]["authorization"], "Bearer " + KEY)

    def test_mint_key_writes_a_private_file_and_prints_only_the_id(self):
        def create(handler, body):
            self.assertEqual(handler.headers["Cookie"], f"ory_kratos_session={SESSION}")
            request = json.loads(body)
            self.assertEqual(request["ownerId"], "tea-qa")
            self.assertTrue(request["name"].startswith("qa-"))
            return as_json(201, {"id": "9a9a", "name": request["name"], "secret": "s3cr3t.v4lue~x", "key": KEY})

        self.server.routes[("POST", "/v1/api-keys")] = create
        out = self.tmp / "minted.json"
        result = self.run_helper("mint-key", "--jar", str(self.jar), "--out", str(out), "--owner-id", "tea-qa", key=False)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("ok id=9a9a", result.stdout)
        self.assertEqual(out.stat().st_mode & 0o777, 0o600)
        self.assertEqual(json.loads(out.read_text())["key"], KEY)
        self.assertNotIn("authorization", self.server.requests[0]["headers"])

        again = self.run_helper("mint-key", "--jar", str(self.jar), "--out", str(out), key=False)
        self.assertEqual(again.returncode, 2)
        self.assertIn("never overwrites", again.stderr)
        self.assertEqual(len(self.server.requests), 1)

    def test_mint_key_failure_writes_nothing(self):
        # The refusal echoes the cookie, as a proxy's error page might: the
        # session value must still never be printed.
        self.server.routes[("POST", "/v1/api-keys")] = lambda h, b: as_json(403, {"message": "forbidden", "cookie": h.headers["Cookie"]})
        out = self.tmp / "minted.json"
        result = self.run_helper("mint-key", "--jar", str(self.jar), "--out", str(out), key=False)
        self.assertEqual(result.returncode, 2)
        self.assertFalse(out.exists())
        self.assertIn("ory_kratos_session=[REDACTED]", result.stdout)

    def test_revoke_key_deletes_the_file_only_after_the_revoke(self):
        self.server.routes[("DELETE", "/v1/api-keys/7f3c1a52")] = lambda h, b: (500, {"Content-Type": "text/plain"}, "boom")
        result = self.run_helper("revoke-key", "--jar", str(self.jar), "--key-file", str(self.key_file), key=False)
        self.assertEqual(result.returncode, 2)
        self.assertTrue(self.key_file.exists(), "a failed revoke must keep the only handle on the key")

        self.server.routes[("DELETE", "/v1/api-keys/7f3c1a52")] = lambda h, b: (204, {}, "")
        result = self.run_helper("revoke-key", "--jar", str(self.jar), "--key-file", str(self.key_file), key=False)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse(self.key_file.exists())
        sent = self.server.requests[-1]
        self.assertEqual(sent["path"], "/v1/api-keys/7f3c1a52?ownerId=tea-qa")
        self.assertEqual(sent["headers"]["cookie"], f"ory_kratos_session={SESSION}")

    def test_a_jar_without_a_session_is_refused(self):
        self.jar.write_text("# Netscape HTTP Cookie File\n")
        result = self.run_helper("mint-key", "--jar", str(self.jar), "--out", str(self.tmp / "k.json"), key=False)
        self.assertEqual(result.returncode, 2)
        self.assertIn("qa-login.sh", result.stderr)
        self.assertEqual(self.server.requests, [])


if __name__ == "__main__":
    unittest.main()
