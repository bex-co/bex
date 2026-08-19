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
"""

import argparse
import json
import os
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
    return subprocess.run(cmd, check=True, capture_output=True, text=True, **kw)


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
    """Reduce tools/list to the stable parity surface: names and argument names.

    Descriptions and annotations are deliberately excluded -- they drift for
    editorial reasons and would make the pin fire on non-contractual churn.
    """
    out = []
    for t in tools:
        schema = t.get("inputSchema") or {}
        out.append(
            {
                "name": t["name"],
                "args": sorted((schema.get("properties") or {}).keys()),
                "required": sorted(schema.get("required") or []),
            }
        )
    return sorted(out, key=lambda t: t["name"])


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--ref", default="main", help="upstream git ref to capture")
    ap.add_argument("--out", help="write JSON here instead of stdout")
    ap.add_argument(
        "--tools-only",
        action="store_true",
        help="emit only the tools array (what the drift check compares)",
    )
    args = ap.parse_args()

    source, tools = capture(args.ref)
    doc = tools if args.tools_only else {"source": source, "tools": tools}
    text = json.dumps(doc, indent=2, sort_keys=False) + "\n"

    if args.out:
        with open(args.out, "w", encoding="utf-8") as fh:
            fh.write(text)
        print(f"wrote {args.out} ({len(tools)} tools from {source['commit'][:12]})",
              file=sys.stderr)
    else:
        sys.stdout.write(text)


if __name__ == "__main__":
    main()
