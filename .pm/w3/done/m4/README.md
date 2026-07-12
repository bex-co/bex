# w3 · m4 — Resource-metrics history: Prometheus-backed CPU/memory/instances

**Worker:** worker3 **Goal:** upgrade cpu/memory/instance_count from metrics-server's single-point snapshot to real Prometheus-backed time series, so the metrics page's Application Metrics charts (and the time-range picker) show history — Render metrics-page parity. **Status:** done (2026-07-06)

## Tasks (in order)

| id   | title                                                                                                     | est | depends_on |
| ---- | --------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | GitOps: scrape kubelet/cAdvisor in prometheus.yaml (container CPU/memory, pod labels, RBAC) — **DONE**    | 40m | —          |
| t002 | PrometheusResourceSource: cpu/memory/instance_count via query_range, percentage-of-limit mode — **DONE**  | 45m | t001       |
| t003 | Source selection: prefer Prometheus when BEX_PROM_URL is set, fall back to metrics-server — **DONE**      | 30m | t002       |
| t004 | Verify end-to-end on the cluster (stepped series over a real range) + docs update — **DONE**              | 30m | t002, t003 |
| t005 | Simplify — run `/simplify` over the code this milestone changed — **DONE**                                | 20m | t004       |
| t006 | Test coverage — meaningful tests for the Prometheus resource source + fallback selection — **DONE**       | 30m | t004       |

## Completion notes (2026-07-06)

- Code landed in `lego/backend` (task files predate the lego/ refactor and still say `operator/…` paths): `NewPrometheusResourceSource` + shared `promQueryRange` in `internal/api/metricsource.go`; `ResourceMetricsRangeSource` + `rangedResourceSeries` funnel in `internal/api/metrics.go`; wiring in `cmd/api/main.go`.
- cAdvisor is scraped **through the apiserver proxy** (`/api/v1/nodes/<node>/proxy/metrics/cadvisor`), not the kubelets directly — the CAPD mock's pod network can't reach every node's :10250, the apiserver always can, and TLS verifies properly (no `insecure_skip_verify`).
- App pods are matched by Deployment pod-name shape (`pod=~"<app>-[a-z0-9]+-[a-z0-9]{5}"`) since kubelet metrics carry no pod labels; kube-state-metrics (label joins) stays out of scope — recorded as a documented heuristic beside the Traefik service selector in docs/ADR010-observability.md.
- Verified on the mock cluster against a 2-replica `tier: starter` App: 1h-range cpu/memory/instance-count all returned per-instance stepped series (16–22 points, 60s step) over REST and GraphQL; `percentage=true` 0–100 against the 500m/512Mi limits; fallback (no `BEX_PROM_URL`) still single-point snapshot + pod-count + 503 for request metrics. Render's live metrics page (Memory/CPU/Total Instances under a "Last 12 hours" picker) matches the shipped contract.
- `cpu_limit`/`memory_limit` intentionally stay single-point (no fabricated limit history) — documented in docs/ADR010-observability.md.
- Mock-cluster-only helm overrides (control-plane pinning for prometheus-server; hostNetwork:10251 for metrics-server) are session-local, not in gitops — prod doesn't need them.

## Definition of done

With Prometheus configured (`BEX_PROM_URL`), `GET /v1/metrics/cpu|memory|instance-count` and GraphQL `metrics(...)` return resolution-stepped time series that honor `startTime`/`endTime`/`resolutionSeconds` (multiple points over a queried range, per-instance labels intact), and `percentage=true` still reports 0–100 against pod limits; with `BEX_PROM_URL` unset, behavior falls back to today's metrics-server snapshot (or 503 when that's absent too). Verified against the real cluster over a ≥1h range; `make test` green; `docs/ADR010-observability.md`'s "snapshot resolution" deviation replaced with the new behavior.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` 2026-07-06 ("more for w3") — the "snapshot resolution" gap recorded in `docs/ADR010-observability.md` during w3/m2; confirmed against Render's live metrics page (element inventory in `w3/m3/README.md`).
- **Goal linkage:** `GOAL.md` #2 ("basic obs for operation") and the standing goal of feature parity with render.com — Render's metrics page is history-stepped for all six charts; bex's Application Metrics half is not.
- **Expected outcome:** the w3/m3 metrics page renders real CPU/memory/instances history instead of a single dot per chart, and its time-range picker actually works for the Application Metrics card.
- **Why now:** w3/m3 is in flight and looks broken without this — half its page has no history. The supply chain is fresh: Prometheus is already deployed (`deploy/gitops/base/prometheus.yaml`, Traefik-only scrape) and the query plumbing (`NewPrometheusRequestSource`, `operator/internal/api/metricsource.go`) landed in w3/m2, so extending both is the cheapest it will ever be.
