# w3 · m28 — Managed Postgres logs across API and dashboard

**Worker:** worker3 **Goal:** A managed Postgres has a truthful, authorized Logs
surface like Render's: one datastore-scoped core query backed by Loki/CNPG pod
logs, exposed consistently through REST/GraphQL/MCP and a dedicated dashboard
tab. **Status:** DONE 2026-07-15

## Tasks (in order)

| id   | title                                                          | est | status |
| ---- | -------------------------------------------------------------- | --- | ------ |
| t001 | Pin Render's Postgres-log viewer contract and bex attribution  | 30m | done   |
| t002 | Core datastore-log query + CNPG pod-log source + authorization | 60m | done   |
| t003 | REST/GraphQL/MCP adapters for Postgres logs                    | 45m | done   |
| t004 | Dashboard Postgres Logs tab and viewer                         | 45m | done   |
| t005 | Render parity ledger update                                    | 30m | done   |
| t006 | Simplify (no regressions found)                                | 30m | done   |
| t007 | Test coverage (6 unit tests in postgres/logs_test.go)          | 45m | done   |
| t008 | Closeout                                                       | 15m | done   |

## What shipped

- **`lego/backend/internal/core/base.go`** — `PodLogSource` type, `PodLabelCNPGCluster` /
  `CNPGPostgresContainer` constants, `DatabasePods` method.
- **`lego/backend/internal/logs/service.go`** — `PodLogSource` changed to a type alias of
  `core.PodLogSource` (100% backward compatible).
- **`lego/backend/internal/postgres/service.go`** — `PodLogs core.PodLogSource` field.
- **`lego/backend/internal/postgres/logs.go`** (NEW) — `DatabaseLogQuery`, `DatabaseLogEntry`,
  `QueryDatabaseLogs`, `readDBPodLogs`, `parseDBLogLine`.
- **`lego/backend/internal/postgres/rest.go`** — `GET {base}/{id}/logs` + `parsePGTimeWindow`.
- **`lego/backend/internal/postgres/graphql.go`** — `databaseLogs` query + `databaseLogGQLType`.
- **`lego/backend/internal/postgres/mcp.go`** — `get_postgres_logs` tool + `registerLogsMCP`.
- **`lego/backend/internal/api/server.go`** — `PodLogs: d.PodLogs` wired into postgres service.
- **`lego/backend/internal/postgres/logs_test.go`** (NEW) — 6 unit tests.
- **`dashboard/src/features/databases/api/databases.graphql`** — `DatabaseLogs` query.
- **`dashboard/src/features/databases/api/operations.ts`** — `DatabaseLogEntry`, `DatabaseLogsDocument`.
- **`dashboard/src/features/databases/hooks/use-database-logs.ts`** (NEW).
- **`dashboard/src/features/databases/components/database-logs-panel.tsx`** (NEW).
- **`dashboard/src/routes/databases.$databaseId.tsx`** — `<DatabaseLogsPanel>` wired in.
- **`dashboard/src/features/databases/locales/{en,zh}.ts`** — 5 i18n keys each.
- **`docs/ADR018-render-parity.md`** — row updated ✖→◐, gap backlog marked done.

## Honest degraded ◐

CNPG pods are not shipped to Loki — `QueryDatabaseLogs` is a direct pod-log read,
live only. Full durable history requires routing CNPG logs to Loki (platform-ops task,
out of m28 scope). The dashboard panel and parity ledger both call this out explicitly.

## Definition of done

A caller authorized to view one managed Postgres can query its timestamped log
history through REST, GraphQL, MCP, and a dedicated dashboard Logs tab; another
workspace's database cannot be read; Loki-backed history and an honest
configured fallback/degraded state are defined; range/search/instance behavior
matches the pinned Render contract; tests prove attribution cannot mix tenant or
database logs.

## Source + Goal linkage

- **Source:** 2026-07-15 live dashboard parity walk — Render has a dedicated
  Postgres Logs tab (`render-walk-postgres-logs.png`), while bex's consolidated
  datastore page has no database-log source or UI. See
  [the datastore walk](../../../docs/render-artifacts/dashboard-walk/datastores.md#page-by-page-verdicts).
- **Goal linkage:** Render parity + w3 observability; extends the existing logs
  architecture without routing datastore reads through the operator.
- **Expected outcome:** operators and agents can diagnose a managed database
  without cluster access, with the same tenant-safe contract on all surfaces.
- **Why now:** the page-level audit found a real cross-layer omission too large
  for a sub-hour inbox note; the Loki/query infrastructure already exists and
  is the right shared seam.
- **Render parity:** included — t005 re-walks the live Render and bex pages and
  records any deliberate differences.
