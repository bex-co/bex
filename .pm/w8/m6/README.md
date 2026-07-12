# w8 · m6 — Usage history: GraphQL period support + dashboard multi-month view

**Worker:** worker8 **Goal:** Close the surface asymmetry where REST (`?period=`) and MCP (`period` arg) can already return a past month's usage but GraphQL can't, and give the dashboard a way to see it — a month picker plus a short trend view over the last few months, reusing the metrics feature's existing chart component. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                      | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Design: month-picker vs. trailing-N-months trend, referencing Render's historical/previous-period billing view (Playwright capture, extends m3's reference) | 30m | —          |
| t002 | GraphQL: add a `period` argument to the `usage` query, reusing the core `Usage` verb (mirrors REST)         | 30m | t001       |
| t003 | Apollo wiring + `useUsage` hook: accept a period param, refetch on change                                   | 30m | t002       |
| t004 | Dashboard: month selector on the Usage page wired to the hook                                                | 35m | t003       |
| t005 | Dashboard: trend view — last-N-months totals per meter, reusing `svg-line-chart` from the metrics feature   | 40m | t004       |
| t006 | Render parity: compare against Render's historical/previous-period billing view; flag drift                 | 30m | t005       |
| t007 | Simplify — `/simplify` over the code this milestone changed                                                  | 20m | t006       |
| t008 | Test coverage — GraphQL period-arg tests, hook period-change tests, empty-history state                      | 35m | t006       |
| t009 | Closeout — DoD met → move milestone to `done/`                                                               | 10m | t008       |

## Definition of done

A user can pick a past month on the dashboard Usage page and see that month's real totals, matching what REST `?period=` already returns for the same month; GraphQL, REST, and MCP all accept the same `period` shape (`YYYY-MM`) with identical semantics.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones for w8` 2026-07-10 — REST (`docs/ADR023-usage-metering.md`) and MCP `get_usage` already accept `period`; `lego/backend/internal/usage/graphql.go`'s `usage` query doesn't, and the docs say so explicitly ("period always defaults to the current calendar month; use REST `?period=` for historical queries"). The dashboard's `UsagePage` (`dashboard/src/features/usage/components/usage-page.tsx`) has no month picker or trend view — current month only.
- **Goal linkage:** `GOAL.md` item 5 (usage metering); pillar 1 (API-first — GraphQL shouldn't be able to do less than REST/MCP for the same core verb); Render-parity dashboard surface.
- **Expected outcome:** any client, including the dashboard, can see a past month's usage without falling back to curl; the three API surfaces are capability-symmetric.
- **Why now:** m1 has been rolling up real data since 2026-07-09 — there's now enough history to make a trend view meaningful, and the GraphQL/REST asymmetry only gets more entrenched (more client code built against the smaller GraphQL surface) the longer it's left unfixed.
- **Render parity: included** (t006) — this changes what surfaces across GraphQL/UI (REST/MCP already have period support, so the parity check for those two is only re-verification, not new ground).
