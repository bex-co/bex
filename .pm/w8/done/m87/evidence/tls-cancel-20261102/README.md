# m87 live cancellation check — 2026-11-02

Production `*.db.bex.co` endpoint (pg-sni-proxy, TLS passthrough), fresh Free PostgreSQL 18 `qa-20261102-w8m87-pg` (`dpg-dbk71uj0tl4c738gc3ag`) in `bex-canary`, client libpq 18.0.6 (Homebrew) with [cancel.c](cancel.c): `SELECT pg_sleep(12)`, cancel after 2 s, then reuse the connection.

| cancel path | result |
| --- | --- |
| libpq 17+ TLS cancel (`PQcancelCreate` + `PQcancelBlocking`) | `57014` after 2.71 s; same backend pid reused |
| legacy `PQcancel` (what psql sends on Ctrl+C) | reported sent, query ran to completion (12.25 s, `PGRES_TUPLES_OK`) |
| `SELECT pg_cancel_backend(pid)` from a second psql session | first session: `ERROR: canceling statement due to user request` |

Local control (postgres:18 container, no proxy): TLS cancel `57014` after 2.01 s, same pid reused. Fixture deleted by ID (`DELETE` 204, `GET` 404); the workspace's pre-existing `beancount-backend-db` untouched; QA sessions revoked.
