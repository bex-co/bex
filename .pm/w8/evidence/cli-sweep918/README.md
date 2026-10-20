# CLI sweep 918 — synthetic datastore verb discipline (live, 2026-10-20)

Refused/synthetic probes only; the baseline Postgres was never touched (an export-create idea against it was deliberately not run). No new bug filed.

- `POST /blueprints/generate {"postgresIds":[synthetic]}` → coded 404 (selection members resolve before generation).
- `POST /postgres/{synthetic}/credentials` → 405 `method_not_allowed` (credential creation is not a REST verb; rotation is), and `…/credentials/{name}/rotate` on a synthetic database → bare 404.
