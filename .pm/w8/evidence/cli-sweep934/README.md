# CLI sweep 934 — Key Value connection-info lifecycle gate (live, 2026-10-20)

Two disposable free Key Value instances created, read, deleted (204s; GETs 404). Credential values were never printed (key names only). No new bug filed.

- `GET /key-value/{id}/connection-info` while `status: creating` → named 409 `conflict: key value credentials are still converging` — the credential surface fails closed during provisioning instead of handing out a half-ready connection string.
- After ~40 s (`available`), the same call → 200 with exactly `cliCommand` / `externalConnectionString` / `internalConnectionString`; after delete → 404.
