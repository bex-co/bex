# w5 · m43 — Chart event markers on the service Metrics tab (Render parity)

**Worker:** worker5 **Goal:** Every chart on `/services/$serviceId/metrics` overlays the service's events the way Render's dashboard does — a full-height vertical dashed line per event, an icon badge in a strip above the plot (deploy started = neutral dashed, deploy live = green, deploy failed = red, lifecycle = neutral dot), overlapping events collapsed into a count badge, a hover tooltip (label + time + "Click to view details"), and badges linking to the deploy detail page (or the Events tab) — so a usage spike or drop is instantly attributable to the deploy/restart that caused it. **Status:** DONE 2026-07-17 — implemented directly (not task-by-task) from a live Render capture; verified end-to-end in the browser against the extended `local-bex` stub; full dashboard suite 223 files / 1,367 tests, lint + typecheck clean.

## Tasks (in order)

| id   | title                                                                                                                                                                                                              | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Capture Render's live event-marker UX via Playwright (dashboard.render.com metrics page: dashed line + 30px icon strip in uPlot `.u-over`, cluster count badge, click-to-zoom, hover popover, filter taxonomy) — **DONE** | 45m | —          |
| t002 | `features/metrics/lib/chart-events.ts`: event→marker mapping (kind/labelKey per event type, wire `deployStatus` succeeded\|canceled\|failed) + x-position clustering with edge clamping — **DONE**                       | 45m | t001       |
| t003 | `features/metrics/components/chart-event-markers.tsx` (`ChartEventLines` SVG overlay + `ChartEventStrip` badge strip with tooltip and deploy/Events links) wired into `SvgLineChart` + `SvgBarChart`, both metrics cards, and the route's existing `useServiceEvents` feed; en+zh locales — **DONE** | 1h  | t002       |
| t004 | Render parity check: UI-only surface (REST/GraphQL/MCP untouched — reuses the shipped `serviceEvents` read); marker semantics verified against `store.RenderDeployStatus`'s wire enum and Render's captured treatments — **DONE** | 15m | t003       |
| t005 | Test coverage: `lib/__tests__/chart-events.test.ts` (window filtering/sorting, per-type kind+label mapping incl. wire + defensive statuses, clustering threshold, edge clamping); full suite 1,367 green — **DONE**       | 30m | t003       |
| t006 | End-to-end verification: extend `scripts/local-bex.mjs` with synthetic Metrics waveforms + boot-relative wire-vocabulary events; browser-verify markers, tooltip, and deploy-page click-through on all six charts — **DONE** | 45m | t004, t005 |

## Definition of done

All six charts on the service Metrics tab (Memory, CPU, Total Instances, Total Requests, Response Times, Outbound Bandwidth) show a dashed vertical line + kind-styled icon badge at each in-window service event; overlapping events collapse to a count badge; hovering shows label + time + hint; clicking a deploy-backed badge lands on that deploy's detail page. Verified in the browser against the dev stub; unit tests cover the mapping/clustering lib.

## Source + Goal linkage

- **Source:** user request 2026-07-17 — "research how event is marked and displayed in dashboard.render.com/…/metrics and add similar to dashboard.bex.co"; built on the m36-shipped Event timeline card and the `serviceEvents` GraphQL read (`w3/m7`).
- **Goal linkage:** pillar 1 (Render dashboard parity) — Render overlays events on every metrics chart; bex's metrics page had the timeline card but bare charts, so metric anomalies weren't attributable in place.
- **Expected outcome:** deploys/restarts are visually correlated with metric changes on the charts themselves, matching Render's UX (captured evidence: uPlot overlay structure, `chart-event-*` treatments, cluster badge, hover popover).
- **Why now:** direct user request; the enabling pieces (events feed, timeline card, deploy detail page `w5/m29`) had all shipped, leaving this a pure-UI, dependency-free close.
- **Standing closing tasks:** Render parity folded in as t004 (this milestone _is_ a parity closure; no API surface changed). Test coverage is t005. `/simplify` was not run as a separate pass (implemented directly in-session, reusing the existing `referenceValue`/crosshair chart patterns); flag it on the next touch of `features/metrics/` if desired. Closeout is this record.
- **Not done here:** Render's click-a-cluster-to-zoom (bex's range model has fixed presets, no custom start/end) and the checkbox-tree event-type filter on the toolbar (bex's timeline card has a simpler category select). Both are candidates for a future note if custom time ranges ever land.
