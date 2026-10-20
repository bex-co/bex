# CLI sweep 912 — root/meta path surface (live, 2026-10-20)

Read-only probes. No new bug filed.

- `GET /v1/`, `/healthz`, `/health`, `/readyz`, `/version`, `/openapi.json` → uniform JSON 404 `not found` — no meta endpoints leak build/version/schema information on the tenant API host, and unknown paths answer in the API's error envelope rather than a bare mux page (contrast the wrong-plural `/exports` 404 page note in sweep 824, which came from outside the `/v1` mux).
- `GET /graphql` → 405 coded (the endpoint is POST-only; no GraphiQL playground is exposed).
