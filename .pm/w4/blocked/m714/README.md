# w4 · m714 — Managed Postgres, Key Value and registry image volumes are encrypted at rest, as Render's are

**Worker:** worker4 **Goal:** every Postgres instance (primary, HA standby, read replica), every persistent Key Value instance and the registry that holds every service's OCI images keep their data on a LUKS-encrypted volume with its own passphrase, new ones from the start and existing ones after a migration, and ADR018 says so, as Render promises AES-256 at rest for all three. **Status:** blocked (t001–t003 done: new Postgres and Key Value volumes claim `BEX_DATASTORE_STORAGE_CLASS`, production's `hcloud-volumes-luks`, each with its own passphrase; moving the volumes made before that, and the registry's, waits for the user's sign-off, see Parked below)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | An encrypted StorageClass and a per-volume passphrase for datastore volumes — **DONE** | 1h | — |
| [t002](done/t002.md) | A new Postgres's volumes are encrypted: primary, standby, read replicas, restores — **DONE** | 1h15m | t001 |
| [t003](done/t003.md) | A new Key Value's volume is encrypted — **DONE** | 45m | t001 |
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
- **Passphrases:** one random passphrase per volume, in the datastore's own namespace, as [ADR082](../../../../docs/ADR082-persistent-disks.md) D3 does for service disks. The operator never rewrites one, and never deletes one while its PVC exists. A missing or empty key fails closed, as `ensureDiskPassphrase` does (`lego/operator/internal/controller/disk.go:366`).
- **Registry:** the Zot PVC in `bex-registry` is on the encrypted class, every image that existed before the move still pulls, and a new build pushes and deploys.
- **Existing volumes:** on production no Database or KeyValue PVC and no registry PVC is left on plain `hcloud-volumes`. Each Postgres moved with at most one CNPG switchover of downtime and no data loss. Each Key Value kept its keys.
- **Local:** the CAPD mock and every `dev-N` still create Postgres and Key Value instances (the class there may alias `local-path`, which cannot encrypt). `scripts/gitops-validate.sh` pins production to the encrypted class.
- **Docs:** [ADR018 § Managed Postgres](../../../../docs/ADR018/managed-postgres.md) and [ADR018 § Other datastores & storage](../../../../docs/ADR018/other-datastores-storage.md) state AES-256 encryption at rest for Postgres (primary, replicas, backups), Key Value and OCI images, with Render's citations. The `hipaa-best-practices` row in [docs-pages.md](../../../../docs/render-artifacts/docs-pages.md) no longer lists encryption at rest as a gap.

## Source + Goal linkage

- **Source:** promoted from [w4/747](../../done/747.md) on 2026-11-28 by `/loopx w4`. That note came from w4/m688 t002, a full read of Render's hipaa-best-practices page on 2026-11-27.
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
  - No ADR018 row states encryption at rest for Postgres, Key Value or images. [docs-pages.md](../../../../docs/render-artifacts/docs-pages.md)'s `hipaa-best-practices` row lists it as the gap `w4/747`.
- **Goal linkage:** Render parity (managed data and security), [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first).
- **Expected outcome:** a tenant's database rows, cached keys and built images rest on encrypted volumes in every workspace, as on Render, and a stolen or mis-recycled Hetzner volume yields ciphertext.
- **Why now:** Render's security and HIPAA pages promise it, bex's service disks and backups already have it, and every new database created before this ships adds a volume the migration must move later.
- **Render parity task omitted:** the change has no REST, GraphQL, MCP or dashboard surface. Render's API and dashboard show no encryption state either, so there is nothing to compare there. The parity record is t007's ADR018 rows.

## Notes for the worker

- **CNPG's PVC names change.** CNPG names an instance's PVC `<cluster>-<serial>`, and the serial grows when it replaces an instance or adds a standby, so a class templated on `${pvc.name}-luks` needs a Secret for a name the operator cannot know at Cluster create. Two ways out, for t001 to pick and record: mint `<pvc>-luks` when a PVC labelled `cnpg.io/cluster` appears (the kubelet retries the mount until the Secret exists), or a datastore class whose node-publish secret name is `${pvc.annotations['<key>']}` (external-provisioner templates PVC annotations for node-publish secrets), set through the Cluster's `inheritedMetadata`. Either way one passphrase per volume, never shared across tenants.
- **Existing Clusters.** The operator reapplies each Cluster's spec on every reconcile. Check whether CNPG accepts a changed `storage.storageClass` on an existing Cluster and applies it only to new PVCs, before flipping the class for all of them (t002).
- **No maintenance scheduling.** Render's `/maintenance` runs are a recorded non-goal ([ADR018 gap backlog](../../../../docs/ADR018/gap-backlog.md)), so t005 and t006 are platform operations: one database or instance at a time, bex's own fixture workspaces (`scripts/lib/first_party_workspaces.txt`) first.
- **Out of scope:** the platform's own volumes (`deploy/gitops/charts/bex-postgres`, `auth-dbs`, OpenBao, Loki). They are not tenant datastores; file them separately if wanted.

## Parked 2026-11-28 (`/loopx w4`)

- **New volumes ship encrypted (t001–t003).** Production's operator sets `BEX_DATASTORE_STORAGE_CLASS=hcloud-volumes-luks` (`lego/operator/config/prod/kustomization.yaml`). Every CNPG Cluster's `storage.storageClass` and every new Key Value claim template name it, and each claim gets its own `<pvc>-luks` passphrase (`lego/operator/internal/controller/volume_key.go`). The w2/m86 drill read that class's volumes as LUKS1 `aes-xts-plain64` with a 512-bit key, which is AES-256 in XTS mode. The CSI image's `cryptsetup --help` agrees: `LUKS: aes-xts-plain64, Key: 256 bits` and "Default keysize with XTS mode (two internal keys) will be doubled".
- **How a CNPG claim finds its Secret (t001's choice).** The operator mints `<pvc>-luks` as soon as a claim appears, with the PVC as its only owner (`DatastoreVolumeKeyReconciler`). It does not use a `${pvc.annotations[...]}` template. The class and its `${pvc.name}-luks` template stay the disks' own, already proven on production, and each volume gets its own passphrase. An annotation set through `inheritedMetadata` would give every instance of a Cluster the same one. Datastores keep the whole-volume class, not the disks' bounded one, so a database's filesystem stays the size of its provider volume, as on `hcloud-volumes`.
- **Existing Clusters (t002 step 3).** CloudNativePG 1.30.1's webhook refuses only a storage shrink (`validateStorageConfigurationChange`). Its claim reconciler updates only the size and the VolumeAttributesClass of an existing claim (`pkg/reconciler/persistentvolumeclaim/existing.go`), and it reads the class only when it builds a new claim (`build.go`). So every Cluster's spec flips to the new class, and only claims made after that take it: an existing database's next standby, replacement or replica is encrypted, and its live volumes stay on `hcloud-volumes`.
- **Not done: moving volumes made before the pin (t004–t006).** A claim's class is immutable, and the hcloud driver formats LUKS only on a new volume, so every existing volume would have to be rebuilt:
  - each Postgres: one more instance on the new class, a switchover to it, then CNPG destroys the old instance's claim (t005);
  - each Key Value: a snapshot, a StatefulSet recreated on a new claim, and a restore, with the store down meanwhile (t006);
  - the Zot registry in `bex-registry`: a push freeze, a copy onto a new claim, a swap of its StatefulSet, and a restart that every kubelet pull depends on (t004). `zot.yaml` cannot name the class until that swap runs, because the StatefulSet's claim template is immutable and Argo would fail the sync.

  Each move ends by deleting or detaching a live customer volume, and the Key Value and registry moves need customer-facing downtime. Render's `/maintenance` scheduling is a recorded non-goal. No migration code is written yet. This worker could not count the affected volumes either: this checkout has no `.env`, so `scripts/fetch-app-kubeconfig.sh` had no `HCLOUD_TOKEN`.
- **Docs (t007, half done).** [ADR018 § Managed Postgres](../../../../docs/ADR018/managed-postgres.md) and [§ Other datastores & storage](../../../../docs/ADR018/other-datastores-storage.md) each have an "Encryption at rest" row, ◐ on every surface. Each row cites Render and names the code that encrypts new volumes. ADR009 § Security floor, ADR021 § 5 and ADR082 D3 state the mechanism. The `hipaa-best-practices` row of [docs-pages.md](../../../../docs/render-artifacts/docs-pages.md) names this milestone for what is left. t007 turns both rows ✅ once t004–t006 have moved the old volumes.
- **Shipped in `050aa6bdb2`.**
- **Gate (the user, `/unblock w4`):** sign off on moving the volumes made before production carries this change, with the downtime each move costs, or have ADR018 record those volumes as a divergence. After that a worker builds t005 and t006, runs t004 and both of them on bex's own fixtures first, then on every other volume, then runs t007–t011.
  - Postgres: one CNPG switchover per database, a few seconds of writes refused and connections dropped.
  - Key Value: a restart with a restore, its length measured on dev-4 before the run.
  - The registry: a push freeze and a Zot restart.

  The pin part of the gate (`pin must carry`) has the next worker check new volumes on production first (t008 step 1): a disposable HA Postgres with a read replica and a paid Key Value in `bex-canary`, each claim on `hcloud-volumes-luks` with its Secret, and a `crypt` mapping on the node. That worker then parks the item again on the user's part alone.
