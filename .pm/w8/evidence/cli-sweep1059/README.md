# CLI sweep 1059 — MCP transport discovery (live, 2026-10-20)

Read-only MCP probes with the QA session (initialize + `tools/list` only; no tool called). No new bug filed.

- `POST /mcp initialize` → 200 SSE-framed JSON-RPC result (`serverInfo: bex 0.1.0`, protocol `2024-11-05`, tools capability with `listChanged`). `tools/list` → 200 with the tool catalog. The MCP surface answers the standard handshake under the same session auth as REST/GraphQL.
