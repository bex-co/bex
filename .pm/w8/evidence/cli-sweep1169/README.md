# CLI sweep 1169 — MCP endpoint handshake discipline (live, 2026-10-20)

Read-only MCP probes; no tools were invoked and nothing was created. No new bug filed.

- `tools/list` over JSON-RPC at `/mcp` → 200 with the tool catalog (the per-type create/get/update/delete tool set live-verified in sweeps 1103–1107).
- An unknown JSON-RPC method → 400 (`JSON RPC not handled: … unsupported`), and a body missing the `jsonrpc` field → 400: the envelope is validated before dispatch.
