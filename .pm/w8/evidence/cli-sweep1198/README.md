# CLI sweep 1198 — Postgres credentials live only on the dedicated path (live, 2026-10-20)

Read-only on the baseline Postgres (never mutated). No new bug filed. Credential values were never printed or recorded — only key names.

- `GET /postgres/{id}/connection-info` → 200 with the credential key set (`externalConnectionString`, `internalConnectionString`, `password`, `psqlCommand`, `serverCaCertificate`): this dedicated endpoint is the one place connection secrets are served, to the authenticated owner.
- The general Postgres read carries no password/connection-string keys (only the non-secret `connectionPool` config), matching the secrets-never-in-general-reads rule.
