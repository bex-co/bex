# CLI sweep 771 — metrics filter-discovery endpoints (live, 2026-10-19)

Read-only.

- `GET /metrics/kv-memory?resource=<synthetic>` → 404 `NOT_FOUND`.
- `GET /metrics/filters/application?resource=<baseline>` → 200 `[{"filter":"instance","values":[<instance id>]}]`.
- `GET /metrics/filters/http?resource=<baseline>` → 200 with `statusCode:["200"]` and an empty `host` value list — the discovery endpoints answer from live data.

No bug filed.
