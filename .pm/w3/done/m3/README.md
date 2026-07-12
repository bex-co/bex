# w3 · m3 — Metrics page PoC: beancount-cms (Render-style)

**Worker:** worker3 **Goal:** the dashboard's first real bex-api data page — a Render-style Metrics page for `beancount-cms` rendering all six live metrics (memory, CPU, instances, requests, response times, bandwidth) from bex-api's GraphQL `metrics(...)` query. **Status:** done

## Tasks (in order)

| id   | title                                                                     | est | depends_on   |
| ---- | ------------------------------------------------------------------------- | --- | ------------ |
| t001 | Wire Apollo to bex-api GraphQL: metrics query + codegen types + session auth — **DONE** | 40m | — |
| t002 | Metrics page route with Render IA: Application + Network metric charts — **DONE** | 45m | t001 |
| t003 | Live wiring: CORS for the dashboard origin, empty/503 states, verify against prod — **DONE** | 30m | t002 |
| t004 | Simplify — `/simplify` over the code this milestone changed — **DONE**    | 20m | t003         |
| t005 | Test coverage — meaningful tests for query mapping + chart/page behavior — **DONE** | 30m | t003 |

## Definition of done

Visiting the dashboard's metrics page for `beancount-cms` (logged in via Kratos) renders the six charts in Render's layout — Application Metrics (Memory with limit + Percentage/Total toggle, CPU, Total Instances) and Network Metrics (Total Requests, Response Times with percentile picker, Outbound Bandwidth) — populated with **live data** from `https://api.bex.co/graphql`; the request charts show real points (Prometheus-backed), and a metric whose source is missing renders an explicit "metrics source not configured" state instead of an empty chart. `yarn build` + dashboard tests green; screenshot captured to `.playwright-mcp/`.

## Learned from Render (source design)

Captured live from `dashboard.render.com/web/srv-d2rnr3jipnbc73deuvgg/metrics` (screenshots `.playwright-mcp/render-metrics-srv-d2rnr3.png`, `render-metrics.png`):

- Page chrome: time-range picker ("Last 12 hours"), events filter; sidebar Monitor → Metrics.
- **Application Metrics** card with a **Percentage | Total** toggle (Percentage default): **Memory** (sublabel `Limit 512 MB`, 0–100% line), **CPU** (sublabel `Limit 0.5 CPU`, 0–100% line), **Total Instances** (integer line).
- **Network Metrics** card ("Aggregated across all instances"; Status Code / Host / Path filters): **Total Requests** (total-count sublabel, bar chart, Group-by dropdown), **Response Times** (Percentile dropdown, ms line), **Outbound Bandwidth** (MB line + "Usage this month" footer).
- bex-api's GraphQL `metrics(resource!, metric!, startTime, endTime, resolutionSeconds, quantile, percentage, statusCode, host, path, groupBy) → [{unit, labels, points{timestamp,value}}]` is a 1:1 backend for every element above, including the Percentage toggle (`percentage: true`) and the percentile picker (`quantile`).

## Source + Goal linkage

- **Source:** user request 2026-07-06 — "learn from Render's metrics page, add a beancount-cms-v2 metrics page as simple PoC" (the App's resource/CR name is `beancount-cms`; `beancount-cms-v2` is only its `.onbex.co` hostname — confirmed live via bex-api's /v1/services during t001); design captured live via Playwright (see above); backend shipped in `w3/m2` (Metrics API) + Prometheus request-metrics backend (deploy commits `cbcd577`, `851b43a`).
- **Goal linkage:** `GOAL.md` #2 ("basic obs for operation") — the human-facing half of the metrics obs slice; `docs/ADR008-vision.md` Render-parity dashboard pillar (w5). Proves the dashboard→bex-api GraphQL path end-to-end (`docs/ADR006-bex-api.md` "Render dashboard compatible" was built exactly for this client).
- **Expected outcome:** an operator opens the dashboard and sees beancount-cms's live CPU/memory/instances/requests/latency/bandwidth without curl or kubectl — the first non-auth page backed by real bex-api data, and the template for wiring the remaining dashboard pages.
- **Why now:** the entire supply chain just landed — Metrics API (w3/m2), the Prometheus request-metrics backend (deployed + verified today), and the dashboard scaffold with auth (w5/m1). The dashboard currently renders only sample data; this PoC is the smallest slice that turns it into a real client, and it exercises the Kratos-session + CORS path every future dashboard page needs.
