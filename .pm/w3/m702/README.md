# w3 · m702 — Every in-place Postgres major upgrade succeeds on bex's operands: a relocatable `pg_duckdb`, and the source major's settings while `pg_upgrade` runs

**Worker:** worker3 **Goal:** a database on any bex operand major 13–17 upgrades in place to 18 through the verb (REST, GraphQL, MCP and the dashboard's guided page) and ends `SUCCEEDED` with its data, as Render's does from any supported version. Today every such upgrade ends `FAILED` after its downtime, because `pg_upgrade` cannot start the old server. **Status:** todo (t001 and t003 done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [The operand's `pg_duckdb` loads the `libduckdb.so` beside it, wherever CloudNativePG copies it](done/t001.md) — **DONE** | 45m | — |
| t002 | [Publish the operands and adopt 16, 17 and 18 from one run](t002.md) | 1h | t001 |
| t003 | [While a major upgrade is pending, the Cluster carries the source major's extension settings](done/t003.md) — **DONE** | 1h15m | — |
| t004 | [Walk real upgrades on dev-3: 13, 15 and 16 to 18, and 16 with `pg_duckdb` installed](t004.md) | 1h30m | t002, t003 |
| t005 | [Repeat it on production before w3/m676's walk](t005.md) | 45m | t004 |
| t006 | [Render parity](t006.md) | 20m | t004 |
| t007 | [Simplify](t007.md) | 15m | t006 |
| t008 | [Test coverage](t008.md) | 30m | t006 |
| t009 | [Closeout](t009.md) | 10m | t005, t007, t008 |

Total: **9 tasks, about 6h30m**.

## Definition of done

1. On the local cluster, on the adopted operands, Free databases on PostgreSQL 13, 15 and 16, each with a seeded row, upgrade to 18 through the verb. Each ends `SUCCEEDED` in `lastUpgradeRun` and keeps its row. Afterwards `SHOW shared_preload_libraries` lists `pg_duckdb` on each, and `local_preload_libraries` is `pgextwlist`.
2. A 16 database with `pg_duckdb` installed (`CREATE EXTENSION pg_duckdb`, admitted on 16) upgrades to 18 the same way, and a query under `SET duckdb.force_execution = true` still answers on 18.
3. Tests hold both rules:
   - For every source 13–17 and every target above it, the Cluster projected while the upgrade is pending preloads only libraries the source's operand carries, and sets no parameter the source's server does not know. Once the target is observed, the projection is the target's own.
   - The operand gate refuses an image whose `pg_duckdb.so`, copied as CloudNativePG's `instance upgrade prepare` copies it, cannot resolve its `libduckdb.so` from the copy.
4. `bexOperandImages` carries 16, 17 and 18 digests built with t001. ADR092 D4 records that adoption's restart, and the extra restart after an upgrade from 13–15 (when `pg_duckdb` joins the preload). ADR009 § Major-version upgrades records the rule.
5. Production runs on a pin carrying t002 and t003. On it:
   - every 16, 17 and 18 managed cluster is healthy after the adoption's restart;
   - a 15 and a 16 database in `bex-canary`, each disposable with a seeded row, upgrade to 18 through the verb as the QA user, end `SUCCEEDED` and keep the row.

## Source + Goal linkage

- **Source:** promoted from [w3/134](../done/134.md) on 2026-11-27 by `/loopx w3`. w3/m676 t007's local walk found the bug on 2026-11-26: on dev-3, 16 → 18 three times and 15 → 18 twice, every run `FAILED` within 30 s with CloudNativePG's `MajorUpgradeRollback`, and the rows survived.
- **How CloudNativePG 1.30.1 runs the upgrade** (read 2026-11-27 at tag `v1.30.1`):
  - The Job's `prepare` init container runs on the source image (`status.pgDataImageInfo.image`). It copies `pg_config --bindir`, `--pkglibdir` and `--sharedir` with `cp -a` to the same paths under `/controller/old` (`internal/cmd/manager/instance/upgrade/prepare/cmd.go`). The `major-upgrade` container runs on the target image (`pkg/reconciler/majorupgrade/job.go`, `BackoffLimit` 0).
  - `execute` (`internal/cmd/manager/instance/upgrade/execute/cmd.go`) runs initdb into `pgdata-new`. It writes the new directory's configuration from the Cluster (`prepareConfigurationFiles`, `OperationType_TYPE_UPGRADE`), then runs `pg_upgrade --link --old-bindir /controller/old/usr/lib/postgresql/<old>/bin --old-datadir --new-datadir`. It passes no `--old-options`, so the old server reads its own data directory's `custom.conf`.
  - The source's instance manager rewrites that `custom.conf` from the Cluster spec (`RefreshConfigurationFilesFromCluster`) when the operator's update lands, before CloudNativePG deletes the pods for the upgrade. The 15 → 18 runs show that it does: the old server met the target's `pg_duckdb`.
  - Nothing in the Cluster spec reaches the old server's settings. `--pg-upgrade-args` is not reachable from the spec. `setupExtensionEnvironment` extends `LD_LIBRARY_PATH` only for image-volume extensions (`spec.postgresql.extensions`), and bex does not use them.
- **Three causes, all verified:**
  - **From 16 or 17:** `pg_duckdb.so` links `libduckdb.so` through an absolute RUNPATH. pg_duckdb v1.1.1's `Makefile` line 52 sets `-Wl,-rpath,$(PG_LIB)/`. On the adopted 18.7 operand, the only path string in `pg_duckdb.so` is `/usr/lib/postgresql/18/lib/`, with no `$ORIGIN`, and `ldd` resolves `libduckdb.so` there. `libduckdb.so` itself is 59 MB, sha256 `638ece18…` on 18.7. The copy under `/controller/old` carries `libduckdb.so`, but the target image has no `/usr/lib/postgresql/16/lib/`. That gives `FATAL: could not load library "/controller/old/usr/lib/postgresql/16/lib/pg_duckdb.so": libduckdb.so: cannot open shared object file`.
  - **From 13–15:** the rewritten `custom.conf` preloads the target's `pg_duckdb` (`pgExtensionPreloadLibraries`, `database_operand.go`), which the old major does not carry: `FATAL: could not access file "pg_duckdb"`.
  - **From 13, not run on 2026-11-26:** the same rewrite also sets `output_plugin_libraries`. A 13 server does not know that parameter and refuses to start. `pgExtensionParameters` leaves it out on 13 for exactly that reason.
- **Why both fixes are needed, and the ones not taken:**
  - **Dropping `pg_duckdb` from the preload during every upgrade** fails any database that has `pg_duckdb` installed. pg_upgrade's `check_loadable_libraries` runs `LOAD` on the new server for every C library that `pg_proc` references. pg_duckdb's `_PG_init` raises an error outside `shared_preload_libraries` (`src/pgduckdb.cpp`, v1.1.1). Those databases need the old copy to load `pg_duckdb` itself (t001).
  - **The source major's settings while pending (t003)** is what the operator already does for the logical-replication settings, which read `runningMajor` (`database_subscription.go`, `syncsLogicalSlots` in `database_publication.go`). For a 16 or 17 source those settings equal 18's, so nothing changes there. For 13–15 sources they are what the old cluster already runs, so the instance manager's rewrite is a no-op and there is no race.
  - **`LD_LIBRARY_PATH` through the Cluster's `spec.env`** would reach the Job. But it restarts every cluster of every major when adopted, it depends on CloudNativePG's private `/controller/old` path, and it would hand the new server the old `libduckdb.so` first.
  - **A target image that carries the old majors' `libduckdb.so` at their absolute paths** couples the images, and every future target would have to carry every older major's library.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first). Render upgrades in place to its latest major from any supported version ([render.com/docs/postgresql-upgrading](https://render.com/docs/postgresql-upgrading)). [ADR092](../../../docs/ADR092-postgres-extension-supply.md) owns the operand images and how they roll (D4).
- **Expected outcome:** every in-place major upgrade that a database's extensions allow succeeds. w3/m676's guided page can then show a succeeded run, and its production walk can proceed.
- **Why now:** since the operands were adopted on 2026-11-23 (w21/m584), every upgrade a customer starts ends `FAILED` after its downtime, on every surface. w3/m676 is parked on this milestone, and its own pin gate (`4c334eddf0`) is already met on production pin `486e3f4afbf6`.
- **Render parity included:** the result reaches REST, GraphQL, MCP and the dashboard's guided page (t006).
