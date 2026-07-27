# w5 · m56 — Metrics percentile "All" overlay + Custom / Last-30-days ranges

**Worker:** worker5 **Goal:** The service Metrics page closes the two parity drifts w5/m42 recorded: the Network card's percentile control gains an "All" option that overlays p50/p90/p99 from a single multi-quantile backend read, and the shared range dropdown gains a "Last 30 days" preset and a "Custom" start/end picker — matching Render's captured metrics controls (bex adds them ungated, without Render's plan gate). **Status:** done

## Tasks (in order)

| id   | title                                                                                     | est | depends_on | status     |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- | ---------- |
| t001 | Backend: metrics read returns multiple quantiles (p50/p90/p99) in one response (REST/GraphQL/MCP) | 45m | —          | — **DONE** |
| t002 | Metrics UI: percentile control gains "All" — overlays the multi-quantile series with a legend | 40m | t001       | — **DONE** |
| t003 | Shared range dropdown: add "Last 30 days" preset + a "Custom" start/end picker (shared with Logs) | 40m | —          | — **DONE** |
| t004 | Render parity — cross-surface consistency check                                           | 30m | t002, t003 | — **DONE** |
| t005 | Simplify — run `/simplify` over the changed code                                          | 20m | t004       | — **DONE** |
| t006 | Test coverage — multi-quantile read + percentile "All" + range presets                   | 30m | t004       | — **DONE** |
| t007 | Closeout — verify DoD, mark done, move milestone                                          | 15m | t006       | — **DONE** |

## Walk findings (live Render Metrics walk 2026-07-27)

Walked Render's `cuckoo-backend` Metrics page authenticated, confirming both controls:

- **Range dropdown** presets: Last 30 min → Last 14 days, then **"Last 30 days" (disabled — Render plan-gates it)** and **"Custom"**. Custom opens an absolute picker: Start (date) + Start time, End (date) + End time, "Apply custom range", with a "up to 14 days on Professional" plan-window note. → bex adds "Last 30 days" + "Custom" **ungated**, bounded by `BEX_MAX_QUERY_HOURS` (default 720h = 30d).
- **Percentile control** on Response Times: options p50/p90/p99 + **"All"** (its menu was disabled on this low-traffic service — no recent data — so the p50/p90/p99/All set is taken from m42's captured evidence). → bex's "All" overlays p50/p90/p99 from one multi-quantile read.

## Implementation notes

- **Backend (one core):** `MetricQuery.Quantiles []float64` + `Service.MetricsWithQuantiles` fans `http_latency` out over the requested quantiles (one `Metrics` call each), tagging every series with a `quantile` label; single-quantile / non-latency reads are byte-identical (no label). REST repeats `?quantile=`, GraphQL collects `parameters[].quantile` (+ echoes `parameters { quantile }` per series), MCP takes `quantiles[]` — all three call the one core. No `definitions.ts` splice needed (the GraphQL `parameters` input/echo already existed).
- **UI:** the Network card's Percentile gains "All" (overlays p50/p90/p99 with a legend); the shared `RangeSelect` gains the "Last 30 days" preset + a "Custom…" option opening an absolute start/end Dialog (validated against `MAX_CUSTOM_RANGE_HOURS`); `useLiveRange` treats a custom window as fixed. The Logs tab URL-backs the custom window (`range=custom&rangeStart&rangeEnd`).

## Definition of done

On dev-5: the Network metrics card's percentile control offers "All" and, when selected, overlays p50/p90/p99 series (from a single backend read that returns all requested quantiles) with a legend distinguishing them; the shared range dropdown (used by both Metrics and Logs) offers "Last 30 days" and a "Custom" start/end range, the latter bounded by `BEX_MAX_QUERY_HOURS`; over-window and store-less cases stay honest (a named 503/400, never a silent empty). The multi-quantile field is consistent across REST, GraphQL, and MCP. Backend + dashboard suites green.

## Source + Goal linkage

- **Source:** w5/m42's recorded drift (`.pm/w5/done/m42/README.md` — "percentile 'All' overlay (bex's `metrics` query takes a single `quantile`); the plan-gated 'Last 30 days' and 'Custom' range options"); re-surfaced by `/pm-brainstorm` 2026-07-27. Shell/Disk/One-Off-Jobs nav (the third m42 drift) stays excluded per `.pm/DO_NOT_DO.md`.
- **Goal linkage:** Render-parity pillar (`docs/ADR018-render-parity.md`); closes the last two open metrics-page cells; evidence in `docs/render-artifacts/metrics-page.md`.
- **Expected outcome:** the metrics page reads at parity with Render's percentile + range controls; a user can compare p50/p90/p99 at a glance and query a custom or 30-day window.
- **Why now:** w5/m42 + m43 already rebuilt the metrics page and recorded these as the two remaining known gaps; the range dropdown is shared with Logs, so one change improves both pages.
- **Render parity task included:** yes — the change spans the backend metrics read (REST/GraphQL/MCP) and the dashboard; the parity check exercises all surfaces against Render's captured metrics page.
