# ADR092 — Managed Postgres extensions: built into the image, enabled per database by the owner

**Status:** Accepted, 2026-10-29 (user decision in `/unblock w8`, topic 2). Not yet implemented; tracked by w8/m92 (the shared image and activation path), then w8/m104 and w8/m109–m112 (one extension each).

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

**Render's contract is only the top layer.** Render publishes "the owner runs `CREATE EXTENSION` per database". It does not publish how a non-superuser is allowed to do that. In PostgreSQL 13+ a database owner can create an extension only when its control file is marked `trusted` (hstore is, and works on bex today, w8/m92 finding). PostGIS, TimescaleDB, pg_partman, pg_duckdb, pg_ivm and pg_similarity are not trusted, so a non-superuser can create them only through an allowlist mechanism that runs the extension's install script with elevated rights. Render must run such a mechanism. This ADR records bex's choice of mechanism, because it is a security boundary bex owns, not something parity dictates.

Facts carried over from the [m92 design](../.pm/w8/m92/design.md):

- The serving operand image comes from `cnpgExportImages` (`lego/operator/internal/controller/database_exports.go`, used at `database_controller.go`). Changing a digest rolls every managed cluster of that major: each pod restarts once. The same map feeds the logical-export `pg_dump` Jobs.
- No upstream CNPG image carries all six. CNPG publishes PostGIS-only images.
- pg_duckdb and TimescaleDB need `shared_preload_libraries`.
- The application owner is not a superuser. `vector` is preinstalled in every application database through the retained CNPG Database child (w8/m84).

## Decision

### D1 — One bex-built operand image per major

bex builds `FROM` the reviewed CNPG `ghcr.io/cloudnative-pg/postgresql:<major>` digest, adding PGDG or upstream packages for each extension on exactly the majors where Render offers it (table above). The image is published by a workflow shaped like `cnb-run-image.yml` (publish, never pin) for both architectures, and the operator pins the digests in `cnpgExportImages` by a reviewed commit (ADR060 D7). The upstream CNPG digest stays recorded as the image's base. Export Jobs use the same image, so `pg_dump` of a database that uses an extension keeps working.

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

### D4 — Rollout: one rolling restart, approved

Adopting D1 restarts every managed Postgres pod once. The user approved that one rolling restart (2026-10-29). The CNPG manager upgrade originally planned for w8/m65 is already done: w7/m156 moved CNPG to 1.30.1 with an in-place update. So the restart is for the new operand image alone, not a combined window:

- the restart runs off-peak, after each major passes on an isolated cluster (extension permissions, backup/restore, data retention);
- every cluster gets health checks before and after its restart, and a recorded rollback (re-pin the previous digests);
- a Free single-instance database is briefly unavailable during its own restart. That is accepted.

## Consequences

- Render's documented `CREATE EXTENSION` flows (Django/GeoDjango, Rails PostGIS adapters, pg_partman schedules, TimescaleDB hypertables) run unchanged. Each extension's milestone then closes its own ADR018 row.
- bex now owns per-major operand images and their refresh cadence: each CNPG or PGDG update is publish, then pin.
- The allowlist is a new privileged surface inside every tenant database. Its closed list and tests (D3) are what security reviews check. Adding an extension later means amending D1's table here.
