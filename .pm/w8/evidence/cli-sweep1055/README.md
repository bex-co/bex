# CLI sweep 1055 — CORS posture of the API (live, 2026-10-20)

Read-only probes. No fixture created. No new bug filed.

- A preflight from the dashboard origin → 204 with `Access-Control-Allow-Origin` echoing that origin, credentials allowed, `Authorization, Content-Type, Idempotency-Key, X-Session-Token` headers allowed, max-age 7200.
- An actual request carrying an unlisted origin (`example.com`) gets no `Access-Control-Allow-Origin` back — cross-origin browser reads stay closed to non-allowlisted origins.
