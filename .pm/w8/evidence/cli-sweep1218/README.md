# CLI sweep 1218 — timestamp filters window the service list (live, 2026-10-20)

Read-only sweep. No new bug filed.

- `createdBefore` in the distant past and `createdAfter` in the distant future both → 200 `[]`: the window filters apply (out-of-window results excluded, not an error). A malformed date → 400. Timestamp filters parse strictly and compose with the owner filter.
