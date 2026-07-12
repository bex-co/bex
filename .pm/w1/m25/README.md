# w1 · m25 — Managed Postgres observability

**Worker:** worker1 **Goal:** give managed-Postgres users the same live-query/top-queries/sizes/table-scans/parameter-override introspection Render offers. **Status:** todo

## Tasks (in order)

| id   | title                                                                                     | est | depends_on         |
| ---- | ------------------------------------------------------------------------------------------ | --- | ------------------- |
| t001 | `processes` endpoint: live `pg_stat_activity` snapshot (REST/GraphQL/MCP)                   | 45m | —                    |
| t002 | `top-queries` endpoint: `pg_stat_statements`-backed; verify/enable the extension on CNPG cluster spec | 1h  | —                    |
| t003 | `sizes` endpoint: database/table sizes via `pg_database_size`/`pg_total_relation_size`       | 30m | —                    |
| t004 | `table-scans` endpoint: seq-scan vs index-scan stats via `pg_stat_user_tables`               | 30m | —                    |
| t005 | `parameter-overrides`: read (and where CNPG allows, write) non-default `postgresql.conf` params | 45m | —                    |
| t006 | Dashboard Postgres-detail Insights panel surfacing processes/top-queries/sizes/table-scans   | 1h  | t001,t002,t003,t004 |
| t007 | Render parity — verify REST/GraphQL/MCP/UI field/shape consistency vs render.com             | 30m | t005,t006            |
| t008 | Simplify — `/simplify` over the code this milestone changed                                  | 15m | t007                 |
| t009 | Test coverage — meaningful tests for each endpoint's real-data behavior + failure modes       | 30m | t007                 |
| t010 | Closeout — verify DoD met, then move the milestone to `done/`                                | 10m | t008,t009             |

## Definition of done

Each of the five Render endpoints (processes, top-queries, sizes, table-scans, parameter-overrides) has a REST/GraphQL/MCP equivalent returning live data from a real CNPG cluster; the dashboard surfaces at least processes + sizes.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more` 2026-07-12, `docs/ADR018-render-parity.md` "Postgres observability" row (extends `w1/m17`).
- **Goal linkage:** pillar 1 (Render-compatible REST/GraphQL/MCP).
- **Expected outcome:** managed-Postgres users get the same live-query/top-queries/sizes/table-scans introspection Render offers, closing the last open Postgres parity gap.
- **Why now:** `pg_stat_statements` needs enabling in the CNPG cluster spec regardless; no other open milestone owns this row.
- **Render parity closing task:** included — new REST/GraphQL/MCP/UI surface.
