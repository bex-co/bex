# w3 · m4.5 — Metrics page parity: application charts, limits, network filters

**Worker:** worker3 **Goal:** close the visible gaps between bex's metrics page and Render's — draw the (now Prometheus-backed) CPU/memory/instances history as real charts with limit labels, wire the network-metric filters the backend already supports, and make the longer time ranges actually hold data. **Status:** done (2026-07-07; implemented + verified locally against prod data — prod rollout itself awaits the next `/ship`)

## Tasks (in order)

| id   | title                                                                                                   | est | depends_on             |
| ---- | -------------------------------------------------------------------------------------------------------- | --- | ---------------------- |
| t001 | Application Metrics history charts: Memory/CPU/Total Instances draw stepped series — **DONE**            | 45m | —                      |
| t002 | Limit labels on Memory/CPU cards from CPU_LIMIT/MEMORY_LIMIT (aggregateAllMethod: MAX) — **DONE**        | 30m | t001                   |
| t003 | Network filters: Status Code dropdown (metricsFilters) + Group by on Total Requests — **DONE**           | 45m | —                      |
| t004 | Prometheus retention bump + Render-like time-range presets — **DONE**                                    | 30m | t001                   |
| t005 | Simplify — run `/simplify` over the code this milestone changed — **DONE**                               | 20m | t001, t002, t003, t004 |
| t006 | Test coverage — meaningful tests for the charts, limit overlay, filters, and range presets — **DONE**    | 30m | t001, t002, t003, t004 |

## Completion notes (2026-07-07)

- Application Metrics render as real charts now: `SvgLineChart` grew multi-series (one line per instance, x mapped by timestamp so late-starting pods align), a dashed limit reference line, and a shared frame/tooltip/legend layer in `chart-layout.ts`; `SvgBarChart` stacks grouped series per time bucket. Both charts converged on a single `series` prop API.
- Group-by needed a small backend parity fix: GraphQL's `aggregateBy` (`STATUS_CODE`/`METHOD`) now maps onto Core's `GroupBy` exactly like REST's `groupBy` param (`lego/backend/internal/metrics/graphql.go` + test).
- Live-verified via the dev-server tunnels against prod data (`beancount-cms`): all three application charts drew 121-point history, the Status Code dropdown populated with real discovered codes (200/301/304/307/403/404/405/500) and filtering to 404 visibly re-scoped requests+latency (bandwidth deliberately unfiltered — no `code` label on Traefik's bytes counter); limit values (0.5 CPU / 512 MiB) verified against the local cluster's `metrics-demo`, which has tier limits. Screenshot: `.playwright-mcp/m45-metrics-parity.png`.
- Group-by's visual (stacked+legend) is unit-tested; its live check needs the shipped backend (prod ignores `aggregateBy` until deployed). Same for prod retention (3h → 3d + 8Gi PV) — both land with the next `/ship` + Argo sync.
- `/simplify` (2 reviewers) drove: `useMemo` on the hook's series mapping (the chart memos now actually cache), one page-level live-range timer, limit queries riding the same tick (no stray 30s Apollo poll), shared `MetricSection`/frame-builder/palette, dead `StatTile` deleted.

## Definition of done

Side-by-side with Render's live metrics page (`dashboard.render.com/web/srv-…/metrics`): bex's Application Metrics section renders Memory, CPU, and Total Instances as multi-point stepped charts over the selected time range (Percentage/Total both working; the stale "from metrics-server" caption replaced), Memory and CPU show a "Limit …" label sourced from the `MEMORY_LIMIT`/`CPU_LIMIT` metrics; Total Requests offers a working Group-by (status/method) and a Status Code filter populated from `metricsFilters`' discovered values that actually filters the chart; the 12h range (and any longer presets added) draws real data on prod because Prometheus retention covers it. `yarn build` + dashboard tests green; verified live against prod `beancount-cms`.

## Source + Goal linkage

- **Source:** user request 2026-07-07 — side-by-side check of Render's live metrics page (element inventory re-captured this session: charts + Limit→/plan links + Status Code/Host/Path filters + Group by + Percentile controls) vs `dashboard.bex.co/services/beancount-cms/metrics` after w3/m4 shipped.
- **Goal linkage:** `GOAL.md` #2 ("basic obs for operation") and the standing render.com feature-parity goal — w3/m4 made the data history-stepped; this makes that history *visible*, which was m4's declared expected outcome ("charts draw lines, not dots").
- **Expected outcome:** an operator sees Render-equivalent resource-history charts with limits and can slice request metrics by status code / method — no chart on the page is a stat-only placeholder anymore.
- **Why now:** m4's backend history is deployed to prod but invisible — the page still renders stat cards, so the shipped capability has no user-facing payoff until this lands. All the API pieces (stepped series, `*_LIMIT`, `metricsFilters`, `groupBy`, `statusCode`) already exist; this is pure consumption, cheapest right after m4 while the contract is fresh.

## Out of scope (documented deviations, not gaps to close here)

- **Host/Path filters** — the backend accepts them but cannot apply them (Traefik service-level counters carry no host/path labels; recorded in docs/observability.md). Rendering dead controls would fake capability.
- **Event timeline / "Filter events"** — Render overlays deploy events on charts; bex has no events feed yet (separate milestone if pursued).
- **"Manage scaling" / "Limit → plan" link targets** — bex has no scaling/plan pages; labels render as text.
- **Metrics streaming integrations** (Render's footer upsell).
