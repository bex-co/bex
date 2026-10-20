# CLI sweep 1105 — MCP delete lands in the same store (live, 2026-10-20)

One disposable free web service created via REST and deleted via MCP. No new bug filed.

- MCP `get_service` reads the fixture; MCP `delete_service` → `{"deleted":true}` (text and structured content agree); the REST GET immediately 404s. MCP is a full peer transport for the service lifecycle, not a read-only mirror.
