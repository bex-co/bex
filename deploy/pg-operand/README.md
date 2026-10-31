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
| pgextwlist v1.20 (the allowlist) | 13–18 | PGDG |

`matrix.json` is the per-major recipe: the digest-pinned base, the PGDG packages, and the allowlist the gate exercises. The operator's `pgExtensionAllowlist` must match it (`TestPGExtensionAllowlistMatchesTheImageGate`).

Notes on the recipe:

- **Bookworm, not bullseye.** bex's earlier pins were CNPG's bullseye images. PGDG has retired bullseye, and its archive has no PostGIS for 18 or TimescaleDB for 13. bex sets no locale, so CNPG initializes every cluster with `C`, whose order does not depend on glibc. A database someone created with a glibc locale would still need a reindex check; the rollout verifies none exist (w8/m92 t001).
- **The server never moves.** The recipe holds the base's server and client packages, and fails if an extension dependency would change them.
- **PostGIS 3.6.4 on 14 and 15.** PostGIS 3.7 split the tiger geocoder into its own package, which PGDG builds only for 16+. 13 is already on 3.6.
- **Owner grants.** pgextwlist runs a whitelisted extension's script as the bootstrap superuser, so its objects belong to that superuser. `extwlist/<extension>/after-create.sql` grants the database owner the narrow rights an ordinary app needs: custom SRIDs in `spatial_ref_sys`, topology and geocoder metadata, and `pg_partman`'s `part_config`. No TimescaleDB catalog is granted, because its job table names the role that runs each job.

## The gate

`check.sh` runs inside the pushed digest, per platform, with networking disabled. It configures a throwaway server the way the operator projects a cluster (`pgExtensionParameters`) and then checks, as a non-superuser database owner:

- every allowlisted extension is created and does real work: a GiST spatial query, a raster, a topology, address normalization, an incremental view, `pg_partman` partitions and maintenance, a hypertable;
- untrusted extensions not on the list (`file_fdw`, `dblink`, `adminpack`) still fail with Postgres's normal error, and trusted ones (`hstore`) still work;
- the owner cannot change `extwlist.extensions` by `SET` or `ALTER ROLE`, and a non-owner cannot use the allowlist;
- TimescaleDB refuses compression, the owner cannot change its license, and even a superuser cannot switch to the TSL license;
- no `pg_partman` background worker runs, and the owner is still not a superuser afterwards;
- `pg_dump` and a `pg_restore` as the owner round-trip every extension's data.

The renewed gate retains the complete hypertable and original IMMV. Operand exports add `pg_ivm_restore.sql`, captured against the same snapshot as the dump. Metadata/dump failures prevent a successful archive. Restore applies this metadata and proves subsequent writes maintain the original view. A package without the required metadata functions fails explicitly.

Ordinary-owner complete restore is mandatory and attempted first. Platform bootstrap recovery is labeled separately and cannot satisfy that owner criterion or grant tenant catalog privileges. The initial ARM six-major proof used CNPG 1.27, deleted the hypertable before restore and accepted the IMMV as a plain table; it does not establish these renewed gates. The new CNPG test requires the pinned 1.30.1 controller on an isolated loopback API, keeps actual Free 256Mi resources and requires all six image inputs unless `BEX_CNPG_MAJORS` explicitly narrows a diagnostic run. PG13's actual admission/runtime support must be observed, not inferred.

DuckDB on 16+ uses the existing owner as `duckdb.postgres_role`,32MB per connection and one thread on Free. The image gate runs with 256Mi,64MB shared buffers and two concurrent real scans, recording backend private pages and cgroup peak. External access and extension auto-install/load are disabled. These settings and source tests are not completed runtime or parity qualification.

Locally:

```sh
docker build --build-arg BASE_IMAGE="$(jq -r '."18".base' deploy/pg-operand/matrix.json)" \
  --build-arg DUCKDB_IMAGE="$(jq -r '."18".duckdb' deploy/pg-operand/matrix.json)" \
  --build-arg PG_MAJOR=18 --build-arg PGDG_PACKAGES="$(jq -r '."18".packages' deploy/pg-operand/matrix.json)" \
  -t bex-pg:18 deploy/pg-operand
docker run --rm --network none -v "$PWD/deploy/pg-operand:/check:ro" bex-pg:18 \
  bash /check/check.sh 18 "$(jq -r '."18".extensions | join(",")' deploy/pg-operand/matrix.json)" 18.6
```

`TestApplicationDatabaseExtensionsOnCNPG` (`BEX_CNPG_KUBECONFIG`, `BEX_PG_OPERAND_IMAGES="18=<image>"`) repeats the core of the gate through bex's real Cluster projection on a real CloudNativePG.

## Publish, then pin

The workflow publishes `ghcr.io/bex-co/bex-operator:<pg version>-postgres-<commit>` (the public package the CSI derivative also uses; CNPG reads the major from the tag's leading version) for `linux/amd64` and `linux/arm64`. It signs the image and attests an SBOM. It never pins. Adopting a digest is a reviewed commit to `bexOperandImages` in `lego/operator/internal/controller/database_operand.go`. That commit restarts every managed cluster of the major once, so it follows ADR092 D4: off-peak, health checks before and after each cluster, and the previous pin kept as the rollback.

Container-free protocols: `python3 -B -m unittest discover -s deploy/pg-operand -p '*_test.py'`. The workflow runs these before its image matrix. Cosign 3.1.4 and Syft 1.55.0 are pinned tool inputs; the existing signing/SBOM policy is unchanged. No vulnerability-audit or native signing-config migration is added. The current single SBOM command must not be described as proving both architectures; both architecture runtime gates remain required.

The first renewed canonical PG13/arm64 local attempt stopped at its 4GiB disk-cap margin during package installation (164 seconds; conservative peak4,069,052,976 bytes). It produced no output image and ran no SQL. Its nonce-owned builder, state volumes and newly pulled auxiliary image were removed successfully. This resource stop is distinct from a package, SQL or owner-restore failure; none of those outcomes was reached by that attempt.
