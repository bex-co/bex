# w5 · m56 — Metrics percentile "All" overlay + Custom / Last-30-days ranges

**Worker:** worker5 **Goal:** The service Metrics page closes the two parity drifts w5/m42 recorded: the Network card's percentile control gains an "All" option that overlays p50/p90/p99 from a single multi-quantile backend read, and the shared range dropdown gains a "Last 30 days" preset and a "Custom" start/end picker — matching Render's captured metrics controls (bex adds them ungated, without Render's plan gate). **Status:** todo

## Tasks (in order)

| id   | title                                                                                     | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Backend: metrics read returns multiple quantiles (p50/p90/p99) in one response (REST/GraphQL/MCP) | 45m | —          |
| t002 | Metrics UI: percentile control gains "All" — overlays the multi-quantile series with a legend | 40m | t001       |
| t003 | Shared range dropdown: add "Last 30 days" preset + a "Custom" start/end picker (shared with Logs) | 40m | —          |
| t004 | Render parity — cross-surface consistency check                                           | 30m | t002, t003 |
| t005 | Simplify — run `/simplify` over the changed code                                          | 20m | t004       |
| t006 | Test coverage — multi-quantile read + percentile "All" + range presets                   | 30m | t004       |
| t007 | Closeout — verify DoD, mark done, move milestone                                          | 15m | t006       |

## Definition of done

On dev-5: the Network metrics card's percentile control offers "All" and, when selected, overlays p50/p90/p99 series (from a single backend read that returns all requested quantiles) with a legend distinguishing them; the shared range dropdown (used by both Metrics and Logs) offers "Last 30 days" and a "Custom" start/end range, the latter bounded by `BEX_MAX_QUERY_HOURS`; over-window and store-less cases stay honest (a named 503/400, never a silent empty). The multi-quantile field is consistent across REST, GraphQL, and MCP. Backend + dashboard suites green.

## Source + Goal linkage

- **Source:** w5/m42's recorded drift (`.pm/w5/done/m42/README.md` — "percentile 'All' overlay (bex's `metrics` query takes a single `quantile`); the plan-gated 'Last 30 days' and 'Custom' range options"); re-surfaced by `/pm-brainstorm` 2026-07-27. Shell/Disk/One-Off-Jobs nav (the third m42 drift) stays excluded per `.pm/DO_NOT_DO.md`.
- **Goal linkage:** Render-parity pillar (`docs/ADR018-render-parity.md`); closes the last two open metrics-page cells; evidence in `docs/render-artifacts/metrics-page.md`.
- **Expected outcome:** the metrics page reads at parity with Render's percentile + range controls; a user can compare p50/p90/p99 at a glance and query a custom or 30-day window.
- **Why now:** w5/m42 + m43 already rebuilt the metrics page and recorded these as the two remaining known gaps; the range dropdown is shared with Logs, so one change improves both pages.
- **Render parity task included:** yes — the change spans the backend metrics read (REST/GraphQL/MCP) and the dashboard; the parity check exercises all surfaces against Render's captured metrics page.
