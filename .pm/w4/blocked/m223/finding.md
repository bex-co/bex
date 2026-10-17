# Available precedes managed vector initialization

Why: first availability should be a usable boundary for an application's normal vector migration.

**Major · a868, 2026-10-17 UTC.** Researched main8f80d74efaffc23342cccad2f275051a1501396f. The relevant source files are unchanged from8f37dc877; their blob hashes and exact requests/responses are in [evidence.json](evidence.json). Serving operator Git SHA was not independently mapped. No product fix was implemented.

## Two fresh reproductions

Create a private Free PostgreSQL 18 through New Postgres, leaving physical database/user names at their generated defaults. While Creating, open SQL console and prepare:

    SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'vector') AS vector_installed,
           current_timestamp::text AS observed_at;

Observe the natural Available transition and immediately run that SELECT, then the ordinary confirmed write:

    CREATE EXTENSION IF NOT EXISTS vector;

No response interception, artificial database delay, SQL sleep, role change or duplicate-write retry was introduced. The console uses the CNPG application connection, as confirmed by its source and the mobile PostgreSQL error record.

| Fixture | First UI Available | Catalog observation | Immediate migration | Independent API |
| --- | --- | --- | --- | --- |
| Desktop 1440×1000, dpg-db9ip44rd81c73829ga0 | 08:08:31.992Z | false at database time08:08:33.282540Z | bad request (SQLSTATE 42501); native receipt08:08:37.076Z | database.status=available |
| Fresh 390×844 document, dpg-db9irjcrd81c73829gd0 | 08:13:56.060Z | false at database time08:13:57.595209Z | bad request (SQLSTATE 23505); native receipt08:14:03.346Z | database.status=available |

Names were qa-20261017-a868-readiness and qa-20261017-a868-mobile. The mobile page was hard-loaded while Creating, its document width was390, and its probe explicitly recorded alreadyAvailable=false before waiting. Desktop's active observation chunk waited17.436s for Available; an earlier30s chunk had remained Creating. Each migration is one native ExecuteDatabaseQuery with allowWrites=true after the normal Run statement confirmation.

The two SQLSTATEs are distinct, not normalized into one explanation. The mobile's filtered own-database PostgreSQL log at08:14:02.637Z identifies pg_extension_name_index, with extname vector already present. It records the generated application user and the same CREATE EXTENSION statement. That is a real catalog conflict during convergence, not a browser-only error. [PostgreSQL's code table](https://www.postgresql.org/docs/18/errcodes-appendix.html) names42501 insufficient_privilege and23505 unique_violation. No additional catalog or privilege mechanism is inferred.

## Independent conditions and settled controls

Selected exact-ID condition projections show:

| Fixture | Core Ready=True / Provisioned | ExtensionsReady=True / Applied |
| ------- | ----------------------------- | ------------------------------ |
| Desktop | 08:08:30Z                     | 08:08:48Z                      |
| Mobile  | 08:13:54Z                     | 08:14:02Z                      |

Thus Core readiness preceded the extension verdict by 18s and 8s. These timestamps corroborate the directly observed false catalog and failed migrations; timestamps alone were not used as proof.

The same databases later report vector_installed=true. The identical native migrations succeed at08:09:49.137Z and08:15:00.881Z. Each recorded vector(3)/distance query returns [1,2,3] and 1. Automatic installation is working; the failed boundary is first availability.

The separate a867 settled control also installed pg_trgm, ran similarity queries on three synthetic rows, repeated them from a fresh 390px visit, then dropped pg_trgm. All rows, including Unicode/newline text, remained, as did vector0.8.6 and pg_stat_statements. Those exact neighbor receipts are included and explicitly labeled as the earlier settled control.

The mobile failure screenshot was opened and inspected: .playwright-mcp/qa-a868-mobile-ready-migration-failure.png. It visibly combines Available, the migration and SQLSTATE 23505. It is an ignored supplement; durable ARIA, native/API responses and the selected PostgreSQL log survive in the JSON.

## Root, consumer and target

[database_controller.go:835](../../../../lego/operator/internal/controller/database_controller.go#L835) reconciles the application Database child. [database_extensions.go:111](../../../../lego/operator/internal/controller/database_extensions.go#L111) turns its current applied/observedGeneration verdict into ExtensionsReady; pending becomes Unknown/Pending and errors remain a capability condition. Its comment at:74 intentionally keeps this child optional to serving.

Later, [database_controller.go:1366–1380](../../../../lego/operator/internal/controller/database_controller.go#L1366) marks the parent Ready and Provisioned solely from healthy cluster instances and restart state. It never consults that extension verdict for initial readiness. This is the missing dependency.

The consumer is independently traced. [postgres/service.go:426](../../../../lego/backend/internal/postgres/service.go#L426) maps DBPhaseReady through [core/datastore_status.go:47](../../../../lego/backend/internal/core/datastore_status.go#L47). A current true Ready condition becomes available. PostgreSQL REST embeds the resulting PostgresView at rest.go:41–75; GraphQL reads its status at graphql.go:42; MCP's list_postgres_instances/get_postgres return the same view. Dashboard [use-database.ts:42–56](../../../../dashboard/src/features/databases/hooks/use-database.ts#L42) consumes that status and reduces its convergence polling after available. The raw GraphQL read also says available, so this is not stale browser-only status.

**Target:** for an eligible newly provisioned application database, publish its first Ready/Available only after the managed vector child is successfully applied for the current intended configuration. Keep it Creating while initial work is pending. An initial ApplyFailed/ApplyError must have a named truthful failure state and existing safe status-reason handling. Its first Available must permit the recorded migration and vector query immediately.

Scope this to initial provisioning. An already working database retains service if later optional extension/backfill work fails. Preserve the existing reserved legacy-name disposition. The current condition and lifecycle types can express pending and failure with existing creating/unavailable statuses; there is no need for a new wire status or a new installer.

**Reliable latch:** two production sites set Provisioned=true, the healthy branch at:1376 and settleSuspended at:1113. The type's comment at database_types.go:536 assumes suspension follows serving. However, [postgres/lifecycle.go:49–77](../../../../lego/backend/internal/postgres/lifecycle.go#L49) has no provisioning-state prerequisite. A predicate based on that bit must not be bypassed by suspending a never-served instance. Preserve a previous false latch through initial suspension; set it only when actual first readiness is complete. Keep a previous true latch sticky. This is a source-traced regression obligation, not an additional live suspend-before-ready finding.

## Boundaries and meaningful verification

There is one production application-database reconcile entry and two Provisioned=true sites in this controller. The shared DatastoreReadyStatus has two production consumers, Postgres and Key Value. Prefer the PostgreSQL controller/latch repair; a global change to that helper would reach the unrelated Key Value lifecycle.

The application identity resolver covers generated/custom names and restores. The shared provisioning path covers supported Postgres13–18 and instance plans; only fresh default Free 18 was exercised here. Existing-parent backfill, reserved legacy identities, restore/import, upgrade and suspension require explicit compatibility review. Web, static, cron, worker and private App readiness do not use this Database controller.

Exercise the actual controller with a healthy cluster and a pending/stale/failed child before first provision: the old implementation must publish premature Ready and fail the new assertion. Apply the child verdict and prove the first Ready, then assert ordinary SQL bootstrap succeeds on the supported CNPG dependency. Cover a never-served suspend/resume and an already-provisioned database whose backfill is pending/failed. Preserve data, current status vocabulary, upgrade/restart status and the settled extension neighbors. Do not test by mocking the API's database hook as already correct.

The console path at [postgres/query.go:186–224](../../../../lego/backend/internal/postgres/query.go#L186) resolves the application Secret and runs that URI through pgx. Its SQLSTATE-only map at:423–459 explains the displayed errors and should keep its existing policy. Read-only queries and the explicit write confirmation remain. Existing bad input, missing database, refused access, suspension, transport timeout and genuine SQL errors keep their established handling; lifecycle readiness must be corrected at its producer.

## Deduplication and original guarantees

Searched 9,666 board Markdown files, indexed 91 open/blocked milestone READMEs, reviewed exact readiness/extension matches, anti-goals, recent 40 product commits and targeted controller/extension history. The latest relevant change is24f738603 (w8/m84); no later first-readiness repair is on the researched main.

[w8/m84](../../../w8/blocked/m84/README.md) introduced installation and the correct capability verdict. Its implementation tasks are done; its remaining task is hosted pre-fix existing-parent backfill. This is a temporal residual in the first Core Ready consumer, not another request to install vector or a duplicate of that remaining backfill run. Keep the installer and later optional-serving behavior.

Its original DoD walk is explicit:

1. Fresh default/custom identities, both native clients and majors 13–18 have recorded settled acceptance there. Here, fresh default 18 through the dashboard fails at first Available and passes after settling. External clients, custom names and other majors were not rerun.
2. Owned child/backfill/lifecycle behavior is retained. The first Core Ready transition and never-served latch are this new delta; existing-parent hosted backfill remains m84's open gate.
3. Existing installation regressions do not assert this consumer ordering. Add meaningful first-readiness cases while preserving the recorded hstore/pg_stat_statements and current query contracts. a867 supplies an independently exercised pg_trgm/table neighbor.
4. Both a868 fixtures, baseline inventories and session are fully cleaned as below.

w4/m215 owns an accepted major-upgrade readiness gap, not new vector initialization. w4/m137 supplies the first-provision/restart latch vocabulary. w4/m157 owns typed SQL results; these are real engine error responses. w4/m170 owns reserved names, which remain a boundary. No anti-goal matches this functional initial-readiness repair.

## Parity, limits and cleanup

[Render](https://render.com/docs/postgresql-extensions) lists vector and its CREATE EXTENSION workflow for PostgreSQL 13+. [PostgreSQL 18](https://www.postgresql.org/docs/18/sql-createextension.html) documents the installed-extension IF NOT EXISTS no-op. Bex deliberately preinstalls vector through CNPG; authenticated Render first-Available timing was not tested, so no such comparison is claimed.

The actual [CNPG 1.30 documentation](https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/v1.30.0/docs/src/declarative_database_management.md) was opened: successful reconciliation sets applied and the matching observedGeneration. The repository's existing condition reader consumes that distinction correctly. The pending verdict must be included in new-parent readiness without changing what later backfill failures mean for serving.

Only new Free/private PostgreSQL 18 default identities were live tested. No application deploy, paid plan, role change, external credential, external CLI/MCP session, restore, import, initial suspension or existing-parent backfill was exercised.

Desktop deletion08:11:14.755Z and mobile deletion08:16:33.294Z returned true. Both REST IDs return404, all seven baseline sets match, and each exact bex Database/CNPG Cluster/application-Database/pod/PVC count is zero. Exact-session logout exited0; whoami401, cookies0, one about:blank page and removed owned handles are recorded. Own SQL history was cleared/removed. No provider disk-erasure claim is made beyond PVC absence. No other workspace or resource changed.
