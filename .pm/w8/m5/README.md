# w8 · m5 — Meter managed Postgres & Key Value instance-seconds

**Worker:** worker8 **Goal:** Extend the usage-metering pipeline beyond `App` services to managed `Database` (Postgres) and `KeyValue` instances, so their billable compute shows up as `instance_seconds` rows across REST/GraphQL/MCP and the dashboard, keyed by the resource's plan. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                                              | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Design: meter-applicability matrix (instance_seconds yes; egress_bytes/build_seconds N/A for DB/KV — document why) in `docs/ADR023-usage-metering.md`                      | 30m | —          |
| t002 | Rollup: list `Database`+`KeyValue` CRs (k8s client, tenant-labeled) alongside Apps in `catchUp`/`rollup`; query instance_seconds via the existing cAdvisor/Prometheus path keyed to CNPG/Valkey pod names; upsert `HourlyRow{ServiceID: db/kv id, Tier: plan}` | 45m | t001       |
| t003 | Store/`MonthToDate` layer: confirm the generic `service_id` keying already surfaces the new rows in `services[]`; add a resource-kind field if needed to disambiguate from App rows | 35m | t002       |
| t004 | Adapter-consistency tests: REST/GraphQL/MCP all return the new rows identically (thin-adapter design — no adapter code expected to change)                          | 30m | t003       |
| t005 | Dashboard: Compute section labels resource kind next to `serviceId` so Postgres/Key Value rows aren't read as web services                                          | 35m | t004       |
| t006 | Render parity: compare against Render's billing usage view for Postgres/Key Value compute; flag any field/semantic drift                                            | 30m | t005       |
| t007 | Simplify — `/simplify` over the code this milestone changed                                                                                                          | 20m | t006       |
| t008 | Test coverage — rollup + acceptance tests for Database/KeyValue windows                                                                                              | 35m | t006       |
| t009 | Closeout — DoD met → move milestone to `done/`                                                                                                                       | 10m | t008       |

## Definition of done

A workspace with a managed Postgres and/or Key Value instance sees `instance_seconds` rows for them in `GET /v1/usage`, the GraphQL `usage` query, MCP `get_usage`, and the dashboard Usage page — same totals across all four, tier = the resource's plan.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones for w8` 2026-07-10 — gap analysis of `usage.Service.catchUp`/`rollup` (`lego/backend/internal/usage/service.go:180`/`:207`) iterating only `s.Store.ListApps`, never `Database`/`KeyValue` CRs (which aren't rows in the control-plane `apps` table at all — they're k8s CRs tenant-labeled via `core.LabelTenant`/`core.LabelWorkspace`, confirmed in `internal/postgres/service.go`).
- **Goal linkage:** `GOAL.md` item 5 (usage metering) — closes the App-only scope gap left by m1; Render meters Postgres and Key Value compute hourly by plan just like web services, so bex's meter should too.
- **Expected outcome:** managed Postgres/Key Value compute is metered and visible across REST/GraphQL/MCP/UI, same as App services — no more silent metering blind spot for billable-shaped resources.
- **Why now:** every day this gap persists is under-metered history that m4's retention will soon start compacting — cheaper to close the gap before compaction logic has to account for a schema change.
- **Render parity: included** (t006) — this changes what surfaces across REST/GraphQL/MCP/UI (new `services[]` rows, a new label in the dashboard's Compute section).
