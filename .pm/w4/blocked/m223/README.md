# w4 · m223 — Make first Postgres availability include vector

**Worker:** worker4 **Goal:** the first Available status on a newly provisioned database means its supported vector migration can run immediately. **Status:** blocked — t001–t005 done 2026-10-17; t006 (fresh Free PostgreSQL 18 replay at 1440/390 on a deployed build, then cleanup) open

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Gate first readiness on managed vector initialization — **DONE** | 40m | — |
| t002 | Audit the provisioning latch and lifecycle/status consumers — **DONE** | 35m | w4/m223/t001 |
| t003 | Render parity — **DONE** | 20m | w4/m223/t002 |
| t004 | Simplify — **DONE** | 15m | w4/m223/t003 |
| t005 | Test coverage — **DONE** | 45m | w4/m223/t003 |
| t006 | Closeout | 20m | w4/m223/t004, w4/m223/t005 |

Six tasks, 175m total (2h55m). Major functional readiness failure. Implemented 2026-10-17; awaiting the deployed-build replay (t006).

## Definition of done

Replay [the two fresh journeys](finding.md) using owned Free/private PostgreSQL 18 databases with default physical names, first at 1440×1000 and then a fresh 390×844 document. Open SQL console while Creating, prepare the recorded catalog SELECT, and run the recorded ordinary vector migration immediately when Available first appears.

- Before managed vector initialization completes, the new database remains Creating rather than Available. Its first Available observation has vector_installed=true, CREATE EXTENSION IF NOT EXISTS vector succeeds through the ordinary native write confirmation, and the recorded vector(3)/distance SELECT returns [1,2,3] and 1. An additional settling delay is unnecessary.
- UI and the independent GraphQL database status agree on that initial transition. Initial pending and initial extension-application failure have truthful existing lifecycle states; a named initial failure is actionable rather than Available or an unexplained permanent Creating state.
- Preserve the positive controls: the same migration and distance query still work after settling. The a867 pg_trgm enable/use/drop sequence retains its three ordinary table rows and leaves vector/pg_stat_statements installed.
- Controller/adapter regressions establish the source-traced boundaries in t002: current-generation child success; pending/stale/failed child; a never-served suspend/resume; an already-provisioned database with a backfill problem; and the existing grandfathered reserved-name treatment. These are required implementation checks, not cases claimed live-tested by a868.
- Both new databases and their exact CNPG Database/Cluster/pod/PVC objects are removed, seven baseline resource-ID sets match, and only the run's QA session is revoked.

## Source + Goal linkage

- **Source:** continuous functional qa-find-bugs w4, a868, 2026-10-17 UTC; helper login consumed muse.env privately. [Finding and fix research](finding.md), [complete native/API receipts, probe code, condition projections and cleanup](evidence.json).
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md) reliable usable hosting, [ADR009](../../../../docs/ADR009-postgresql-management.md) managed application-database extensions and [ADR018](../../../../docs/ADR018-render-parity.md) truthful compatible status.
- **Expected outcome:** a tenant can use first availability as the boundary for an ordinary vector bootstrap, without receiving an initial migration error while installation is still running.
- **Why now:** the vector installer from w8/m84 is deployed and succeeds, but first Core Ready is published8–18 seconds before its applied verdict in these two fixtures. A migration inside that window fails at both breakpoints.
- **Lineage and ownership:** [w8/m84](../../../w8/blocked/m84/README.md) owns extension installation and its still-open hosted existing-parent backfill replay. That installer and capability verdict work here. This follow-up owns the distinct first Core Ready/Available consumer and its initial-provisioning latch; it neither reimplements installation nor closes m84's backfill gate. [Detailed DoD gap analysis](finding.md#deduplication-and-original-guarantees).
- **Render parity included:** the operator's status is consumed by REST, GraphQL, MCP and the dashboard. Render documents vector support and its CREATE EXTENSION workflow; authenticated Render first-Available timing was not tested. Keep Bex's documented preenabled-vector policy explicit.

## Scope

The correction belongs to new PostgreSQL readiness and its reliable first-provisioning state. Existing databases keep serving when later extension reconciliation/backfill is pending or fails. Keep the shared Key Value readiness helper, ordinary SQL connection role and SQLSTATE handling, supported version/schema policy, existing restore/upgrade behavior, and published lifecycle vocabulary compatible.

Only two fresh default-identity Free PostgreSQL 18 fixtures were live exercised. Custom names, other majors/plans, restore/import, initial suspension, external CLI/MCP execution and hosted pre-fix backfill remain source/test audit work. The implementation must preserve those contracts without claiming a868 verified them.

## Implementation record (2026-10-17)

- **t001:** in `reconcileDatabaseReadiness`, `firstReadyHeldForExtensions` keeps a database whose `status.provisioned` is false out of its first Ready until `ExtensionsReady` is true: the healthy cluster then stays `Provisioning` with Ready reason `InitializingExtensions` (bex-api `creating`) and the existing 10s fallback (the CNPG Database child is watched, so the verdict reconciles at once). An initial `ApplyFailed` or non-transient `ApplyError` sets phase `Failed` with the new Ready reason `ExtensionsFailed` (bex-api `unavailable`, a fixed sentence, statusReasonCode translated by the dashboard; CNPG's raw text is not published) and retries every 30s or sooner for exports/disk. A conflict, timeout or busy API server on the child write now reads `Pending` rather than `ApplyError`. Once provisioned, the child is optional again; the PostgreSQL-owned reserved name keeps its legacy first Ready. `settleSuspended` no longer sets `provisioned`, so a never-served database suspended and resumed still waits for vector; bex-api reads that resume moment as `creating`. The post-upgrade base backup now starts on the first healthy pass, before any hold (which still records the new version).
- **t002 audit:** the one application-database reconcile entry and both former `Provisioned=true` sites were reviewed. Restore/import creates an unprovisioned database, so it is gated, and the child applies once the restored database exists. Ordinary restart and major upgrade act on provisioned databases and are ungated; generated/custom names already flow through the child. Key Value has its own controller and latch (`CredentialRevision`), and the shared `DatastoreReadyStatus` is unchanged. `provisioned`'s only consumer is bex-api's Postgres status. Source-only: every database healthy-reconciled since w4/m137 (2026-09-26) carries `provisioned=true`, so the existing estate is not held; production was not queried for stragglers.
- **t003:** ADR009 (pgvector section) and ADR018 (Postgres row) record the first-Available rule; Render's first-Available timing was not measured.
- **t004:** three /simplify passes (pure gate check reusing the Provisioning tail, requeue honoring exports, transient write errors as pending, backup ordering, reason constants, helpers extracted for gocyclo).
- **t005:** operator `TestFirstReadyWaitsForPendingExtensions` (pending child; verdict for another generation), `TestInitialExtensionFailureIsNamedAndRetried`, `TestServedDatabaseKeepsServingThroughALaterExtensionFailure`, `TestReservedDatabaseNameKeepsItsLegacyFirstReady`, `TestSuspendingANeverServedDatabaseKeepsItsFirstReadyPending`, `TestRefusedChildWriteOnAHealthyNewDatabase` (admission refusal → ExtensionsFailed; conflict → still creating) and `TestPostUpgradeBackupStartsWhileFirstReadyIsHeld`; the pending/failure/suspend cases fail against the old controller. Upgrade, restart, instance and convergence fixtures now model CNPG applying the child (or an established `provisioned` database). Backend `status_test.go` covers `creating` while installing, the never-served resume moment and the `ExtensionsFailed` sentence/code; the dashboard reason-vocabulary drift test covers the new translation. Operator `make test` and `make lint` (all four modules) pass; the backend real-DB suite runs in the ship gate. The supported-CNPG SQL assertion (`database_extensions_cnpg_test.go`) still needs `BEX_CNPG_KUBECONFIG` and is left to the t006 live replay.
