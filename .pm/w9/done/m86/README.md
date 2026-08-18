# w9 · m86 — Honest error states for datastore metric charts

**Worker:** worker9 **Goal:** a datastore metrics query that errors renders a distinct error card, not the empty-window "No data in range" state that hid the `w5/m71` wrong-identifier bug for months. **Status:** done

## Tasks (in order)

| id   | title                                                                       | est | depends_on |
| ---- | --------------------------------------------------------------------------- | --- | ---------- |
| t001 | Audit the chart/metric panels for the empty-vs-error conflation             | 25m | —          — **DONE** |
| t002 | Surface `result.error` as a generic error card distinct from the empty state | 40m | t001       — **DONE** |
| t003 | Render parity (closing)                                                     | 15m | t002       — **DONE** |
| t004 | Simplify (closing)                                                          | 20m | t003       — **DONE** |
| t005 | Test coverage (closing)                                                     | 30m | t003       — **DONE** |
| t006 | Closeout (closing)                                                          | 10m | t005       — **DONE** |

## Definition of done

A datastore metrics query that errors (any GraphQL/network/404 beyond the one recognized `"metrics source not configured"` string) renders a distinct error card matching the logs/metrics pages' error UX; a genuinely empty window still renders "No data in range". Tests assert both branches. If the t001 audit finds sibling chart panels (service metrics) share the swallow-all-errors pattern, they are fixed too.

## Source + Goal linkage

- **Source:** `.pm/w5/045.md` (promoted 2026-08-17 via `/pm-brainstorm` "what to do for w9"); found by `w5/m71`: `DatastoreMetricsPanel`'s only error state is the exact `"metrics source not configured"` message, so with `errorPolicy: "all"` every other error yields `data === undefined` and renders as the empty "No data in range" state — indistinguishable from a genuinely empty window, which is what hid the wrong-identifier Key Value metrics bug for months. `useDatastoreMetrics` already returns `result.error`; the panel ignores it.
- **Goal linkage:** first-party dashboard correctness (`dashboard/CLAUDE.md`); fixes the class (swallow-all-errors panel) behind the symptom w9 just fixed in `w5/m71`.
- **Expected outcome:** metrics failures are visible instead of masquerading as empty windows, so an identifier/wiring bug can't hide again.
- **Why now:** w9 just shipped `w5/m71`'s symptom fix while the metrics-panel code is warm; small, concrete, high leverage. **Render parity INCLUDED (light/UI)** — the change is a dashboard error-state; compare against Render's chart error UX and record any divergence.
- **Sizing note:** this is borderline; the milestone is justified on the bet that the t001 audit finds the service-metrics siblings share the pattern. If it finds only the one panel, `/pm` should record the closeout residual as an inbox note and keep the milestone scoped to what shipped.
