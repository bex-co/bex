# w21 · m606 — An owner's `pgaudit` survives a restart: CloudNativePG leaves an extension whose library the cluster preloads, from a bex build until upstream releases it

**Worker:** worker21 **Goal:** on a bex operand, an owner's `CREATE EXTENSION pgaudit` survives a pod restart, a failover and an in-place instance-manager upgrade, as it does on Render, and no database gets a `pgaudit` it did not ask for. Production runs a CloudNativePG operator built from the pinned upstream tag plus one reviewed patch, and the patch is ready to go upstream. **Status:** in progress (t001, t002 and t005 done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Patch CloudNativePG's instance manager: an unused managed extension whose library the cluster's own `shared_preload_libraries` lists is not dropped, and a change to that list runs the check again — **DONE** | 1h | — |
| [t002](done/t002.md) | Build and publish the patched operator image from the pinned upstream tag plus `deploy/cnpg-operator/patches/`, for arm64 and amd64, by digest — **DONE** | 1h30m | t001 |
| [t003](t003.md) | `TestApplicationDatabaseExtensionsOnCNPG` covers the restart, CI runs it on kind against the patched image, and it fails on stock 1.30.1 | 1h15m | t002 |
| [t004](t004.md) | Adopt the patched image: `cnpg-operator.yaml`, `gitops-validate.sh`, the qualification pin, mock-cluster and dev-env, ADR092 and ADR018 | 45m | t003 |
| [t005](done/t005.md) | Ready the patch for upstream: an issue text and a PR branch on a `bex-co` fork of `cloudnative-pg`, and an inbox note for the user's DCO sign-off — **DONE** | 30m | t001 |
| [t006](t006.md) | Production: an owner's `pgaudit` survives a primary restart on a new Free database (disposable), and another new database still creates it | 45m | t004 |
| [t007](t007.md) | Simplify | 20m | t006 |
| [t008](t008.md) | Test coverage | 40m | t006 |
| [t009](t009.md) | Closeout | 10m | t005, t007, t008 |

## Definition of done

1. On production, on a new Free database of a bex-operand major (disposable), the owner runs Render's `CREATE EXTENSION pgaudit;`. After the primary pod is deleted and comes back, `SELECT extname FROM pg_extension WHERE extname = 'pgaudit'` still returns it, and the owner's `DROP EXTENSION pgaudit;` then succeeds.
2. A second new database (disposable) answers `CREATE EXTENSION pgaudit;` with success, not "already exists". Before or after a restart, no database has `pgaudit` unless its owner created it.
3. Production's CloudNativePG operator runs the bex build, recorded by digest. Every managed cluster's instance manager moved to it in place without a PostgreSQL restart (postmaster start times unchanged), and bex's own platform clusters (`bex-postgres`, the auth databases) stay healthy.
4. `TestApplicationDatabaseExtensionsOnCNPG` covers the restart and runs in CI on kind against the patched image. Run against stock CloudNativePG 1.30.1, it fails at the restart step.
5. The patch sits on a `bex-co/cloudnative-pg` branch with its issue and PR text, and an inbox note holds the user's step (DCO sign-off, opening the PR). ADR092 says when the patch comes out: when an upstream release carries an equivalent fix.

## Source + Goal linkage

- **Source:** inbox note [w21/122](../done/122.md), filed by w21/m585 t001 on 2026-11-23 and promoted the same day by `/loopx w21`. It was verified at promotion against CloudNativePG v1.30.1:
  - `internal/management/controller/instance_controller.go` `reconcileDatabases` (:711-757): on the first pass after a manager start `extensionStatus` is empty, so `reconcileExtensions` runs in every database that allows connections, on the primary.
  - `reconcileExtensions` (:785-846) runs `DROP EXTENSION` when `!extensionIsUsed && extensionIsInstalled`.
  - `pkg/postgres/configuration.go` `ManagedExtensions` (:459-483) lists `pgaudit`, `pg_stat_statements`, `auto_explain` and `pg_failover_slots`. `IsUsed` (:429-438) counts only a parameter with the extension's prefix.
  - Upstream `main` and v1.30.2 (2026-11-17) have the same `DROP EXTENSION` branch, and no upstream issue reports it.
  - Production sets `ENABLE_INSTANCE_MANAGER_INPLACE_UPDATES: "true"` (`deploy/gitops/base/cnpg-operator.yaml`), so every CloudNativePG upgrade starts a new instance manager in every cluster, and each of them drops the extension.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first). Render's [postgresql-extensions](https://render.com/docs/postgresql-extensions) says "To enable any supported extension, run the CREATE EXTENSION command" and lists `pgaudit` on every major. [ADR092](../../../docs/ADR092-postgres-extension-supply.md) D1 (the `pgaudit` row) and D2 (nothing preinstalled).
- **Expected outcome:** an owner who enables `pgaudit` keeps it as on Render, through restarts, failovers, plan changes and CloudNativePG upgrades. `\dx` keeps listing it, `pg_dump` keeps carrying it, and a later `DROP EXTENSION pgaudit` works.
- **Why now:** w21/m585 admits `pgaudit` at the next pin that carries `979d68fdc1`. From then on, every restart, failover or CloudNativePG upgrade silently removes an extension an owner created, and the next CloudNativePG bump would remove it from every cluster at once.
- **Decision state (the plan's recommendation, not a user decision):** patch CloudNativePG, and keep the other two options from w21/122 out.
  - Option 2, re-creating `pgaudit` from bex after each start, needs a superuser SQL path from bex into every tenant database. The operator has none today: no `database/sql` client and no `pods/exec`. It also needs a record of which databases had the extension, and it still leaves a window in which the extension is gone and an owner's `DROP` would be undone.
  - Option 3, a `pgaudit.*` parameter, makes CloudNativePG create the extension in every database. ADR092 D2 rules that out, and Render's `CREATE EXTENSION pgaudit;` would then fail with "already exists".
  - The patch's rule is that CloudNativePG does not drop an extension whose library the cluster preloads itself. It keeps the reason upstream drops extensions at all: an installed `pgaudit` whose library is no longer preloaded fails every DDL statement, because its event triggers load `pgaudit.so`, whose `_PG_init` refuses to load outside `shared_preload_libraries`. Creation is unchanged, so bex's `pg_stat_statements` (used through `pg_stat_statements.track`) behaves as before.
  - Running a patched upstream build until upstream ships the fix follows the sandbox pool's gVisor `release-20260622.0-bex.<n>` ([ADR042](../../../docs/ADR042-sandbox-cluster-substrate.md), w4/m545). CloudNativePG requires a real-name DCO sign-off, so only the user can open the upstream PR. Opening it is a separate inbox note (t005), so parity does not wait on it.
- **Render parity task omitted:** no REST, GraphQL, MCP or dashboard surface changes. Render's extension surface is SQL only, and ADR018's row is `—` for all four surfaces. The SQL behaviour against Render's documentation is the definition of done.
- **Sequence:** adopting the image is an in-place instance-manager update with no PostgreSQL restart. The new manager is already patched, so the adoption itself drops nothing. It can reach production before or after w21/m584–m586. Until it does, a restart after m585's pin drops an owner's `pgaudit`, as w21/122 recorded.

## Progress

- **The patch (t001, `cce1b285dd`):** `deploy/cnpg-operator/patches/0001-instance-keep-extensions-the-cluster-preloads.patch` on `v1.30.1` (`2a35abb46`, `deploy/cnpg-operator/UPSTREAM`).
- **The image (t002):** `ghcr.io/bex-co/bex-operator:cnpg-1.30.1-bex.1-07557237b85d3984fdf38665f8310288fb166596@sha256:5f8658442ad6979fb2c9060e3889dd5e54b13f4107d65243c67c79d9f88daa85`, published, signed and verified by `cnpg-operator-image.yml` run 45288684433.
- **Upstream (t005):** branch `bex-co/cloudnative-pg@bex/keep-preloaded-extensions` (`d7310e91b7`, on upstream `main` `f076d5ba99`); the user's steps are inbox note [w21/123](../123.md).
