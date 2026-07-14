# w2 · m16 — Managed datastore plan/instance-type updates (Postgres + Key Value)

**Worker:** worker2 **Goal:** A workspace can change a managed Postgres or Key Value instance's plan after creation — across REST/GraphQL/MCP/dashboard — instead of being stuck with the plan chosen at create time. **Status:** DONE

## Tasks (in order)

| id   | title                                                                                                                                                              | est | depends_on       |
| ---- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --- | ----------------- |
| t001 | Postgres: `Service.SetPlan` verb — validate target plan, write CP row (if managed) then patch `Database.spec.plan`; reuse `apps.SetPlan`'s row-then-CR ordering and Render-plan-spelling mapping | 45m | —                  | — **DONE** |
| t002 | Postgres: `PATCH /v1/postgres/{id}` (`plan` field) + GraphQL `updateDatabasePlan(id, plan)` + MCP `update_postgres_plan`                                          | 40m | t001               | — **DONE** |
| t003 | Key Value: `Service.SetPlan` verb (same shape as t001, KeyValue CR)                                                                                                | 30m | —                  | — **DONE** |
| t004 | Key Value: `PATCH /v1/key-value/{id}` (`plan` field) + GraphQL `updateKeyValuePlan(id, plan)` + MCP `update_key_value_plan`                                        | 30m | t003               | — **DONE** |
| t005 | Operator: confirm CNPG `Cluster`/Valkey `StatefulSet` resource requests already re-reconcile on `spec.plan` change — envtest asserting a plan-change patch flows through to updated pod resources within one reconcile | 40m | t001, t003         | — **DONE** |
| t006 | Dashboard: Postgres + Key Value detail-page plan section, reusing the service Settings plan-picker card component                                                 | 45m | t002, t004         | — **DONE** |
| t007 | Live verification: change plan on a real Postgres and KeyValue instance in the mock cluster, confirm pod resources actually resize                                | 30m | t005, t006         | — **DONE** |
| t008 | Render parity: check REST/GraphQL/MCP/UI shape consistency for the new plan-update verb against render.com's documented Postgres/Key Value update behavior, flag any drift as follow-up | 30m | t007               | — **DONE** |
| t009 | Simplify: run `/simplify` over the code this milestone changed                                                                                                     | 30m | t008               | — **DONE** |
| t010 | Test coverage: meaningful tests for `SetPlan` validation, the row-then-CR write ordering, and each surface's PATCH/mutation/tool                                  | 45m | t008               | — **DONE** |
| t011 | Closeout: verify DoD, mark done, move to `w2/done/m16/`                                                                                                            | 15m | t009, t010         | — **DONE** |

## Definition of done

A workspace can change a Postgres or Key Value instance's plan from the dashboard (or REST/GraphQL/MCP), the underlying pod's resource requests update within one reconcile without data loss, and the change is visible on the next `get`/list call across all four surfaces — verified live, not just unit-tested.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones to work on` 2026-07-13, citing `docs/ADR018-render-parity.md` § Managed Postgres row ("No `PATCH` update yet (◐, low)") — verified against code (`lego/backend/internal/postgres/rest.go`, `lego/backend/internal/keyvalue/rest.go`) that neither resource has any update route at all.
- **Goal linkage:** Render-parity core surface; closes bex's last documented `◐` on the managed-Postgres parity row and an undocumented equivalent gap on Key Value.
- **Expected outcome:** the parity ledger's Postgres row's remaining `◐` note resolves; Key Value gains the same capability its sibling datastore already has.
- **Why now:** the `SetPlan` pattern and precedent (App plan changes, shipped) already exist in `lego/backend/internal/apps`, and both CRDs already carry `spec.Plan` as a mutable field (`lego/types/v1alpha1/database_types.go`, `keyvalue_types.go`) — the cheapest remaining Render-parity gap on the board, no new design or dependency risk. Render parity applies (touches REST/GraphQL/MCP/UI), so the standing Render-parity closing task is included, not omitted.
