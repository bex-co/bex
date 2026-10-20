# CLI sweep 1063 — MCP error shapes (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- An unknown MCP tool → JSON-RPC error `-32602` naming the tool. A wrong-typed argument → a tool-level `isError` result with the schema path spelled out (`/properties/serviceId: type: 12345 has type "integer", want "string"`). Both failures are structured and instructive; neither is a bare 500.
