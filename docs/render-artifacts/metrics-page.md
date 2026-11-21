# Service Metrics page — Render capture vs bex (w5/m42)

Authenticated side-by-side capture, 2026-07-17: Render `dashboard.render.com/web/srv-d2rnr3jipnbc73deuvgg/metrics` (`backend-v2`, a live Web Service) against bex `dashboard.bex.co/services/srv-d9dd16roviqs738quds0/metrics`. The gaps below drove `w5/m42` (metrics-page simplification); the "after" column reflects the shipped page, verified live in a browser twice — against the local-bex stub (structure + dropdown contents) and against dev-5 with a real bex-api/operator/cluster (fresh image-backed service `m42-metrics-web`: Render-shaped title, 12 h default range, hidden-then-toggled timeline showing real deploy events, Limit/Manage-scaling links, per-section Percentile p90, honest source-unavailable states without Prometheus) — plus the 1,363-test dashboard suite.

## Page structure (Render, captured live)

| Element | Render |
| --- | --- |
| Document title | `backend-v2 ・ Web Service ・ Render Dashboard` — name + type + brand, identical on every tab of the service, never the raw `srv-` id |
| Page-level toolbar | `[filter-events icon] [Last 12 hours ▾] … [show-event-timeline icon]` — time is the only page-level dimension |
| Time-range dropdown | Last 30 min / hour / 4 h / **12 h (default)** / 24 h / 2 days / 7 days / 14 days / 30 days (disabled, plan-gated) / Custom |
| Event timeline | Hidden until the toolbar toggle reveals it |
| Application Metrics | One card; `Percentage \| Total` tabs in the card header; sections Memory (`Limit 512 MB` → `/plan`), CPU (`Limit 0.5 CPU` → `/plan`), Total Instances (`Manage scaling` → `/scaling`) |
| Network Metrics | Card header: subtitle "Aggregated across all instances" + filters `Status Code \| Host \| Path` |
| — Total Requests | Aggregate count beside the heading ("7,266 requests") + `Group by` dropdown on the section |
| — Response Times | `Percentile` dropdown on the section: All / p50 / p75 / **p90 (default)** / p99 |
| — Outbound Bandwidth | Hourly-resolution note + "Usage this month 11.69 GB" |
| Footer | "Stream your metrics to another observability tool" promo |

## bex before → after (w5/m42)

| Surface | Before | After |
| --- | --- | --- |
| Document title | `srv-… · Metrics · bex dashboard` on first paint; name-only swap after load; per-tab segment | `<name> · <type label> · bex dashboard` on every service tab once resolved; `srv-` id only as the SSR/first-paint fallback |
| Toolbar | Six inline range buttons (default 1 h) + Percentage/Total tabs + quantile combobox (p95) + Status Code — all page-level | `[event filter] [range ▾] … [timeline toggle]`; Render's eight presets, default **Last 12 hours** |
| Event timeline | Always-open card with its own filter combobox | Hidden by default; toolbar toggle reveals it; filter lives in the toolbar |
| Application card | Subtitle; no plan/scaling links; tabs on the page | Tabs in the card header; `Limit <value>` → Instance Type tab on Memory/CPU; `Manage scaling` → Scaling tab |
| Network card | Status Code + quantile page-level; "Response Times (0.95)" | Status Code in the card header; `Percentile` (All / p50/p75/p90/p99, default p90) on Response Times, "All" overlaying p50/p90/p99 (w5/m56); aggregate request count |
| Logs tab (shared) | Same six preset buttons | Same shared dropdown component; Logs keeps its own 1 h default (bex-api's default span) |

## Accepted drift (recorded, not built)

| Render capability | Why bex diverges |
| --- | --- |
| Observability-integrations banner | External metric drains are an explicit non-goal (`.pm/DO_NOT_DO.md`; same class as log/metric drains) |
| Group-by option set | bex groups by status/method (Traefik labels); Render's set differs — bex's is the honest label set its meter actually has |
| Bandwidth "Usage this month" value | bex composes real HTTP + NAT (direct-public L3) + WebSocket egress (w8/m15); only `privateLink` reports 0 (no such product surface) — an honest subset, never a fabricated total |
| Render logs `?r=` grammar | `15m`/`6h` alias to the nearest preset (`30m`/`4h`); `1h`/`24h`/`7d` now parse natively; retired bex ids (`3h`/`6h`/`1d`) degrade to the default range |

## Closed by w5/m56 (2026-07-27)

The three recorded drifts on the percentile + range controls were closed after a fresh live Render walk (`cuckoo-backend` metrics page, authenticated): Render's Percentile control does offer "All" over p50/p90/p99, its range dropdown offers "Last 30 days" (disabled, plan-gated) and a "Custom" absolute start/end picker with a plan-window note.

| Was drift | Now |
| --- | --- |
| Percentile "All" overlay | ✅ The metrics read returns several quantiles in one call — REST repeats `?quantile=`, GraphQL sends several `parameters[].quantile`, MCP takes `quantiles[]`; each series is tagged with its `quantile` label (GraphQL also echoes `parameters { quantile }`). The card's "All" option overlays p50/p90/p99 with a p50/p90/p99 legend. Single-quantile reads are byte-identical. |
| "Last 30 days" range | ✅ Added as a relative preset on the shared range dropdown, **ungated** (Render plan-gates it). 30 days = `BEX_MAX_QUERY_HOURS`' default, the effective ceiling. |
| "Custom" range | ✅ A "Custom…" dropdown option opens an absolute start/end picker (Metrics + Logs, via the shared control), bounded client-side by `MAX_CUSTOM_RANGE_HOURS` (30 days) and honestly by the backend's over-window 400 beyond it. Custom windows are URL-backed on the Logs tab. |

## Which Traefik series back which service type (w4/m113, 2026-09-18)

The request charts read one of two counter families, chosen by service type. The split is not a preference — it is which series exist.

| Service type | Requests / latency selector | Why |
| --- | --- | --- |
| Compute (`web_service`, `private_service`, `background_worker`) | `traefik_service_requests_total` / `traefik_service_request_duration_seconds_bucket`, selected by `service="<ns>-<app>-<port>@kubernetes"` | The App owns a Kubernetes Service, so the per-service counters are the tightest possible attribution. |
| `static_site` | `traefik_router_requests_total` / `traefik_router_request_duration_seconds_bucket`, selected by `router=~"^(<the App's Ingress routers>)$"` | ADR029 gives a static site no Deployment and no Service of its own — every static host's Ingress points at the shared static server, so the per-service selector names an object that does not exist and matches nothing. The per-router counters carry the App's own Ingress router names, which is also how bandwidth has always been attributed. |

Both router series carry `code`, `method` and `le` exactly as the service series do (verified against the production Prometheus `/api/v1/series`, 2026-09-18), so the status-code matcher, `groupBy`, the `histogram_quantile` shape and the m108 per-bucket `sum(increase(…))` count semantics are identical across the two families. Status-code **discovery** (`metricsFilters` / `/v1/metrics/filters/http`) reads the same selector the chart reads, so the dropdown can never offer a value the graph cannot plot. `INSTANCE` is honestly empty for a static site — it has no pods.

A static site whose Ingress routers cannot be resolved produces **no query** rather than a router-less match: an empty router matcher would select every router in the cluster and serve one tenant another tenant's request counts.

Before this split, a static site read `[]` on requests, latency and status-code discovery while bandwidth for the same traffic was correct — the shape that made it look like a data-freshness problem rather than a dead selector.

## Closed by w5/m58 (2026-07-30)

The last recorded Network-card drift — Host / Path filters — is closed. The t001 design probe refuted the earlier hypothesis that Host could ride Prometheus router labels: `traefik_service_requests_total` / `traefik_service_request_duration_seconds_bucket` carry `service`/`code`/`method`/`le` only, and `addRoutersLabels` adds a router **name**, not the matched `Host()`/`Path()`. So **both** filters are served from the request-log store (Loki), the one backend with a per-request host/path axis (Traefik's access log carries `RequestHost`/`RequestPath` per line, plus `Duration` ns for latency).

| Was drift | Now |
| --- | --- |
| Host network filter | ✅ Card-header **Host dropdown**, discovered via the logs `logLabelValues(label:"host")` read (host resolves from the App's own URLs, so the dropdown populates even with no store). A host-filtered `http_requests`/`http_latency` read is served from Loki (`sum(count_over_time(... \| json \| request_host=… [step]))` / `quantile_over_time(… \| unwrap latency_ns …)/1e9`), so the requests + response-time series change to the filtered subset. |
| Path network filter | ✅ Card-header **free-text Path input** (committed on Enter/blur, clearable). `path` is a high-cardinality line field, not a discoverable Loki label — so, exactly like the Logs tab, it is a text filter, not a fabricated dropdown; its value becomes the Loki `request_path` line filter. |
| Store-gated honest state | ✅ Host/Path apply only to `http_requests`/`http_latency` (bandwidth + host/path → named 400). With no `BEX_LOKI_URL`, a host/path-filtered read returns `ErrLogStoreUnavailable` (503) and the two sections render an explicit "Host and Path filters need the log store" state — **never** a silently-unfiltered chart (the Logs-tab 503 pattern). |

### Cross-surface parity verdicts (t007)

Filter fields/semantics are consistent across every surface, one spelling, one error dialect — asserted by `TestHostPathFilterCrossSurfaceParity` (all three route the same `host`/`path` to the same Loki source):

| Surface | Spelling | Verdict |
| --- | --- | --- |
| REST | `GET /v1/metrics/{http-requests,http-latency}?host=&path=` | ✅ match — the parameter names Render's own metrics API uses; store-unavailable → 503, bandwidth+host/path → 400 |
| GraphQL | `metrics(query:{filters:[{field:"HOST"…},{field:"PATH"…}]})` | ✅ match — same generic filters array as RESOURCE/STATUS_CODE (no schema change); store-unavailable → GraphQL error |
| MCP | `get_metrics(host, path)` | ✅ match — new tool args, same core; store-unavailable → tool error |
| UI (Network card) | Host dropdown + Path text input, card-header placement | ✅ match — Render's captured card-level Host/Path placement; Host discovered, Path free-text, both clearable |

No new divergence filed. Discovery-side note: the Prometheus `metricsFilters` verb still reports empty HOST/PATH values (Prometheus has no host/path axis) — correct, not a dead control, because the UI discovers Host from the logs label-values read instead.

### Live-proof status (t006)

The deferred browser walk was performed on production on 2026-09-06 (`w5/done/028.md`). Host discovery listed both actual App hosts; selecting one sent HOST on both network queries, adding `/robots.txt` sent HOST+PATH and rendered filtered series, and clearing the path emptied the input. Evidence: `.playwright-mcp/w5-metrics-result.json` and `w5-metrics-{host,path}.png`.

The host-only latency read exposed a real defect: the old `quantile_over_time` query retained per-path/stream labels and exceeded Loki's 500-series limit. The fix groups raw samples inside the percentile by only the requested axis (`by ()` without grouping), so it computes a service-wide percentile rather than independent per-path percentiles. Against the identical production Loki 12-hour window, the original query failed, the corrected query returned one series, and status grouping returned five. Host/path and status/code/method regression queries plus `go test -race ./internal/metrics` pass.

This separates the evidence accurately: interactions/path rendering were observed through the deployed dashboard; the corrected host-only query was verified directly against live Loki and in local tests. No post-rollout browser capture is claimed here. The optional store-unavailable scenario remains covered by automated tests; production Loki was not disabled for QA.

## Corrected by w5/m90 (2026-09-08)

m89's replica aggregation exposed that the Application card's Percentage tab divided every point by the latest value of one aggregate limit — mixed-limit replicas (0.4 of 0.5 vs 0.5 of 1 CPU) misread as 40%/50% instead of 80%/50%, and history inherited whatever limit was current. **Not** a Render divergence (Render's own dashboard divides client-side by construction; bex deliberately departs here and documents the extension in ADR010): the card now renders bex-api server-side percentages (each replica normalized by its own trustworthy kube-state-metrics limit history before MIN/MAX/AVG), with mixed limits reading as "Limits vary" in the header and unavailable percentages as explicit copy distinct from no-data. Cross-surface verdicts asserted by `TestPercentageCrossSurfaceParity` (REST/GraphQL/MCP agree on 65% AVG for the 80%/50% pair).

## Corrected by w5/m91 (2026-09-09)

m89 checked no-silent-broadening as complete, but its prune effect removed unavailable selections (an empty selection omits the INSTANCE filter, requesting all instances) and `applyInstanceSelection` inferred filter eligibility from returned instance labels, so an empty supported CPU/memory query became `ErrBadRequest`. **Not** a Render divergence (Render has no documented empty-window INSTANCE contract to mirror; bex documents the correction in ADR010): eligibility is now metric-typed (`cpu`/`memory`/`cpu_limit`/`memory_limit`, including the m89 limit consumers) — an authorized empty window succeeds with no series on REST/GraphQL/MCP/dashboard, unsupported combinations still 400 even when empty, and unknown/foreign selectors never broaden. The dashboard retains the explicit selection across polling, discovery errors, and window changes (explicit Show-all or resource-navigation reset only), marking retained-but-unavailable choices with localized copy distinct from errors. Cross-surface verdicts asserted by `TestInstanceFilterEmptyCrossSurfaceParity` (empty filtered CPU 200 on all three; INSTANCE on `http_requests` 400/errors on all three).

## Cross-surface note

w5/m42 changed only `dashboard/`; **w5/m56 extended the metrics _read_ itself** — REST (`GET /v1/metrics/*` repeated `quantile`), GraphQL (`metrics` `parameters[]`), and MCP (`get_metrics` `quantiles[]`) now all serve multiple quantiles in one call through one `MetricsWithQuantiles` core, so the three API surfaces stay in lock-step. The p90 default and 12 h window remain client-side choices (bex-api's own defaults — quantile 0.95, 1 h span — still apply to direct API callers, matching Render's API/UI split: Render's UI defaults also differ from its API defaults). The percentile "All" and the "Last 30 days"/"Custom" ranges are ungated (Render plan-gates the latter two).

## Corrected by w4/m108 (2026-09-16) — rate vs per-bucket count

Live QA on production (`srv-dal3f2rkmutc73d7q9l0`, `srv-d9bj8s3eg85c7390eb9g`) showed Total Requests under-reporting by roughly the resolution step: 12 curls in one minute produced GraphQL value `0.177…` (= 8/45, a **rate**), and a busy service reading ≈7 on every bucket was summed by the UI into "727 requests" for a 12 h window that actually served ~300k. Both Prometheus (`sum(rate(traefik_service_requests_total[Ns]))`) and Loki (`sum(rate(<selector>[Ns]))`) builders returned req/s while `unit: "count"` and MCP `http_request_count` claimed a count.

| Metric | Before | After (w4/m108) |
| --- | --- | --- |
| `http_requests` (Prometheus) | `sum(rate(...[step]))` → req/s | `sum(increase(...[step]))` → requests in the bucket |
| `http_requests` (Loki, host/path) | `sum(rate(...[step]))` → lines/s | `sum(count_over_time(...[step]))` → lines in the bucket |
| `bandwidth` chart | `egressquery.SumRates` → B/s | `egressquery.SumIncreases` → bytes in the bucket |
| `bandwidth` month-to-date / usage metering | already `Increase` | **unchanged** (billing path untouched) |
| `http_latency` | `histogram_quantile` / `quantile_over_time` | **unchanged** |

**Bandwidth decision (evidenced).** The chart used `rate` while the "N used this month" footer and `usage/service.go` metering used `increase`. A 30-minute capture on `srv-d9bj8s3eg85c7390eb9g` returned ~1e6-scale points with `unit: "bytes"`; if those were B/s, sustained ≈1 MB/s implies ~1.3 TB over ~16 days of the month, against a footer of 164 GiB (~8.5× higher than the month's average rate — consistent with either a peak burst **or** a mislabelled rate). Mechanism check: for a counter sampled at step `S`, `rate(...[S]) * S ≈ increase(...[S])`. Converting the chart to `SumIncreases` over the same step makes Σ(chart points over a day) reconcile with that day's metered bytes and with the month-to-date `Increase` figure; the wire `unit` stays `"bytes"` (per-bucket bytes, not B/s). Render.com's Outbound Bandwidth chart is also a throughput-shaped series with a separate month-to-date total — bex deliberately aligns chart and footer on the **count** primitive so a reader can sum the chart without multiplying by step.

Partial leading buckets: Prometheus `increase()` may return a fractional count when the window starts mid-scrape; bex accepts that (dashboard `Math.round` on the aggregate) rather than clamping — documented on `sumIncrease` in `lego/backend/internal/metrics/source.go`.

## Corrected by w4/m119 (2026-09-21) — an exact count cannot come from a counter

w4/m108's last paragraph above conceded that "Prometheus `increase()` may return a fractional count when the window starts mid-scrape; bex accepts that". Live QA on 2026-09-20 (`srv-dansb83s0ils73bgpaj0`, since deleted) showed that concession was far too generous. 25 `curl` GETs, confirmed as exactly 25 by `GET /v1/logs?type=request` (`nlogs: 25, hasMore: False`), read back:

- `27.795666666666666` over one window alignment (`aggregateBy=statusCode`), and
- `6.666666666666666` over another, shifted by ~35 s.

Neither is an integer; they differ by a factor of four; neither is within ±1 of the truth. This is not a partial-leading-bucket rounding artifact — it is `increase()` doing what it is documented to do. `increase()` scales the first-to-last counter delta by `window / sampled-duration`, so a **burst** occupying part of a bucket is extrapolated by however the scrapes happened to land around it. Steady traffic hides this completely, which is why m108's own acceptance passed; bursty traffic is the free-tier norm.

**There is no extrapolation-free counter-increase in PromQL.** `increase()`, `rate()` and `delta()` all extrapolate by design, and reconstructing a bucket from `max_over_time − min_over_time` loses counter resets and cross-boundary increments. So an exact count has to come from a source that counts _events_, not a counter — the access log.

| Metric | w4/m108 | After (w4/m119) |
| --- | --- | --- |
| `http_requests`, unfiltered | Prometheus `sum(increase(traefik_service_requests_total[step]))` | request-log store `count_over_time(...[step])` — exact, and the same lines the Logs tab shows |
| `http_requests`, host/path-filtered | already `count_over_time` | **unchanged** |
| `http_requests`, no log store wired, or a failed log read | — | falls back to the Prometheus counter (see below) |
| `http_latency` | `histogram_quantile` / `quantile_over_time` | **unchanged** — percentiles come from Traefik's histogram, where `rate()` is the correct primitive |
| `bandwidth` chart | `egressquery.SumIncreases` | **unchanged** — see the decision below |
| `bandwidth` month-to-date / usage metering | `Increase` over the billing window | **unchanged** |

The count and the request log now reconcile by construction: they are the same records. A static site is served from the access log too — w4/m113's router-scoped Prometheus selectors were a workaround for a static site having no per-App Kubernetes Service, and the access log has no such problem since its lines carry the App.

**The fallback is deliberate, and it is not the host/path branch's rule.** A host/path-filtered read _errors_ when the log store is unwired, because Prometheus carries no host/path axis and cannot answer at all. An unfiltered count can be answered, approximately, by the counter — so when the log store is unwired (`BEX_LOKI_URL` unset) or a log read fails, bex serves the counter and logs it rather than blanking a working chart. The approximation is exactly the pre-w4/m119 behavior, so the fallback is never worse than what it replaces, while a 503 would be strictly worse. A wired-but-silent pipeline still reads zero and is indistinguishable from a genuinely idle service at this vantage point — the same limitation the host/path read already dispositions, caught out of band by the scheduled request-logs-liveness probe.

**Bandwidth decision (evidenced, deliberate divergence — w4/m119 t002).** The Outbound Bandwidth chart keeps `SumIncreases` and therefore keeps the same alignment sensitivity, for three reasons that are established in code rather than by measurement:

1. **Money is not affected.** Billing metering calls `egressquery.Increase` directly over the **billing window**, not per chart step (`usage/service.go:1229`), and `SumIncreases` carries an explicit "do not route money through this helper" fence. `increase()`'s extrapolation error is bounded by roughly one scrape interval at each end of the range: across a month-long window that is under ~0.01%, while across a 60 s bucket with a 15 s scrape it is the error this milestone is about. Same function, opposite significance — the ratio of range to sample spacing is the whole story.
2. **There is no exact substitute.** Bandwidth is a **composed** figure: Traefik router response bytes **plus** the WebSocket egress meter **plus** the node-level direct-egress meter (`egressquery.App`), the last of which carries its own counter-loss guard that rejects a window outright rather than misreading a restored counter. The access log carries only the HTTP half — rebuilding bandwidth from it would silently under-count every WebSocket and non-HTTP byte, trading a bounded alignment error for an unbounded omission.
3. **The unit cannot be misread.** `requestUnit` returns `bytes` with the per-bucket meaning pinned in a comment (`"per-bucket bytes after w4/m108 (Increase), not B/s"`), the month-to-date footer reads the same `Increase` primitive, and the table above states it — so a reader cannot mistake the axis for a rate, which is the failure mode m108 actually fixed.

What this leaves open, honestly: a short, bursty transfer can still read high or low on the bandwidth chart by query alignment, the same way requests did. It is a chart-accuracy issue confined to sub-scrape-interval bursts, it does not reach an invoice, and closing it would require a per-App byte-counting event stream that does not exist today. Re-open with a measurement if a user reports a bandwidth chart they cannot reconcile with their invoice.

**Minimum bandwidth step (w4/241, 2026-10-12).** A bucket must hold at least two 15 s scrapes for `increase()` to answer at all; below that the `or vector(0)` fallback reported recorded traffic as a flat zero (a five-minute custom range resolved to a 3 s step). The shared metrics service therefore refuses a bandwidth `resolution` below 60 s, or a window shorter than 60 s, with a named 400 on GraphQL and MCP (REST has no bandwidth `resolutionSeconds` and keeps its 60 s default). The dashboard requests bandwidth at `max(60, window step)` while the other charts keep the finer step. Widening only the lookback was rejected: a 60 s increase evaluated every 3 s counts overlapping bytes. This is a bex policy, not a Render-observed limit.

## Aggregation (w5/m140, 2026-10-20)

Render's own sources, fetched 2026-10-20:

- REST (`lego/backend/internal/api/openapi/render-public-api-1.json`): `/metrics/cpu` declares `aggregationMethod` — "The aggregation method to apply to multiple time series", enum `AVG|MAX|MIN`. `/metrics/http-requests` declares `aggregateBy` — "The field to aggregate by", enum `statusCode|host`. No other metrics path declares either.
- MCP (`render-oss/render-mcp-server` `pkg/metrics/tools.go`, commit `c6b8bb1`): `cpuUsageAggregationMethod` — "Method for aggregating metric values over time intervals. Only supported for CPU usage metrics. Options: AVG, MAX, MIN. Defaults to AVG." `aggregateHttpRequestCountsBy` — "Field to aggregate HTTP request metrics by. Only supported for http_request_count metric. Options: host (aggregate by request host), statusCode (aggregate by HTTP status code). When not specified, returns total request counts." `pkg/metrics/repo.go` passes both straight to REST `aggregationMethod`/`aggregateBy`, and only on the cpu and request-count reads.

**Decision.** The two Render wordings describe one parameter (the MCP server forwards it unchanged), so bex implements the MCP server's documented meaning on every surface: **interval aggregation**. MAX/MIN report, per instance, the highest/lowest CPU rate sampled inside each resolution step; AVG (the default, also when omitted) is the plain rate, which already averages its lookback window, so it is byte-identical to the pre-m140 query. It composes with bex's `aggregateAllMethod` (cross-instance, applied after): MAX then `aggregateAllMethod=MAX` is the peak of the hottest instance. MCP applies it to the cpu metric type only and reads other types in the same call unaggregated, as Render's server does; REST and GraphQL refuse it on any other metric. The host breakdown labels each series `host`, and when the request-log store is unavailable it answers the host filter's 503 `LOG_STORE_UNAVAILABLE` rather than the host-less Traefik counter.

## Database activity and disk graphs (w7/m430, 2026-11-16)

Render's [Service metrics](https://render.com/docs/service-metrics) page (read 2026-11-16) charts these five graphs, which bex lacked. Each is a bex datastore metric on the shared `DatastoreMetrics` verb (`lego/backend/internal/metrics/datastore.go`), so REST, GraphQL `datastoreMetrics` and MCP `get_datastore_metrics` answer the same series. Render publishes no REST path for any of them (the pinned OpenAPI's `/metrics/*` has none), so the REST paths and MCP ids are bex extensions named after the graphs.

| Graph | Render's sentence | Unit, series | Shown for | bex id · REST · GraphQL | bex source |
| --- | --- | --- | --- | --- | --- |
| Disk Activity | "The amount of data your service has read from and written to disk" | bytes per bucket, `direction` read and write, per instance | Postgres, Key Value, a service with a disk | `disk_activity` · `/v1/metrics/disk-activity` · `DISK_ACTIVITY` | cAdvisor `container_fs_{reads,writes}_bytes_total` of the resource's pods on every device but the node's boot disk |
| Disk Operations | "The number of read and write operations your service has performed on its disk" | operations per bucket, read and write | Postgres, a service with a disk | `disk_operations` · `/v1/metrics/disk-operations` · `DISK_OPERATIONS` | cAdvisor `container_fs_{reads,writes}_total`, same selection |
| Network Activity | "The amount of data your database has read from and written to the network" | bytes per bucket, read (received) and write (sent) | Postgres | `network_activity` · `/v1/metrics/network-activity` · `NETWORK_ACTIVITY` | cAdvisor `container_network_{receive,transmit}_bytes_total` on the instance pod's `eth0` |
| Transaction Volume | "The number of transactions executed by your database" | transactions per bucket | Postgres | `transaction_volume` · `/v1/metrics/transaction-volume` · `TRANSACTION_VOLUME` | CNPG exporter `cnpg_pg_stat_database_xact_commit` + `_rollback`, the tenant's databases only |
| Lock-Delayed Queries | "The number of recently completed database queries that were delayed by another operation holding a lock for one second or longer. Queries appear on this graph after they've completed." | queries per bucket | Postgres | `lock_delayed_queries` · `/v1/metrics/lock-delayed-queries` · `LOCK_DELAYED_QUERIES` | `log_lock_waits` "acquired … after N ms" lines with N ≥ 1000, counted in the log store |

A kind that does not show a graph on Render is refused by name (a 400 on every surface), as `db_connections` on a Key Value always was. Every value is a per-bucket sum, so a series adds up to the window's total, as `http_requests` has since w4/m108.

### What the live pods carry (production, 2026-11-17T01:55Z)

Read on the production app cluster before anything was changed:

- **Disk.** cAdvisor already stores `container_fs_{reads,writes}{_bytes,}_total` for every container, one series per block device the container did I/O on (1,388 series cluster-wide). Every node boots from `/dev/sda`. A datastore's volume is a further `sd` device: `dpg-d9nqg95cavls73fp8m20-1`'s `postgres` container wrote 25.6 GB to `/dev/sdb` and 45 KB to `/dev/sda`, and its `plugin-barman-cloud` (WAL archiving) sidecar 272 MB to the same `/dev/sdb`; `red-d9p49kdrtmes73c34ovg-0`'s `valkey` wrote 14 GB to `/dev/sdc`. A service disk is LUKS-encrypted (`hcloud-volumes-luks`), and its I/O is charged to both the `dm` mapping and the `sd` device under it, with equal values (one production service disk: 257,103,949,824 bytes written on `/dev/dm-0` and the same on `/dev/sdb`). So the disk graphs take the resource's pods' containers, drop `/dev/sda`, take each container's largest device (one copy of a LUKS volume's bytes), and add the containers up. No new series is scraped for them.
- **Network.** A pod's network counters exist only on its pod-level row (`container=""`, `interface="eth0"`), which the cAdvisor job dropped with every other pod-level row. The scrape now keeps the two byte counters of that row for Postgres instance pods (`dpg-<id>-<n>`) only. The traffic is the pod's: client connections, replication between instances, WAL archiving to object storage, and the exporter's scrapes.
- **Transactions.** The CNPG exporter serves `cnpg_pg_stat_database_xact_commit` and `_rollback` per `datname`: the tenant's database (16.5M commits on `dpg_d9nqg95cavls73fp8m20`), `postgres` (1.3M, the instance manager's and exporter's own queries), `template0`, `template1` and the shared row `""`. The scrape keeps the pair for tenant instance pods and drops those four `datname`s, so the graph is the tenant's own work.
- **Lock waits.** `log_lock_waits` was `off` and `deadlock_timeout` `1000` (default) on PostgreSQL 18.4. The operator now sets `log_lock_waits=on` for every tenant database. On the local CAPD cluster (CNPG 1.30.1, the production version) adding it reloaded the server without a restart (same postmaster start time and pod uid, restart count 0), and a three-second row-lock wait logged `process 228 still waiting for ShareLock on transaction 761 after 1000.191 ms` at one second and `process 228 acquired ShareLock on transaction 761 after 2500.040 ms` when it ended, while a 0.4-second wait logged nothing. Only the "acquired" line is counted, so a wait counts once, after it ends, and never while it lasts.

### Not captured from Render, and bex's choice

No Render database was available to compare against (a Render account is `w10/blocked/002`). Each open point and the choice bex made:

| Point | bex |
| --- | --- |
| The graphs' resolution | The request's step, as every bex chart (default 60 s; the datastore panel reads the last hour). A step finer than four scrapes spreads the counter's rate over the bucket: four cAdvisor scrapes (60 s) for network and disk, four CNPG scrapes (240 s) for transactions. |
| Transaction Volume: commits only, or commits and rollbacks | Both: a rolled-back transaction was executed. |
| Network Activity: client traffic only, or all of the database's | All of the instance pod's traffic, including replication and WAL archiving. |
| Disk Activity: page-cache hits | Not counted: the counters are block I/O. |
| Lock-Delayed Queries: a wait that ends in a deadlock or a cancel | Not counted: PostgreSQL logs "acquired" only when the lock is granted. A wait counts in the bucket where it ended. A `deadlock_timeout` raised above one second by a legacy declared parameter misses the waits between one second and that timeout; one lowered below it is held to one second by the line's own duration. |
| Disk Activity on a Free Key Value | Shown: bex's Free Key Value has a disk (ADR018 Key Value row); Render's Free tier has none and hides it. |
| A cluster whose volumes share the boot disk | The local CAPD mock's `local-path` volumes do; their disk graphs are empty there. |

### Series growth

The new series are the network pair per Postgres instance pod and the transaction pair per tenant database per instance pod. Production head series before the rollout: 121,049 (2026-11-17T01:55Z), with 4 tenant Postgres instance pods holding one tenant database each, so about 16 new series. The measured growth after the rollout is recorded in w7/m430.

## Status-code label (w3/080, 2026-11-20)

A per-status breakdown of Total Requests named its code differently on each bex surface: REST and MCP `statusCode`, GraphQL `projectMetrics` `statusCode`, and GraphQL `metrics` the sources' own `code`. Render's GraphQL label had not been captured.

**Read from Render's public dashboard bundle, without sign-in.** `https://dashboard.render.com/` loads `assets/index-*.js`; following its `./*.js` imports fetches about 450 chunks (the method of [project-observability.md § Operators per field](project-observability.md#operators-per-field-read-from-the-bundle-2026-11-19-w3m523)). The hashes change with every Render deploy, so find the chunks by name.

- A GraphQL `MetricSeries` carries `labels: [MetricResultLabel {field, value}]`, and `field` is the `MetricQueryFilter` enum (`BUILD | CUSTOM | … | HOST | INSTANCE | … | RESOURCE | … | STATUS_CODE | TASK_NAME | TASK_STATE | …`, in `git-*.js`). The same enum names the filters and `aggregateBy` entries of `MetricsQueryInput` and the `outputFilters` of `metricsFilters`.
- The service Metrics page colors a status line by `labels.find(l => l.field === STATUS_CODE)?.value` (`ServiceMetrics-*.js`), and sorts a `STATUS_CODE` breakdown by its first label's value.
- The project Service Metrics page keys a series by its `RESOURCE` and `INSTANCE` labels (`ProjectMetrics-*.js`), and its request group-by is `RESOURCE | STATUS_CODE`.

**bex's choice.** Each surface names the code as Render's same surface does. REST `aggregateBy=statusCode` and MCP `aggregateHttpRequestCountsBy: statusCode` label it `statusCode`, Render's REST vocabulary (its `aggregateBy` value and `filters/http` field). GraphQL `metrics(aggregateBy: ["STATUS_CODE"])` and `projectMetrics(groupBy: "STATUS_CODE")` label it `STATUS_CODE`. No surface answers `code`. `TestStatusCodeBreakdownLabelOnEverySurface` pins all four.

**Still apart then.** bex's other GraphQL labels kept REST's spelling (`resource`, `instance`, `host`, `method`). [GraphQL label names](#graphql-label-names-w3087-2026-11-20) below moved them.

## GraphQL label names (w3/087, 2026-11-20)

w3/080 found that Render's GraphQL names a label by its `MetricQueryFilter` value but moved only the status code. Read again from the same public bundle the same way (about 457 chunks that day; `git-DtnyV415.js`, `ServiceMetrics-mxN_k-Uz.js`, `metrics-BONcUgOy.js`, `DatabaseMetrics-DaYl3aGg.js`, `ProjectMetrics-reBvu8pN.js`, `index-bOVEr6Yr.js`). Render's charts take a `seriesKey`, and the shared chart reads each series' key as `labels.find(l => l.field === seriesKey)?.value`.

- **The enum** (`git-*.js`): `BUILD | CUSTOM | DEFINITION_TYPE | ENVIRONMENT | HOST | INSTANCE | IS_ROOT_TASK | LEVEL | METHOD | PATH | PROJECT | REGION | RESOURCE | RESOURCE_TYPE | RUN_ID | SANDBOX_STATE | SERVICE | STATUS_CODE | TASK_NAME | TASK_STATE | TRAFFIC_DIRECTION | TRAFFIC_SOURCE | WORKFLOW_VERSION_NAME`.
- **Host.** The service page's Total Requests breakdown offers `INSTANCE` (All requests), `STATUS_CODE` (Status code) and `HOST` (Host). It sends the choice as `aggregateBy: [HOST]` or `[STATUS_CODE]`, else `[RESOURCE]`, and charts with `seriesKey` set to the same value. A host breakdown's label is therefore `HOST`. w3/080 had seen `HOST` only as a filter value.
- **Instance.** The service page's CPU, Memory and Disk charts use `seriesKey: INSTANCE` and name each line by its `INSTANCE` label (`metrics-*.js`). So does the Key Value page's Active Connections chart (`index-*.js`).
- **Resource.** The database page's Active Connections, Table Scans and Transaction Exhaustion charts use `seriesKey: RESOURCE`. The project page keys a series by `RESOURCE` and `INSTANCE` (`ProjectMetrics-*.js`).
- **Method.** No metrics chart breaks requests down by method. `METHOD` is a filter value only. bex's method breakdown is its own, and it takes the enum's value.
- **Read and write.** The database page's Disk Activity, Disk Operations and Network Activity, and the Key Value page's Disk Activity (`index-*.js`), split their two lines by `CUSTOM`, with a value per graph: `DISK_READ_THROUGHPUT | DISK_WRITE_THROUGHPUT`, `DATABASE_DISK_READ_OPERATIONS | DATABASE_DISK_WRITE_OPERATIONS`, `NETWORK_READ_THROUGHPUT | NETWORK_WRITE_THROUGHPUT`. Memory splits the same way (`MEMORY | MEMORY_RSS | MEMORY_CACHE`), and Lock-Delayed Queries too (`DATABASE_SLOW_LOCK_COUNT | DATABASE_SLOW_LOCK_TOTAL_SECONDS`). Replication Lag keys by `HOST` and names a line `${CUSTOM} ${HOST}`. Bandwidth sources key by `TRAFFIC_SOURCE`, and `TRAFFIC_DIRECTION` picks private-link traffic's direction.
- **Percentile.** A latency line is named from `series.parameters.quantile` (`p${quantile.toFixed(2).slice(2)}`), not from a label. `parameters` is a `MetricResultParameters` object, `{quantile}`, on each series.

**bex's choice.** GraphQL `metrics`, `datastoreMetrics` and `projectMetrics` now name `resource`, `instance`, `host` and `method` `RESOURCE`, `INSTANCE`, `HOST` and `METHOD`, beside w3/080's `STATUS_CODE`. REST and MCP keep the lowercase names. `quantile`, `aggregate`, `direction` and `degraded_sources` are Bex extensions and keep their names on every surface ([ADR010 § Series label names](../ADR010-observability.md#series-label-names-w3080-w3087-2026-11-20)). `direction` stays because bex serves the read and write graphs through its own `datastoreMetrics`, which no Render client calls. `degraded_sources` stays because shipped mobile builds read it. `TestGraphQLMetricLabelNames` pins the names.

**Percentile, since w3/095 (2026-11-20).** bex's GraphQL `parameters` was a list, `[{quantile}]`, so a Render client's `series.parameters.quantile` read nothing. It is now one `MetricResultParameters` object on every series of `metrics`, `projectMetrics` and `datastoreMetrics`: `{quantile}`, with `quantile` the percentile a latency series was read at and null on every other series, as in Render's mock series. The input side, `MetricsQueryInput.parameters: [{quantile}]`, is a list on Render too and stays one. No dashboard or shipped mobile document read the old list, and the dashboard keeps naming its lines from the `quantile` label. `TestGraphQLSeriesParametersRenderShape` reads each series the way Render's `ServiceMetrics` does and pins the names.
