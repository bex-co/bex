# w5 · m42 — Metrics page simplification: Render's card-level controls + Render-shaped title

**Worker:** worker5 **Goal:** bex's service Metrics tab presents Render's exact control placement — a minimal page-level toolbar (one time-range dropdown + event-timeline toggle), Percentage/Total tabs on the Application card, Status Code filter + per-section Percentile picker on the Network card — and every service tab carries Render's `<name> · <type> · <brand>` document title instead of leading with the raw `srv-` id. **Status:** done (2026-07-17 — implemented in one pass; DoD verified live on dev-5 with a fresh image-backed service plus the local-bex stub walk; 222 files / 1,363 dashboard tests, typecheck + lint green; evidence in [docs/render-artifacts/metrics-page.md](../../../docs/render-artifacts/metrics-page.md); ADR018's Metrics rows checked — no stale claims, the w5/m36 timeline note stays accurate)

## Tasks (in order)

| id   | title                                                                              | est | depends_on             |
| ---- | ---------------------------------------------------------------------------------- | --- | ---------------------- |
| t001 | Toolbar: single time-range dropdown with Render's preset list (default 12h)        | 40m | — — **DONE**           |
| t002 | Event timeline: default-hidden show/hide toggle, event filter stays in the toolbar | 30m | t001 — **DONE**        |
| t003 | Application Metrics card: Percentage/Total tabs + Limit / Manage-scaling links     | 40m | t001 — **DONE**        |
| t004 | Network Metrics card: card-level Status Code filter + per-section Percentile       | 45m | t001 — **DONE**        |
| t005 | Render-shaped document title: `<name> · <service type> · bex dashboard`            | 30m | — — **DONE**           |
| t006 | Render parity: cross-surface check + `docs/render-artifacts/metrics-page.md`       | 30m | t002–t005 — **DONE**   |
| t007 | Simplify: `/simplify` over the code this milestone changed                         | 30m | t006 — **DONE**        |
| t008 | Test coverage: presets, moved controls, timeline toggle, title format              | 45m | t006 — **DONE**        |
| t009 | Closeout                                                                           | 15m | t008 — **DONE**        |

## Definition of done

On a live bex dashboard (dev-5 or prod), `/services/<id>/metrics` shows: a page-level toolbar containing only an event filter, one time-range dropdown (Render's preset list — Last 30 min / hour / 4 hours / 12 hours / 24 hours / 2 days / 7 days / 14 days, default **Last 12 hours**), and an event-timeline show/hide toggle (timeline hidden by default); the Application Metrics card owns the Percentage/Total tabs and its Memory/CPU sections carry a `Limit` link to `/plan` (value shown when configured) with `Manage scaling` → `/scaling` on Total Instances; the Network Metrics card owns the Status Code filter and the Response Times section owns a Percentile picker (p50/p75/p90/p99, default p90) — no page-level quantile/status-code/percentage controls remain. The browser tab reads `<service name> · Web Service · bex dashboard` (type-appropriate label per service type), never a bare `srv-` id after load. `yarn typecheck && yarn lint && yarn test` pass; `docs/render-artifacts/metrics-page.md` records the side-by-side evidence and the accepted drift list.

## Source + Goal linkage

- **Source:** user request 2026-07-17 — `/pm learn from https://dashboard.render.com/web/srv-d2rnr3jipnbc73deuvgg/metrics and simplify https://dashboard.bex.co/services/srv-d9dd16roviqs738quds0/metrics to be consistent with render. including seo header.` Both pages were captured live and authenticated in the same session (Playwright), including Render's expanded range dropdown (30 min/hour/4h/**12h selected**/24h/2d/7d/14d/30d-disabled/Custom) and Response Times percentile dropdown (All/p50/p75/**p90 selected**/p99). Render's title: `backend-v2 ・ Web Service ・ Render Dashboard`; bex's: `srv-d9dd16roviqs738quds0 · Metrics · bex dashboard` on first paint, name-swapped after load.
- **Goal linkage:** Render parity, UI surface (`docs/ADR018-render-parity.md` Metrics row; `docs/ADR006-bex-api.md` — the dashboard is "Render dashboard compatible"). Pure `dashboard/` work; no REST/GraphQL/MCP contract change is expected, which is exactly what the Render-parity task (t006) verifies — it is **included** because this is user-facing feature work on a parity surface.
- **Expected outcome:** the Metrics tab stops cramming range/percentage/quantile/status-code into one page-level bar (bex today) and matches Render's scoping: page-level = time only; card-level = the controls that alter that card. Titles lead with the human service name + type, Render's SEO/head shape.
- **Why now:** a live user comparison on the real prod service (2026-07-17) showed the drift; w5/m36 just added the event timeline (always-open — Render hides it behind a toggle) and w5/m40 finished the Deploys polish, making Metrics the service tab furthest from its Render counterpart. The title still flashes the raw `srv-` id on SSR/first paint (`services.$serviceId.tsx:44-55` documents the swap as name-only, not Render's `name · type` format).
- **Explicitly excluded (anti-goals + accepted drift, recorded rather than built):** Render's "stream your metrics to another observability tool" banner (external metric drains are a `DO_NOT_DO` non-goal); Host/Path network filters (bex's metrics API rejects those filters — w3/m12 — and discovery offers no values; drift, noted in t006); the percentile "All" overlay (bex's `metrics` query takes a single `quantile`); the plan-gated "Last 30 days" and "Custom" range options; Shell/Disk/One-Off Jobs nav (non-goals per `DO_NOT_DO.md`).
