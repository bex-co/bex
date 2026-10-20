# CLI sweep 675 — metrics endpoint dialects (live, 2026-10-19)

Read-only against the baseline `hello-go` service; nothing mutated (public GETs only, which woke it from idle spin-down: first hit 503, retry 200 — same activator contract as sweep 640).

- Dialect is per-metric paths, per `lego/backend/internal/metrics/rest.go`: `/metrics/cpu|memory|instance-count|http-requests|http-latency|bandwidth|…`. `/services/{id}/metrics`, `/metrics?type=…`, and `/metrics/bogus` → 404.
- `/metrics/cpu` without `resource` → 400 `resource is required`; synthetic resource → 404 `NOT_FOUND`; wrong window params (`start`/`end`) → 400 `unsupported query parameters "end", "start" for get-cpu` (correct names are `startTime`/`endTime`).
- Default window on the idle service returned `[]` for cpu/memory; with an explicit `startTime`/`endTime` window the cpu series returned samples, and `instance-count` returned a live point (`value:1`) after wake.

Validation and data paths behave as coded. No bug filed.
