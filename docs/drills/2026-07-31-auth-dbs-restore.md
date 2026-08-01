# Auth database Barman Cloud restore drill — 2026-07-31

**Scope:** w7/m67 t004, production application cluster

**Outcome:** PASS — fresh plugin backups for `auth/kratos-db`, `auth/hydra-db`, and `auth/openfga-db` landed under distinct PostgreSQL-18 archive identities. `kratos-db` then restored from object storage into a new throwaway Cluster; a known pre-existing identity was present exactly once, and every recovery resource was removed afterward. No source Cluster was overwritten or repointed.

**Versions:** Kubernetes v1.34.9; CloudNativePG 1.30.0; Barman Cloud plugin/sidecar v0.13.0; PostgreSQL 18.4.

The drill ran during the evening of July 31 in the operator timezone; Kubernetes timestamps below are UTC on 2026-08-01. No kubeconfig, Secret data, database credential, identity UUID, email address, token, or private key is recorded here. Object-store paths and archive identities are non-secret routing metadata.

## Backup activation

| Cluster | ObjectStore / serverName | Initial Backup | Evidence |
| --- | --- | --- | --- |
| `kratos-db` | `auth/auth-dbs` / `kratos-db-pg18` | `kratos-db-m67-initial`; `completed`; `05:21:16Z`–`05:21:20Z`; backup id `20260801T052116` | `s3://bex-tfstate/auth-dbs/kratos-db-pg18/` had 2 `base/` objects and 1 `wals/` object |
| `hydra-db` | `auth/auth-dbs` / `hydra-db-pg18` | `hydra-db-m67-initial`; `completed`; `05:21:16Z`–`05:21:21Z`; backup id `20260801T052116` | `s3://bex-tfstate/auth-dbs/hydra-db-pg18/` had 2 `base/` objects and 1 `wals/` object |
| `openfga-db` | `auth/auth-dbs` / `openfga-db-pg18` | `openfga-db-m67-initial`; `completed`; `05:21:17Z`–`05:21:21Z`; backup id `20260801T052117` | `s3://bex-tfstate/auth-dbs/openfga-db-pg18/` had 2 `base/` objects and 1 `wals/` object |

Each source Cluster was 2/2 Ready with `ContinuousArchiving=True` after plugin activation. Kratos, Hydra, and OpenFGA stayed 2/2 Ready with zero unavailable replicas. The Barman plugin took all three initial base backups from a standby, leaving each primary serving.

The committed `ScheduledBackup` resources cover the recurring path at 04:15, 04:30, and 04:45 UTC. The initial on-demand `Backup` resources above remain as immediately inspectable Kubernetes evidence; object data is governed by the `auth/auth-dbs` 7-day retention policy.

## Kratos recovery

| Evidence | Value |
| --- | --- |
| Source / restore | `auth/kratos-db` / `auth/kratos-db-m67-restore` |
| Recovery source | plugin external-cluster reference to `auth/auth-dbs`, `serverName: kratos-db-pg18` |
| Storage isolation | new 5 Gi `hcloud-volumes` PVC; no source PVC reuse |
| Created / Ready | `05:24:38Z` / `05:25:40Z` (62 seconds) |
| Restored image | `ghcr.io/cloudnative-pg/postgresql:18.4-system-trixie` |
| Data verification | one source identity UUID captured without printing or recording it; recovered `count(*) WHERE id = <known UUID>` = 1; recovered total identities = 35 |

The recovery Cluster used `bootstrap.recovery` and a plugin `externalClusters` source. It did not carry a backup plugin block of its own, so it could not write into the source archive identity. PostgreSQL reached `database system is ready to accept connections`; the Cluster reported one Ready instance and `Cluster in healthy state` before the verification query ran.

This representative restore covers the shared transport and CNPG-I recovery mechanism used by all three auth databases. Hydra and OpenFGA were each independently proven to archive WAL and complete a base backup; repeating the identical recovery shape for their schemas would add production churn without testing another mechanism.

## Alert and cleanup evidence

Prometheus discovered one `cnpg-platform-db` primary target for each of `bex-db`, `kratos-db`, `hydra-db`, and `openfga-db`, stamped with namespace and cluster labels. All four were `up`; the three auth archive-age series were under three minutes old. `PlatformDatabaseBackupStale` was `ok`, `inactive`, and had zero active alerts. Its promtool fixture independently proves that a 27-hour timestamp fires one alert per cluster and a one-hour timestamp does not.

Cleanup deleted `auth/kratos-db-m67-restore` and then verified that no matching Cluster, Pod, PVC, PV, Service, or generated Secret remained. The three source Clusters ended 2/2 Ready, and Kratos, Hydra, and OpenFGA remained 2/2 Ready with zero unavailable replicas.
