# CLI sweep 960 — auth and path discipline re-check (live, 2026-10-20)

Read-only probes. No fixture created. No new bug filed.

- No `Authorization` header → 401; a bogus bearer token → 401 (same status, no oracle difference in the status line).
- Valid token without the workspace header → 200 (session default workspace applies, as established in earlier sweeps); valid token with the user's other workspace header → 200 scoped to that workspace.
- Path discipline is exact: `/services/` (trailing slash) and `/Services` (case) both 404 rather than silently normalizing to the list route.
