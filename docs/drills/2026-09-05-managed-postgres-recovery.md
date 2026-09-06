# Managed Postgres recovery drill — 2026-09-05

Why: exercise provisioning RBAC and the platform recovery projection that direct CNPG restore scripts bypassed (w7/036 and w7/039).

Ran `TestManagedPostgresRecoveryDrill` against hetzner-prod in workspace `tea-d98210cbbpdc73dcrkvg`, following [ADR031](../ADR031-platform-data-backup.md#managed-database-recovery-drill). Both runs passed.

| Run | Disposable source | Disposable target | Total including cleanup |
| --- | --- | --- | --- |
| Initial | `dpg-dae9b8a9086m5ju6h7vg` | `dpg-dae9b8a9086m5ju6h800` | 579.73 s |
| Final verifier | `dpg-dae9gh29086n38lnjve0` | `dpg-dae9gh29086n38lnjveg` | 453.37 s |

The final run reached source Ready in 2m6s, completed a fresh plugin backup in 1m21s, and reached target Ready in 2m30s. Selecting the synthetic marker from the recovered database returned the exact source ID. Recovery specified only `sourceDatabase`, with no explicit archive-generation override.

All five provisioning artifacts passed: connection Secret, delegated Barman Role, namespace ObjectStore, its referenced backup credential, and managed-database quota charge. Cleanup completed both archive-purge finalizers and verified both Clusters and their PVCs disappeared. Afterwards the workspace contained only its original three databases, all Ready, and quota usage returned to three. No tenant records or credentials were printed.

The associated GitOps RBAC review checked the Barman patch, bex-api namespace delegation, operator day-to-day Role permissions, and tenant role definitions. Barman retains the Secret get/list/watch verbs it delegates; bex-api explicitly holds bind for its six named tenant ClusterRoles; the day-to-day operator role does not create delegated Roles. No equivalent local removal of upstream Secret grants was found for CNPG, cert-manager, or kpack. Live authorization checks confirmed Barman can get the tenant backup Secret and bex-api can bind the tenant operator role. This is a scoped delegation review, not a general security audit.

Repeat on ADR031's tenant Postgres cadence and after recovery projection, backup RBAC, or plugin changes. Completed Backup rows remain backup-production evidence; successful exact-marker restores provide recovery evidence. Automatic restoration of every backup is not introduced by this drill.
