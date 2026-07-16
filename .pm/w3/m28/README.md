# w3 · m28 — Managed Postgres logs across API and dashboard

**Worker:** worker3 **Goal:** A managed Postgres has a truthful, authorized Logs
surface like Render's: one datastore-scoped core query backed by Loki/CNPG pod
logs, exposed consistently through REST/GraphQL/MCP and a dedicated dashboard
tab. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Pin Render's Postgres-log viewer contract and bex attribution rules | 30m | — |
| t002 | Core datastore-log query + Loki/CNPG source and authorization | 60m | t001 |
| t003 | REST/GraphQL/MCP adapters for Postgres logs | 45m | t002 |
| t004 | Dashboard Postgres Logs tab and viewer | 45m | t003 |
| t005 | Render parity | 30m | t004 |
| t006 | Simplify | 30m | t005 |
| t007 | Test coverage | 45m | t005 |
| t008 | Closeout | 15m | t007 |

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
