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
| Network card | Status Code + quantile page-level; "Response Times (0.95)" | Status Code in the card header; `Percentile` (p50/p75/p90/p99, default p90) on Response Times; aggregate request count |
| Logs tab (shared) | Same six preset buttons | Same shared dropdown component; Logs keeps its own 1 h default (bex-api's default span) |

## Accepted drift (recorded, not built)

| Render capability | Why bex diverges |
| --- | --- |
| Host / Path network filters | bex's metrics API rejects HOST/PATH filters (w3/m12) — Traefik's service-level series carry no such labels; discovery (`metricsFilters`) offers no values, so the dropdowns would be dead controls |
| Percentile "All" overlay | The `metrics` GraphQL query carries a single `quantile` parameter; a four-series overlay would need multi-quantile queries bex-api doesn't serve |
| "Last 30 days" range | Plan-gated (disabled) even on Render's own UI; bex's Prometheus retention makes it the retained-window anyway |
| "Custom" range | Deferred — bex offers the eight relative presets only |
| Observability-integrations banner | External metric drains are an explicit non-goal (`.pm/DO_NOT_DO.md`; same class as log/metric drains) |
| Group-by option set | bex groups by status/method (Traefik labels); Render's set differs — bex's is the honest label set its meter actually has |
| Bandwidth "Usage this month" value | bex composes real HTTP + NAT (direct-public L3) + WebSocket egress (w8/m15); only `privateLink` reports 0 (no such product surface) — an honest subset, never a fabricated total |
| Render logs `?r=` grammar | `15m`/`6h` alias to the nearest preset (`30m`/`4h`); `1h`/`24h`/`7d` now parse natively; retired bex ids (`3h`/`6h`/`1d`) degrade to the default range |

## Cross-surface note

w5/m42 changed only `dashboard/` — REST (`GET /v1/metrics/*`), GraphQL (`metrics`, `metricsFilters`, `monthToDateBandwidth`), and MCP metrics verbs in `lego/backend/internal/metrics` are untouched, so the three API surfaces remain in lock-step; the p90 default and 12 h window are client-side choices (bex-api's own defaults — quantile 0.95, 1 h span — still apply to direct API callers, matching Render's API/UI split: Render's UI defaults also differ from its API defaults).
