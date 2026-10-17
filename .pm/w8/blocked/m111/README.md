# w8 · m111 — Make documented text similarity usable in managed Postgres

**Worker:** worker8 **Goal:** ordinary applications can use documented PostgreSQL13–15 text-similarity functions and operators. **Status:** blocked — needs the user's managed Postgres extension decision ([decision](../m92/design.md)); no task started

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply pinned pg_similarity for the documented PostgreSQL majors](t001.md) | 40m | — |
| t002 | [Make text matching usable through normal application connections](t002.md) | 35m | t001 |
| t003 | [Audit the shared supplier and database lifecycles](t003.md) | 35m | t002 |
| t004 | [Verify Render parity across database authoring surfaces](t004.md) | 20m | t003 |
| t005 | [Simplify managed text-matching delivery](t005.md) | 15m | t004 |
| t006 | [Test actual similarity functions, operators and lifecycle neighbors](t006.md) | 55m | t004 |
| t007 | [Accept both native migrations and close milestone](t007.md) | 20m | t005, t006 |

**Total:**7 tasks/220m.

## Definition of done

On the documented managed PostgreSQL13–15 operands, both unchanged native clients' original CREATE EXTENSION IF NOT EXISTS pg_similarity commands exit0 and normal applications execute real similarity functions/operators with independently expected results. Reviewed immutable CNPG-compatible packages and managed activation preserve existing vector, pg_stat_statements, data, readiness and shared export/backup/client behavior. Preserve the13–15 contract boundary and explicitly verify/document existing-parent, restore/import/export/upgrade and major-transition constraints. Actual image/CNPG/SQL regressions fail before/pass after; fresh hosted acceptance, required checks and exact cleanup pass. Record actual version/architecture/lifecycle limits. Local compilation or catalog presence alone does not meet this definition.

## Source + Goal linkage

- **Source:** [Functional native CLI sweep450 finding](finding.md), [complete evidence](evidence/cli-sweep450/README.md): both migrations exit1 with missing package/control file, both ordinary SQL controls and a real source-compiled local function/operator reference pass.
- **Goal linkage:** [ADR008 usable hosting](../../../../docs/ADR008-vision.md), [ADR009 managed PostgreSQL](../../../../docs/ADR009-postgresql-management.md), [ADR018 Render compatibility](../../../../docs/ADR018-render-parity.md).
- **Expected outcome:** Applications can use documented text-similarity migrations and SQL through their existing database and unchanged clients.
- **Why now:** Both native consumers connect and execute ordinary SQL, but the documented module is absent. Package delivery, normal usability and lifecycle/major compatibility require coordinated work.
- **Render parity:** Included because the database capability is tenant-visible across REST/GraphQL/MCP/dashboard/Blueprint authoring. Preserve fields/errors/selectors and coordinate the existing shared supplier work from m92/m104/m109/m110; those separate modules do not provide this consumer.
