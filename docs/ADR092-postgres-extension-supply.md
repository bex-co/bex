# ADR092 — Managed Postgres extensions: built into the image, enabled per database by the owner

**Status:** Accepted, 2026-10-29 (user decision in `/unblock w8`, topic 2). The initial recipe, allowlist and operator projection shipped with w8/m92 t008 (2026-10-31). Production pins remain empty. Complete owner restore, current CNPG 1.30.1, resource and dual-architecture qualification remain open before adoption (w8/m92 t001) and the extension milestones.

**Related decisions:** [ADR009](ADR009-postgresql-management.md) owns managed Postgres (its § pgvector in every application database stays as written); [ADR018](ADR018-render-parity.md) is the parity ledger; [ADR060](ADR060-build-worker-reliability-and-performance.md) D7 governs reviewed digest pins.

## Context

Render documents a fixed set of [Postgres extensions](https://render.com/docs/postgresql-extensions) per major version. A database user enables one by running `CREATE EXTENSION <name>;` in their own database. Nothing is preinstalled into every database, and no ticket is needed. Six of them fail on bex today because the CNPG operand images bex pins carry none of them:

| Extension     | Render's restriction          | bex milestone |
| ------------- | ----------------------------- | ------------- |
| PostGIS       | PG 13+                        | w8/m92        |
| pg_partman    | PG 14+, background worker off | w8/m104       |
| pg_duckdb     | PG 16+                        | w8/m109       |
| pg_ivm        | PG 13+                        | w8/m110       |
| pg_similarity | not on PG 16+                 | w8/m111       |
| TimescaleDB   | Apache-licensed features only | w8/m112       |

**Render's contract is only the top layer.** Render publishes "the owner runs `CREATE EXTENSION` per database". It does not publish how a non-superuser is allowed to do that. In PostgreSQL 13+ a database owner can create an extension only when its control file is marked `trusted` (hstore is, and works on bex today, w8/m92 finding). PostGIS, TimescaleDB, pg_duckdb, pg_ivm and pg_similarity are not trusted (pg_partman uses its ordinary `superuser=false` path), so a non-superuser creates those through an allowlist mechanism that runs the extension's install script with elevated rights. Render must run such a mechanism. This ADR records bex's choice of mechanism, because it is a security boundary bex owns, not something parity dictates.

Facts carried over from the [m92 design](../.pm/w8/blocked/m92/design.md):

- The serving operand image comes from `cnpgExportImages` (`lego/operator/internal/controller/database_exports.go`, used at `database_controller.go`). Changing a digest rolls every managed cluster of that major: each pod restarts once. The same map feeds the logical-export `pg_dump` Jobs.
- No upstream CNPG image carries all six. CNPG publishes PostGIS-only images.
- pg_duckdb and TimescaleDB need `shared_preload_libraries`; current pg_ivm also requires a shared or session preload to maintain IMMVs correctly in subsequent connections.
- The application owner is not a superuser. `vector` is preinstalled in every application database through the retained CNPG Database child (w8/m84).

## Decision

### D1 — One bex-built operand image per major

bex builds `FROM` the reviewed CNPG `ghcr.io/cloudnative-pg/postgresql:<major>` digest, adding PGDG or upstream packages for each extension on exactly the majors where Render offers it (table above). The image is published by a workflow shaped like `cnb-run-image.yml` (publish, never pin) for both architectures, and the operator pins the digests in `bexOperandImages` (`database_operand.go`, which takes precedence over the upstream `cnpgExportImages` per major) by a reviewed commit (ADR060 D7). The upstream CNPG digest stays recorded as the image's base. Export Jobs use the same image, so `pg_dump` of a database that uses an extension keeps working.

Rejected: CNPG's PostGIS-only images. They would close m92 alone and leave five extensions as divergences.

### D2 — Activation stays per database, by the owner (Render's contract)

Nothing new is preinstalled. The owner runs `CREATE EXTENSION`, and `DROP EXTENSION` / `ALTER EXTENSION … UPDATE` as Postgres allows. `vector` keeps its existing preinstall (ADR009 § pgvector). Preinstalling the six everywhere was rejected: PostGIS alone adds `spatial_ref_sys` and about 1,000 functions to `public` in every database, which Render does not do.

### D3 — The allowlist that admits non-trusted extensions

The image loads one allowlist extension that lets the database owner create exactly the extensions in D1's table, and no others. Candidates are supautils (`privileged_extensions`) and pgextwlist. The implementation (m92) picks one after verifying the properties below on every major 13–18. If neither satisfies them on some major, the implementation stops and records it here rather than weakening a property.

Required properties:

1. **Closed list.** Only the reviewed names in D1's table, per major, configured in the image or the operator-projected cluster config. Never tenant-supplied. Every other non-trusted extension keeps failing with Postgres's normal permission error.
2. **No superuser leak.** The owner gains no superuser role, attribute or session. Only the extension's own install, update and drop scripts run with elevated rights, and the objects they create belong where Postgres's normal rules put them.
3. **Per-extension guardrails stay enforced:**
   - pg_partman's background worker is not enabled;
   - pg_duckdb exists only on 16+, and pg_similarity only below 16;
   - TimescaleDB runs with `timescaledb.license = 'apache'`, which keeps Render's Community-feature exclusion.
4. **Preload libraries are projected by the operator** (`shared_preload_libraries` for TimescaleDB and pg_duckdb, alongside the existing `pg_stat_statements`), not set by tenants. The added per-backend memory is accounted for in each plan's sizing before rollout.
5. **Tests prove it:** for each extension, the owner can `CREATE EXTENSION`, and a non-listed non-trusted extension still fails; the owner is not superuser afterwards; `pg_dump`/restore of a database using it works.

**Chosen: pgextwlist v1.20 (2026-10-31, w8/m92 t008).** It is packaged by PGDG for 13–18 on amd64 and arm64. supautils is not packaged anywhere, so it would be a from-source build bex maintains. v1.20 adds `extwlist.restrict_to_database_owner` and rejects schema/owner names that could inject into custom scripts. How each property holds:

1. `extwlist.extensions` is `PGC_SUSET`, so an owner's `SET` or `ALTER ROLE … SET` is refused. The operator writes it after the tenant's parameters, and bex-api refuses `extwlist.*`, `local_preload_libraries` and `session_preload_libraries` as platform-managed. A non-listed extension falls through to Postgres's own check, so trusted ones keep working and untrusted ones (`file_fdw`, `dblink`, `adminpack`) fail as before. `restrict_to_database_owner = on` keeps the path to the database owner.
2. pgextwlist runs only the extension's create, update, comment and drop scripts as the bootstrap superuser, then drops back. The objects belong to that superuser, as they would for a `trusted` extension. Because of that, the image's `extwlist.custom_path` scripts grant the owner only what an ordinary app needs: `spatial_ref_sys`, the topology and geocoder metadata, and `pg_partman`'s `part_config`. They never grant on TimescaleDB's catalog, whose job table names the role a job runs as.
   - The residual risk is the one every allowlist, and Postgres's own `trusted` flag, shares: a superuser-run install script resolving a name the owner planted in `public`.
3. The guardrails:
   - pg_partman's `pg_partman_bgw` is never preloaded.
   - pg_similarity is installed only on 13–15, and pg_duckdb only on 16–18.
   - TimescaleDB is the Apache-only build: PGDG's `+dfsg` package on 14+, and on 13 Timescale's 2.15.3 with the TSL module deleted from the image. The owner cannot change `timescaledb.license` (superuser-only), and even a superuser's switch to `timescale` is rejected because no TSL module exists.
   - pg_duckdb on 16+ is available to the existing database owner directly, with no new proxy role or membership. External access and extension auto-install/load are disabled; owners cannot change those controls. Its file/network behavior still requires the m109 acceptance, without claiming parity from an arithmetic bound.
4. The operator projects `pg_stat_statements,timescaledb,pg_ivm` and, on 16+, `pg_duckdb`, plus the local pgextwlist preload and Apache-only license. API writes and old/direct CRs cannot override preload or extension-control families. Operand defaults reserve one quarter of plan RAM for shared buffers, preserving explicit user tuning. DuckDB receives one eighth of plan RAM per connection (32MB on Free) and one thread. These are per-connection settings, not a proof that arbitrary concurrency fits. The earlier idle ARM measurement (+1.4MB launcher, +1.2MB/backend, about 10MB with TimescaleDB) is limited evidence; the current gate retains the actual 256Mi Free cgroup and exercises two concurrent DuckDB connections before adoption.
5. `deploy/pg-operand/check.sh` and `TestApplicationDatabaseExtensionsOnCNPG` preserve each complete dataset through logical export/restore. The initial ARM / CNPG 1.27 proof omitted the TimescaleDB fixture before restore and accepted restored IMMVs as plain tables; it does not satisfy complete restore or pinned 1.30.1 qualification. The renewed image gate attempts the original ordinary-owner restore and retains its exact failure. It also tests platform recovery as separate evidence; that stronger actor cannot satisfy the owner criterion. No existing test result establishes the renewed gates as passed.

**Coherent exports.** An adopted operand's export helper holds one repeatable-read transaction and an exported snapshot while reading `pgivm.get_restore_immv_commands()` and running the directory-format `pg_dump` against that snapshot. The existing archive gains `pg_ivm_restore.sql` beside the database directory. Metadata or dump failure prevents upload. Restore must apply the sidecar and prove that the original IMMV still maintains subsequent base-table writes; merely creating a different IMMV is insufficient. This needs pg_ivm's documented metadata functions and fails explicitly if a selected package lacks them. Upstream images retain their existing export command.

TimescaleDB's complete restore uses its pre/post-restore protocol without deleting hypertable data. Ordinary-owner restore remains its own required outcome; platform bootstrap recovery never grants the owner superuser or Timescale catalog rights and is not a substitute. The isolated CNPG gate requires the exact shipped1.30.1 controller and retains Free resources. Its actual PG13 admission/runtime result remains required because CNPG's documentation limits support to PGDG-supported majors.

Primary contracts: [pg_ivm metadata and preload](https://github.com/sraoss/pg_ivm/tree/v1.17), [DuckDB role and per-connection settings](https://github.com/duckdb/pg_duckdb/blob/v1.1.1/docs/settings.md), [Timescale logical restore](https://www.tigerdata.com/docs/deploy/self-hosted/backup-and-restore/logical-backup), [CNPG image support](https://cloudnative-pg.io/docs/1.30/container_images/).

**Actual D3 result — 2026-10-31.** The PG14 arm64 gate in run41196538568/job124586881316 fails ordinary-owner `timescaledb_pre_restore()` at `ALTER DATABASE … SET timescaledb.restoring`, with the exact permission error retained. The upstream invoker functions and PGC_SUSET/catalog-write requirements are outside pgextwlist's extension-utility elevation. Per D3, m92 is parked on qualified supported raw owner restore; platform recovery and an unimplemented alternative export proposal do not satisfy the criterion. The already-approved single rolling restart is unchanged. [Evidence and next engineering path](../.pm/w8/blocked/m92/evidence/owner-restore-consumer-20261031/README.md).

**Maintained bounded consumer.** `bex-postgres-restore` consumes the existing dump plus original IMMV sidecar as one transaction under the caller's identity. It rejects Timescale and large-object archives before connecting; it never discards their data. Command controls and real plain-PG15.13 rollback/data/actor/large-object-refusal controls pass. Actual pg_ivm, complete Timescale, CNPG and both-architecture qualification for this delta remain open.

**Base distribution.** The bex operand is built on CNPG's `system-bookworm` image of the same major and patch. The earlier upstream pins were bullseye images, and PGDG's bullseye archive has no PostGIS for 18 or TimescaleDB for 13. bex sets no locale, so CNPG initializes each cluster with `C`, whose order is independent of glibc. The rollout still checks for databases created with a glibc locale before the restart.

### D4 — Rollout: one rolling restart, approved

Adopting D1 restarts every managed Postgres pod once. The user approved that one rolling restart (2026-10-29). The CNPG manager upgrade originally planned for w8/m65 is already done: w7/m156 moved CNPG to 1.30.1 with an in-place update. So the restart is for the new operand image alone, not a combined window:

- the restart runs off-peak, after each major passes on an isolated cluster (extension permissions, backup/restore, data retention);
- every cluster gets health checks before and after its restart, and a recorded rollback (re-pin the previous digests);
- a Free single-instance database is briefly unavailable during its own restart. That is accepted.

## Consequences

- Render's documented `CREATE EXTENSION` flows (Django/GeoDjango, Rails PostGIS adapters, pg_partman schedules, TimescaleDB hypertables) run unchanged. Each extension's milestone then closes its own ADR018 row.
- bex now owns per-major operand images and their refresh cadence: each CNPG or PGDG update is publish, then pin.
- The allowlist is a new privileged surface inside every tenant database. Its closed list and tests (D3) are what security reviews check. Adding an extension later means amending D1's table here.
