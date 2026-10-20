# CLI sweep 824 — Postgres observability/exports read surfaces (live, 2026-10-20)

Read-only probes on the baseline Postgres (never mutated). No new bug filed.

- `GET /postgres/{id}/parameter-overrides` → 200 with the effective parameter set (`name`/`setting`/`source`/`description` per row).
- `GET /postgres/{id}/query/sizes` → 200 `{"sizes":[{"Database":…,"Bytes":…}]}` with the real database size.
- `GET /postgres/{id}/credentials` → 200 with usernames and the `default` flag only — no password field anywhere in the response.
- Exports live at the singular `/postgres/{id}/export` (the pinned spelling; my plural probe's bare `404 page not found` was a wrong-path harness error): GET → 200 `[]`. `GET /postgres/{id}/recovery` → 200 `{"recoveryStatus":"NOT_AVAILABLE"}` for the free baseline.
