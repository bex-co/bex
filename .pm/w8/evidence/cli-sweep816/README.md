# CLI sweep 816 — metrics surface + w4/370 still live (live, 2026-10-20)

Read-only probes on the baseline `hello-go` (read only). No new bug filed.

- `GET /metrics/cpu|memory?resource=srv-daif…` over the last hour → 200 with per-instance label sets and real sample values (the baseline is awake and serving).
- Unknown resource-prefix id → coded 404.
- Missing `resource` parameter → named 400 `resource is required`; explicit empty `resource=` → 500 `internal error` — the already-filed **w4/370** defect, re-confirmed live at pin `ca30db3a3646` and not refiled here.
