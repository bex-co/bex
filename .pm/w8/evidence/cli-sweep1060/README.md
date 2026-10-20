# CLI sweep 1060 — MCP read tool agrees with REST (live, 2026-10-20)

Read-only MCP probes. No fixture created. No new bug filed.

- The MCP catalog exposes 189 tools. `list_services` (a read tool) answers over the same session and returns the workspace's services in the same field shape as REST — the fourth transport (after REST, GraphQL, CLI) reading one store.
