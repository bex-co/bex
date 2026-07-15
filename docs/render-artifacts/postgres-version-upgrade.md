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

## Public API surface

- Render's public `PATCH /v1/postgres/{postgresId}` accepts name, plan, disk, autoscaling, pooling, Datadog, IP allowlist, parameter overrides, and read replicas, but **not version**.
- Render's official MCP server exposes `version` only on `create_postgres`; it has no version-upgrade tool.
- Render's dashboard GraphQL operation is private and not a public compatibility contract.

Sources: [public Update Postgres reference](https://api-docs.render.com/reference/update-postgres) and [the official Render MCP server's tool inventory](https://github.com/render-oss/render-mcp-server#postgres-databases).

## bex parity and deliberate extensions

- The dashboard mirrors Render's version row, explicit confirmation, downtime warning, and `upgrading` status.
- bex allows any **newer** supported target from its 13–18 catalog instead of forcing the latest. This is a deliberate CNPG-backed convenience; unknown, equal, and lower versions return named 400 errors.
- bex exposes the same core verb on REST (`PATCH /v1/postgres/{id}` `version`), GraphQL (`updateDatabaseVersion`), and MCP (`update_postgres_version`). These are deliberate public extensions because Render's public REST/MCP omit the dashboard-only operation.
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
2. REST upgraded a seeded Free database from 16 to 17. GraphQL `updateDatabaseVersion` independently upgraded another seeded database from 16 to 17, and MCP `update_postgres_version` then upgraded that database from 17 to 18.
3. Each public-verb run exposed `Database.status.phase = Upgrading`, returned to `Ready` with the new `currentVersion`, and retained the `survives-major-upgrade` row. PostgreSQL reported 17.10 and 18.4 after the respective runs.
4. A headless Chrome session registered through the mock cluster's real Kratos service, opened `/databases/m12-browser-proof-2`, read the offline-upgrade downtime warning, and clicked **Upgrade to PostgreSQL 18**. The page visibly transitioned from Available to Upgrading and back to Available at PostgreSQL 18. The seeded `survives-dashboard-upgrade` row remained intact afterward.

The live run also verified the two asynchronous edges that unit tests cannot simulate faithfully: CNPG Cluster status-only updates must trigger Database reconciliation, and the dashboard must start polling immediately after the mutation because the first eager refetch can still observe the pre-upgrade Available state.
