# w10 · m5 — Audit-log fidelity: correct verbs, plan detail, friendly target names

**Worker:** worker10 **Goal:** The audit-log read surface stops mislabeling and dropping detail: an idempotent datastore `SetPlan` no longer masquerades as `UpdatePostgres`/`UpdateKeyValue`, datastore plan changes carry the typed `PlanFrom`/`PlanTo` pair their apps sibling already records, and the stored `target_name` (migration 0038) finally reaches GraphQL and the dashboard so audit rows show human names instead of raw ids. **Status:** todo

## Tasks (in order)

| id   | title                                                                                     | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Fix the no-op `SetPlan` audit verb: same-plan sets stop recording `Update*`               | 20m | —          |
| t002 | Datastore plan-changed audit rows carry `PlanFrom`/`PlanTo` like apps                     | 20m | t001       |
| t003 | Expose `targetName` on the GraphQL `AuditLog` type                                        | 30m | —          |
| t004 | Dashboard audit page renders friendly target names (id fallback)                          | 30m | t003       |
| t005 | Render parity                                                                             | 30m | t002, t004 |
| t006 | Simplify                                                                                  | 20m | t005       |
| t007 | Test coverage                                                                             | 30m | t005       |
| t008 | Closeout                                                                                  | 15m | t007       |

## Definition of done

A datastore plan change's audit row carries the typed from/to plan pair (visible wherever apps' `PlanFrom`/`PlanTo` already surfaces); an idempotent `SetPlan` (same plan) no longer records a misleading `UpdatePostgres`/`UpdateKeyValue` row — it records the plan-set intent or nothing, per t001's decided semantics, identically for postgres and keyvalue; the GraphQL `AuditLog` type exposes `targetName` and the dashboard audit table shows the friendly name with a raw-id fallback for pre-0038 rows (which carry `target_name = ''`); REST/GraphQL/MCP audit shapes are verified consistent, with tests asserting the verb choice, the plan pair, and the name fallback.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 20, 2026-07-15 (proposal #4) — shipped-diff mine over `4627caaf..045fbae6` findings 2–3 (`postgres/service.go:569-573` + `keyvalue/service.go:541-545` no-op `SetPlan` records `Database/KeyValueUpdated`; `core/audit.go` `RecordDatabaseEffect`/`RecordKeyValueEffect` never set `PlanFrom`/`PlanTo` unlike apps' `RecordPlanChanged` at `:260-261`) + dashboard-gap mine G4 (migration 0038's `target_name` absent from GraphQL — `audit/graphql.go`'s own comment records the omission as "a scope cut, not an oversight… until a consumer exists"; this milestone is that consumer). All four re-verified live in code after the 2026-07-15 `6f745042` pull.
- **Goal linkage:** audit-log truthfulness (`docs/ADR006-bex-api.md` § Audit log; the w4/m26 shape work) — a multi-tenant platform's audit read surface must not mislabel what happened; cross-surface consistency is the ADR006 "one core, thin adapters" rule.
- **Expected outcome:** the audit log reads identically truthful across the three datastore siblings and across REST/GraphQL/dashboard: correct verb on idempotent plan sets, typed plan detail on real ones, human-readable target names everywhere.
- **Why now:** all three defect sites are in day-old code (migration 0038 and the datastore webhook-effects refactor shipped 2026-07-15) — cheapest to fix while the surface is warm, before consumers bake in the raw-id rendering; assigned to w10 per its spare-capacity charter (m4 closed same day).
- **Render parity: included** (t005) — the change touches GraphQL and the dashboard audit surface; parity checks REST/GraphQL/MCP field consistency and compares against Render's audit-log shape captured in `docs/render-artifacts/`.
