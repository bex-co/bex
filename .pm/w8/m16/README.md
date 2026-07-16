# w8 · m16 — Managed-datastore polish chores: create ipAllowList parity + pg_stat_statements backfill + KV version assessment

**Worker:** worker8 **Goal:** Three datastore polish items land as one chores round (the w7/m37 pattern): a GraphQL/MCP-created Postgres carries its create-time IP allow-list like its keyvalue sibling, pre-m25 databases gain the query insights they currently degrade out of forever, and the KV version-upgrade parity question gets an evidenced answer. **Status:** todo

## Tasks (in order)

| id   | title                                                                                     | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | `ipAllowList` on GraphQL `createDatabase` + MCP `create_postgres`, mirroring keyvalue      | 40m | —          |
| t002 | Backfill `pg_stat_statements` onto pre-m25 CNPG clusters via the parameter-override seam   | 45m | —          |
| t003 | KV version-change parity assessment (either-outcome acceptance)                            | 30m | —          |
| t008 | Accept Render's inline `parameterOverrides` on PATCH /v1/postgres/{id}                     | 30m | —          |
| t004 | Render parity                                                                               | 20m | t001, t008 |
| t005 | Simplify                                                                                    | 15m | t004       |
| t006 | Test coverage                                                                               | 30m | t004       |
| t007 | Closeout                                                                                    | 15m | t006       |

## Definition of done

A Postgres created via GraphQL or MCP with an `ipAllowList` carries it (cross-surface test) and the stale `postgres/mcp.go:42-43` comment is fixed; a database provisioned before w2/m25 returns real Top Queries rows after the operator reconcile (verified on a dev-N or prod pre-m25 cluster); the KV version question has a recorded answer with spec/docs evidence (mirror-milestone filed if yes, parity-by-absence recorded if no); a Render-shaped `PATCH /v1/postgres/{id}` with inline `parameterOverrides` applies them via the existing override seam (no silent ignore) and `docs/ADR018-render-parity.md:88`'s note is updated; source notes `w8/003` + `w8/006` closed.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 14, 2026-07-15 — consistency miner (verified: `CreatePostgresRequest.IPAllowList` decoded at `lego/backend/internal/postgres/rest.go:135-144`, absent from GraphQL `createDatabase` `graphql.go:451-490` and MCP `create_postgres` `mcp.go:44-53`, while keyvalue accepts it on all three surfaces — `keyvalue/rest.go:212-219`, `mcp.go:60`, `graphql.go:149,179`) + open notes `.pm/w8/006.md` (`postgres/insights.go:257` — pre-m25 clusters degrade to an empty Top Queries list forever; the w2/m25 parameter-override reconcile is the seam) and `.pm/w8/003.md` (KV `spec.version` is create-time-only, `keyvalue_types.go:36`; assess whether Render supports version changes at all before mirroring w8/m12).
- **Goal linkage:** managed-datastore parity (docs/ADR009-postgresql-management.md, docs/ADR021-keyvalue-management.md) — w8's charter.
- **Expected outcome:** the datastore create surface is symmetric across REST/GraphQL/MCP; old databases get the insights feature they already pay reconcile cost for; two aging inbox notes cleared.
- **Why now:** keyvalue just got the same create-parity treatment, so the postgres asymmetry is fresh drift; grouping clears the notes while the datastore code is warm. w8 was honestly dry in round 13 — this is its real backlog.
- **Render parity:** included — t001 is a GraphQL/MCP surface change (Render's own POST /postgres accepts `ipAllowList`).
- **t008 source:** `/pm-brainstorm` round 19, 2026-07-15 — dashboard-gap mine: `PostgresPatch` (`postgres/service.go:579`) silently drops Render's documented inline `parameterOverrides` (ADR018:88 documents the subresource preference; the silent-ignore half is the defect).
- **Coordinate with — never duplicate:** `w4/m24` (descriptions on the same lists), `w9/m38` (owns the postgres bad-request error-body sweep — out of scope here).
