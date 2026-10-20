# CLI sweep 684 — Postgres connection-info shape (live, 2026-10-19)

Read-only against the baseline Postgres; secret values were never printed, only key presence.

- `GET /postgres/{baseline}/connection-info` → 200 with keys `externalConnectionString`, `internalConnectionString`, `password`, `psqlCommand`, `serverCaCertificate` (the CA field is the documented bex extension, ADR018).
- Synthetic id → 404 `NOT_FOUND`.

No bug filed.
