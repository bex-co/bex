# w2 · m13 — Health check path: full-surface parity (PATCH · GraphQL · MCP · dashboard)

**Worker:** worker2 **Goal:** `healthCheckPath` is readable/writable identically across REST, GraphQL, MCP, and the dashboard — today it's REST-create-only, even though the operator has wired it into a real `ReadinessProbe` since `w1/m23`. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                        | est | depends_on           |
| ---- | --------------------------------------------------------------------------------------------------------------------------------------------- | --- | --------------------- |
| t001 | REST: `PATCH /v1/services/{id}` accepts `healthCheckPath` updates (today create-only)                                                         | 30m | —                      |
| t002 | GraphQL: `updateService` mutation gains `healthCheckPath` (create + update parity)                                                             | 25m | t001                   |
| t003 | MCP: expose `healthCheckPath` on the create/update tool                                                                                        | 20m | t001                   |
| t004 | Dashboard: Settings → Health & Alerts field, wired to the mutation (folds `w5/009`; live-capture Render's Health & Alerts section for parity reference into `docs/render-artifacts/`) | 40m | t002                   |
| t005 | Acceptance: change a service's health check path end-to-end, confirm readiness gating reacts                                                  | 25m | t002, t003, t004       |
| t006 | Render parity — verify `healthCheckPath` field/shape/semantics consistency across REST/GraphQL/MCP/UI vs render.com; update the ADR018 row     | 20m | t005                   |
| t007 | Simplify — `/simplify` over the code this milestone changed                                                                                    | 15m | t006                   |
| t008 | Test coverage — meaningful tests for the update path across all surfaces                                                                       | 30m | t006                   |
| t009 | Closeout — verify DoD met, then move the milestone to `done/`                                                                                  | 10m | t008                   |

## Definition of done

`healthCheckPath` is readable/writable identically across REST, GraphQL, MCP, and the dashboard — a change made on any one surface is visible via the others, and the ReadinessProbe reacts to it end-to-end. `docs/ADR018-render-parity.md`'s health-checks row moves off ✖ for the GraphQL/MCP/UI columns.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones to work on` 2026-07-12 — `docs/ADR018-render-parity.md` line 21 gap; folds inbox note `w5/009`, which had explicitly deferred the UI half pending this exact backend work ("file that as a w2 inbox note then, and promote this alongside it").
- **Goal linkage:** pillar 1 (Render parity) — closes a gap the `w1/m13` parity audit itself flagged and that has sat partially scoped since.
- **Expected outcome:** `docs/ADR018-render-parity.md`'s health-checks row moves off ✖ for GraphQL/MCP/UI; no more silent asymmetry between what REST accepts on create and what every other surface can do.
- **Why now:** unblocked by `w1/m23/t001`, which decided to keep the field (wired into a real `ReadinessProbe`) rather than drop it — the precondition `w5/009` was gated on is now met.
- **Render parity closing task: included** — the milestone's entire scope is closing a cross-surface (REST/GraphQL/MCP/UI) parity gap.
