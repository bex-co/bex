# w8 · m109 — Make documented DuckDB analytics usable in managed Postgres

**Worker:** worker8 **Goal:** ordinary applications can use the documented PostgreSQL16+ DuckDB capability. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply a compatible pinned managed-Postgres DuckDB package](t001.md) | 65m | — |
| t002 | [Reconcile DuckDB preload and ordinary application usability](t002.md) | 50m | t001 |
| t003 | [Audit shared database lifecycle and other extension suppliers](t003.md) | 40m | t002 |
| t004 | [Verify Render parity across database authoring surfaces](t004.md) | 25m | t003 |
| t005 | [Simplify managed DuckDB delivery](t005.md) | 15m | t004 |
| t006 | [Test real managed analytics and missing-package regressions](t006.md) | 75m | t004 |
| t007 | [Accept fresh native analytics and close milestone](t007.md) | 20m | t005, t006 |

**Total:**7 tasks/290m.

## Definition of done

On supported managed PostgreSQL16–18, an ordinary application connection can execute the unchanged native CREATE EXTENSION IF NOT EXISTS pg_duckdb command with exit0 and use DuckDB for real own-table analytics. Both native clients return independently expected Unicode/signed grouped totals, with actual DuckDB engine-plan evidence. Selected operands contain a reviewed immutable compatible package/dependency closure, required preload/application configuration and appropriate plan resource bounds. Existing vector/pg_stat_statements/normal SQL, data/identity, parameters, backup/export/client and lifecycle behavior are preserved. Actual image/CNPG regressions fail before/pass after; post-publication fresh hosted checks, exact cleanup and required validation pass. Local references, code/config alone and a catalog-presence check cannot close this milestone. Record actual architecture/version/topology/external-feature limits.

## Source + Goal linkage

- **Source:** [Functional native CLI sweep444 finding](finding.md) and [complete evidence](evidence/cli-sweep444/README.md): both native enable commands fail1, both ordinary SQL controls pass and four catalogs omit the package; a real local DuckDB reference succeeds.
- **Goal linkage:** [ADR008 AI-native usable hosting](../../../docs/ADR008-vision.md), [ADR009 managed PostgreSQL](../../../docs/ADR009-postgresql-management.md), [ADR018 Render compatibility](../../../docs/ADR018-render-parity.md).
- **Expected outcome:** Applications can use the documented PostgreSQL16+ DuckDB analytics capability through the existing managed database and unchanged clients.
- **Why now:** The standard operand/configuration has no supplier for a documented extension. Packaging alone would leave preload and application use incomplete; explicit lifecycle and shared-export checks prevent the repair from disrupting existing databases.
- **Render parity:** Included because this changes the tenant-visible database capability across REST/GraphQL/MCP/dashboard/Blueprint authoring; preserve existing fields and document any preenabled Bex policy. Coordinate with m92/m104 supplier work while retaining their distinct consumers.
