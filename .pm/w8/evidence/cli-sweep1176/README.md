# CLI sweep 1176 — URL forms are exact (live, 2026-10-20)

Read-only sweep. No new bug filed.

- `/services/` (trailing slash), `/Services`, `/SERVICES`, and `/services` without the `/v1` prefix all → 404. Routes match case-sensitively with no slash normalization or unversioned alias: clients must use the declared paths exactly as the pinned CLI does.
