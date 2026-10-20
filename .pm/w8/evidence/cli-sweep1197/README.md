# CLI sweep 1197 — KV connection-info refuses while credentials converge (live, 2026-10-20)

One disposable free Key Value instance created and deleted (204, verified 404). No new bug filed. Credential values were never printed (only status/keys inspected).

- `GET /key-value/{id}/connection-info` immediately after create → 409 `key value credentials are still converging`: the credential read refuses with a coded conflict during provisioning instead of returning half-formed connection details.
- KV rename via PATCH → 200. After DELETE, connection-info → 404.
