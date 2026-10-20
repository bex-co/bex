# CLI sweep 828 — log label-values surface + w8/132 regression spot-check (live, 2026-10-20)

Read-only probes on the baseline (read only). No new bug filed.

- `GET /logs/values?label=type&resource={hello-go}` → 200 `["app","request"]`. A bare `?type` (missing the `label` parameter) → named 400 `invalid query parameter "label"`; an unknown parameter → 400 naming it as unsupported for `list-logs-values`.
- w8/132 stays fixed at pin `ca30db3a3646`: `label=task` → 400 `label must be one of instance|host|statusCode|method|level|type`, and the `/logs` `task=` filter → the named "bex runs no Render Workflows or sandboxes" refusal.
