# Managed Postgres extension supply — decision needed (w8, 2026-10-17)

**Gate for:**

- m92 PostGIS
- m104 pg_partman
- m109 pg_duckdb
- m110 pg_ivm
- m111 pg_similarity

[Render lists all five](https://render.com/docs/postgresql-extensions) as on-demand `CREATE EXTENSION` for PG 13+, with these restrictions:

- pg_partman: PG 14+, background worker off
- pg_duckdb: PG 16+
- pg_similarity: not on PG 16+

## Facts

- **Changing the image restarts every database.** The serving operand image is chosen on every reconcile from `cnpgExportImages` (`lego/operator/internal/controller/database_exports.go`, used at `database_controller.go:486`). Changing a digest rolls every managed Postgres cluster of that major: pods restart, and the PostgreSQL patch may move. This is the same kind of rollout that [m65](../m65/README.md) waits on for its CNPG 1.30.1 upgrade. The map also feeds the logical-export `pg_dump` Jobs.
- **No upstream CNPG image carries all five.**
  - The current `ghcr.io/cloudnative-pg/postgresql:<major>` pins carry none of them.
  - CNPG publishes PostGIS-only operand images, for example `ghcr.io/cloudnative-pg/postgis:18.6-3.6.4-…-system-bookworm`, for 13–18 on bookworm and trixie ([catalog](evidence/cli-sweep403/official-CNPG-PostGIS-supplier-catalogs.json)).
  - pg_partman, pg_ivm, pg_similarity and pg_duckdb need a bex-built operand image: a reviewed CNPG base plus PGDG or upstream packages, per major.
- **pg_duckdb has extra requirements.** It also needs `shared_preload_libraries`, which means another restart and DuckDB memory in every backend, and PG 16+.
- **Activation today is pre-installation.** The application owner is not a superuser, so bex pre-installs `vector` in every application database through the retained CNPG Database child (w8/m84). Pre-installing five more everywhere is heavy: PostGIS alone adds `spatial_ref_sys` and about 1,000 functions to `public` in every database. Render's model is on-demand `CREATE EXTENSION` by the owner. Postgres can only offer that through an allowlist extension in the image, such as supautils' `privileged_extensions` or pgextwlist.

## Decisions

1. **Image.**
   - **(a)** Adopt CNPG's PostGIS images. This closes m92 only; m104 and m109–m111 become documented divergences.
   - **(b)** A bex-built operand image per major with all five where Render has them. It would be published by a workflow shaped like `cnb-run-image.yml` (publish, never pin) and pinned by a reviewed commit. This is the same pipeline shape as the [native-tools decision](../m91/design.md), option C.
2. **Activation.**
   - **(a)** Pre-install in every application database, like `vector`.
   - **(b)** On-demand `CREATE EXTENSION` by the owner through an allowlist extension (Render's contract).
3. **Rollout.** Approve one rolling restart of every managed Postgres pod to adopt the new image. It could share the m65 CNPG upgrade window. Alternatively, apply the image to new databases only, which leaves existing databases frozen on today's image until they are moved.

**Recommendation:** 1(b), 2(b), and 3 done together with m65 in one maintenance window. If fewer moving parts matter more than Render's on-demand contract, 2(a) for PostGIS alone with 1(a) closes m92 and parks the rest as divergences.

Nothing has been implemented. Every option changes the production database image or its rollout, so no task is started until the user decides.
