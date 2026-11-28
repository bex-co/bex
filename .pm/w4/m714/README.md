# w4 · m714 — Managed Postgres, Key Value and registry image volumes are encrypted at rest, as Render's are

**Worker:** worker4 **Goal:** every Postgres instance (primary, HA standby, read replica), every persistent Key Value instance and the registry that holds every service's OCI images keep their data on a LUKS-encrypted volume with its own passphrase, new ones from the start and existing ones after a migration, and ADR018 says so, as Render promises AES-256 at rest for all three. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | An encrypted StorageClass and a per-volume passphrase for datastore volumes | 1h | — |
| [t002](t002.md) | A new Postgres's volumes are encrypted: primary, standby, read replicas, restores | 1h15m | t001 |
| [t003](t003.md) | A new Key Value's volume is encrypted | 45m | t001 |
| [t004](t004.md) | The registry's images move to an encrypted volume | 1h15m | t001 |
| [t005](t005.md) | Existing Postgres volumes move to the encrypted class, one switchover each | 1h30m | t002 |
| [t006](t006.md) | Existing Key Value volumes move to the encrypted class | 1h | t003 |
| [t007](t007.md) | ADR018, ADR009, ADR021 and ADR082 state encryption at rest | 30m | t002, t003, t004 |
| [t008](t008.md) | Live: on production, every datastore and registry volume is encrypted | 40m | t004, t005, t006, t007 |
| [t009](t009.md) | Simplify | 15m | t008 |
| [t010](t010.md) | Test coverage | 30m | t008, t009 |
| [t011](t011.md) | Closeout | 10m | t010 |

Total: **11 tasks, about 8h50m**.

## Definition of done

- **New Postgres:** a new database's every PVC (the primary, an HA standby, each read-replica cluster, a restore-to-new or PITR recovery, and an instance CNPG adds later to replace one) is on an encrypted StorageClass, and its passphrase Secret exists before the instance pod mounts. The node shows a `crypt` mapping under the mount (`lsblk -o NAME,TYPE,MOUNTPOINT`).
- **New Key Value:** a new paid instance's `data-<name>-0` PVC is on the encrypted class with its passphrase Secret. A grow still applies online (`allowVolumeExpansion`).
- **Passphrases:** one random passphrase per volume, in the datastore's own namespace, as [ADR082](../../../docs/ADR082-persistent-disks.md) D3 does for service disks. The operator never rewrites one, and never deletes one while its PVC exists. A missing or empty key fails closed, as `ensureDiskPassphrase` does (`lego/operator/internal/controller/disk.go:366`).
- **Registry:** the Zot PVC in `bex-registry` is on the encrypted class, every image that existed before the move still pulls, and a new build pushes and deploys.
- **Existing volumes:** on production no Database or KeyValue PVC and no registry PVC is left on plain `hcloud-volumes`. Each Postgres moved with at most one CNPG switchover of downtime and no data loss. Each Key Value kept its keys.
- **Local:** the CAPD mock and every `dev-N` still create Postgres and Key Value instances (the class there may alias `local-path`, which cannot encrypt). `scripts/gitops-validate.sh` pins production to the encrypted class.
- **Docs:** [ADR018 § Managed Postgres](../../../docs/ADR018/managed-postgres.md) and [ADR018 § Other datastores & storage](../../../docs/ADR018/other-datastores-storage.md) state AES-256 encryption at rest for Postgres (primary, replicas, backups), Key Value and OCI images, with Render's citations. The `hipaa-best-practices` row in [docs-pages.md](../../../docs/render-artifacts/docs-pages.md) no longer lists encryption at rest as a gap.

## Source + Goal linkage

- **Source:** promoted from [w4/747](../done/747.md) on 2026-11-28 by `/loopx w4`. That note came from w4/m688 t002, a full read of Render's hipaa-best-practices page on 2026-11-27.
- **Render (read 2026-11-28):**
  - [hipaa-best-practices](https://render.com/docs/hipaa-best-practices): "Render provides minimum AES-256 encryption at rest for database systems and OCI images", and "minimum AES-256 encryption for databases, backups, and secrets".
  - [postgresql-creating-connecting](https://render.com/docs/postgresql-creating-connecting): "Render Postgres databases are encrypted at rest using AES-256 data encryption. This encryption applies to both primary and replica instances, along with all backups."
  - [key-value](https://render.com/docs/key-value): persisted data "is encrypted at rest using AES-256 encryption", and "Data persistence is not available for free Key Value instances".
- **Evidence (`origin/main` `ff123f31b6`):**
  - `dbStorageClass = "hcloud-volumes"` (`lego/operator/internal/controller/database_controller.go:76`) is every CNPG Cluster's `storage.storageClass` (`cnpgClusterSpec`, line 529).
  - `kvStorageClass = "hcloud-volumes"` (`lego/operator/internal/controller/keyvalue_controller.go:60`) is the Valkey StatefulSet's claim class (line 913), set once at create.
  - `deploy/gitops/base/disk-storageclass.yaml` says "Hetzner Cloud volumes are not encrypted by the provider" and gives LUKS only to service disks (`hcloud-volumes-luks`, `${pvc.name}-luks`); production disks use `hcloud-volumes-luks-bounded` (`lego/operator/config/prod/kustomization.yaml:131`).
  - The Zot chart values in `deploy/gitops/base/zot.yaml` set `persistence: true` and `pvc.storage: 100Gi` with no class, so the registry takes the cluster default, plain `hcloud-volumes`.
  - `scripts/dev-env/hcloud-volumes-storageclass.yaml` aliases `hcloud-volumes` onto `local-path` for dev-N, and `scripts/dev-env.test.sh:556` asserts both operator constants equal that alias.
  - No ADR018 row states encryption at rest for Postgres, Key Value or images. [docs-pages.md](../../../docs/render-artifacts/docs-pages.md)'s `hipaa-best-practices` row lists it as the gap `w4/747`.
- **Goal linkage:** Render parity (managed data and security), [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first).
- **Expected outcome:** a tenant's database rows, cached keys and built images rest on encrypted volumes in every workspace, as on Render, and a stolen or mis-recycled Hetzner volume yields ciphertext.
- **Why now:** Render's security and HIPAA pages promise it, bex's service disks and backups already have it, and every new database created before this ships adds a volume the migration must move later.
- **Render parity task omitted:** the change has no REST, GraphQL, MCP or dashboard surface. Render's API and dashboard show no encryption state either, so there is nothing to compare there. The parity record is t007's ADR018 rows.

## Notes for the worker

- **CNPG's PVC names change.** CNPG names an instance's PVC `<cluster>-<serial>`, and the serial grows when it replaces an instance or adds a standby, so a class templated on `${pvc.name}-luks` needs a Secret for a name the operator cannot know at Cluster create. Two ways out, for t001 to pick and record: mint `<pvc>-luks` when a PVC labelled `cnpg.io/cluster` appears (the kubelet retries the mount until the Secret exists), or a datastore class whose node-publish secret name is `${pvc.annotations['<key>']}` (external-provisioner templates PVC annotations for node-publish secrets), set through the Cluster's `inheritedMetadata`. Either way one passphrase per volume, never shared across tenants.
- **Existing Clusters.** The operator reapplies each Cluster's spec on every reconcile. Check whether CNPG accepts a changed `storage.storageClass` on an existing Cluster and applies it only to new PVCs, before flipping the class for all of them (t002).
- **No maintenance scheduling.** Render's `/maintenance` runs are a recorded non-goal ([ADR018 gap backlog](../../../docs/ADR018/gap-backlog.md)), so t005 and t006 are platform operations: one database or instance at a time, bex's own fixture workspaces (`scripts/lib/first_party_workspaces.txt`) first.
- **Out of scope:** the platform's own volumes (`deploy/gitops/charts/bex-postgres`, `auth-dbs`, OpenBao, Loki). They are not tenant datastores; file them separately if wanted.
