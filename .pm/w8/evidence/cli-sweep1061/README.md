# CLI sweep 1061 — MCP env read on a fresh service (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- MCP `list_env_vars` on a service with no vars → `{"envVars":[]}` in both the text content and `structuredContent` — an honest empty result over MCP, matching the REST empty list.
