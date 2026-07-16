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
| t007 | Cross-adapter, attribution, authz, and fallback test coverage | 45m | done   |
| t008 | Closeout                                                       | 15m | done   |

## What shipped

- Render's generic `resource=dpg-…` contract now dispatches through the logs
  service's `AuthorizeDatabase(can_view_logs)` path across REST, GraphQL, and MCP.
- The dedicated Postgres REST, GraphQL, and MCP compatibility adapters remain;
  typed ids delegate to that same production core, while legacy name-shaped CRs
  retain the direct CNPG pod fallback.
- Alloy ingests only operator-marked tenant Database pods, retains the immutable
  CNPG cluster id as the Loki `database` label, and records `type=postgres`; the
  exact CNPG pod/container fallback remains available when Loki is not configured.
- The Database detail route has a directly linkable `?tab=logs` view with time
  range, text search, and instance filters plus distinct empty, unauthorized,
  unavailable, and generic error states.
- Tests cover REST/GraphQL/MCP parity, exact Loki and pod attribution,
  cross-workspace refusal before source access, label discovery, delegation,
  fallback behavior, operator labels, and GitOps pipeline invariants.
- ADR010, ADR018, the dashboard walk, and the captured Render contract document
  the durable path and the deliberately smaller database filter vocabulary.

## Durable history and fallback

With `BEX_LOKI_URL`, managed Postgres history survives pod restarts and is scoped
by `{namespace,database=<immutable dpg-id>}`. Without Loki, the same query reads
only the matching CNPG pods' `postgres` container and reports that limitation
honestly when no live source is configured.

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
