# w8 · m92 — Make PostGIS usable in managed Postgres

**Worker:** worker8 **Goal:** ordinary managed PostgreSQL app connections can complete PostGIS migrations and geospatial queries. **Status:** blocked — needs the user's managed Postgres extension decision ([decision](design.md)); no task started

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply compatible pinned PostGIS operand images](t001.md) | 70m | — |
| t002 | [Reconcile PostGIS in the existing application Database child](t002.md) | 45m | t001 |
| t003 | [Audit shared image, extension and lifecycle callers](t003.md) | 35m | t002 |
| t004 | [Verify Render parity across database consumers](t004.md) | 20m | t003 |
| t005 | [Simplify the changed supply and reconciliation code](t005.md) | 15m | t004 |
| t006 | [Test actual PostGIS supply activation and existing data](t006.md) | 45m | t004 |
| t007 | [Accept fresh native migrations and close the milestone](t007.md) | 20m | t005, t006 |

**Total:**7 tasks/250m. [Finding](finding.md) · [complete evidence](evidence/cli-sweep403/README.md).

## Definition of done

Reviewed compatible PostGIS operand supply plus retained CNPG application-Database reconciliation makes the original migration and real spatial data/distance/GiST operations work under normal app identities. Maintained real CNPG/image/SQL checks cover supported major/architecture/patch boundaries, default/custom names and a controlled owned pre-fix existing-parent transition without data/extension loss. Vector, pg_stat_statements, hstore, export/restore compatibility, retention, suspension and optional extension-status behavior remain correct. After publication both native clients' fresh permitted Free migrations and spatial queries pass; exact cleanup preserves baselines/quota and removes own connection material. Distinguish integration from hosted acceptance and retain unverified limits. Code alone does not close the milestone.

## Source + Goal linkage

- **Source:** Authorized functional CLI sweep403,2026-10-17: both unchanged native psql clients fail the original PostGIS migration on one fresh Bex-created Free PostgreSQL18.6 database; independent SQL/hstore/point controls pass and four catalogs remain identical.
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md), [managed data ADR009](../../../../docs/ADR009-postgresql-management.md) and [ADR018](../../../../docs/ADR018-render-parity.md): supported geospatial migrations should work through ordinary managed database connections.
- **Expected outcome:** Applications can store/query geospatial data without installing server libraries themselves or changing authority/CLI requests.
- **Why now:** Actual catalog absence and two matching native errors establish this separate gap while vector and ordinary extension/SQL behavior already work. No PostGIS repair is scheduled in the searched board/history.
- **Render parity:** Included because the shared managed database image/extension change reaches REST/GraphQL/MCP/Blueprint/dashboard/native consumers. Core PostGIS activation only; no general extension catalog/API product.
