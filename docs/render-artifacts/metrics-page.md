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

## Closed by w5/m58 (2026-07-30)

The last recorded Network-card drift — Host / Path filters — is closed. The t001 design probe refuted the earlier hypothesis that Host could ride Prometheus router labels: `traefik_service_requests_total` / `traefik_service_request_duration_seconds_bucket` carry `service`/`code`/`method`/`le` only, and `addRoutersLabels` adds a router **name**, not the matched `Host()`/`Path()`. So **both** filters are served from the request-log store (Loki), the one backend with a per-request host/path axis (Traefik's access log carries `RequestHost`/`RequestPath` per line, plus `Duration` ns for latency).

| Was drift | Now |
| --- | --- |
| Host network filter | ✅ Card-header **Host dropdown**, discovered via the logs `logLabelValues(label:"host")` read (host resolves from the App's own URLs, so the dropdown populates even with no store). A host-filtered `http_requests`/`http_latency` read is served from Loki (`sum(rate(... \| json \| request_host=… [step]))` / `quantile_over_time(… \| unwrap latency_ns …)/1e9`), so the requests + response-time series change to the filtered subset. |
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

## Cross-surface note

w5/m42 changed only `dashboard/`; **w5/m56 extended the metrics _read_ itself** — REST (`GET /v1/metrics/*` repeated `quantile`), GraphQL (`metrics` `parameters[]`), and MCP (`get_metrics` `quantiles[]`) now all serve multiple quantiles in one call through one `MetricsWithQuantiles` core, so the three API surfaces stay in lock-step. The p90 default and 12 h window remain client-side choices (bex-api's own defaults — quantile 0.95, 1 h span — still apply to direct API callers, matching Render's API/UI split: Render's UI defaults also differ from its API defaults). The percentile "All" and the "Last 30 days"/"Custom" ranges are ungated (Render plan-gates the latter two).
