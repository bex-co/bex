# CLI sweep 961 — logs window pagination + open-findings spot check (live, 2026-10-20)

Read-only probes on the baseline `hello-go` service. No fixture created. No new bug filed.

- `GET /logs?resource={hello-go}&limit=1` → 200 with `hasMore: true` and `nextStartTime`/`nextEndTime` bounding the next window; log entries carry id, message, and timestamp. The nested `/services/{id}/logs` path 404s — logs live at the top-level resource query, matching the surface used in earlier log sweeps.
- Open-findings spot check at the unchanged pin `3ede47d3c5a9`: w8/137 still 405, w8/139 still 400. Both fixes are on `origin/main` but not pinned; both findings stay open.
