# w5 · m140 — Metrics aggregation matches Render: CPU MAX/MIN, per-host requests, no silently ignored arguments

**Worker:** worker5 **Goal:** REST `aggregationMethod`, MCP `cpuUsageAggregationMethod` and GraphQL `aggregationMethod` return Render's MAX/MIN CPU series instead of a 400 or a silent no-op. `aggregateBy=host` / `aggregateHttpRequestCountsBy=host` / GraphQL `HOST` break request counts out by host. No metrics argument any surface accepts is dropped without effect. **Status:** in progress (t001–t005, t007–t009 done; t006 production read pending the release pipeline)

## Tasks (in order)

| id   | title                                                                                 | est | depends_on |
| ---- | ------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Pin Render's aggregation contract and decide what `aggregationMethod` means — **DONE** | 30m | —          |
| t002 | CPU MIN/MAX on the Prometheus resource source, per t001 — **DONE** | 75m | t001       |
| t003 | REST, MCP and GraphQL read `aggregationMethod`; nothing accepted is ignored — **DONE** | 45m | t002       |
| t004 | Request counts break down by host from the request-log store — **DONE** | 60m | t001       |
| t005 | Dashboard: the HTTP requests chart can group by host — **DONE** | 30m | t004       |
| t006 | dev-5 walk; ADR018 row 230, `metrics-page.md` and tool descriptions match the shipped | 30m | t003, t005 |
| t007 | Render parity — **DONE** | 30m | t006       |
| t008 | Simplify — **DONE** | 20m | t007       |
| t009 | Test coverage — **DONE** | 45m | t007, t008 |
| t010 | Closeout                                                                              | 10m | t009       |

Total: **10 tasks, about 6h 15m**.

## Definition of done

- On dev-5, a multi-instance service with bursty CPU returns a MAX series that differs from its AVG series, and a MIN series below it, through REST `GET /v1/metrics/cpu?aggregationMethod=MAX`, MCP `get_metrics(cpuUsageAggregationMethod: "MAX")` and GraphQL `metrics(query: {aggregationMethod: "MAX"})`. All three return the same values for the same window.
- `aggregationMethod` on a non-CPU metric gets the same named 400 on every surface. When only the metrics-server fallback is wired, MAX/MIN is still a named refusal (no fake answer).
- On dev-5, a service answering on two hosts returns one request-count series per host through REST `aggregateBy=host`, MCP `aggregateHttpRequestCountsBy: "host"` and GraphQL `aggregateBy: ["HOST"]`, each labelled `host`. With the request-log store unavailable, the host breakdown answers the host filter's 503 `LOG_STORE_UNAVAILABLE`, never the host-less Traefik counter.
- A test enumerates every argument each surface's metrics query accepts and fails if any is parsed without changing the query or producing a refusal.
- The dashboard's HTTP requests chart offers Group by: Host and shows the per-host series.
- ADR018 row 230 and `docs/render-artifacts/metrics-page.md` describe the shipped behavior; the MCP tool descriptions no longer say "metrics-server snapshot" or "no host group-by axis".

## Source + Goal linkage

- **Source:** second `/pm-brainstorm for w5` (2026-10-19), item 4, chosen by the user 2026-10-20.
  - **Render's pinned REST contract** (`lego/backend/internal/api/openapi/render-public-api-1.json`): `aggregationMethod`: "The aggregation method to apply to multiple time series", enum `AVG|MAX|MIN`. `aggregateBy` on request metrics: "The field to aggregate by", enum `statusCode|host`. Render's MCP `get_metrics` takes `cpuUsageAggregationMethod` and `aggregateHttpRequestCountsBy` (`render-mcp-tools.json`, argument names only).
  - **REST and MCP refuse on stale premises.** `cpuAggregation` (`lego/backend/internal/metrics/mcp.go:111-123`, called from `rest.go:402`) refuses MAX/MIN because "bex CPU is a metrics-server snapshot". Production CPU has been a Prometheus range query since the resource source was wired (`source.go:328`, lookback `source.go:223`); metrics-server is only the fallback. `requestGroupBy` (`mcp.go:129-145`) refuses `host` because "neither … Traefik … nor the Loki request-log path expose a host group-by axis". The Loki pipeline already extracts `request_host` (`lokisource.go:154-161`) and already groups by label (`lokisource.go:143`, `lokiGroupLabel` `:205-214`). Both refusals were decided in `w1/done/m155`.
  - **GraphQL drops arguments silently.** It declares `aggregationMethod` (`graphql.go:116`) but `metricsQueryInputFromArgs` (`graphql.go:381-454`) never reads it, and its `aggregateBy` loop maps only STATUS_CODE and METHOD, dropping HOST. This is the same class of bug w1/m155 fixed for REST.
  - **Owners:** nothing on the board owns these.
- **Goal linkage:** ADR018 Render parity (metrics row 230); ADR008's AI-native thesis — agents asking an MCP or API client for "peak CPU" or "requests per host" get Render's answer; ADR010 observability.
- **Expected outcome:** a Render user, script or agent gets peak/minimum CPU and per-host request counts from bex as from Render, and no surface accepts an argument it then ignores.
- **Why now:** the stale refusal reasons sit on code paths that already have the capability (Prometheus range queries, Loki's extracted host), so the gap is mostly wiring. The GraphQL silent drop misreports what bex supports today. Nothing blocks it: m139 touches `logs/`, not `metrics/`.
- **Render parity included:** this changes REST, MCP and GraphQL metrics queries and the dashboard metrics chart, so t007 re-checks every surface against Render.

## Progress

- **t001 decision (2026-10-20):** Render's MCP server forwards `cpuUsageAggregationMethod` unchanged to REST `aggregationMethod` (`render-mcp-server` `pkg/metrics/repo.go:156-157`, commit `c6b8bb1`) and documents it as aggregation "over time intervals… Defaults to AVG". So every surface implements interval aggregation: MAX/MIN of a 60s-lookback rate sampled every 15 s inside each step; AVG/omitted is byte-identical to the old query. Recorded in `docs/render-artifacts/metrics-page.md` § Aggregation.
- **Shipped behaviour:** REST/MCP/GraphQL share `parseCPUAggregation`/`checkCPUAggregation` (`internal/metrics/aggregation.go`); services and datastores honour MAX/MIN; the metrics-server fallback and non-cpu metrics give named 400s. `aggregateBy=host` / `aggregateHttpRequestCountsBy=host` / GraphQL `HOST` read Loki `sum by (request_host)` relabelled `host`, never the Traefik counter (503 `LOG_STORE_UNAVAILABLE` unwired). Dashboard Group by → Host.
- **Evidence so far (t006, local):** dev-N stacks have no Prometheus and the local cluster's Loki has no tenant request logs, so the walk ran on the real engines instead. `TestCPUAggregationQueryOnPrometheus` (promtool 3.14; CI pins 2.55.1) proves MAX reports a 1-core burst exactly and MIN < AVG < MAX; `TestLokiHostBreakdownOnLoki` (pinned Loki) returns `a.example.com`=3 and `b.example.com`=1. `TestNoMetricsArgumentIsAcceptedWithoutEffect` guards every GraphQL input field and every Render-declared `/metrics/cpu` and `/metrics/http-requests` parameter. Remaining: a read-only production read on `api.bex.co` after deploy (CPU MAX vs AVG vs MIN on a multi-instance service; host breakdown on a two-host service).
- **Simplify (t008):** inner rate kept at 60s under MAX/MIN (cost linear in the window, real peaks visible); helpers moved to `aggregation.go`; Loki host label a constant. Skipped: sharing the OpenAPI-resolver/Loki-push/promtool test scaffolding across packages (two callers each).
