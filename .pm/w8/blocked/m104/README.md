# w8 · m104 — Supply pg_partman for ordinary partition migrations

**Worker:** worker8 **Goal:** app owners can install and use the documented partition-management extension. **Status:** blocked — needs the user's managed Postgres extension decision ([decision](../m92/design.md)); no task started

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply pinned SQL-only pg_partman in compatible serving operands](t001.md) | 70m | — |
| t002 | [Audit shared image selection and existing database lifecycles](t002.md) | 30m | t001 |
| t003 | [Verify documented Render behavior across database consumers](t003.md) | 20m | t002 |
| t004 | [Simplify the changed extension supply](t004.md) | 15m | t003 |
| t005 | [Test real extension installation partitions and retained data](t005.md) | 45m | t003 |
| t006 | [Accept original native migrations and close the milestone](t006.md) | 20m | t004, t005 |

**Total:**6 tasks/200m. [Finding](finding.md) · [complete evidence](evidence/cli-sweep428/README.md).

## Definition of done

Compatible reviewed immutable SQL-only pg_partman supply for PostgreSQL14..18 permits normal app-owner on-demand installation and real partition generation, routing and manual maintenance with the background worker disabled. Maintained real CNPG/image/SQL proof covers supported architecture/base/patch choices, default/custom identities/schemas, controlled existing-parent transition and retained data/extension configuration. PG13 and existing vector/pg_stat_statements, restore/upgrade/reconcile/suspend behavior remain correct. After publication both clients' unchanged permitted Free migrations and actual partition operations pass, exact cleanup preserves baselines/quota and removes own material, and unexercised limits remain explicit. Implementation alone does not close this milestone.

## Source + Goal linkage

- **Source:** Authorized ordinary functional CLI sweep428,2026-10-17: both native SQL clients fail pg_partman installation on a fresh owned Free PostgreSQL18.6, with matching catalogs and healthy extension/partition controls. A separate ordinary-owner localPG17 SQL-only reference works.
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md), [managed PostgreSQL ADR009](../../../../docs/ADR009-postgresql-management.md), [Render parity ADR018](../../../../docs/ADR018-render-parity.md): supported app migrations should work through normal managed database connections.
- **Expected outcome:** Partition-management migrations and maintenance run without server-file workarounds or changed app authority.
- **Why now:** Two original failures and actual missing catalog entry establish a distinct supply gap; no existing pg_partman work was found. Coordinate the shared image rollout with [m92](../m92/README.md); The m92 plan does not establish pg_partman supply in its future candidate images.
- **Render parity:** Included because shared database image supply reaches native/REST/GraphQL/MCP/Blueprint/dashboard consumers. No extension-management product or automatic background-worker service is introduced.
