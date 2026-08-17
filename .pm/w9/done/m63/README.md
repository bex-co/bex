# w9 · m63 — Log-list virtualization + honest loading states on the detail tabs

**Worker:** worker9 **Goal:** kill the dominant render-time costs the w9/m69 list-route skeletons don't cover — virtualize the unbounded log line list (the live-tail jank source) and fix the detail-tab loading states that today read as empty data ("No data in range" flash on Metrics, `null`/bare spinners on Logs, layout pop-in on Scaling) **Status:** done (log virtualization deferred → `045`)

## Results (2026-08-16)

- **t001 — ANSI parse once at ingest (shipped, the dominant per-frame cost):** `makeLogLine` now computes `line.spans` at ingest; `LogLineList` reads it instead of re-parsing the whole retained buffer's ANSI on every SSE frame. **Row virtualization was attempted and reverted** — `@tanstack/react-virtual` would not render a range under jsdom (tried `initialRect`, a state-backed scroll ref, and geometry/ResizeObserver stubs), so it couldn't be verified, and virtualizing the platform's most-watched screen without a working test harness was too risky for an autonomous pass. Deferred as a focused follow-up: **`045`**.
- **t002 — Metrics loading shimmer (shipped):** `MetricSection` shows a chart-height `Skeleton` while a metric's first fetch is in flight (no series yet) instead of falling through to the child chart's "No data in range" empty state; a poll refetch over existing data keeps the chart. The event timeline shows 3 skeleton rows instead of a spinner. Datastore panels inherit it (shared `MetricSection`).
- **t003 — Logs/Scaling skeletons (shipped):** new shared `LogPanelSkeleton` (monospace-bar rows) replaces the log-viewer's loading spinner AND fills `NonStaticRoute`'s new `loadingFallback` (the Logs tab never renders a blank `null` body during type-resolve); the Scaling tab reserves the manual-card slot with a `CardSkeleton` while autoscaling loads (no pop-in). Only the logs route uses `holdWhileLoading`; shell/scaling/plan render children while loading (unaffected).
- **t006:** `metric-section.test.tsx` (shimmer-while-loading, not the chart; chart once resolved; chart kept during a poll) + a `map.test.ts` ANSI-at-ingest assertion. **2,140 tests + ESLint + tsc green.**

## Tasks (in order)

| id   | title                                                                | est | depends_on |
| ---- | -------------------------------------------------------------------- | --- | ---------- |
| t001 | ANSI parse once at ingest — **DONE**; virtualization deferred → `045` | 1h  | —          |
| t002 | Metrics tab: loading shimmer instead of the "No data in range" flash — **DONE** | 45m | — |
| t003 | Logs/Scaling tabs: skeletons instead of null/spinner + reserved slot — **DONE** | 45m | — |
| t004 | Render parity — **DONE** (UI loading states only; matches Render's loading affordances, no API surface) | 20m | t001, t002, t003 |
| t005 | Simplify — **DONE** (shared `LogPanelSkeleton`; central `MetricSection` shimmer; lint 0) | 20m | t004 |
| t006 | Test coverage — **DONE**                                             | 40m | t004       |
| t007 | Closeout — **DONE**                                                  | 10m | t006       |

## Definition of done

- The log viewer renders only visible rows (virtualized) and a busy live tail no longer re-parses/reconciles all ~1,100 retained lines per SSE frame (ANSI parsing happens once per line at ingest, not per render); scrolling a full buffer stays smooth (spot-check via the React profiler or frame timing on a busy build tail).
- Metrics charts show a chart-shaped loading shimmer during fetch — the literal "No data in range" empty state appears **only** when the fetch resolved empty; the metrics event timeline shows skeleton rows instead of a centered spinner.
- The Logs tab never renders a blank body (`null`) or bare spinner while the service type / first page resolves — a log-panel skeleton holds the space; the Scaling tab reserves the manual-scaling card slot while autoscaling state loads (no pop-in shift).
- Autoscroll/follow behavior, line selection, and instance-slug affordances in the log viewer are unchanged.
- All dashboard tests green.

## Source + Goal linkage

- **Source:** perf sweep 2026-08-16 (rendering leg of the w5/m67–m69 follow-on, handed to w9 by user direction). Evidence: `dashboard/src/features/logs/components/log-line-list.tsx:113` maps the full retained buffer (up to `DEFAULT_MAX_LINES = 1000` live + 100 history, `use-live-logs.ts:82`) into DOM with **no** virtualization anywhere in the app, and the `parsed` ANSI `useMemo` at `log-line-list.tsx:73` re-runs over **all** lines on every SSE frame append; `metric-section.tsx:28-33` ignores `result.loading` so `SvgLineChart` falls through to `EmptyChart` ("No data in range") during every fetch on the highest-traffic detail tab; `event-timeline.tsx:48-52` bare spinner; `non-static-route.tsx:34-36` renders `null` while `useServer` resolves (Logs tab blank), `log-viewer.tsx:161-165` bare spinner; `services.$serviceId.scaling.tsx:45` withholds the manual card until autoscaling resolves (layout shift).
- **Goal linkage:** ADR008 vision — the dashboard is the human surface; the live log tail is the platform's most-watched screen during a deploy, and misleading "no data" flashes on Metrics read as product breakage, not latency. Completes the m69 skeleton story on the detail tabs m69 explicitly scoped out.
- **Expected outcome:** smooth live tails on busy builds (bounded per-frame work), and every detail-tab loading phase reads as *loading* — never as empty data or a blank region.
- **Why now:** w9/m62's deploy-detail waterfall fix mounts the log panels earlier, making their loading states more visible — the two land coherently; virtualization is also the prerequisite for ever raising the retained-line cap.
- **Render parity task:** **included** — this changes visible UI loading/streaming states on the service detail tabs; check against render.com's log tail + metrics loading UX and keep consistent (no REST/GraphQL/MCP surface is touched, so the parity check is UI-only — the w9/m69 precedent).
