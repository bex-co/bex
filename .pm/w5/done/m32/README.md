# w5 · m32 — Dashboard parity walk: page-by-page against Render

**Worker:** worker5 **Goal:** A systematic, evidence-backed page-level walk of Render's dashboard vs bex's — the m13 audit pattern aimed at UI depth. Output: a dated walk artifact, corrected ledger UI cells, and one filed polish note per real gap — the backlog-refill for future polish rounds. **Status:** done

## Tasks (in order)

| id   | title                                                                                              | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Walk Render live (Playwright): capture each page's controls/sections/empty-states → `docs/render-artifacts/dashboard-walk/` — **DONE** | 60m | —          |
| t002 | Walk bex the same route on the mock cluster; per-page diff table (match / missing control / divergent behavior) — **DONE** | 60m | t001       |
| t003 | File the output: one polish note per real gap in the owning workstream; correct ledger UI cells the walk contradicts — **DONE** | 45m | t002       |
| t004 | Simplify — `/simplify` over anything this milestone changed (docs-only likely; record it) — **DONE**             | 15m | t003       |
| t005 | Test coverage — record explicitly: audit milestone, walk reproducibility documented in the artifact — **DONE**    | 15m | t003       |
| t006 | Closeout — DoD met → move milestone to `done/` — **DONE**                                                        | 10m | t005       |

## Definition of done

A dated walk artifact covers every shipped page pair (service overview / settings / environment / logs / metrics / events / deploy detail; datastore pages; workspace surfaces — team, usage, audit, env groups, projects/environments), with a per-page verdict; every gap found is either filed as an evidence-citing inbox note in its owning workstream or explicitly judged not-a-gap; every ADR018 UI cell the walk contradicts is corrected with a pointer to the walk.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 10 (2026-07-15). UI cells in `docs/ADR018-render-parity.md` flip ✅ when a page ships, but no systematic page-level side-by-side has run since the early captures (w3/m4.5 metrics, w6/m5 workspaces) — and the API-level wells are dry, so UI depth is where unrecorded parity debt hides.
- **Goal linkage:** Render parity (pillar 1), UI column integrity; the m13 audit precedent (its output ordered two weeks of the parity queue).
- **Expected outcome:** the next polish rounds draw from verified evidence instead of re-censusing dry wells; ledger UI claims become trustworthy at page depth.
- **Why now:** ten brainstorm rounds exhausted the API-level sources; this is the designated backlog-refill, and it's cheapest while the Playwright harness and mock cluster are warm from the m31 work.
- **Render parity closing task: omitted** — the milestone **is** the parity check (the m13 / w7-m30 precedent); it ships no product surface change.

## Resolution

Completed an authenticated live Render walk and an equivalent bex mock-cluster
walk across every required service, datastore, project/environment, and
workspace page family. The tracked artifact records the route inventory,
screenshot names, seeded fixtures, per-page verdicts, limitations, and exact
reproduction method. Six bounded UI gaps were filed as `w5/015`–`020`; the
cross-layer Postgres Logs gap was sized as `w3/m28`; Env Group creation and
metrics host/path work were linked to their existing owners instead of
duplicated. Every other divergence is explicitly classified as a functional
match, deliberate superset, information-architecture difference, configured
degraded state, unreachable comparator, or repository non-goal.

ADR018 now correctly says Render's service root lands on Deploys (with the bex
redirect polish filed), upgrades static route/header UI from ◐ to ✅ based on
the live editor comparison, and adds the missing ✖ row for Postgres log history.
This was a docs/board-only audit, so no product simplify or product tests
applied; Markdown formatting and artifact reproducibility are the verification
surface.
