# Barman Cloud plugin PITR drill — 2026-07-28

**Scope:** w1/m56 t007, production application cluster

**Outcome:** PASS — fresh plugin backups for a disposable tenant Database and `bex-system/bex-db` both restored to new targets at explicit timestamps. Each restore contained its pre-target marker and excluded its post-target marker. No source Cluster was overwritten.

**Versions:** Kubernetes v1.34.9; CloudNativePG 1.30.0; Barman Cloud plugin/sidecar v0.13.0; tenant PostgreSQL 16; bex-db PostgreSQL 18.

No kubeconfig, Secret data, database credentials, token, or private key is recorded here. Object-store locations are non-secret routing metadata; credential references remained the existing out-of-band `bex-db-backup-s3` Secrets.

## Tenant Database

| Evidence | Value |
| --- | --- |
| Source / restore | `default/dpg-m56t007src0000000000` / `default/dpg-m56t007rst0000000000` |
| ObjectStore / server | `default/bex-tenant-postgres` / `dpg-m56t007src0000000000` |
| Object prefix | `s3://bex-tfstate/postgres/dpg-m56t007src0000000000/` |
| Plugin Backup | `dpg-m56t007src0000000000-m56base`, `completed`; started `2026-07-28T09:25:18Z`, stopped `09:25:31Z`; catalog id `20260728T092517` |
| PITR target | `2026-07-28T09:26:04.689Z` |
| Markers | pre committed `09:24:46.700034Z`; post committed `09:26:14.988349Z` |
| Restored result | `pre_count=1`, `post_count=0`, `pg_is_in_recovery=false` |

The backed-up `basic-256mb` source reached `Ready` with a plugin sidecar, `BackupsEnabled=true`, server name equal to the Database id, and `ContinuousArchiving=True`. The base backup completed through `method: plugin`. The post-target transaction and an independent `pg_switch_wal()` archived WAL `000000010000000000000007` at `09:28:49Z`.

The first recovery attempt correctly refused to promote at the end of WAL 6 because the original switch had shared the post-marker's implicit transaction and its commit record therefore landed in WAL 7. After the independent switch archived WAL 7, CNPG's bounded Job retry selected backup `20260728T092517`, replayed through the explicit target, and produced a healthy new Database. This is useful drill evidence: a PITR target needs an archived commit after the target, not merely a base backup plus a switch issued inside the same transaction.

The restore was deleted first. The source stayed `Ready`, retained both source rows, and continued archiving throughout. Source deletion then completed its normal object-prefix purge finalizer. The remaining Backup CR was deleted explicitly; no matching Database, CNPG Cluster, ScheduledBackup, Backup, PVC, Job, or Pod remained.

## Control-plane bex-db

| Evidence | Value |
| --- | --- |
| Source / restore | `bex-system/bex-db` / `bex-m56-t007-drill/bex-db-m56-t007-restore` |
| ObjectStore / server | `bex-system/bex-db` (copied as a credential-reference-only drill ObjectStore) / `bex-db` |
| Object prefix | `s3://bex-tfstate/bex-db/bex-db/` |
| Plugin Backup | `bex-db-m56-t007-base`, `completed`; started `2026-07-28T09:35:41Z`, stopped `09:35:47Z`; plugin backup id `20260728T093541` |
| PITR target | `2026-07-28T09:36:21.073Z` |
| Markers | pre committed `09:35:34.209905Z`; post committed `09:36:32.494832Z` |
| Restored result | `pre_count=1`, `post_count=0`, `tenants=15`, `schema_migrations=1`, `pg_is_in_recovery=false` |

The source remained 2/2 Ready with `ContinuousArchiving=True`. The plugin ran the base backup from standby `bex-db-2`, preserving primary service, and archived the post-target WAL `0000000D0000000C0000003B` at `09:36:34.694764Z`. The new Cluster restored in the dedicated `bex-m56-t007-drill` namespace from an explicit plugin external-cluster reference. It recovered the known marker plus the expected control-plane schema/data counts while excluding the post-target marker.

Cleanup dropped only the drill schema from the source, deleted the drill Backup CR, and deleted the dedicated namespace. Its CNPG Cluster, ObjectStore, copied Secret, RBAC, Pod, PVC, and dynamically provisioned PV were all gone. The source ended 2/2 Ready with continuous archiving, the drill schema absent, the same `tenants=15` and `schema_migrations=1` counts, `https://api.bex.co/healthz` returning `ok`, and the platform/Barman/bex-postgres Argo Applications Synced and Healthy. The completed physical backup remains under the ObjectStore's normal 7-day retention as production recovery inventory.

## Gate result

w1/m56 t008 may now remove active native `barmanObjectStore` compatibility paths. Historical drill records may retain the old field names as evidence of what they exercised; current manifests, validation, and runbook instructions must remain plugin-only.
