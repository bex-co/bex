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
