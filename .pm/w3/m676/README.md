# w3 · m676 — A Postgres's major-version upgrade is Render's guided page at `/version-upgrade`: clone first, what cannot be undone, status and logs

**Worker:** worker3 **Goal:** clicking a Postgres database's version opens Render's guided upgrade page at `/databases/<id>/version-upgrade` (and Render's `/d/<id>/version-upgrade`). The page shows the current version and the one target Render offers, the latest supported major, and recommends upgrading a PITR clone first. It lists what the upgrade changes for good, names extensions that block it, shows the upgrade's status and logs while it runs, and keeps the last attempt's result until it is acknowledged. A clone carries Render's "This database is a clone of …" banner with Delete this clone. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Record Render's upgrade page, and check what bex can show for each part | 45m | — |
| [t002](t002.md) | GraphQL: the last upgrade attempt, acknowledging it, and the extensions that block an upgrade | 1h30m | t001 |
| [t003](t003.md) | The page: current version, the one target, steps, irreversible changes, blockers, downtime, confirm | 1h30m | t001 |
| [t004](t004.md) | Clone first: "Upgrade a clone of this database (recommended)", View Postgres clone, and the clone's banner | 1h30m | t003 |
| [t005](t005.md) | Upgrade status and logs, the result, and Last Upgrade Attempt | 1h | t002, t003 |
| [t006](t006.md) | Info's version row opens the page in place of the dialog; the page's pending frame | 45m | t003 |
| [t007](t007.md) | Verify with real upgrades locally, and with a clone on production | 1h | t004, t005, t006 |
| [t008](t008.md) | Render parity | 25m | t007 |
| [t009](t009.md) | Simplify | 15m | t008 |
| [t010](t010.md) | Test coverage | 45m | t008 |
| [t011](t011.md) | Closeout | 10m | t009, t010 |

Total: **11 tasks, about 9h35m**.

## What Render does

- **Evidence:** Render's public dashboard bundle, read 2026-11-25 without sign-in: `index-D5V4ZgbE.js` (sha256 `8d15889ace2d…`), and `VersionUpgrade-CFsAHNsC.js` (`33cb1b0f4613…`, reached from `AsyncComponents-*.js`). Chunk names change with every deploy, so find the page by its strings. [Render's guide](https://render.com/docs/postgresql-upgrading) and `docs/render-artifacts/postgres-version-upgrade.md` (captured 2026-07-15) agree.
- **Entry:** Info's version value opens `/d/<id>/version-upgrade`. An in-place upgrade always goes to the latest supported major; any other target means a new database.
- **Two ways, in order:**
  - "Upgrade a clone of this database (recommended)". It shows only when the database's `pointInTimeRecoveryEligibility` is enabled and the database is not itself a clone. It restores a PITR clone (`restoreNewDatabase`), then offers "View Postgres clone".
  - "Upgrade this database".
- **What it shows:**
  - the current version, or "This database is running the latest supported version";
  - the changes that cannot be undone: PITR history restarting, extensions updated (`ALTER EXTENSION … UPDATE`), PostGIS 2 → 3;
  - the extensions that block upgrading;
  - the downtime notice;
  - "Upgrade status and logs" while it runs;
  - the result, succeeded or failed;
  - "Last Upgrade Attempt".
- **Operations** (the `database-*.js` chunk):
  - `upgradeDatabaseVersion` → `upgradeDatabase(id, newVersion)`;
  - `databaseExtensions(id) { blocksUpgradeDatabase name }`;
  - the `database` query's `lastUpgradeRun { acknowledged runId startedAt state toVersion }`;
  - `acknowledgeDatabaseUpgrade(id, runId)`, which dismisses a finished run's result.
- **Clone banner:** on the clone's Info page, "This database is a clone of {name}." with "Clones are billed identically to any other database with the same specs. Make sure to delete this…" and Delete this clone. The source is looked up through `ES({cloneId})` → `sourceDb`, which the clone flow appears to save on the client; t001 confirms.

## bex today

Verified 2026-11-25 on main `c196617a8b`.

- The upgrade is a dialog on Info's Details card (`database-version-control.tsx`) that offers any newer version from bex's 13–18 catalog. It calls GraphQL `updateDatabaseVersion`; REST `PATCH /v1/postgres/{id}` `version` and MCP `update_postgres(version:)` call the same guarded verb.
- The guards:
  - a durable plan needs a completed backup in the current archive generation (409 `POSTGRES_UPGRADE_BACKUP_REQUIRED`);
  - a logical-replication publisher whose slots lag gets 409 `POSTGRES_UPGRADE_LOGICAL_SLOTS_BEHIND`;
  - an upgrade from 13–16 returns an `upgradeNotice` naming the slots it drops.
- The status reads `upgrading` until the target is healthy (w4/m215). The operator moves CNPG's `Cluster.spec.imageName`, CNPG runs a `<cluster>-major-upgrade` Job, and each major starts a new Barman generation (`status.backupServerName` `<db>-pg<target>`).
- There is no record of past attempts, no acknowledgement, no blocking-extension check, no upgrade log view and no clone step.
- PITR restore to a new database exists: GraphQL `RecoverDatabase` and `DatabaseRecoveryInfo`, the Recovery page.
- `/databases/<id>/version-upgrade` lands on Info (`databaseSubpathLanding` in `features/databases/lib/database-pages.ts`, w3/m641).
- ADR018's "Postgres major-version upgrade" row (`docs/ADR018/managed-postgres.md`) marks the UI ✅ and calls the any-newer target "deliberate and recorded", with no user sign-off for the dashboard's dialog.

## Decision recorded at filing

The page offers Render's single target, the latest supported major, as Render's dashboard does (AGENTS.md: ship the parity choice, never park it on an optional decision). bex's public REST, GraphQL and MCP keep accepting any newer target. That is an extension where Render's public API has no upgrade verb at all, and it stays as ADR018 records it. If the dashboard should keep offering a choice of target, that would be a divergence that needs the user's sign-off in ADR018, filed separately, not a gate on this milestone.

## Definition of done

1. `/databases/<id>/version-upgrade` and Render's `/d/<id>/version-upgrade` open the guided page. Info's version row links there, and the old dialog is gone.
2. Locally, a 16 → 18 upgrade from the page passes through upgrading with status and logs visible, and ends with a succeeded result that Last Upgrade Attempt keeps until acknowledged. A forced failure (a blocking extension or a refused guard) shows the failed result or the blocker before any downtime.
3. Locally, "Upgrade a clone of this database (recommended)" restores a PITR clone and offers View Postgres clone. The clone's Info page shows Render's banner, and Delete this clone deletes it.
4. On a database already at the latest major, the page says so and offers no upgrade.
5. On production, once a pin carries the milestone, the QA user in `bex-canary` (`tea-daif693dqjvc73e7as3g`) clones a Postgres created for the walk (disposable) and upgrades the clone from the page, and both are deleted.
6. The page's pending frame matches its ready state at 1440 and 390.
7. ADR018's row and `docs/render-artifacts/postgres-version-upgrade.md` describe the page, and the row no longer marks the dashboard ✅ for a dialog Render does not have.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w3` (2026-11-25), item 5. Before filing it was checked against Render's bundle (the operations above, re-read 2026-11-25) and bex's code and records (the ADR018 row, the version-upgrade artifact, the guards). The conflict check against w1/m645–m648 found none. w1/m645's operation ledger points `upgradeDatabaseVersion`, `acknowledgeDatabaseUpgrade`, `databaseExtensions` and `lastUpgradeRun` at this milestone (added to its t002 when this was filed).
- **Goal linkage:**
  - [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first);
  - a managed database's data safety: Render recommends rehearsing the upgrade on a clone first, because some of its changes cannot be undone;
  - [ADR009 § major-version upgrades](../../../docs/ADR009-postgresql-management.md#major-version-upgrades-w8m12).
- **Expected outcome:** before up to an hour of downtime that cannot be undone, a user rehearses on a clone, sees what will change for good and what blocks the upgrade, watches it run, and finds the result afterwards, as on Render.
- **Why now:** one confirmation in a dialog is all that stands before an upgrade of a production database today. w3/m641 gave the database Render's pages, and `/version-upgrade` is the one Render page bex still lands on Info.
- **Render parity task:** included (t008). The dashboard and GraphQL change. REST and MCP keep their upgrade verb unchanged, and t008 confirms that and checks whether the attempt record should also show on them.
