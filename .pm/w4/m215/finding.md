# An accepted Postgres upgrade briefly returns Available and slows its own dashboard polling

Why: an owner accepting an offline major upgrade should see its progress until the target is healthy; a premature Available read currently cancels the fast polling that would show that progress.

**Severity:** major, misleading lifecycle state. **Scope:** researched filing only; no product fix. Source checked on synced main `f1c91f7d48cebb7f02868ab80aa5710c6c53578e`; deployed application revisions were not mapped to Git. Functional QA a712/a713, 2026-10-16 UTC.

## Fresh native reproduction

Create one private Free PostgreSQL 16 database, wait for Available, reload its detail page, then use Details → Upgrade → PostgreSQL 17. Repeat after completion from a fresh 390px page with PostgreSQL 17 → 18. Both native requests are `UpdateDatabaseVersion` for this pass's own database, `dpg-db90llhn9t2s73cq25a0` / `qa-20261016-a713-status`.

| Observation | 16 → 17, desktop | 17 → 18, fresh mobile |
| --- | --- | --- |
| Accepted native response | 11:31:40.649Z, `config_restart`, observed 16 | 11:34:52.709Z, `config_restart`, observed 17 |
| Dashboard's own immediate `Database` refetch | Started 11:31:40.662Z; `available`, version 16 | Started 11:34:52.724Z; `available`, version 17 |
| Direct API then reports `upgrading` while page says Available | 11:31:44.017Z through last initial sample 11:31:51.801Z | 11:34:54.903Z through last initial sample 11:35:03.621Z |
| Later sampled page still Available / API Upgrading | Not sampled as a separate later receipt | 11:35:17.682Z |
| Supported upgrade eventually completes | Available/version 17 at 11:33:54.001Z | Available/version 18 at 11:36:43.782Z |

The initial immediate direct reads also report `available`; this is not solely a badge renderer or a cached request we invented. Both accepted mutation responses and the dashboard's real follow-up query are recorded. Watcher records are explicitly projections of the captured `Database` responses; the native and direct timeline requests/responses are complete.

**Matching Kubernetes evidence:** at **11:34:54.089128Z**, the Database is generation 3, desired 18, observed 17, phase Ready, condition Ready=True/Provisioned with observedGeneration 3. Its CNPG Cluster already desires the pinned PostgreSQL 18 image but still reports `Cluster in healthy state`, data major 17 and one ready instance. Twenty metadata/status samples have zero read errors. This explains why a generation check alone cannot catch the gap.

**Controls and limits:** Both upgrades succeed. The final a713 native SQL read reports `180006`; no data-loss or failed-upgrade claim is made. Independent a712's 16 → 18 upgrade preserves three seeded rows exactly: Unicode, SQL NULL, empty text, numeric scale and JSON, with process version `160015` → `180006`. Fresh pages render Upgrading correctly once that state is fetched. Offscreen mobile SQL is deliberately deferred; its section link mounts it. No screenshot, authenticated Render mutation, paid backup, replica or exact outage-duration claim is made.

## Root cause and bounded target

- `lego/operator/internal/controller/database_controller.go:1267–1340`: `reconcileDatabaseReadiness` reads the observed major, checks CNPG's textual major-upgrade running/failed phases, then accepts the old `readyInstances` count. Before CNPG changes its phase, that count can set Database Ready for the new generation while desired and observed majors differ. The sole production call is `:866`.
- The completed manual-restart fix is `restartPending` at `:1391–1415`. It follows `spec.restartedAt` and the matching Pod annotation; an ordinary version update sets no restart timestamp. Do not remove or repurpose that check.
- `lego/backend/internal/postgres/service.go:427–438` maps Ready through `core.DatastoreReadyStatus` (`internal/core/datastore_status.go:47`); a current-generation Ready=True becomes `available`. `pgView:459–481` consumes that mapper, and `Service.view:531` feeds normal reads. Production direct `pgView` callers also include `lifecycle.go:62` and `recovery.go:325`. The mapper is faithfully publishing the operator's false Ready, not guessing the target version.
- `dashboard/src/common/lib/polling.ts:37–62` resets from the 3-second converging interval to the 30-second baseline when a settled read arrives. `features/databases/hooks/use-database.ts:63–75`, `database-version-control.tsx:47–55`, and `routes/databases.$databaseId.tsx:254–256` connect the accepted upgrade to an eager detail refetch. The false Available response therefore cancels fast polling. The two native captures record exactly this response, followed by no further `Database` watcher within their initial 12-second windows.
- **Framework checked:** chart lock `deploy/helm-artifacts.lock:20` pins CloudNativePG chart 0.29.0. The examined [CNPG v1.30.0 major-upgrade reconciler](https://github.com/cloudnative-pg/cloudnative-pg/blob/v1.30.0/pkg/reconciler/majorupgrade/reconciler.go#L80) checks desired major against `PGDataImageInfo`, resolves the upgrade, then separately patches its upgrade phase/target at lines 120–136 before preparing Pods. A Cluster spec write is not an acknowledgement that this status patch has happened. The live old-major/healthy-phase sample agrees; no precise deployed CNPG image assertion is required for the finding.

**Target behavior:** after a supported accepted major change, hold the Database non-Ready while a known target is ahead of the known observed major, including the interval before CNPG announces its upgrade phase. Preserve observed version rather than echoing desired version. Keep the existing failed-upgrade branch and its explicit spec revert ahead of any pending-upgrade decision, so failure/revert can recover. Unknown initial observations keep provisioning semantics; unchanged healthy majors remain Ready. Derive the pending decision from existing intent and observation, not a new timer or guessed major. Do not make rejected targets look accepted, retry an upgrade write, or broaden the backend's version validation.

Correct the authoritative operator status first. The current dashboard already polls fast through Restarting/Upgrading; do not change the shared polling policy merely to conceal a false Ready. If replay still finds a separate client gap after that fix, research and bound that change separately.

## Isolated local proof and verification scope

An ignored Go overlay appends one test to the existing `database_upgrade_test.go` fixture. It sets CNPG phase healthy, `readyInstances=1`, observed major 16 and desired major 17, then runs the real reconciler. It logs **desired=17 observed=16 phase=Ready Ready=True** and fails the desired non-Ready assertion. Actual `go test` exit **1**, one expected diagnostic failure; this is not a passing regression suite or production fault injection. [Exact appended source](diagnostic.go.txt), [complete output](diagnostic.log.txt).

Add the regression at this pre-acknowledgement frame, then test CNPG Upgrading, target observed but not ready, target healthy, initial missing version, unchanged Ready, manual restart, suspended/resumed and failed-upgrade revert. The existing `TestReconcileMajorVersionUpgradeLifecycle` starts with CNPG already Upgrading and `readyInstances=0`; it misses the earlier frame. Preserve post-upgrade backup-generation handling and existing failure-status retention.

**Shared surface census:** one production readiness call and one `dbStatus` consumer (`pgView`). PostgreSQL is the affected resource family; web/static/worker/cron/private services and Key Value use other reconcilers. Version writes are REST `PATCH /v1/postgres/{id}`, GraphQL `updateDatabaseVersion`, and MCP `update_postgres`; status readers include REST list/detail, GraphQL `databases`/`database`, and MCP `list_postgres_instances`/`get_postgres`. Dashboard `/databases/<id>` and its `/d/<id>` redirect reach the one version-control mount. Those aliases and non-GraphQL public readers are source-traced, not live race replays. No schema change is needed: existing Upgrading/non-Ready phases are expressible. Forbidden, unauthenticated, not-found, unsupported targets, backup prerequisites and failed upgrades retain their existing classes.

## Dedupe, parity and cleanup

Searched all **9,445** board Markdown records and reviewed **80** actual active milestone titles, including blocked items; checked DO_NOT_DO, the last 40 product commits, targeted history and the final main sync. `w8/done/m12/t003` establishes truthful major-upgrade status but its test begins after CNPG acknowledgement. `w4/done/m137/t010` fixes manual restart annotation acknowledgement, a different trigger/guard. `w4/blocked/177` owns SQL execution availability; this filing neither duplicates nor expands that gate. `w4/blocked/308` owns unsupported upgrade targets; both reproduced targets here are supported. No matching pending implementation addresses this pre-phase version mismatch.

[Render's upgrade documentation](https://render.com/docs/postgresql-upgrading) describes downtime and completion as Upgrading → Available. Exact authenticated Render timing was not observed. Bex's selectable intermediate targets and `config_restart` vocabulary are existing retained differences, not changes scheduled here.

The owned a713 database returns 404 at **11:37:33.648Z**; all seven baseline resource-ID sets match. Its Pod/PVC selector is empty at **11:37:23.890455Z**. The exact SQL history was removed; logout confirmed, then whoami401/cookies0/one blank page at **11:37:39.045Z**, and only this session's exact handles were removed. The independent a712 control is also deleted and revoked. Durable evidence contains no password, connection string, cookie, environment file or kubeconfig.
