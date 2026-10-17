# w4 · m223 — Make first Postgres availability include vector

**Worker:** worker4 **Goal:** the first Available status on a newly provisioned database means its supported vector migration can run immediately. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Gate first readiness on managed vector initialization | 40m | — |
| t002 | Audit the provisioning latch and lifecycle/status consumers | 35m | w4/m223/t001 |
| t003 | Render parity | 20m | w4/m223/t002 |
| t004 | Simplify | 15m | w4/m223/t003 |
| t005 | Test coverage | 45m | w4/m223/t003 |
| t006 | Closeout | 20m | w4/m223/t004, w4/m223/t005 |

Six tasks, 175m total (2h55m). Major functional readiness failure. Filing only; no product fix is included.

## Definition of done

Replay [the two fresh journeys](finding.md) using owned Free/private PostgreSQL 18 databases with default physical names, first at 1440×1000 and then a fresh 390×844 document. Open SQL console while Creating, prepare the recorded catalog SELECT, and run the recorded ordinary vector migration immediately when Available first appears.

- Before managed vector initialization completes, the new database remains Creating rather than Available. Its first Available observation has vector_installed=true, CREATE EXTENSION IF NOT EXISTS vector succeeds through the ordinary native write confirmation, and the recorded vector(3)/distance SELECT returns [1,2,3] and 1. An additional settling delay is unnecessary.
- UI and the independent GraphQL database status agree on that initial transition. Initial pending and initial extension-application failure have truthful existing lifecycle states; a named initial failure is actionable rather than Available or an unexplained permanent Creating state.
- Preserve the positive controls: the same migration and distance query still work after settling. The a867 pg_trgm enable/use/drop sequence retains its three ordinary table rows and leaves vector/pg_stat_statements installed.
- Controller/adapter regressions establish the source-traced boundaries in t002: current-generation child success; pending/stale/failed child; a never-served suspend/resume; an already-provisioned database with a backfill problem; and the existing grandfathered reserved-name treatment. These are required implementation checks, not cases claimed live-tested by a868.
- Both new databases and their exact CNPG Database/Cluster/pod/PVC objects are removed, seven baseline resource-ID sets match, and only the run's QA session is revoked.

## Source + Goal linkage

- **Source:** continuous functional qa-find-bugs w4, a868, 2026-10-17 UTC; helper login consumed muse.env privately. [Finding and fix research](finding.md), [complete native/API receipts, probe code, condition projections and cleanup](evidence.json).
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) reliable usable hosting, [ADR009](../../../docs/ADR009-postgresql-management.md) managed application-database extensions and [ADR018](../../../docs/ADR018-render-parity.md) truthful compatible status.
- **Expected outcome:** a tenant can use first availability as the boundary for an ordinary vector bootstrap, without receiving an initial migration error while installation is still running.
- **Why now:** the vector installer from w8/m84 is deployed and succeeds, but first Core Ready is published8–18 seconds before its applied verdict in these two fixtures. A migration inside that window fails at both breakpoints.
- **Lineage and ownership:** [w8/m84](../../w8/blocked/m84/README.md) owns extension installation and its still-open hosted existing-parent backfill replay. That installer and capability verdict work here. This follow-up owns the distinct first Core Ready/Available consumer and its initial-provisioning latch; it neither reimplements installation nor closes m84's backfill gate. [Detailed DoD gap analysis](finding.md#deduplication-and-original-guarantees).
- **Render parity included:** the operator's status is consumed by REST, GraphQL, MCP and the dashboard. Render documents vector support and its CREATE EXTENSION workflow; authenticated Render first-Available timing was not tested. Keep Bex's documented preenabled-vector policy explicit.

## Scope

The correction belongs to new PostgreSQL readiness and its reliable first-provisioning state. Existing databases keep serving when later extension reconciliation/backfill is pending or fails. Keep the shared Key Value readiness helper, ordinary SQL connection role and SQLSTATE handling, supported version/schema policy, existing restore/upgrade behavior, and published lifecycle vocabulary compatible.

Only two fresh default-identity Free PostgreSQL 18 fixtures were live exercised. Custom names, other majors/plans, restore/import, initial suspension, external CLI/MCP execution and hosted pre-fix backfill remain source/test audit work. The implementation must preserve those contracts without claiming a868 verified them.
