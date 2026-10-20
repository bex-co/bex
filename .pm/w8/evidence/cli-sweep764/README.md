# CLI sweep 764 — unauthenticated request shapes (live, 2026-10-19)

Read-only `GET /v1/users` variants; no credentials were created or used beyond the probes shown.

| Variant | Result |
| --- | --- |
| no `Authorization` header | 401 `{"error":"unauthorized","id":"unauthorized","message":"unauthorized"}` |
| `Bearer rnd-bogus764` | 401, same body |
| `Basic …` scheme | 401, same body |

The refusal is uniform — no distinction leaks between missing, malformed, and wrong-scheme credentials.

No bug filed.
