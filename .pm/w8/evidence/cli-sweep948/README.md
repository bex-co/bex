# CLI sweep 948 — Postgres read subpaths inventory (live, 2026-10-20)

Read-only probes on the baseline (never mutated). No new bug filed.

- `GET /postgres/{id}/ip-allow-list` → 200 `{"cidrs":["0.0.0.0/0","::/0"]}` (the open default, string-list shape as on Key Value).
- `/query/biggest-tables`, `/query/bloat`, and `/metrics/active-connections` under the database → 404: those are not bex routes (the registered query surface is the `sizes` read verified in sweep 824; datastore metrics ride the top-level `/metrics/*` family). Wrong-path probes, recorded so they are not later mistaken for missing features.
