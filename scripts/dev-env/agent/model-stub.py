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
reply. It ignores the request's content, with one exception: the point is the
transport, not the generation.

The exception is a turn that has to stay running (w22/m403 t011: a message sent
during a turn is queued, and Stop turn ends only that turn). When the newest
user message of a streaming request contains `stub-hold:<seconds>`, the stub
streams one `tick <n>` line a second for that long before it finishes, so a
person or a script has a live turn to queue behind and to stop, and the
transcript shows how far the reply got. Anything else is answered at once.

A second directive lets a turn touch its disk (w22/m413 t008: a stopped turn's
uncommitted work is there in the next turn). When the newest user message of a
streaming request that offers the engine's `Bash` tool contains
`stub-run:<command>` (the rest of that line), the stub answers with one call of
that tool. The engine runs the command in the sandbox and sends its output
back; the stub then replies with that output, and holds the turn if the same
prompt also carries `stub-hold`. So `stub-run:echo hi > kept.txt` followed by
`stub-hold:300` is a turn that has written a file and is still running, and
`stub-run:cat kept.txt` in a later turn puts the file's content in the reply.
"""

import http.server
import json
import re
import ssl
import sys
import time

REPLY = "1\n2\n3\n4\n5"

# The longest a held turn may run. A typo must not pin a sandbox for a day.
MAX_HOLD_SECONDS = 900
HOLD_DIRECTIVE = re.compile(r"stub-hold:(\d+)")
RUN_DIRECTIVE = re.compile(r"stub-run:([^\n]+)")
# The engine's shell tool (Claude Code's name for it). A request that does not
# offer it, a title or summary side request for one, is never answered with a call.
RUN_TOOL = "Bash"
# A later turn runs in a fresh process, and the driver primes it by putting the
# earlier prompts and replies in front of the new prompt, inside these tags
# (lego/agent-image/driver/src/continuity.ts). They are history, not the request.
REPLAYED_CONTEXT = re.compile(r"<session-context>.*</session-context>", re.DOTALL)
# Seconds between two ticks of a held turn; tests shorten it.
TICK_SECONDS = 1.0


def _blocks(message: dict, kind: str) -> list:
    content = message.get("content")
    if not isinstance(content, list):
        return []
    return [b for b in content if isinstance(b, dict) and b.get("type") == kind]


def _text(message: dict) -> str:
    content = message.get("content")
    if isinstance(content, str):
        return content
    return "\n".join(block.get("text") or "" for block in _blocks(message, "text"))


def _user_messages(body: dict) -> list:
    return [
        m for m in body.get("messages") or []
        if isinstance(m, dict) and m.get("role") == "user"
    ]


def prompt_text(body: dict) -> str:
    """The text of the turn's prompt: the newest user message a person wrote.

    A user message that carries a tool's result is the engine reporting back
    inside the same turn, so it is skipped. Of the prompt only what follows the
    replayed earlier conversation counts: a directive in an earlier prompt must
    not act on every later turn.
    """
    for message in reversed(_user_messages(body)):
        if _blocks(message, "tool_result"):
            continue
        return REPLAYED_CONTEXT.sub("", _text(message))
    return ""


def hold_seconds(body: dict) -> int:
    """How long the turn's prompt asks the turn to be held, 0 for not at all."""
    found = HOLD_DIRECTIVE.findall(prompt_text(body))
    if not found:
        return 0
    return min(int(found[-1]), MAX_HOLD_SECONDS)


def tool_output(body: dict):
    """The output of the command the stub asked for, None before it has run."""
    users = _user_messages(body)
    if not users:
        return None
    results = _blocks(users[-1], "tool_result")
    if not results:
        return None
    content = results[-1].get("content")
    if isinstance(content, list):
        content = "\n".join(
            b.get("text") or "" for b in content if isinstance(b, dict) and b.get("type") == "text"
        )
    return content if isinstance(content, str) else ""


def run_command(body: dict) -> str:
    """The command the turn's prompt asks the engine to run, "" for none.

    Empty once the command has run (the engine's newest message is its result)
    and for a request that does not offer the shell tool.
    """
    if tool_output(body) is not None:
        return ""
    tools = body.get("tools") or []
    if not any(isinstance(t, dict) and t.get("name") == RUN_TOOL for t in tools):
        return ""
    found = RUN_DIRECTIVE.findall(prompt_text(body))
    return found[-1].strip() if found else ""


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
            try:
                command = run_command(body)
                if command:
                    self._stream_tool_call(model, command)
                else:
                    self._stream(model, hold_seconds(body), tool_output(body))
            except (BrokenPipeError, ConnectionResetError):
                # A stopped turn kills the engine, which drops this request.
                sys.stderr.write("model-stub: the client went away mid-stream\n")
                self.close_connection = True
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

    def _stream_start(self, model: str):
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Cache-Control", "no-cache")
        # The stream carries no length, so only closing the connection ends the
        # body. Kept alive, the engine read message_stop and then waited for an
        # end that never came: the turn stayed "running" for good (w22/m403 t011).
        self.send_header("Connection", "close")
        self.close_connection = True
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

    def _stream_end(self, stop_reason: str):
        self.wfile.write(sse("content_block_stop", {"type": "content_block_stop", "index": 0}))
        self.wfile.write(
            sse(
                "message_delta",
                {"type": "message_delta",
                 "delta": {"stop_reason": stop_reason, "stop_sequence": None},
                 "usage": {"output_tokens": 9}},
            )
        )
        self.wfile.write(sse("message_stop", {"type": "message_stop"}))
        self.wfile.flush()

    def _stream_tool_call(self, model: str, command: str):
        """One call of the engine's shell tool, which the engine then runs."""
        sys.stderr.write("model-stub: asking the engine to run a command\n")
        self._stream_start(model)
        self.wfile.write(
            sse(
                "content_block_start",
                {"type": "content_block_start", "index": 0,
                 "content_block": {"type": "tool_use", "id": f"toolu_devstub{time.time_ns()}",
                                   "name": RUN_TOOL, "input": {}}},
            )
        )
        arguments = json.dumps({"command": command, "description": "dev stub command"})
        self.wfile.write(
            sse(
                "content_block_delta",
                {"type": "content_block_delta", "index": 0,
                 "delta": {"type": "input_json_delta", "partial_json": arguments}},
            )
        )
        self._stream_end("tool_use")

    def _stream(self, model: str, hold: int = 0, output=None):
        self._stream_start(model)
        self.wfile.write(
            sse(
                "content_block_start",
                {"type": "content_block_start", "index": 0,
                 "content_block": {"type": "text", "text": ""}},
            )
        )
        # After a command the reply is its output; otherwise the fixed reply.
        chunks = REPLY.split("\n") if output is None else ["ran:", *output.rstrip("\n").split("\n")]
        if hold > 0:
            sys.stderr.write(f"model-stub: holding the turn for {hold}s\n")
            ticks = [f"tick {n}" for n in range(1, hold + 1)] + ["done"]
            chunks = ticks if output is None else ["\n".join(chunks), *ticks]
        for position, chunk in enumerate(chunks):
            if hold > 0 and position > 0:
                time.sleep(TICK_SECONDS)
            self.wfile.write(
                sse(
                    "content_block_delta",
                    {"type": "content_block_delta", "index": 0,
                     "delta": {"type": "text_delta", "text": chunk + "\n"}},
                )
            )
            self.wfile.flush()
        self._stream_end("end_turn")


def main():
    http.server.ThreadingHTTPServer.daemon_threads = True
    server = http.server.ThreadingHTTPServer(("0.0.0.0", 8443), Handler)
    context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    context.load_cert_chain("/etc/model-stub/tls.crt", "/etc/model-stub/tls.key")
    server.socket = context.wrap_socket(server.socket, server_side=True)
    sys.stderr.write("model-stub: serving https on :8443 as api.anthropic.com\n")
    server.serve_forever()


if __name__ == "__main__":
    main()
