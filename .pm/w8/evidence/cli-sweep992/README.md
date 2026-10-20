# CLI sweep 992 — metrics kind vocabulary (live, 2026-10-20)

Read-only probes on the baseline `hello-go` service. No fixture created. No new bug filed.

- The kind segment is the pinned vocabulary (`http-requests`, `http-latency`, `cpu`, `memory`, `bandwidth`, … plus bex's `cpu-target`/`memory-target` extensions, per `metrics/rest.go`). A guessed `/metrics/http` and a bogus kind both 404 — unregistered segments, not data errors.
- `/metrics/http-requests` and `/metrics/memory` on the baseline → 200 with labeled time series; `/metrics/cpu` → 200. Missing `resource` → 404 not-found rather than a fabricated workspace-wide series.
