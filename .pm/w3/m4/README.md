# w3 · m4 — Resource-metrics history: Prometheus-backed CPU/memory/instances

**Worker:** worker3 **Goal:** upgrade cpu/memory/instance_count from metrics-server's single-point snapshot to real Prometheus-backed time series, so the metrics page's Application Metrics charts (and the time-range picker) show history — Render metrics-page parity. **Status:** todo

## Tasks (in order)

| id   | title                                                                                          | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | GitOps: scrape kubelet/cAdvisor in prometheus.yaml (container CPU/memory, pod labels, RBAC)    | 40m | —          |
| t002 | PrometheusResourceSource: cpu/memory/instance_count via query_range, percentage-of-limit mode  | 45m | t001       |
| t003 | Source selection: prefer Prometheus when BEX_PROM_URL is set, fall back to metrics-server      | 30m | t002       |
| t004 | Verify end-to-end on the cluster (stepped series over a real range) + docs update              | 30m | t002, t003 |
| t005 | Simplify — run `/simplify` over the code this milestone changed                                | 20m | t004       |
| t006 | Test coverage — meaningful tests for the Prometheus resource source + fallback selection       | 30m | t004       |

## Definition of done

With Prometheus configured (`BEX_PROM_URL`), `GET /v1/metrics/cpu|memory|instance-count` and GraphQL `metrics(...)` return resolution-stepped time series that honor `startTime`/`endTime`/`resolutionSeconds` (multiple points over a queried range, per-instance labels intact), and `percentage=true` still reports 0–100 against pod limits; with `BEX_PROM_URL` unset, behavior falls back to today's metrics-server snapshot (or 503 when that's absent too). Verified against the real cluster over a ≥1h range; `make test` green; `docs/observability.md`'s "snapshot resolution" deviation replaced with the new behavior.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` 2026-07-06 ("more for w3") — the "snapshot resolution" gap recorded in `docs/observability.md` during w3/m2; confirmed against Render's live metrics page (element inventory in `w3/m3/README.md`).
- **Goal linkage:** `GOAL.md` #2 ("basic obs for operation") and the standing goal of feature parity with render.com — Render's metrics page is history-stepped for all six charts; bex's Application Metrics half is not.
- **Expected outcome:** the w3/m3 metrics page renders real CPU/memory/instances history instead of a single dot per chart, and its time-range picker actually works for the Application Metrics card.
- **Why now:** w3/m3 is in flight and looks broken without this — half its page has no history. The supply chain is fresh: Prometheus is already deployed (`deploy/gitops/base/prometheus.yaml`, Traefik-only scrape) and the query plumbing (`NewPrometheusRequestSource`, `operator/internal/api/metricsource.go`) landed in w3/m2, so extending both is the cheapest it will ever be.
