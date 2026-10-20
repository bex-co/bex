# CLI sweep 975 — Postgres read surface on the baseline (live, 2026-10-20)

Read-only probes on the baseline Postgres `qa-20261017-64552e-pg14`; nothing mutated, no export/snapshot triggered. No new bug filed.

- `GET /postgres/{id}` → 200 with the full database view (plan, version, region, allow-list, pooler/replica flags).
- `GET /postgres/{id}/connection-info` → 200 for the owner with the expected credential keys (external/internal connection strings, password, psql command, CA cert). Values were never printed or stored — only the key set and a response hash were recorded.
- Unknown and malformed database ids → 404; the owner-scoped list → 200; a guessed `/schemas` subresource correctly 404s.
