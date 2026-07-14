# w8 · m9 — Meter managed Postgres & Key Value storage separately from compute

**Worker:** worker8 **Goal:** add a `storage_gb_seconds` meter alongside the existing compute instance-seconds metering, matching Render's per-GB-month storage billing **Status:** done

## Tasks (in order)

| id   | title                                                                                              | est  | depends_on | status     |
| ---- | -------------------------------------------------------------------------------------------------- | ---- | ---------- | ---------- |
| t001 | New `storage_gb_seconds` meter — schema/column addition alongside the existing hourly usage rollup table | 45m  | —          | — **DONE** |
| t002 | Collector: query CNPG/Valkey volume usage periodically, roll into the hourly usage pipeline (`w8/m1`'s pattern) | 1.5h | t001       | — **DONE** |
| t003 | REST/GraphQL/MCP usage-surface addition for the new meter (`w8/m2`'s pattern)                      | 1h   | t002       | — **DONE** |
| t004 | Dashboard Usage page: add a storage line item (`w8/m3`'s pattern)                                  | 45m  | t003       | — **DONE** |
| t005 | Price sheet integration: fold storage into the estimated-spend calculation (`w8/m7`)               | 45m  | t003       | — **DONE** |
| t006 | Render parity: verify the new meter's shape/semantics consistent across REST/GraphQL/MCP + dashboard | 30m  | t004, t005 | — **DONE** |
| t007 | Simplify                                                                                            | 30m  | t006       | — **DONE** |
| t008 | Test coverage                                                                                       | 1h   | t006       | — **DONE** |
| t009 | Closeout                                                                                            | 15m  | t008       | — **DONE** |

## Definition of done

A workspace's usage response includes storage-GB-seconds for its Postgres/KeyValue instances, visible on the dashboard Usage page and reflected in the price-sheet spend estimate.

## Closeout evidence

- Migration `0022_usage_storage_kind` extends both hourly and monthly row-oriented meter constraints; generic compaction preserves storage totals, covered by unit and real-Postgres acceptance tests.
- The collector integrates average `kubelet_volume_stats_used_bytes` over each closed hour for anchored CNPG and Valkey PVC patterns, persists decimal GB-seconds, uses a per-meter cursor, and retries transient failures without gaps or double-counting.
- REST, GraphQL, and MCP expose the same generic storage row and stable ordering; cross-adapter tests compare complete responses. GraphQL uses `Float` for `total` so a 1-TB-month counter cannot overflow its 32-bit `Int` scalar.
- The dashboard adds localized Storage quantity and trend views in GB-hours; the embedded price sheet adds `$0.21/GB-month` (30% below Render Postgres's `$0.30`) to estimated spend.
- Render parity is documented: Render bills provisioned Postgres capacity and has no separate Key Value storage line; bex deliberately prices actual used PVC bytes for both managed datastore kinds.
- Simplify review consolidated ordering in the core summary and contiguous retry behavior in one datastore catch-up helper. Verification passed backend `go build ./...`, `go test ./...`, and `make lint-backend`; dashboard lint/typecheck, 928 tests, and production build.

## Source + Goal linkage

- **Source:** `.pm/w8/002.md` — named as a follow-up directly in `docs/ADR018-render-parity.md`'s usage-metering row. Promoted via `/pm-brainstorm more` 2026-07-13 (fourth pass).
- **Goal linkage:** Render parity on the usage-metering surface (`GOAL.md` #5).
- **Expected outcome:** storage costs are visible and estimable, matching Render's per-GB-month billing model.
- **Why now:** scope hasn't grown since filing; all prerequisite pipeline pieces (`m1`, `m2`, `m3`, `m5`, `m7`) are done — a clean extension of shipped machinery. Render parity included — the new meter must be consistent across REST/GraphQL/MCP/dashboard.
