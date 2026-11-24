# w21 · m606 — An owner's `pgaudit` survives a restart: CloudNativePG leaves an extension whose library the cluster preloads, from a bex build until upstream releases it

**Worker:** worker21 **Goal:** on a bex operand, an owner's `CREATE EXTENSION pgaudit` survives a pod restart, a failover and an in-place instance-manager upgrade, as it does on Render, and no database gets a `pgaudit` it did not ask for. Production runs a CloudNativePG operator built from the pinned upstream tag plus one reviewed patch, and the patch is ready to go upstream. **Status:** done (2026-11-24) — accepted on production: an owner's `pgaudit` survived a primary pod delete and Render's restart, a second new database created its own, and every managed cluster's instance manager moved to the bex build in place

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Patch CloudNativePG's instance manager: an unused managed extension whose library the cluster's own `shared_preload_libraries` lists is not dropped, and a change to that list runs the check again — **DONE** | 1h | — |
| [t002](done/t002.md) | Build and publish the patched operator image from the pinned upstream tag plus `deploy/cnpg-operator/patches/`, for arm64 and amd64, by digest — **DONE** | 1h30m | t001 |
| [t003](done/t003.md) | `TestApplicationDatabaseExtensionsOnCNPG` covers the restart, CI runs it on kind against the patched image, and it fails on stock 1.30.1 — **DONE** | 1h15m | t002 |
| [t004](done/t004.md) | Adopt the patched image: `cnpg-operator.yaml`, `gitops-validate.sh`, the qualification pin, mock-cluster and dev-env, ADR092 and ADR018 — **DONE** | 45m | t003 |
| [t005](done/t005.md) | Ready the patch for upstream: an issue text and a PR branch on a `bex-co` fork of `cloudnative-pg`, and an inbox note for the user's DCO sign-off — **DONE** | 30m | t001 |
| [t006](done/t006.md) | Production: an owner's `pgaudit` survives a primary restart on a new Free database (disposable), and another new database still creates it — **DONE** | 45m | t004 |
| [t007](done/t007.md) | Simplify — **DONE** | 20m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 40m | t006 |
| [t009](done/t009.md) | Closeout — **DONE** | 10m | t005, t007, t008 |

## Definition of done

1. On production, on a new Free database of a bex-operand major (disposable), the owner runs Render's `CREATE EXTENSION pgaudit;`. After the primary pod is deleted and comes back, `SELECT extname FROM pg_extension WHERE extname = 'pgaudit'` still returns it, and the owner's `DROP EXTENSION pgaudit;` then succeeds.
2. A second new database (disposable) answers `CREATE EXTENSION pgaudit;` with success, not "already exists". Before or after a restart, no database has `pgaudit` unless its owner created it.
3. Production's CloudNativePG operator runs the bex build, recorded by digest. Every managed cluster's instance manager moved to it in place without a PostgreSQL restart (postmaster start times unchanged), and bex's own platform clusters (`bex-postgres`, the auth databases) stay healthy.
4. `TestApplicationDatabaseExtensionsOnCNPG` covers the restart and runs in CI on kind against the patched image. Run against stock CloudNativePG 1.30.1, it fails at the restart step.
5. The patch sits on a `bex-co/cloudnative-pg` branch with its issue and PR text, and an inbox note holds the user's step (DCO sign-off, opening the PR). ADR092 says when the patch comes out: when an upstream release carries an equivalent fix.

## Source + Goal linkage

- **Source:** inbox note [w21/122](../../done/122.md), filed by w21/m585 t001 on 2026-11-23 and promoted the same day by `/loopx w21`. It was verified at promotion against CloudNativePG v1.30.1:
  - `internal/management/controller/instance_controller.go` `reconcileDatabases` (:711-757): on the first pass after a manager start `extensionStatus` is empty, so `reconcileExtensions` runs in every database that allows connections, on the primary.
  - `reconcileExtensions` (:785-846) runs `DROP EXTENSION` when `!extensionIsUsed && extensionIsInstalled`.
  - `pkg/postgres/configuration.go` `ManagedExtensions` (:459-483) lists `pgaudit`, `pg_stat_statements`, `auto_explain` and `pg_failover_slots`. `IsUsed` (:429-438) counts only a parameter with the extension's prefix.
  - Upstream `main` and v1.30.2 (2026-11-17) have the same `DROP EXTENSION` branch, and no upstream issue reports it.
  - Production sets `ENABLE_INSTANCE_MANAGER_INPLACE_UPDATES: "true"` (`deploy/gitops/base/cnpg-operator.yaml`), so every CloudNativePG upgrade starts a new instance manager in every cluster, and each of them drops the extension.
- **Goal linkage:** [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first). Render's [postgresql-extensions](https://render.com/docs/postgresql-extensions) says "To enable any supported extension, run the CREATE EXTENSION command" and lists `pgaudit` on every major. [ADR092](../../../../docs/ADR092-postgres-extension-supply.md) D1 (the `pgaudit` row) and D2 (nothing preinstalled).
- **Expected outcome:** an owner who enables `pgaudit` keeps it as on Render, through restarts, failovers, plan changes and CloudNativePG upgrades. `\dx` keeps listing it, `pg_dump` keeps carrying it, and a later `DROP EXTENSION pgaudit` works.
- **Why now:** w21/m585 admits `pgaudit` at the next pin that carries `979d68fdc1`. From then on, every restart, failover or CloudNativePG upgrade silently removes an extension an owner created, and the next CloudNativePG bump would remove it from every cluster at once.
- **Decision state (the plan's recommendation, not a user decision):** patch CloudNativePG, and keep the other two options from w21/122 out.
  - Option 2, re-creating `pgaudit` from bex after each start, needs a superuser SQL path from bex into every tenant database. The operator has none today: no `database/sql` client and no `pods/exec`. It also needs a record of which databases had the extension, and it still leaves a window in which the extension is gone and an owner's `DROP` would be undone.
  - Option 3, a `pgaudit.*` parameter, makes CloudNativePG create the extension in every database. ADR092 D2 rules that out, and Render's `CREATE EXTENSION pgaudit;` would then fail with "already exists".
  - The patch's rule is that CloudNativePG does not drop an extension whose library the cluster preloads itself. It keeps the reason upstream drops extensions at all: an installed `pgaudit` whose library is no longer preloaded fails every DDL statement, because its event triggers load `pgaudit.so`, whose `_PG_init` refuses to load outside `shared_preload_libraries`. Creation is unchanged, so bex's `pg_stat_statements` (used through `pg_stat_statements.track`) behaves as before.
  - Running a patched upstream build until upstream ships the fix follows the sandbox pool's gVisor `release-20260622.0-bex.<n>` ([ADR042](../../../../docs/ADR042-sandbox-cluster-substrate.md), w4/m545). CloudNativePG requires a real-name DCO sign-off, so only the user can open the upstream PR. Opening it is a separate inbox note (t005), so parity does not wait on it.
- **Render parity task omitted:** no REST, GraphQL, MCP or dashboard surface changes. Render's extension surface is SQL only, and ADR018's row is `—` for all four surfaces. The SQL behaviour against Render's documentation is the definition of done.
- **Sequence:** adopting the image is an in-place instance-manager update with no PostgreSQL restart. The new manager is already patched, so the adoption itself drops nothing. It can reach production before or after w21/m584–m586. Until it does, a restart after m585's pin drops an owner's `pgaudit`, as w21/122 recorded.

## Progress

- **The patch (t001, `cce1b285dd`):** `deploy/cnpg-operator/patches/0001-instance-keep-extensions-the-cluster-preloads.patch` on `v1.30.1` (`2a35abb46`, `deploy/cnpg-operator/UPSTREAM`).
- **The image (t002):** `ghcr.io/bex-co/bex-operator:cnpg-1.30.1-bex.1-07557237b85d3984fdf38665f8310288fb166596@sha256:5f8658442ad6979fb2c9060e3889dd5e54b13f4107d65243c67c79d9f88daa85`, published, signed and verified by `cnpg-operator-image.yml` run 45288684433.
- **Upstream (t005):** branch `bex-co/cloudnative-pg@bex/keep-preloaded-extensions` (`d7310e91b7`, on upstream `main` `f076d5ba99`); the user's steps are inbox note [w21/123](../../blocked/123.md).
- **Adoption and gate (t003, t004, `4fdf3e3338`):** `deploy/gitops/base/cnpg-operator.yaml` pins the build; `gitops-validate.sh`, the mock and every `dev-N` follow it. `cnpg-operator-qualify.yml` runs `TestOwnerPgauditAcrossRestartOnCNPG/PG18` on kind on arm64 and amd64: it passed on the build (run 45293927285) and failed on the release at the restart, `pgaudit` gone (run 45293944381). The full `TestApplicationDatabaseExtensionsOnCNPG` covers the same step but its owner export and restore phase has never passed on a real CloudNativePG (w21/124).
- **Definition of done:** item 4 holds (CI and the release's failure). Item 5 holds (the branch, the note, and ADR092's removal condition). Items 1–3 hold on production (2026-11-24, below).

## Parked (2026-11-23)

- **Gate:** ``pin must carry `4fdf3e3338` `` (the adoption). Production already carries w21/m585's `979d68fdc1`, which admits `pgaudit` (pin `f46c44d9dd`, `eb52a59622`).
- **Next, when it clears (t006):**
  1. `bash scripts/fetch-app-kubeconfig.sh <path outside the repo>`. `cnpg-system`'s Deployment runs `…:cnpg-1.30.1-bex.1-07557237b8…@sha256:5f8658442ad6…` and is Available.
  2. No PostgreSQL restart: an in-place update keeps each instance pod and its postgres container (same pod UID, `restartCount`, container `startedAt`), so for every managed cluster `pg_postmaster_start_time()` is earlier than the creation time of the operator's new ReplicaSet, while the cluster's `status.cloudNativePGCommitHash` is `2a35abb46-bex.1` and its pods report the new instance manager. `bex-postgres` and the auth databases are healthy.
  3. In `bex-canary`, a new Free database (disposable) on PG18: as the default user over the external URL, Render's `CREATE EXTENSION pgaudit;`, `\dx`; delete the primary pod and wait for `available`; `pgaudit` is still in `pg_extension`; `DROP EXTENSION pgaudit;` succeeds. A second new database (disposable) answers `CREATE EXTENSION pgaudit;` with success. Delete both (404).
  4. Record it here, in ADR092 § The CloudNativePG build and in ADR018's extensions row, then close with t009.

## Unblocked (2026-11-24)

- **Affected:** w21/m606 t006 and t009. **Source:** verified evidence (the `/loopx w21` orchestrator read production at about 05:40Z, and this worker read it again), not a user decision.
- **The gate's form was wrong.** ``pin must carry `4fdf3e3338` `` waited for the platform-image pin, but the CloudNativePG operator is not in it. Argo CD syncs `deploy/gitops/base/cnpg-operator.yaml` from `main` (`deploy/gitops/bootstrap/prod.yaml`, `targetRevision: main`), so `4fdf3e3338` (committed 23:37:05Z) reached production when Argo synced it: the operator's new ReplicaSet was created at 23:40:27Z, half an hour before the item was parked at 00:10Z. The pin `f46c44d9ddb1` does not contain `4fdf3e3338` and never had to. A change that only Argo deploys has no self-clearing gate form; its check is the live object, read on production before parking.
- **Verified:** `cnpg-system/cnpg-operator-cloudnative-pg` runs `ghcr.io/bex-co/bex-operator:cnpg-1.30.1-bex.1-07557237b85d3984fdf38665f8310288fb166596@sha256:5f8658442ad6979fb2c9060e3889dd5e54b13f4107d65243c67c79d9f88daa85` and is Available, and the Argo Application `cnpg-operator` is Synced and Healthy. No gate remains, so t006 ran the same day.

## Result (2026-11-24)

- **Item 3, the adoption:** the operator's log starts with `Starting CloudNativePG Operator`, version `1.30.1`, commit `2a35abb46-bex.1`, at 23:40:31Z.
  - From 23:40:37Z to 23:40:41Z it logged `Upgrading instance manager` 15 times, once per instance of the 11 clusters then running: two each in the four platform clusters, one each in seven app clusters. Every one went from hash `6d915657…` to `e4bb31f7…`. Each cluster read unhealthy for 4 to 14 s while its manager re-executed, and all were `Cluster in healthy state` again by 23:40:52Z.
  - It logged no restart, switchover or pod creation until 00:00:58Z, when the bex operator's w21/m584 rollout changed every app cluster's operand image ([m584 t005](../m584/done/t005.md)).
  - The four platform clusters (`bex-db`, which `bex-postgres.yaml` defines, and the auth databases `hydra-db`, `kratos-db` and `openfga-db`) kept all eight pods. Every postmaster started on 2026-11-12 or 2026-11-14 (`pg_postmaster_start_time()`, unchanged), with 0 container restarts.
  - The app clusters' postmasters started from 00:04Z to 00:17Z, during m584's operand roll, not the adoption.
  - At 05:30Z and again at 05:55Z, all nine clusters were `Cluster in healthy state`, with `cloudNativePGCommitHash` `2a35abb46-bex.1`. In every pod, `/controller/manager version` reports `1.30.1` `2a35abb46-bex.1` and its sha256 is `e4bb31f7a78d…`.
- **Item 2, nothing preinstalled:** at 05:30Z no database had `pgaudit`. That covers all 27 connectable databases in the nine clusters, read as `postgres` on each primary, although every app cluster preloads `pgaudit`.
- **Item 1, on `qa-m606-pgaudit-a`** (`dpg-dc2i44ha05ic738s5mq0`, Free, PG18, `bex-canary`, (disposable)). It went `creating` → `available` in 89 s, with a publicly trusted certificate (`sslmode=verify-full`, `sslrootcert=system`).
  - The steps ran as the default user `dpg_dc2i44ha05ic738s5mq0_user` (`rolsuper` f) over the external URL. `pgaudit` count 0, then Render's `CREATE EXTENSION pgaudit;` → `CREATE EXTENSION`, exit 0. `\dx` lists `pgaudit` 18.0, and DDL still works.
  - The primary pod was deleted at 05:37:38Z. The new pod was created at 05:37:47Z, its postmaster started at 05:37:57Z, and REST read `available` at 05:38:07Z.
  - `pgaudit` 18.0 was still in `pg_extension` at 05:38:14Z and at 05:42:16Z, 4 min 19 s after the postmaster started. The owner's `DROP EXTENSION pgaudit;` → exit 0, count 0.
- **Item 2, on `qa-m606-pgaudit-b`** (`dpg-dc2i93pnpn2c73ckvfvg`, Free, PG18, `bex-canary`, (disposable)). It was created after `a` was deleted, because a workspace holds one Free database, as on Render (`this workspace already has a free Postgres database`).
  - `pgaudit` count 0 before, then `CREATE EXTENSION pgaudit;` → success, with no "already exists".
  - It survived Render's restart verb: `POST /v1/postgres/{id}/restart` → 202, postmaster started at 05:47:46Z, still there at 05:48:30Z, and the owner's `DROP` succeeded. The `postgres` database had none.
  - Created again, it survived a second pod delete: postmaster started at 05:49:13Z, still there at 05:51:29Z (2 min 16 s later), and the owner's `DROP` succeeded.
- **Cleanup:** both databases answer 404 (05:45:43Z and 05:51:42Z), and `bex-canary` lists no Postgres. Both QA API keys named `qa-m606-t006` are revoked (the one used now gets 401), and the QA session is logged out.
- **Not covered here:** a failover, because a Free database has one instance, and other majors, which t006 leaves out of scope.
