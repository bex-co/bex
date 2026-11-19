#!/usr/bin/env python3
"""Drive bex-api's MCP endpoint as the QA user without exposing a credential.

    python3 scripts/qa-mcp.py [--url URL] [--key-file FILE | --anonymous] COMMAND ...

The /qa-find-bugs-mcp hunt (.claude/skills/qa-find-bugs-mcp/SKILL.md) sends
every request through this script, so the bearer never reaches argv, a tool
call or a transcript.

Credentials
  mint-key --jar JAR --out FILE [--owner-id ID] [--name NAME] [--permissions JSON]
      Mint an API key with the Kratos session in JAR (the cookie jar that
      `bash scripts/qa-login.sh` names on its `logout` line) and write
      {id, name, ownerId, key} to FILE, created 0600 and never overwritten.
      Prints the key's id and name only. JSON is a permissions grant
      ({"grants": [{"permission": ..., "level": ...}]}); omitted means All.
  revoke-key --jar JAR --key-file FILE
      Revoke that key through the same session and delete FILE.

MCP (POST URL, default https://api.bex.co/mcp; the bearer is FILE's key)
  initialize [--protocol-version V]   tools-list   resources-list
  prompts-list   call TOOL [JSON-ARGS]   rpc METHOD [JSON-PARAMS]
  notify METHOD [JSON-PARAMS]
  raw [--method M] [--accept A] [--content-type T] [--body TEXT] [--no-protocol-header]

Controls with the same bearer, on the URL's origin
  rest METHOD PATH [JSON-BODY]        graphql QUERY [JSON-VARIABLES]
  get-url URL                         an unauthenticated GET (discovery documents)

Each response prints its status, elapsed time, an allowlist of headers and the
body: the JSON-RPC messages of a JSON or SSE answer, or the raw text. The key,
any `bex_<id>_<secret>` or `ory_*_` token and the session cookie are redacted
from everything printed. Exit 0 whatever the HTTP status (the status is the
result), 2 for usage or credential problems, 3 for a network failure or a
timeout. HTTP is refused except on loopback.
"""

import argparse
import datetime
import http.client
import ipaddress
import json
import os
import re
import secrets
import socket
import stat
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

DEFAULT_URL = "https://api.bex.co/mcp"
DEFAULT_PROTOCOL = "2025-06-18"
SESSION_COOKIE = "ory_kratos_session"
SHOWN_HEADERS = (
    "content-type",
    "www-authenticate",
    "allow",
    "mcp-protocol-version",
    "mcp-session-id",
    "retry-after",
    "ratelimit-limit",
    "ratelimit-remaining",
    "ratelimit-reset",
    "rndr-id",
    "x-request-id",
    "location",
    "cf-ray",
)
TOKEN_PATTERNS = (
    re.compile(r"bex_[0-9A-Za-z-]+_[0-9A-Za-z._~-]+"),
    re.compile(r"ory_[a-z]{2}_[0-9A-Za-z._~-]+"),
)
MAX_PRINTED = 200_000


class UsageError(Exception):
    """A bad argument or an unusable credential: exit 2."""


class NetworkError(Exception):
    """No HTTP answer at all (refused, reset, DNS, timeout): exit 3."""


class Redactor:
    def __init__(self):
        self.secrets = []

    def add(self, value):
        if value and len(value) >= 6:
            self.secrets.append(value)

    def __call__(self, text):
        for value in self.secrets:
            text = text.replace(value, "[REDACTED]")
        for pattern in TOKEN_PATTERNS:
            text = pattern.sub("[REDACTED]", text)
        return text


REDACT = Redactor()


def out(text=""):
    print(REDACT(text))


def check_url(url):
    parts = urllib.parse.urlsplit(url)
    if parts.scheme not in ("https", "http") or not parts.hostname:
        raise UsageError(f"not an http(s) URL: {url}")
    if parts.scheme == "http" and not is_loopback(parts.hostname):
        raise UsageError("plain HTTP is accepted only on loopback")
    return parts


def is_loopback(host):
    if host == "localhost":
        return True
    try:
        return ipaddress.ip_address(host).is_loopback
    except ValueError:
        return False


def origin_of(url):
    parts = check_url(url)
    return f"{parts.scheme}://{parts.netloc}"


def private_file(path):
    """Refuse a credential file that others can read: it holds a live bearer."""
    try:
        info = os.stat(path)
    except OSError as err:
        raise UsageError(f"cannot read {path}: {err.strerror}") from None
    if not stat.S_ISREG(info.st_mode):
        raise UsageError(f"{path} is not a regular file")
    if info.st_mode & 0o077:
        raise UsageError(f"{path} is readable by others; chmod 600 it")
    if info.st_uid != os.getuid():
        raise UsageError(f"{path} belongs to another user")


def load_key(path):
    private_file(path)
    try:
        with open(path, encoding="utf-8") as handle:
            record = json.load(handle)
    except (OSError, ValueError) as err:
        raise UsageError(f"{path} is not a key file written by mint-key: {err}") from None
    key = record.get("key") if isinstance(record, dict) else None
    if not isinstance(key, str) or not key.startswith("bex_"):
        raise UsageError(f"{path} holds no bex_ bearer key")
    REDACT.add(key)
    return record


def load_session(path):
    """The Kratos session cookie from a curl/Netscape jar, never printed."""
    private_file(path)
    value = None
    with open(path, encoding="utf-8") as handle:
        for line in handle:
            line = line.rstrip("\n")
            if line.startswith("#HttpOnly_"):
                line = line[len("#HttpOnly_"):]
            elif line.startswith("#") or not line.strip():
                continue
            fields = line.split("\t")
            if len(fields) == 7 and fields[5] == SESSION_COOKIE:
                value = fields[6]
    if not value:
        raise UsageError(f"{path} holds no {SESSION_COOKIE} cookie; sign in with scripts/qa-login.sh")
    REDACT.add(value)
    return value


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args, **kwargs):
        return None


OPENER = urllib.request.build_opener(NoRedirect)


def send(method, url, headers, body, timeout):
    check_url(url)
    data = body.encode("utf-8") if isinstance(body, str) else body
    request = urllib.request.Request(url, data=data, method=method, headers=headers)
    started = time.monotonic()
    try:
        response = OPENER.open(request, timeout=timeout)
    except urllib.error.HTTPError as err:
        response = err
    except (urllib.error.URLError, OSError, http.client.HTTPException) as err:
        reason = getattr(err, "reason", err)
        if isinstance(reason, (socket.timeout, TimeoutError)) or "timed out" in str(reason):
            raise NetworkError(f"{method} {url}: no answer within {timeout}s") from None
        raise NetworkError(f"{method} {url}: {reason}") from None
    try:
        raw = response.read()
    except (socket.timeout, TimeoutError):
        raise NetworkError(f"{method} {url}: the answer did not finish within {timeout}s") from None
    except (OSError, http.client.HTTPException) as err:
        raise NetworkError(f"{method} {url}: the answer was cut: {err}") from None
    elapsed = time.monotonic() - started
    return response.status, response.headers, raw.decode("utf-8", "replace"), elapsed


def show(method, url, status, headers, text, elapsed):
    out(f"{method} {url} -> {status} in {elapsed * 1000:.0f} ms")
    for name in SHOWN_HEADERS:
        for value in headers.get_all(name) or []:
            out(f"  {name}: {value}")
    messages = parse_messages(headers.get("content-type", ""), text)
    if messages is None:
        body = text if len(text) <= MAX_PRINTED else text[:MAX_PRINTED] + f"\n… cut at {MAX_PRINTED} of {len(text)} characters"
        if body:
            out(body)
        return []
    for message in messages:
        out(json.dumps(message, indent=2, ensure_ascii=False))
    return messages


def parse_messages(content_type, text):
    """JSON-RPC messages of a JSON or SSE answer; None when it is neither."""
    if "text/event-stream" in content_type:
        messages, data = [], []
        for line in text.splitlines() + [""]:
            if line.startswith("data:"):
                data.append(line[5:].lstrip(" "))
            elif not line and data:
                try:
                    messages.append(json.loads("\n".join(data)))
                except ValueError:
                    messages.append({"unparsed_event_data": "\n".join(data)})
                data = []
        return messages
    if "json" in content_type:
        try:
            return [json.loads(text)]
        except ValueError:
            return None
    return None


def bearer_headers(args):
    if args.anonymous:
        return {}
    if not args.key_file:
        raise UsageError("pass --key-file FILE (from mint-key) or --anonymous")
    return {"Authorization": "Bearer " + load_key(args.key_file)["key"]}


def json_arg(text, what):
    if text is None:
        return None
    try:
        return json.loads(text)
    except ValueError as err:
        raise UsageError(f"{what} is not JSON: {err}") from None


def mcp_post(args, payload, protocol=True):
    headers = {
        "Accept": "application/json, text/event-stream",
        "Content-Type": "application/json",
        "User-Agent": "bex-qa-mcp/1",
        **bearer_headers(args),
    }
    if protocol:
        headers["MCP-Protocol-Version"] = args.protocol_version
    body = json.dumps(payload)
    status, response_headers, text, elapsed = send("POST", args.url, headers, body, args.timeout)
    return show("POST", args.url, status, response_headers, text, elapsed)


def rpc_payload(method, params, notification=False):
    payload = {"jsonrpc": "2.0", "method": method}
    if not notification:
        payload["id"] = secrets.randbelow(1 << 30) + 1
    if params is not None:
        payload["params"] = params
    return payload


def cmd_initialize(args):
    mcp_post(
        args,
        rpc_payload(
            "initialize",
            {
                "protocolVersion": args.protocol_version,
                "capabilities": {},
                "clientInfo": {"name": "bex-qa-mcp", "version": "1"},
            },
        ),
    )


def list_all(args, method, field):
    """Follow nextCursor; print each page and a summary of every name."""
    cursor, names, pages = None, [], 0
    while True:
        params = {"cursor": cursor} if cursor else None
        messages = mcp_post(args, rpc_payload(method, params))
        pages += 1
        result = next((m.get("result") for m in messages if isinstance(m, dict) and "result" in m), None)
        if not isinstance(result, dict):
            if pages == 1:
                out(f"{method}: no result (see the answer above)")
                return
            out(f"{method}: page {pages} carried no result; the names below are partial")
            break
        names += [item.get("name") or item.get("uri") for item in result.get(field) or []]
        cursor = result.get("nextCursor")
        if not cursor or pages >= 50:
            break
    out(f"{method}: {len(names)} {field} over {pages} page(s)")
    duplicates = sorted({n for n in names if names.count(n) > 1})
    if duplicates:
        out(f"duplicate names: {', '.join(map(str, duplicates))}")
    out(" ".join(sorted(map(str, names))))


def cmd_call(args):
    mcp_post(args, rpc_payload("tools/call", {"name": args.tool, "arguments": json_arg(args.arguments, "arguments") or {}}))


def cmd_rpc(args):
    mcp_post(args, rpc_payload(args.method, json_arg(args.params, "params")))


def cmd_notify(args):
    mcp_post(args, rpc_payload(args.method, json_arg(args.params, "params"), notification=True))


def cmd_raw(args):
    headers = {"User-Agent": "bex-qa-mcp/1", **bearer_headers(args)}
    if args.accept is not None:
        headers["Accept"] = args.accept
    if args.content_type is not None:
        headers["Content-Type"] = args.content_type
    if not args.no_protocol_header:
        headers["MCP-Protocol-Version"] = args.protocol_version
    body = args.body if args.method not in ("GET", "HEAD") else None
    status, response_headers, text, elapsed = send(args.method, args.url, headers, body, args.timeout)
    show(args.method, args.url, status, response_headers, text, elapsed)


def cmd_rest(args):
    if not args.path.startswith("/"):
        raise UsageError("PATH must start with /, e.g. /v1/services?limit=5")
    url = origin_of(args.url) + args.path
    headers = {"Accept": "application/json", "User-Agent": "bex-qa-mcp/1", **bearer_headers(args)}
    body = None
    if args.body is not None:
        json_arg(args.body, "body")
        headers["Content-Type"] = "application/json"
        body = args.body
    status, response_headers, text, elapsed = send(args.method.upper(), url, headers, body, args.timeout)
    show(args.method.upper(), url, status, response_headers, text, elapsed)


def cmd_graphql(args):
    url = origin_of(args.url) + "/graphql"
    payload = {"query": args.query}
    variables = json_arg(args.variables, "variables")
    if variables is not None:
        payload["variables"] = variables
    headers = {"Accept": "application/json", "Content-Type": "application/json", "User-Agent": "bex-qa-mcp/1", **bearer_headers(args)}
    status, response_headers, text, elapsed = send("POST", url, headers, json.dumps(payload), args.timeout)
    show("POST", url, status, response_headers, text, elapsed)


def cmd_get_url(args):
    status, headers, text, elapsed = send("GET", args.target, {"Accept": "application/json", "User-Agent": "bex-qa-mcp/1"}, None, args.timeout)
    show("GET", args.target, status, headers, text, elapsed)


def session_headers(jar):
    return {
        "Accept": "application/json",
        "Content-Type": "application/json",
        "Cookie": f"{SESSION_COOKIE}={load_session(jar)}",
        "User-Agent": "bex-qa-mcp/1",
    }


def cmd_mint_key(args):
    if os.path.lexists(args.out):
        raise UsageError(f"{args.out} exists; mint-key never overwrites a key file")
    name = args.name or f"qa-{datetime.date.today():%Y%m%d}-{secrets.token_hex(3)}-mcp"
    body = {"name": name}
    if args.owner_id:
        body["ownerId"] = args.owner_id
    permissions = json_arg(args.permissions, "permissions")
    if permissions is not None:
        body["permissions"] = permissions
    url = origin_of(args.url) + "/v1/api-keys"
    headers = session_headers(args.jar)
    status, response_headers, text, elapsed = send("POST", url, headers, json.dumps(body), args.timeout)
    if status != 201:
        show("POST", url, status, response_headers, text, elapsed)
        raise UsageError(f"the key was not minted ({status})")
    try:
        key = json.loads(text)
    except ValueError:
        raise UsageError("the create answer is not JSON; no key file written") from None
    REDACT.add(key.get("secret"))
    REDACT.add(key.get("key"))
    if not str(key.get("key", "")).startswith("bex_") or not key.get("id"):
        raise UsageError("the create answer carries no bex_ key; nothing written")
    record = {"id": key["id"], "name": key.get("name", name), "ownerId": args.owner_id or "", "key": key["key"]}
    fd = os.open(args.out, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8") as handle:
        json.dump(record, handle)
    out(f"ok id={record['id']} name={record['name']} file={args.out} ({elapsed * 1000:.0f} ms)")


def cmd_revoke_key(args):
    record = load_key(args.revoke_key_file)
    query = f"?ownerId={urllib.parse.quote(record['ownerId'])}" if record.get("ownerId") else ""
    url = origin_of(args.url) + "/v1/api-keys/" + urllib.parse.quote(record["id"]) + query
    status, response_headers, text, elapsed = send("DELETE", url, session_headers(args.jar), None, args.timeout)
    if status not in (204, 404):
        show("DELETE", url, status, response_headers, text, elapsed)
        raise UsageError(f"the key {record['id']} was not revoked ({status}); the key file is kept")
    os.remove(args.revoke_key_file)
    gone = "revoked" if status == 204 else "already gone (404)"
    out(f"ok id={record['id']} {gone}; removed {args.revoke_key_file}")


def parser():
    top = argparse.ArgumentParser(prog="qa-mcp.py", description=__doc__.split("\n\n")[0])
    top.add_argument("--url", default=DEFAULT_URL, help="MCP endpoint (default %(default)s)")
    top.add_argument("--key-file", help="key file written by mint-key")
    top.add_argument("--anonymous", action="store_true", help="send no credential")
    top.add_argument("--protocol-version", default=DEFAULT_PROTOCOL)
    top.add_argument("--timeout", type=float, default=60.0, help="seconds per request (default %(default)s)")
    sub = top.add_subparsers(dest="command", required=True)

    sub.add_parser("initialize").set_defaults(run=cmd_initialize)
    sub.add_parser("tools-list").set_defaults(run=lambda a: list_all(a, "tools/list", "tools"))
    sub.add_parser("resources-list").set_defaults(run=lambda a: list_all(a, "resources/list", "resources"))
    sub.add_parser("prompts-list").set_defaults(run=lambda a: list_all(a, "prompts/list", "prompts"))

    call = sub.add_parser("call")
    call.add_argument("tool")
    call.add_argument("arguments", nargs="?")
    call.set_defaults(run=cmd_call)

    for name, run in (("rpc", cmd_rpc), ("notify", cmd_notify)):
        command = sub.add_parser(name)
        command.add_argument("method")
        command.add_argument("params", nargs="?")
        command.set_defaults(run=run)

    raw = sub.add_parser("raw")
    raw.add_argument("--method", default="POST")
    raw.add_argument("--accept")
    raw.add_argument("--content-type")
    raw.add_argument("--body", default="")
    raw.add_argument("--no-protocol-header", action="store_true")
    raw.set_defaults(run=cmd_raw)

    rest = sub.add_parser("rest")
    rest.add_argument("method")
    rest.add_argument("path")
    rest.add_argument("body", nargs="?")
    rest.set_defaults(run=cmd_rest)

    graphql = sub.add_parser("graphql")
    graphql.add_argument("query")
    graphql.add_argument("variables", nargs="?")
    graphql.set_defaults(run=cmd_graphql)

    get_url = sub.add_parser("get-url")
    get_url.add_argument("target")
    get_url.set_defaults(run=cmd_get_url)

    mint = sub.add_parser("mint-key")
    mint.add_argument("--jar", required=True)
    mint.add_argument("--out", required=True)
    mint.add_argument("--owner-id")
    mint.add_argument("--name")
    mint.add_argument("--permissions")
    mint.set_defaults(run=cmd_mint_key)

    revoke = sub.add_parser("revoke-key")
    revoke.add_argument("--jar", required=True)
    revoke.add_argument("--key-file", dest="revoke_key_file", required=True)
    revoke.set_defaults(run=cmd_revoke_key)
    return top


def main(argv=None):
    args = parser().parse_args(argv)
    try:
        check_url(args.url)
        args.run(args)
    except UsageError as err:
        print(REDACT(f"qa-mcp: {err}"), file=sys.stderr)
        return 2
    except NetworkError as err:
        print(REDACT(f"qa-mcp: {err}"), file=sys.stderr)
        return 3
    return 0


if __name__ == "__main__":
    sys.exit(main())
