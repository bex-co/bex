# Render Postgres major-version upgrades — verified contract

Captured 2026-07-15 from Render's public documentation, public REST reference, and official MCP server. This is the parity reference for `w8/m12`.

## Render behavior

- **Placement:** the in-place major-upgrade flow is in the Dashboard's Postgres **Info** page. Clicking the current version opens the guided upgrade page.
- **Target:** an in-place upgrade always goes to Render's latest supported major (PostgreSQL 18 at capture time), from any older supported version. Choosing an arbitrary supported target requires creating a new database and migrating.
- **Downtime:** the database is unavailable during the upgrade. Render says an upgrade can take up to one hour and recommends scheduling the downtime.
- **Pre-flight:** Render strongly recommends cloning the database through PITR and testing the upgrade on the clone first. PITR is required for that clone workflow, but the public guide describes it as a recommendation rather than a hard in-place-upgrade prerequisite.
- **Lifecycle:** the database status changes to `Upgrading`, then `Available` on success. A failed upgrade leaves the database on its original version. The same credentials and connection strings work after success.
- **Maintenance windows:** Render documents scheduled maintenance for minor releases separately. The major-upgrade guide does not offer a maintenance window; starting the flow begins the upgrade.

Sources: [Render's version-upgrade guide](https://render.com/docs/postgresql-upgrading), [Render's Postgres creation/version support guide](https://render.com/docs/postgresql-creating-connecting), and the [in-place upgrades GA announcement](https://render.com/changelog/in-place-postgresql-upgrades-now-generally-available).

## The guided page

Read 2026-11-26 from Render's public dashboard bundle, without signing in: `index-D5V4ZgbE.js` (sha256 `8d15889ace2d…`), `VersionUpgrade-CFsAHNsC.js` (`33cb1b0f4613…`, reached from `AsyncComponents-XDHzvoGb.js`), the database helpers `utils-BhiyupvF.js` (`8b85b0200189…`) and operations `database-BZ7Tvd7O.js` (`a0a69cc0775a…`). Chunk names change with every deploy, so find the page by its strings. bex's page is w3/m676.

**Where it opens.** `/d/<id>/version-upgrade`, a page with no sidebar entry. Info's PostgreSQL Version value links it: **Upgrade version** while a newer major exists, **View upgrade** while one runs, **Review version details** otherwise, and on PostgreSQL 12 or older a "This version has reached end of life. View upgrade options." warning. Render shows the page, and the link, only for a paid primary (`plan !== FREE && role === PRIMARY`): a free database's version is plain text and its page draws nothing. Info also opens with a dismissible "PostgreSQL {latest} is available. Upgrade for improved performance and additional features." banner (View upgrade options) and, for a failed run nobody dismissed, the failed banner below.

**The page, top to bottom:**

| Part | Render | bex |
| --- | --- | --- |
| Failed banner | "Upgrade failed." / "Your database is still running on its current version. Review the upgrade logs, resolve the issue, then try again." with **Dismiss error**, shown while `lastUpgradeRun.state` is `FAILED` and `acknowledged` is false; Dismiss error calls `acknowledgeDatabaseUpgrade(id, runId)`. | The same, on the page and on Info. |
| Versions block | "Postgres Version Upgrades", with the badge "Current version: PostgreSQL {n}". | The same. |
| Latest | "This database is running the latest supported version of PostgreSQL." when `postgresMajorVersion >= versionUpgradeAvailable` and nothing runs. | The same, with `versionUpgradeAvailable` the latest major bex runs. |
| Steps header | "Steps to upgrade" and "See the docs for more details on upgrading." | The same, linking bex's docs. |
| Irreversible changes | "Render will make the following irreversible changes to your database, even if the upgrade fails:" PITR enabled, or brought up to date; each extension updated with `ALTER EXTENSION UPDATE`; PostGIS 2 → 3; the database user given `ADMIN`. | bex makes none of Render's changes, so it lists its own, which hold once the upgrade succeeds: PITR starts again from the upgrade (a new Barman generation), and the database cannot return to its major. |
| Blocking extensions | "Extensions block upgrading" / "Render can't upgrade a database that has these extensions enabled.", listing each `databaseExtensions` entry with `blocksUpgradeDatabase`; both steps are disabled. | The same, from `databaseExtensions`; the verb also refuses (409 `POSTGRES_UPGRADE_EXTENSIONS_BLOCKED`). |
| Step 1 | "Upgrade a clone of this database (recommended)", only when `pointInTimeRecoveryEligibility` is enabled and the database is not itself a clone: "Perform a test upgrade on a disposable copy to confirm that it completes successfully." and the clone billing sentence; **Clone this database** (disabled until PITR is ready, with "You can clone the database when Point-in-Time Recovery is ready."), then **View Postgres clone** to the clone's own `/version-upgrade`. The clone is `restoreNewDatabase` named `<name>-upgrade-clone-<4>` at the window's end, and a toast says "A clone database has been created {name}." A legacy plan gets "Your database's legacy compute plan does not support this action." | The same through `recoverDatabase` at the latest restorable point, offered when the database has backups. bex has no legacy plans. |
| Step 2 | "Upgrade this database": "The database and any replicas will be temporarily unavailable during the upgrade." / "This typically takes less than an hour. …"; **Upgrade to PostgreSQL {latest}** (or "Upgrading..."), disabled until the database is created, storage is reported and PITR is initialized, a `-upgrade-clone-` database excepted; the error inline; **Read release notes** (`postgresql.org/docs/{n}/release-{n}.html`). It calls `upgradeDatabase(id, newVersion)` and toasts "Starting upgrade to PostgreSQL {n}". | The same, through the guarded verb; a database with no backups has no PITR to wait for, and bex's guards answer inline. |
| Status and logs | While the run is `IN_PROGRESS`: its start time and state badge, and a log explorer filtered to the run (`logType` `databaseUpgrade`/`databaseReindex`, `subresource` = runId). | The run's start and state, and the database's logs from it on: live while it runs, its first two hours once it ended. The upgrade Job's own lines (`initdb`, `pg_upgrade`) reach them through the log shipper. |
| Result toasts | "Upgrade succeeded" / "Your database was updated to Postgres {n}.", or "Upgrade failed" with a docs link, when a watched run ends. | The same; the failure toast says what the banner says. |
| Last Upgrade Attempt | A block holding the last finished run's status and logs, whatever its acknowledgement. | The same. |

**Clone banner.** On every page of a database whose name matches `-upgrade-clone-` and whose source this browser recorded: "This database is a clone of {name}." / "Clones are billed identically to any other database with the same specs. Make sure to delete this clone when you're done testing." with **Delete this clone** (Info's delete card) and **Go to original database** (the source's `/version-upgrade`). Render keeps the pairing only in the browser that made the clone (`localStorage["postgres-upgrade-clone-info"]`, keyed both ways); bex does the same.

**Operations.** `upgradeDatabaseVersion` → `upgradeDatabase(id, newVersion)`; `databaseExtensions(id) { blocksUpgradeDatabase name }`; `acknowledgeDatabaseUpgrade(id, runId) { id }`; the `database` query's `lastUpgradeRun { acknowledged runId startedAt state toVersion }` (state `FAILED`, `IN_PROGRESS` or `SUCCEEDED`) and `versionUpgradeAvailable`.

**Where bex's parts come from (w3/m676 t001):**

- **The attempt record** lives on the Database, in annotations bex-api writes when it accepts an upgrade (run id, source, target, start) and when a member acknowledges one; its state is read from the operator's report of the upgrade (ADR009 § Major-version upgrades). Nothing in the operator or bex-api's store changes.
- **Blocking extensions** are those some bex operand carries that the target's does not (`deploy/pg-operand/matrix.json`): `pg_similarity` (16 and later), `adminpack` and `old_snapshot` (17 and later). bex-api reads the installed ones over the database's read-only SQL path.
- **The upgrade's log.** CNPG runs the upgrade in a `<cluster>-<n>-major-upgrade` Job whose pod carries the database's labels (`app.bex.co/component=database`, `cnpg.io/cluster`), with init containers `bootstrap-controller` and `prepare` and the `major-upgrade` container, CNPG's instance manager writing JSON lines (`logger` `initdb` and `pg_upgrade`, observed on the local cluster 2026-11-26). The shipper kept only the `postgres` container, so those lines never reached the logs; it now keeps them.
- **The clone banner's source** is client-side on Render (`ES({cloneId})` reads `localStorage`), so bex keeps it in the browser too, with no server field.

## Public API surface

- Render's public `PATCH /v1/postgres/{postgresId}` accepts name, plan, disk, autoscaling, pooling, Datadog, IP allowlist, parameter overrides, and read replicas, but **not version**.
- Render's official MCP server exposes `version` only on `create_postgres`; it has no version-upgrade tool.
- Render's dashboard GraphQL operation is private and not a public compatibility contract.

Sources: [public Update Postgres reference](https://api-docs.render.com/reference/update-postgres) and [the official Render MCP server's tool inventory](https://github.com/render-oss/render-mcp-server#postgres-databases).

## bex parity and deliberate extensions

- The dashboard follows Render's guided page (§ The guided page above): one target, the latest major.
- bex's public REST, GraphQL and MCP verb accepts any **newer** supported target from its 13–18 catalog (16 or later as the target) instead of forcing the latest. Render's public API has no upgrade verb at all, so this is an extension, not a divergence; unknown, equal, and lower versions return named 400 errors.
- bex's `PATCH /v1/postgres/{id}` accepts Render's `readReplicas` (full replacement, w5/m138), alongside the other public fields; Render's per-replica `parameterOverrides` is not accepted.
- bex exposes the same core verb on REST (`PATCH /v1/postgres/{id}` `version`), GraphQL (`updateDatabaseVersion`), and MCP (`update_postgres` with `version`; the dedicated tool folded in at `w1/m74`). These are deliberate public extensions because Render's public REST/MCP omit the dashboard-only operation.
- A durable (`basic-*`) bex instance must have a completed physical backup in its **current archive generation** before the upgrade is accepted. This is stricter than Render's stated recommendation and ensures there is a known recovery point before downtime. The ephemeral Free plan has no backup promise and is not blocked by this durability guard.

## CloudNativePG mechanism proof

The GitOps chart pin is `cloudnative-pg` `0.29.0` (CloudNativePG 1.29), whose [PostgreSQL upgrade documentation](https://cloudnative-pg.io/docs/1.29/postgres_upgrades/) supports declarative offline in-place major upgrades when `Cluster.spec.imageName` moves to a higher major on the same OS distribution.

On 2026-07-15 the local CAPD mock cluster was tested with a throwaway CNPG Cluster:

1. Provision `ghcr.io/cloudnative-pg/postgresql:16` and seed `upgrade_probe(1, 'survives-major-upgrade')` (server version `160014`).
2. Patch `spec.imageName` to `ghcr.io/cloudnative-pg/postgresql:17`.
3. Observe `status.phase = Upgrading Postgres major version`, zero ready instances, and the `*-major-upgrade` Job.
4. Wait for `Cluster in healthy state`, then query server version `170010` and the unchanged seed row.
5. Delete the throwaway namespace.

This proves the exact image-tag path the Database reconciler uses, including downtime, CNPG's observable progress phase, and data preservation.

CNPG's `pg_upgrade` creates a new PostgreSQL system ID/timeline, so pre-upgrade PITR history cannot continue into the new major under the same Barman `serverName`. bex changes the archive name to `<database>-pg<target>` atomically with the image change, records the active name in `Database.status.backupServerName`, and starts a new base backup after success. The next guarded upgrade accepts only a completed backup whose reported `status.serverName` matches that active generation.

## Public-surface mock-cluster proof

The completed implementation was exercised on the same CAPD mock cluster on 2026-07-15:

1. REST rejected target `19` with HTTP 400 / `POSTGRES_VERSION_UNKNOWN`, rejected an equal `16` target with HTTP 400 / `POSTGRES_VERSION_NOT_NEWER`, and rejected a simultaneous durable-plan + `17` update without a backup with HTTP 409 / `POSTGRES_UPGRADE_BACKUP_REQUIRED`.
2. REST upgraded a seeded Free database from 16 to 17. GraphQL `updateDatabaseVersion` independently upgraded another seeded database from 16 to 17, and MCP's version verb (then `update_postgres_version`, now `update_postgres`'s `version` field) then upgraded that database from 17 to 18.
3. Each public-verb run exposed `Database.status.phase = Upgrading`, returned to `Ready` with the new `currentVersion`, and retained the `survives-major-upgrade` row. PostgreSQL reported 17.10 and 18.4 after the respective runs.
4. A headless Chrome session registered through the mock cluster's real Kratos service, opened `/databases/m12-browser-proof-2`, read the offline-upgrade downtime warning, and clicked **Upgrade to PostgreSQL 18**. The page visibly transitioned from Available to Upgrading and back to Available at PostgreSQL 18. The seeded `survives-dashboard-upgrade` row remained intact afterward.

The live run also verified the two asynchronous edges that unit tests cannot simulate faithfully: CNPG Cluster status-only updates must trigger Database reconciliation, and the dashboard must start polling immediately after the mutation because the first eager refetch can still observe the pre-upgrade Available state.
