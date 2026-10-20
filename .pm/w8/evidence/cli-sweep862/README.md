# CLI sweep 862 — authentication discipline (live, 2026-10-20)

Read-only/refused probes. No new bug filed.

- No `Authorization` header, an empty bearer, and a garbage token all → uniform 401 `{"error":"unauthorized","id":"unauthorized"}` — no distinction between missing and invalid credentials, and no resource existence information leaks ahead of auth (an authenticated DELETE on a synthetic id is the only probe that reaches the coded 404).
