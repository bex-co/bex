#!/usr/bin/env python3
"""LOCAL-DEV Anthropic Messages API stub for `dev-env.sh N agent-stub`.

⚠️  THIS IS A TEST DOUBLE, NOT A MODEL. ⚠️

It exists so the ADR047 turn path can be exercised on a laptop without a real
provider credential. What it lets a developer prove locally, end to end:

    driver -> ACP adapter -> gateway model proxy -> credential mint (OpenBao)
           -> upstream hop -> streamed response -> transcript -> attach SSE

Everything in that chain is bex's own machinery. The one thing this deliberately
does NOT prove is that a real Anthropic key works — that is a property of the
provider, not of bex, and it is what the operator-run scripts/agent-session-verify.sh
covers against a real deployment.

The gateway reaches this over TLS as `api.anthropic.com` (a hostAlias plus a CA
the gateway trusts via SSL_CERT_FILE), because bex pins each agent profile to its
registered provider endpoint — a session cannot be pointed at an arbitrary host,
by design (agentsession.RegisteredModelEndpoint). Interposing at the gateway's
upstream hop is therefore the only place a stub can sit without weakening that
rule or editing product code.

It answers the non-streaming and streaming Messages API shapes with a fixed
reply. It ignores the request's content: the point is the transport, not the
generation.
"""

import http.server
import json
import ssl
import sys

REPLY = "1\n2\n3\n4\n5"


def sse(event: str, data: dict) -> bytes:
    return f"event: {event}\ndata: {json.dumps(data)}\n\n".encode()


class Handler(http.server.BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, fmt, *args):  # noqa: A003 - stdlib signature
        sys.stderr.write("model-stub: " + (fmt % args) + "\n")

    def _read_body(self) -> dict:
        length = int(self.headers.get("Content-Length") or 0)
        raw = self.rfile.read(length) if length else b"{}"
        try:
            return json.loads(raw or b"{}")
        except json.JSONDecodeError:
            return {}

    def do_POST(self):  # noqa: N802 - stdlib signature
        path = self.path.split("?", 1)[0]
        if not path.startswith("/v1/messages"):
            self.send_error(404, "only /v1/messages is stubbed")
            return
        body = self._read_body()
        model = body.get("model") or "claude-sonnet-4-5"
        # The gateway injects the real credential on this hop; assert it arrived
        # so a broken mint surfaces here instead of silently "working".
        if not (self.headers.get("x-api-key") or self.headers.get("authorization")):
            self.send_error(401, "no credential injected on the upstream hop")
            return
        # count_tokens has its own response shape; answering it with a Message
        # leaves the adapter waiting on a reply it cannot parse.
        if path.startswith("/v1/messages/count_tokens"):
            self._json({"input_tokens": 8})
            return
        if body.get("stream"):
            self._stream(model)
        else:
            self._once(model)

    def _json(self, payload: dict):
        raw = json.dumps(payload).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def _once(self, model: str):
        payload = {
            "id": "msg_devstub",
            "type": "message",
            "role": "assistant",
            "model": model,
            "content": [{"type": "text", "text": REPLY}],
            "stop_reason": "end_turn",
            "stop_sequence": None,
            "usage": {"input_tokens": 8, "output_tokens": 9},
        }
        raw = json.dumps(payload).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def _stream(self, model: str):
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Cache-Control", "no-cache")
        self.send_header("Connection", "keep-alive")
        self.end_headers()
        start = {
            "type": "message_start",
            "message": {
                "id": "msg_devstub",
                "type": "message",
                "role": "assistant",
                "model": model,
                "content": [],
                "stop_reason": None,
                "stop_sequence": None,
                "usage": {"input_tokens": 8, "output_tokens": 0},
            },
        }
        self.wfile.write(sse("message_start", start))
        self.wfile.write(
            sse(
                "content_block_start",
                {"type": "content_block_start", "index": 0,
                 "content_block": {"type": "text", "text": ""}},
            )
        )
        for chunk in REPLY.split("\n"):
            self.wfile.write(
                sse(
                    "content_block_delta",
                    {"type": "content_block_delta", "index": 0,
                     "delta": {"type": "text_delta", "text": chunk + "\n"}},
                )
            )
            self.wfile.flush()
        self.wfile.write(sse("content_block_stop", {"type": "content_block_stop", "index": 0}))
        self.wfile.write(
            sse(
                "message_delta",
                {"type": "message_delta",
                 "delta": {"stop_reason": "end_turn", "stop_sequence": None},
                 "usage": {"output_tokens": 9}},
            )
        )
        self.wfile.write(sse("message_stop", {"type": "message_stop"}))
        self.wfile.flush()


def main():
    server = http.server.ThreadingHTTPServer(("0.0.0.0", 8443), Handler)
    context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    context.load_cert_chain("/etc/model-stub/tls.crt", "/etc/model-stub/tls.key")
    server.socket = context.wrap_socket(server.socket, server_side=True)
    sys.stderr.write("model-stub: serving https on :8443 as api.anthropic.com\n")
    server.serve_forever()


if __name__ == "__main__":
    main()
