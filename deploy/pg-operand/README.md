# Managed-Postgres operand images

`pg-operand-image.yml` publishes one bex operand image per PostgreSQL major ([ADR092](../../docs/ADR092-postgres-extension-supply.md) D1). Each one is CloudNativePG's reviewed `system-bookworm` image of the same major and patch, plus:

| Extension (Render's restriction) | Majors | Source |
| --- | --- | --- |
| PostGIS, with `postgis_raster`, `postgis_topology`, `postgis_tiger_geocoder` | 13–18 | PGDG |
| pg_partman (background worker off) | 14–18 | PGDG |
| pg_ivm | 13–18 | PGDG |
| pg_similarity (not on 16+) | 13–15 | PGDG |
| TimescaleDB (Apache features only) | 13–18 | PGDG (14+, Apache-only `+dfsg` build); Timescale's repository for 13 (2.15.3, its last PG13 line) with the TSL module deleted |
| pg_duckdb | 16–18 | `pgduckdb/pgduckdb` v1.1.1 (PGDG-on-bookworm), shared objects copied as built; owner role with bounded resources, external access disabled |
| pg_repack (Render: 16 or later) | 16–18 | PGDG |
| wal2json (an output plugin, not an extension) | 13–18 | PGDG |
| pgaudit (preloaded; loads from nowhere else) | 13–18 | CNPG's system base |
| Render's untrusted contrib modules (`dblink`, `earthdistance`, `moddatetime`, `bloom` and the rest; `pg_surgery` 14+, `adminpack` 13–16, `old_snapshot` 14–16) | 13–18 | CNPG's system base (PostgreSQL contrib), admitted with PostgreSQL's own grants |
| pg_stat_statements (preloaded; CloudNativePG creates it in every database for query insights) | 13–18 | CNPG's system base, admitted so the owner's restore can run `COMMENT ON EXTENSION pg_stat_statements` and the owner can drop and create it |
| pgvector (`vector`; created on demand, none preinstalled in a new database) | 13–18 | CNPG's system base |
| pgextwlist v1.20 (the allowlist) | 13–18 | PGDG |

`matrix.json` is the per-major recipe: the digest-pinned base, the PGDG packages, `extensions` (every untrusted extension on Render's list the image carries, from its packages or its base, each judged by the gate) and `admitted` (the ones whose gate passed, which is production's `extwlist.extensions`). The operator's `pgExtensionAllowlist` must equal `admitted` (`TestPGExtensionAllowlistMatchesTheImageGate`). An extension the image carries but does not admit keeps its preload, so admitting it later is a reload, not a restart ([ADR092](../../docs/ADR092-postgres-extension-supply.md) D3, D4).

Notes on the recipe:

- **Bookworm, not bullseye.** bex's earlier pins were CNPG's bullseye images. PGDG has retired bullseye, and its archive has no PostGIS for 18 or TimescaleDB for 13. bex sets no locale, so CNPG initializes every cluster with `C`, whose order does not depend on glibc. A database someone created with a glibc locale would still need a reindex check; the rollout verifies none exist (w8/m92 t001).
- **The server never moves.** The recipe holds the base's server and client packages, and fails if an extension dependency would change them.
- **PostGIS 3.6.4 on 14 and 15.** PostGIS 3.7 split the tiger geocoder into its own package, which PGDG builds only for 16+. 13 is already on 3.6.
- **pg_duckdb finds `libduckdb.so` beside itself.** Upstream links it through an absolute RUNPATH, `/usr/lib/postgresql/<major>/lib/`. CloudNativePG's major-upgrade Job copies the source's bindir, pkglibdir and sharedir with `cp -a` to the same paths under `/controller/old` (`instance upgrade prepare`) and `pg_upgrade` starts that copy on the target's image, where the absolute path does not exist. So the recipe sets `pg_duckdb.so`'s RUNPATH to `$ORIGIN`, and the build fails if any library in a relocated copy of the pkglibdir resolves one from the original path (w3/m702). Without it, every upgrade from 16 or 17 failed with `libduckdb.so: cannot open shared object file`.
- **Owner grants.** pgextwlist runs a whitelisted extension's script as the bootstrap superuser, so its objects belong to that superuser. `extwlist/<extension>/after-create.sql` grants the database owner the narrow rights an ordinary app needs: custom SRIDs in `spatial_ref_sys`, topology and geocoder metadata, `pg_partman`'s `part_config`, and `pg_repack`'s `repack` schema (the client creates its log table and copy there). `pg_repack`'s `SECURITY DEFINER` trigger function is handed to the database owner, so the trigger that writes into the owner's log table never runs as a superuser. No TimescaleDB catalog is granted, because its job table names the role that runs each job.

## The gate

`gate.sh` runs `check.sh` inside the exact digest about to be published, on both platforms every time, each on its own native runner (the workflow's `build` job gates arm64, `gate-amd64` gates the same layout on amd64 hardware, and `publish` pushes only a major that passed both; a failure on one never skips the other). Under QEMU the emulator's own memory pushes two concurrent DuckDB scans past the 256 MiB cgroup, so an emulated leg would say nothing about the image, with networking disabled and the Free plan's 256 MiB; the workflow publishes and signs only an image that passed it, so a failed gate leaves nothing in GHCR. `check.sh` configures a throwaway server the way the operator projects a cluster (`pgExtensionParameters`, with the `admitted` list) and then checks, as a non-superuser database owner:

- the image is an upgrade source: a copy of its bindir, pkglibdir and sharedir made as CloudNativePG's major-upgrade Job makes it starts with the projection, and its postmaster maps every preloaded library (pgaudit, and `libduckdb.so` on 16+) from the copy, none from the original pkglibdir (w3/m702);
- the owner cannot change `extwlist.extensions` by `SET` or `ALTER ROLE`; an untrusted extension Render does not list (`postgres_fdw`, before and after the owner creates the rest) and every carried extension that is not admitted fail with PostgreSQL's normal permission error, and trusted ones (`hstore`) still work; a non-owner cannot use the allowlist;
- `pgaudit` is in `shared_preload_libraries`, and a superuser reads an insert through a `wal2json` logical slot;
- a reload widens the list to every carried extension (as a later admission would in production), and each one is created and does real work: a GiST spatial query, a raster, a topology, address normalization, an incremental view, `pg_partman` partitions and maintenance, a `pg_repack` run by the table's owner (rows unchanged, space returned, another role's table refused), a hypertable, DuckDB scans;
- CloudNativePG's `pg_stat_statements`: the gate creates it in `template1` as the superuser before any database, as CloudNativePG does in production, so every database holds it and every owner export carries its comment. Render's plain `CREATE EXTENSION pg_stat_statements;` meets `already exists`, the owner reads every role's statements while the reset stays superuser-only, and the owner drops it and runs Render's command;
- pgvector as Render documents it: the owner's literal `CREATE EXTENSION vector;` in a fresh database, an HNSW index the nearest-neighbour query uses, and the owner's `DROP EXTENSION vector`; on a database where the superuser installed it (as CloudNativePG did before w21/m586) the plain command meets `already exists`, the data keeps working, and the owner can drop it and create its own;
- Render's untrusted contrib modules keep PostgreSQL's own grants: each does one cheap job for the owner (an `earth_distance`, a `moddatetime` trigger, a password `dblink` connection, a bloom index, the inspection views `pg_monitor` opens), while what PostgreSQL keeps for a superuser stays refused (`pageinspect`, `amcheck`, `pg_surgery`, `adminpack`, `dblink_connect_u`, a `file_fdw` server, `pgaudit.log`), a passwordless `dblink` cannot borrow the local trust, and `xml2` reads no server file;
- TimescaleDB refuses compression, the owner cannot change its license, and even a superuser cannot switch to the TSL license;
- no `pg_partman` background worker runs, and the owner is still not a superuser afterwards;
- **property 5, per extension:** for each carried extension, a database using only that extension is exported by its owner and restored by the same ordinary owner into a new database, once with `pg_restore --no-owner --no-privileges --exit-on-error` and once with `bex-postgres-restore` (except TimescaleDB, which the consumer refuses), and its data must come back and keep working both times. Each prints `owner_restore_<extension>=PASS|FAIL`, a failure with its exact error. The gate fails when an admitted extension fails, and when one passes but is not admitted, so `admitted` can never go stale in either direction.

Operand exports add `pg_ivm_restore.sql`, captured against the same snapshot as the dump. Metadata/dump failures prevent a successful archive. Restore applies this metadata and proves subsequent writes maintain the original view. A package without the required metadata functions fails explicitly.

Platform recovery of the combined database (every extension at once, restored by the bootstrap superuser) is checked separately. It is a stronger actor and never substitutes for an extension's owner verdict, nor grants tenant catalog privileges. The gate prints the cgroup's `memory.peak` (page cache included) and the largest anonymous memory it sampled. The CNPG test requires the pinned 1.30.1 controller on an isolated loopback API, keeps actual Free 256Mi resources and requires all six image inputs unless `BEX_CNPG_MAJORS` explicitly narrows a diagnostic run. PG13's actual admission/runtime support must be observed, not inferred.

DuckDB on 16+ uses the existing owner as `duckdb.postgres_role`,32MB per connection and one thread on Free. The image gate runs with 256Mi,64MB shared buffers and two concurrent real scans, recording backend private pages and cgroup peak. External access and extension auto-install/load are disabled. These settings and source tests are not completed runtime or parity qualification.

Locally:

```sh
docker build --build-arg BASE_IMAGE="$(jq -r '."18".base' deploy/pg-operand/matrix.json)" \
  --build-arg DUCKDB_IMAGE="$(jq -r '."18".duckdb' deploy/pg-operand/matrix.json)" \
  --build-arg PG_MAJOR=18 --build-arg PGDG_PACKAGES="$(jq -r '."18".packages' deploy/pg-operand/matrix.json)" \
  -t bex-pg:18 deploy/pg-operand
docker run --rm --network none --memory 256m --memory-swap 256m -v "$PWD/deploy/pg-operand:/check:ro" bex-pg:18 \
  bash /check/check.sh 18 "$(jq -r '."18".extensions | join(",")' deploy/pg-operand/matrix.json)" \
  "$(jq -r '."18".admitted | join(",")' deploy/pg-operand/matrix.json)" 18.7
```

`TestApplicationDatabaseExtensionsOnCNPG` (`BEX_CNPG_KUBECONFIG`, `BEX_PG_OPERAND_IMAGES="18=<image>"`) repeats the core of the gate through bex's real Cluster projection on a real CloudNativePG.

## Restore consumer

The image installs `bex-postgres-restore` for an extracted operand export. Pass the directory containing `toc.dat`; its parent must contain the original `pg_ivm_restore.sql` sidecar, including an empty sidecar for a database without IMMVs. The target database must already exist. Normal libpq credentials select the actor, and `PGDATABASE` selects the target:

```sh
PGDATABASE=orders_restored bex-postgres-restore \
  "/work/extracted-export/orders_prod"
```

The consumer renders the complete dump with `pg_restore --no-owner --no-privileges`, then passes that SQL and the unchanged sidecar to one `psql --single-transaction --set=ON_ERROR_STOP=1` invocation. It preserves pg_dump's `restrict`/`unrestrict` commands and the caller's database identity. `TMPDIR` needs space for the rendered SQL; the consumer removes only its own temporary directory on success or failure.

**Current limits:** TimescaleDB and large-object archives are rejected before connecting to the target. Timescale's raw restore needs a supported ordinary-owner protocol; pg_restore's large-object SQL contains separate commits that would break the transaction containing the sidecar. The consumer never omits those datasets or substitutes a privileged actor. Its container-free command/failure controls (`restore_test.py`) do not qualify SQL rollback. The gate runs it as the owner on every extension's export, an IMMV's maintenance included (property 5 above, on both architectures), and `TestApplicationDatabaseExtensionsOnCNPG` runs it on a real CloudNativePG (w21/124). Complete Timescale/large-object restore remains open.

## Publish, then pin

The workflow publishes `ghcr.io/bex-co/bex-operator:<pg version>-postgres-<commit>` (the public package the CSI derivative also uses; CNPG reads the major from the tag's leading version) for `linux/amd64` and `linux/arm64`. It signs the image and attests an SBOM. It never pins. Adopting a digest is a reviewed commit to `bexOperandImages` in `lego/operator/internal/controller/database_operand.go`. That commit restarts every managed cluster of the major once, so it follows ADR092 D4: off-peak, health checks before and after each cluster, and the previous pin kept as the rollback.

Container-free protocols: `python3 -B -m unittest discover -s deploy/pg-operand -p '*_test.py'`. The workflow runs these before its image matrix. Cosign 3.1.4 and Syft 1.55.0 are pinned tool inputs; the existing signing/SBOM policy is unchanged. No vulnerability-audit or native signing-config migration is added. The current single SBOM command must not be described as proving both architectures; both architecture runtime gates remain required.

The first renewed canonical PG13/arm64 local attempt stopped at its 4GiB disk-cap margin during package installation (164 seconds; conservative peak4,069,052,976 bytes). It produced no output image and ran no SQL. Its nonce-owned builder, state volumes and newly pulled auxiliary image were removed successfully. This resource stop is distinct from a package, SQL or owner-restore failure; none of those outcomes was reached by that attempt.
