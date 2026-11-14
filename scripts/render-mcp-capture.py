#!/usr/bin/env python3
"""Capture render-oss/render-mcp-server's registered MCP tool inventory.

This is the MCP counterpart of `scripts/render-schema-drift.sh`, which pins
Render's public REST contract. It clones the upstream server at a given ref,
builds it, drives it through the MCP stdio handshake, and asks it for its own
`tools/list` -- so the pin records what the server ACTUALLY registers rather
than what a regex over its source suggests.

Output is deterministic for a given upstream commit: tools and argument names
are sorted, and every recorded field is derived from the repository. Re-running
against the same commit produces byte-identical `tools` output.

Usage:
    scripts/render-mcp-capture.py --ref main [--out FILE] [--tools-only]
    scripts/render-mcp-capture.py --ref <pinned commit> --results \\
        --out lego/backend/internal/api/openapi/render-mcp-results.json

--results records the RESULT shapes (w4/m378) instead of tools/list: for each
tool bex shares with upstream, the form of the text its handler returns
(RESULT_FORMS below, read from each handler's json.Marshal / NewToolResultText
call) and the JSON keys of the Go type it marshals, parsed from the generated
client types at the same commit (required = no omitempty). Refresh it at the
commit the tools pin names whenever the tools pin moves; if upstream changes a
handler's formatting, edit RESULT_FORMS to match and re-run. The
rendertest.AssertRenderMCPResult checks in the feature packages then fail on
any bex answer that wraps, re-keys or drops what this file records.
"""

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

UPSTREAM = "https://github.com/render-oss/render-mcp-server.git"
REPO = "github.com/render-oss/render-mcp-server"

# The MCP stdio handshake: initialize, then the initialized notification, then
# tools/list. The server registers its tools at startup, so no real credential
# is needed -- a syntactically valid placeholder keeps it from exiting early.
HANDSHAKE = [
    {
        "jsonrpc": "2.0",
        "id": 1,
        "method": "initialize",
        "params": {
            "protocolVersion": "2024-11-05",
            "capabilities": {},
            "clientInfo": {"name": "bex-parity-capture", "version": "1"},
        },
    },
    {"jsonrpc": "2.0", "method": "notifications/initialized"},
    {"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": {}},
]


def run(cmd, **kw):
    try:
        return subprocess.run(cmd, check=True, capture_output=True, text=True, **kw)
    except subprocess.CalledProcessError as exc:
        # Preserve compiler diagnostics that capture_output otherwise hides.
        if exc.stderr:
            sys.stderr.write(exc.stderr)
        raise


def capture(ref):
    """Clone upstream at `ref`, build it, and return (provenance, tools)."""
    workdir = tempfile.mkdtemp(prefix="render-mcp-capture-")
    try:
        run(["git", "clone", "--quiet", UPSTREAM, workdir])
        run(["git", "checkout", "--quiet", ref], cwd=workdir)
        commit = run(["git", "rev-parse", "HEAD"], cwd=workdir).stdout.strip()
        commit_date = run(
            ["git", "log", "-1", "--format=%cI", "HEAD"], cwd=workdir
        ).stdout.strip()

        binary = os.path.join(workdir, "render-mcp-server.bin")
        run(["go", "build", "-o", binary, "."], cwd=workdir)

        payload = "".join(json.dumps(m) + "\n" for m in HANDSHAKE)
        env = dict(os.environ, RENDER_API_KEY="rnd_placeholder_capture_only")
        proc = subprocess.run(
            [binary],
            input=payload,
            capture_output=True,
            text=True,
            env=env,
            timeout=120,
        )

        tools = None
        for line in proc.stdout.splitlines():
            line = line.strip()
            if not line.startswith("{"):
                continue
            try:
                msg = json.loads(line)
            except json.JSONDecodeError:
                continue
            if msg.get("id") == 2 and "result" in msg:
                tools = msg["result"]["tools"]
                break
        if tools is None:
            raise SystemExit(
                "render-mcp-capture: no tools/list response from upstream server\n"
                f"stderr: {proc.stderr[:2000]}"
            )

        return (
            {"repo": REPO, "ref": ref, "commit": commit, "commitDate": commit_date},
            normalize(tools),
        )
    finally:
        shutil.rmtree(workdir, ignore_errors=True)


def normalize(tools):
    """Reduce tools/list to the stable parity surface.

    Kept: names, argument names, required arguments, each argument's JSON
    type (and enum), and the behavior hints a client acts on
    (readOnlyHint/destructiveHint/idempotentHint/openWorldHint) — w5/m145: a
    type mismatch breaks a call as surely as a missing argument, and a missing
    readOnlyHint makes a client prompt before every read. Descriptions and
    annotation titles stay out: they drift editorially.
    """
    out = []
    for t in tools:
        schema = t.get("inputSchema") or {}
        props = schema.get("properties") or {}
        arg_types = {}
        for name in sorted(props):
            prop = props[name] or {}
            entry = {"type": prop.get("type", "")}
            if prop.get("enum"):
                entry["enum"] = sorted(str(v) for v in prop["enum"])
            arg_types[name] = entry
        hints = {
            k: v
            for k, v in sorted((t.get("annotations") or {}).items())
            if k in ("readOnlyHint", "destructiveHint", "idempotentHint", "openWorldHint")
        }
        out.append(
            {
                "name": t["name"],
                "args": sorted(props.keys()),
                "required": sorted(schema.get("required") or []),
                "argTypes": arg_types,
                "annotations": hints,
            }
        )
    return sorted(out, key=lambda t: t["name"])


# RESULT_FORMS is each shared tool's answer as upstream's handler formats it
# (pkg/<area>/tools.go): "object" or "array" is json.Marshal of the named Go
# type (or of a slice of it), "arrayCursor" that array followed by
# "\n\n cursor: <cursor>", "text" a sentence. emptyText is the sentence the
# handler answers for an empty list. A type of "string" is a JSON string, and
# a type of None leaves the items unchecked (query_render_postgres' row
# objects are keyed by the caller's own columns).
RESULT_FORMS = {
    "approve_blueprint": {"form": "object"},
    "create_blueprint": {"form": "object"},
    "create_cron_job": {"form": "object", "type": "ServiceAndDeploy"},
    "create_key_value": {"form": "object", "type": "KeyValueDetail"},
    "create_postgres": {"form": "object", "type": "PostgresDetail"},
    "create_static_site": {"form": "object", "type": "ServiceAndDeploy"},
    "create_web_service": {"form": "object", "type": "ServiceAndDeploy"},
    "get_blueprint": {"form": "object"},
    "get_deploy": {"form": "object", "type": "Deploy"},
    "get_key_value": {"form": "object", "type": "KeyValueDetail"},
    "get_metrics": {"form": "object", "type": "MetricsResponse"},
    "get_postgres": {"form": "object", "type": "PostgresDetail"},
    "get_service": {"form": "object", "type": "Service"},
    "list_deploys": {"form": "arrayCursor", "type": "Deploy"},
    "list_events": {"form": "arrayCursor", "type": "ServiceEvent"},
    "list_key_value": {"form": "array", "type": "KeyValue", "emptyText": "No Key Value instances found"},
    "list_log_label_values": {"form": "array", "type": "string"},
    "list_logs": {"form": "object", "type": "Logs200Response"},
    "list_postgres_instances": {"form": "array", "type": "Postgres", "emptyText": "No Postgres instances found"},
    "list_services": {"form": "array", "type": "Service"},
    "list_workspaces": {"form": "array", "type": "Owner"},
    "plan_blueprint": {"form": "object"},
    "query_render_postgres": {"form": "array", "type": None},
    "trigger_deploy": {"form": "object", "type": "Deploy"},
    "update_blueprint": {"form": "object"},
    "update_environment_variables": {"form": "text"},
}

# RESULT_TYPES names where each recorded Go type is declared, and which of its
# fields hold another recorded type (an array field's items, or an object).
RESULT_TYPES = {
    "Deploy": ("pkg/client/types_gen.go", {}),
    "KeyValue": ("pkg/client/types_gen.go", {"owner": "Owner", "options": "KeyValueOptions"}),
    "KeyValueDetail": ("pkg/client/types_gen.go", {"owner": "Owner", "options": "KeyValueOptions"}),
    "KeyValueOptions": ("pkg/client/types_gen.go", {}),
    "Log": ("pkg/client/logs/logs_gen.go", {"labels": "LogLabel"}),
    "LogLabel": ("pkg/client/logs/logs_gen.go", {}),
    "Logs200Response": ("pkg/client/types_gen.go", {"logs": "Log"}),
    "MetricData": ("pkg/metrics/repo.go", {"data": "TimeSeries"}),
    "MetricsResponse": ("pkg/metrics/repo.go", {"metrics": "MetricData"}),
    "Owner": ("pkg/client/types_gen.go", {}),
    "Postgres": ("pkg/client/types_gen.go", {"owner": "Owner"}),
    "PostgresDetail": ("pkg/client/types_gen.go", {"owner": "Owner"}),
    "Service": ("pkg/client/types_gen.go", {}),
    "ServiceAndDeploy": ("pkg/client/types_gen.go", {"service": "Service"}),
    "ServiceEvent": ("pkg/client/events/events_gen.go", {}),
    "TimeSeries": ("pkg/client/metrics/metrics_gen.go", {"labels": "TimeSeriesLabel", "values": "TimeSeriesValue"}),
    "TimeSeriesLabel": ("pkg/client/metrics/metrics_gen.go", {}),
    "TimeSeriesValue": ("pkg/client/metrics/metrics_gen.go", {}),
}

# Go type names that differ from the recorded name (metrics' Label would
# collide with nothing today, but reads better qualified).
GO_TYPE_NAME = {"TimeSeriesLabel": "Label"}

JSON_TAG = re.compile(r'`json:"([^",]*)(,[^"]*)?"`')


def struct_keys(source, go_name):
    """Return (required, optional) JSON keys of a Go struct, top level only.

    A field whose tag carries omitempty is optional; every other tagged field
    is always present in json.Marshal's output. Nested anonymous structs count
    as one field (the tag on their closing brace).
    """
    lines = source.splitlines()
    start = None
    for i, line in enumerate(lines):
        if re.match(rf"^type {re.escape(go_name)} struct \{{", line):
            start = i
            break
    if start is None:
        raise SystemExit(f"render-mcp-capture: no struct {go_name}")
    required, optional = [], []
    depth = 0
    for line in lines[start:]:
        opened, closed = line.count("{"), line.count("}")
        before = depth
        depth += opened - closed
        tag = JSON_TAG.search(line)
        # A field at the struct's own level: depth 1 on a plain line, or the
        # closing line of a nested anonymous struct back at depth 1.
        if tag and depth == 1 and (before == 1 or closed > 0):
            name, opts = tag.group(1), tag.group(2) or ""
            if name and name != "-":
                (optional if "omitempty" in opts else required).append(name)
        if depth == 0 and before > 0:
            break
    return sorted(required), sorted(optional)


def capture_results(ref):
    """Clone upstream at `ref` and return (provenance, results document)."""
    workdir = tempfile.mkdtemp(prefix="render-mcp-results-")
    try:
        run(["git", "clone", "--quiet", UPSTREAM, workdir])
        run(["git", "checkout", "--quiet", ref], cwd=workdir)
        commit = run(["git", "rev-parse", "HEAD"], cwd=workdir).stdout.strip()
        commit_date = run(
            ["git", "log", "-1", "--format=%cI", "HEAD"], cwd=workdir
        ).stdout.strip()
        types = {}
        for name in sorted(RESULT_TYPES):
            path, fields = RESULT_TYPES[name]
            with open(os.path.join(workdir, path), encoding="utf-8") as fh:
                required, optional = struct_keys(fh.read(), GO_TYPE_NAME.get(name, name))
            entry = {"required": required, "optional": optional}
            if fields:
                entry["fields"] = dict(sorted(fields.items()))
            types[name] = entry
        source = {"repo": REPO, "ref": ref, "commit": commit, "commitDate": commit_date}
        tools = {}
        for name in sorted(RESULT_FORMS):
            spec = {k: v for k, v in RESULT_FORMS[name].items() if v is not None}
            tools[name] = spec
        return source, {"source": source, "tools": tools, "types": types}
    finally:
        shutil.rmtree(workdir, ignore_errors=True)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--ref", default="main", help="upstream git ref to capture")
    ap.add_argument("--out", help="write JSON here instead of stdout")
    ap.add_argument(
        "--tools-only",
        action="store_true",
        help="emit only the tools array (what the drift check compares)",
    )
    ap.add_argument(
        "--results",
        action="store_true",
        help="emit the shared tools' result shapes instead of tools/list",
    )
    args = ap.parse_args()

    if args.results:
        source, doc = capture_results(args.ref)
        count = len(doc["tools"])
    else:
        source, tools = capture(args.ref)
        doc = tools if args.tools_only else {"source": source, "tools": tools}
        count = len(tools)
    text = json.dumps(doc, indent=2, sort_keys=False) + "\n"

    if args.out:
        with open(args.out, "w", encoding="utf-8") as fh:
            fh.write(text)
        print(f"wrote {args.out} ({count} tools from {source['commit'][:12]})",
              file=sys.stderr)
    else:
        sys.stdout.write(text)


if __name__ == "__main__":
    main()
