# Platform data-backup policy

**Why this exists:** bex has six platform stores plus tenant datastores holding state that is not recoverable from git alone: etcd, OpenBao, the control-plane database, the three auth databases, tenant Postgres, and managed Key Value. High availability protects a database from a node failure, not correlated storage loss. This document is the consolidated policy and restore runbook — the single place to check when something is on fire.

> Encryption-at-rest and per-store credential scoping for everything below are specified in [ADR050-encrypted-platform-backups.md](ADR050-encrypted-platform-backups.md); it modifies this policy without restating it.

| store | what it holds | backup mechanism | schedule | retention | restore mechanism | last verified |
| --- | --- | --- | --- | --- | --- | --- |
| **etcd** | App/Database/KeyValue CRs (user deployments) | CronJob `kube-system/etcd-backup` → `etcd-snapshots/` in `bex-tfstate` | 03:15 UTC daily | 7 snapshots (rolling) | `scripts/restore-etcd.sh` → local throwaway etcd → sanitized CR manifests | 2026-07-31 (production scripted extraction, w7/m69) |
| **OpenBao** | All tenant env-var secrets | CronJob `secrets/openbao-backup` → `openbao-snapshots/` in `bex-tfstate` | 03:45 UTC daily | 7 snapshots (rolling) | `scripts/restore-openbao.sh` → force-restore into a new isolated Raft target | 2026-07-31 (production scripted restore, w7/m69) |
| **paid KeyValue** | Tenant Valkey data for every non-Free managed Key Value | Operator-owned `kvbak-<id>` CronJob → `keyvalue/<id>/<RFC3339-UTC>.rdb.gz` in `bex-tfstate` | One stable per-id slot from 03:20–03:39 UTC daily | 7 snapshots (rolling) | `scripts/restore-keyvalue.sh` → fresh PVC, AOF-safe seed/rewrite/restart | 2026-07-31 (production scripted restore, w7/m69) |
| **tenant Postgres** | Tenant-managed relational data | Barman Cloud plugin → ObjectStore `default/bex-tenant-postgres`, one `serverName` per Database/major + continuous WAL | Daily base backup per backed-up Database; WAL continuous | 30 days of base backups + WAL | `scripts/restore-postgres.sh` → cloned ObjectStore contract and new recovery Cluster | 2026-07-31 (production scripted restore, PostgreSQL 16, w7/m69) |
| **bex-db** | Workspaces, members, audit log, usage, API keys, deploy history | Barman Cloud plugin → ObjectStore `bex-system/bex-db` → `bex-db/` in `bex-tfstate` + continuous WAL | 04:00 UTC daily (full base backup via plugin `ScheduledBackup`); WAL archiving is continuous | 7 days of base backups + WAL (ObjectStore retention) | `scripts/restore-postgres.sh` → new recovery Cluster | 2026-07-31 (production scripted marker restore, w7/m69) |
| **kratos-db** | User identities, credentials, verification/recovery state | Barman Cloud plugin → ObjectStore `auth/auth-dbs`, server `kratos-db-pg18` → `auth-dbs/kratos-db-pg18/` + continuous WAL | 04:15 UTC daily base backup; WAL continuous | 7 days of base backups + WAL | `scripts/restore-postgres.sh` → new recovery Cluster | 2026-07-31 (production scripted aggregate verification, w7/m69) |
| **hydra-db** | OAuth clients, grants, consent, access/refresh-token state | Barman Cloud plugin → ObjectStore `auth/auth-dbs`, server `hydra-db-pg18` → `auth-dbs/hydra-db-pg18/` + continuous WAL | 04:30 UTC daily base backup; WAL continuous | 7 days of base backups + WAL | `scripts/restore-postgres.sh`, same parameter shape as Kratos | 2026-07-31 (production backup + WAL verification, w7/m67) |
| **openfga-db** | Authorization stores, models, and tuples | Barman Cloud plugin → ObjectStore `auth/auth-dbs`, server `openfga-db-pg18` → `auth-dbs/openfga-db-pg18/` + continuous WAL | 04:45 UTC daily base backup; WAL continuous | 7 days of base backups + WAL | `scripts/restore-postgres.sh`, same parameter shape as Kratos | 2026-07-31 (production backup + WAL verification, w7/m67) |

The platform stores and paid KeyValue backups use the same Wasabi/Hetzner Object Storage bucket (`bex-tfstate`) under separate store/server prefixes. Credentials come from out-of-band Secrets (never in git), following the same pattern as `etcd-backup-s3` / `openbao-backup-s3`. Free KeyValue instances are deliberately PVC-only and have no off-cluster recovery point.

## Encryption + credential scoping (ADR050)

[ADR050-encrypted-platform-backups.md](ADR050-encrypted-platform-backups.md) adds two orthogonal protections over everything above; both are **opt-in** (unset ⇒ byte-identical to the pre-ADR050 behavior described in this doc):

- **Tier A — client-side `age` encryption** for the three pipelines bex controls directly (etcd, OpenBao, paid KeyValue). Each backup Job gains an `encrypt` step between `compress` and `upload`, age-encrypting to a committed **public** key so the S3 object (`*.gz.age`) is unreadable without the out-of-band private key. Enable per pipeline:
  - etcd / OpenBao (static CronJobs): create the optional `bex-backup-age` ConfigMap (key `AGE_PUBLIC_KEY`) in `kube-system` and `secrets` respectively.
  - paid KeyValue (operator-templated Job): set the operator env `BEX_BACKUP_AGE_PUBLIC_KEY`.
  - The private half is `AGE_BACKUP_PRIVATE_KEY` in `.env`/GitHub Actions (custodied like the OpenBao unseal keys); `scripts/restore-*.sh` source it to decrypt. OpenBao's Raft snapshot is already encrypted at rest, so its `age` layer is transport-only — a full OpenBao restore chains three secrets in order: reader S3 credential → `AGE_BACKUP_PRIVATE_KEY` → original `BAO_UNSEAL_KEY_1/2/3`.
- **Tier B — provider SSE** (`encryption: AES256`) on the three Barman `ObjectStore` CRs, since bex does not control that upload pipeline. This is explicitly weaker (SSE decrypts transparently on an authenticated `GetObject`); the write-only credential below is what protects Tier B confidentiality against a leaked credential.
- **Write-only, per-store credentials** replace the shared `TF_STATE_*` root key in every backup Secret. `scripts/backup-s3-credentials.sh` (`provision`/`verify`/`revoke-legacy`, mirroring `static-s3-credentials.sh`) mints a `<store>-backup-writer` IAM identity (Put/Delete/List, **no** Get) per prefix for the Jobs, and a `<store>-backup-reader` (Get/List) recorded in `.env` and used **only** by the restore scripts. `restore-postgres.sh` builds the recovery Cluster's credential Secret from the reader identity rather than cloning the (now write-only) source Secret. Policies live in `infra/wasabi/backup-{writer,reader}-policy.json`.

## Barman Cloud plugin

The supported CNPG-I backup path is the [Barman Cloud Plugin](https://cloudnative-pg.io/plugin-barman-cloud/docs/intro/). bex vendors the exact upstream `v0.13.0` installation manifest (SHA-256 pinned in `deploy/gitops/charts/barman-cloud-plugin/README.md`) and installs it beside the CloudNativePG operator in `cnpg-system`; cert-manager supplies the client/server TLS certificates. The plugin controller runs on the platform pool, while its data-plane sidecar follows each CNPG instance pod's existing placement.

Three GitOps-managed `ObjectStore` resources preserve the transport contracts without containing credential bytes:

| ObjectStore | destination | retention | credential reference |
| --- | --- | --- | --- |
| `bex-system/bex-db` | `s3://bex-tfstate/bex-db` | 7d | `bex-system/bex-db-backup-s3` |
| `default/bex-tenant-postgres` | `s3://bex-tfstate/postgres` | 30d | `default/bex-db-backup-s3` |
| `auth/auth-dbs` | `s3://bex-tfstate/auth-dbs` | 7d | `auth/auth-dbs-backup-s3` |

All three references require only `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`; those Secrets remain provisioned out of band. Tenant and auth clusters share transport definitions but pass immutable per-database/per-major archive identities as the plugin's `serverName` parameter. This keeps one credential/config surface per retention/custody policy while preserving archive isolation.

New platform database archives use `serverName: <cluster>-pg<major>` (for example, `kratos-db-pg18`). A PostgreSQL major upgrade creates a new system identifier and WAL history, so the upgrade must atomically advance this suffix rather than append incompatible WAL to the old archive identity. `bex-db` retains the older plain `serverName: bex-db` because that production archive predates the convention; changing it without a major upgrade would strand its current PITR history.

The migration was deliberately staged: the plugin and ObjectStores landed first, then tenant Clusters and `bex-db` switched atomically to `barman-cloud.cloudnative-pg.io`. Fresh backup/PITR/restore drills passed for both paths on 2026-07-28, after which the native implementation and compatibility checks were removed. The three auth Clusters joined the same plugin path on 2026-07-31; all three completed fresh backups and a representative Kratos restore passed. Current manifests, validation, alerts, and runbook examples use only the plugin contract; the 2026-07-14 drill record below retains the former field names solely as historical evidence of what that drill exercised.

```mermaid
graph LR
  subgraph cluster["app cluster (Hetzner)"]
    etcd-cron["CronJob etcd-backup<br/>03:15 UTC"]
    bao-cron["CronJob openbao-backup<br/>03:45 UTC"]
    kv-cron["Paid KeyValue CronJobs<br/>03:20–03:39 UTC"]
    cnpg-sched["ScheduledBackup bex-db-nightly<br/>04:00 UTC + continuous WAL"]
    auth-sched["Auth DB ScheduledBackups<br/>04:15 / 04:30 / 04:45 UTC + continuous WAL"]
  end
  bucket[("bex-tfstate<br/>etcd-snapshots/<br/>openbao-snapshots/<br/>keyvalue/&lt;id&gt;/<br/>bex-db/<br/>auth-dbs/")]
  etcd-cron --> bucket
  bao-cron --> bucket
  kv-cron --> bucket
  cnpg-sched --> bucket
  auth-sched --> bucket
  op@{ shape: tri, label: "human operator" }
  op -->|"scripts/restore-*.sh"| cluster
  op -->|"fetch snapshot/backup"| bucket
```

## One-time setup per cluster

### etcd

See [ADR011-etcd-backup-restore.md](ADR011-etcd-backup-restore.md) §One-time setup. The `etcd-backup-s3` Secret in `kube-system` holds the object-store credentials.

### OpenBao

See [ADR015-openbao-backup-restore.md](ADR015-openbao-backup-restore.md) §One-time setup. The `openbao-backup-s3` Secret in `secrets` holds the object-store credentials.

### bex-db

Two out-of-band steps — the same trust boundary as etcd/openbao:

1. **Create the `bex-db-backup-s3` Secret** (reuses the Terraform-state Object Storage creds from `.env`, exactly like the etcd/openbao S3 secrets):

   ```sh
   source .env && kubectl -n bex-system create secret generic bex-db-backup-s3 \
     --from-literal=AWS_ACCESS_KEY_ID="$TF_STATE_ACCESS_KEY" \
     --from-literal=AWS_SECRET_ACCESS_KEY="$TF_STATE_SECRET_KEY"
   ```

2. **Verify WAL archiving has started** (CNPG starts archiving immediately once the Secret exists and the cluster reaches `Ready`):

   ```sh
   kubectl -n bex-system exec bex-db-1 -- psql -U postgres -c "SELECT * FROM pg_stat_archiver;"
   # last_archived_wal should be non-null and last_archived_time recent
   ```

3. **Trigger an on-demand full base backup** (optional — the ScheduledBackup fires at 04:00 UTC; trigger now to confirm end-to-end):

   ```sh
   kubectl -n bex-system apply -f - <<'EOF'
   apiVersion: postgresql.cnpg.io/v1
   kind: Backup
   metadata:
     name: bex-db-initial
     namespace: bex-system
   spec:
     method: plugin
     pluginConfiguration:
       name: barman-cloud.cloudnative-pg.io
     cluster:
       name: bex-db
   EOF
   kubectl -n bex-system get backup bex-db-initial -w
   # phase → completed
   ```

4. **Confirm the backup artifact landed in object storage**:

   ```sh
   source .env
   # Barman nests artifacts under <destinationPath>/<serverName>/ (serverName = cluster name "bex-db")
   aws --endpoint-url "$TF_STATE_ENDPOINT" s3 ls "s3://$TF_STATE_BUCKET/bex-db/bex-db/" --recursive | head -10
   # Expect: base/YYYYMMDDTHHMMSS/backup.info, base/.../data.tar.gz, wals/.../*.gz
   ```

**Note on `endpointURL`:** `deploy/gitops/charts/barman-cloud-objectstores/bex-db.yaml` has `endpointURL: https://s3.eu-central-2.wasabisys.com`. Update the ObjectStore to match `$TF_STATE_ENDPOINT` if the cluster uses a different provider (Hetzner: `https://fsn1.your-objectstorage.com`).

### Auth databases

The shared `auth/auth-dbs` ObjectStore references one out-of-band Secret. Provision it from the same backup-plane credentials used by the other critical stores; never commit or print them:

```sh
source .env && kubectl -n auth create secret generic auth-dbs-backup-s3 \
  --from-literal=AWS_ACCESS_KEY_ID="$TF_STATE_ACCESS_KEY" \
  --from-literal=AWS_SECRET_ACCESS_KEY="$TF_STATE_SECRET_KEY"
```

After Argo syncs `barman-cloud-objectstores` and `auth-dbs`, verify all three Clusters report `ContinuousArchiving=True` and each nightly schedule exists:

```sh
kubectl -n auth get cluster kratos-db hydra-db openfga-db
kubectl -n auth get scheduledbackup kratos-db-nightly hydra-db-nightly openfga-db-nightly
kubectl -n auth get backup
```

Artifacts live under `s3://bex-tfstate/auth-dbs/<cluster>-pg<major>/{base,wals}/`. The production clusters were PostgreSQL 18 when this policy shipped, so the active server names are `kratos-db-pg18`, `hydra-db-pg18`, and `openfga-db-pg18`.

### Paid KeyValue

The production operator config sets the non-secret `BEX_KV_BACKUP_DESTINATION=s3://bex-tfstate/keyvalue`, Wasabi endpoint, and Secret name `bex-kv-backup-s3`. Provision the credential in the apps namespace from the backup-plane identity without committing or printing it:

```sh
source .env && kubectl -n default create secret generic bex-kv-backup-s3 \
  --from-literal=AWS_ACCESS_KEY_ID="$TF_STATE_ACCESS_KEY" \
  --from-literal=AWS_SECRET_ACCESS_KEY="$TF_STATE_SECRET_KEY"
```

All three settings are required. With any unset, the operator adds no backup CronJob or finalizer to new KeyValue resources. With the contract complete, every non-Free instance gets `kvbak-<id>`; Free receives none. Verify names and schedules without reading either Secret value:

```sh
kubectl -n default get cronjob -l app.bex.co/component=keyvalue-backup
```

## Restore runbooks

The four `scripts/restore-*.sh` programs are the executable recovery truth; the prose here explains why each step exists. They share one contract:

- `DRY_RUN=1` may read the backup inventory, download and integrity-check a snapshot, inspect a local throwaway, and render the intended Kubernetes objects, but it never mutates Kubernetes or object storage.
- Kubernetes recovery targets must be new namespaces named `restore-*`. A real run requires `--confirm <that-exact-namespace>`; teardown verifies the namespace carries `bex.co/restore-target=true` before deleting it and waits for its absence.
- No script has an in-place/live restore mode. Production cutover is a separate, human-reviewed incident step after the recovered target is verified.
- Credential values come from environment, gitignored `.env`, or copied Secret streams; they never appear in output or argv. Verification query/path/key results are suppressed.
- A script change must re-earn a live drill record. CI supplies the complementary hermetic gate: ShellCheck plus PATH-shimmed `DRY_RUN`, corrupt-input, latest-selection, and bad-parameter tests.

ADR031 already owns the platform data-protection topic, so w7/m69 amended this ADR instead of creating a duplicate restore ADR. The all-store production evidence is [the 2026-07-31 scripted restore drill](drills/2026-07-31-scripted-restore-e2e.md).

Recovery remains deliberately human-initiated—there is no automatic disaster failover or cutover—but it is no longer prose-transcribed. Snapshot selection, integrity checks, isolated target construction, data verification, and teardown are committed and tested code.

### etcd restore

Run ADR011 Path A through the script. The default is read-only against S3 and uses only an ephemeral local Docker etcd; persisting manifests is explicit. Applying them requires a separately provisioned kube context whose name begins `restore-` and a second confirmation token:

```sh
DRY_RUN=1 scripts/restore-etcd.sh
scripts/restore-etcd.sh --output-dir /secure/recovered-crs
# After reviewing the files and provisioning a fresh cluster/context:
scripts/restore-etcd.sh --apply-dir /secure/recovered-crs \
  --target-context restore-new-cluster --confirm APPLY-restore-new-cluster
```

See [ADR011-etcd-backup-restore.md](ADR011-etcd-backup-restore.md) §Restore for why selective CR re-apply is preferred over full node-state recovery. ADR011 Path B remains an unscripted, node-identity-dependent emergency procedure.

### OpenBao restore

OpenBao's original Shamir keys live only in the masked GitHub secrets, so production uses the manual workflow; local/custodied-key operators can invoke the same script directly:

```sh
gh workflow run openbao-restore-drill.yml \
  -f target_namespace=restore-bao-incident \
  -f confirm=restore-bao-incident

# Equivalent direct preview/run when the four BAO_* values are in env/.env:
DRY_RUN=1 scripts/restore-openbao.sh \
  --target-namespace restore-bao-incident --verify-path tenants/data/<known-path>
scripts/restore-openbao.sh \
  --target-namespace restore-bao-incident --verify-path tenants/data/<known-path> \
  --confirm restore-bao-incident --teardown-on-success
```

The script creates a new one-member Raft target, initializes only that target, uses the target token for `snapshot-force`, restarts, unseals with the **original** keys, verifies a known `tenants/` path without printing it, and optionally tears down. It rejects `--live`/same-instance requests. See [ADR015](ADR015-openbao-backup-restore.md) for the master-key explanation.

### bex-db restore

CNPG recovery for `bex-db`, tenant Databases, and auth databases is one parameterized script. It clones the source ObjectStore and only its referenced credentials into a new namespace, creates a one-instance recovery Cluster without a backup/WAL-writer plugin, waits for Ready, runs a suppressed verification query, and optionally tears down:

```sh
DRY_RUN=1 scripts/restore-postgres.sh \
  --source-namespace bex-system --source-cluster bex-db \
  --object-store bex-db --server-name bex-db \
  --target-namespace restore-pg-bex-db \
  --database bex --query 'SELECT count(*) FROM schema_migrations' --expect 1

scripts/restore-postgres.sh \
  --source-namespace bex-system --source-cluster bex-db \
  --object-store bex-db --server-name bex-db \
  --target-namespace restore-pg-bex-db \
  --database bex --query '<known-row count query>' --expect 1 \
  --confirm restore-pg-bex-db --teardown-on-success
```

Add `--target-time <RFC3339>` for PITR; the script renders CNPG's `bootstrap.recovery.recoveryTarget.targetTime`. Never put personal data in the query or drill record. For a full cutover after verification, deliberately update the GitOps Cluster/connection Secret to the recovered authority; the restore script never performs cutover.

### Auth database restore

Use the same script with `--source-namespace auth`, `--object-store auth-dbs`, and the exact per-major server name:

```sh
scripts/restore-postgres.sh \
  --source-namespace auth --source-cluster kratos-db \
  --object-store auth-dbs --server-name kratos-db-pg18 \
  --target-namespace restore-pg-kratos \
  --database kratos --query 'SELECT count(*) FROM identities' --expect <captured-count> \
  --confirm restore-pg-kratos --teardown-on-success
```

The same shape covers Hydra and OpenFGA. It deliberately omits a backup plugin on the recovery Cluster, so the throwaway cannot write to the source archive identity.

### KeyValue restore

The KeyValue script selects the lexicographically newest RFC3339 RDB by default, checks gzip plus `valkey-check-rdb`, requires a new PVC, refuses surviving AOF paths, boots with AOF off, verifies a known key, enables/rewrites AOF, rolls the target, and verifies again:

```sh
DRY_RUN=1 scripts/restore-keyvalue.sh \
  --id red-<id> --target-namespace restore-kv-incident \
  --verify-key '<known-key>' --expect '<known-value>'

scripts/restore-keyvalue.sh \
  --id red-<id> --target-namespace restore-kv-incident \
  --verify-key '<known-key>' --expect '<known-value>' \
  --confirm restore-kv-incident --teardown-on-success
```

KeyValue backups are snapshots, not PITR. Valkey prefers an existing AOF over `dump.rdb`; the script's fresh-volume rule is therefore a safety invariant, not cleanup advice. [ADR021](ADR021-keyvalue-management.md) owns the mechanism details.

> **CNPG 1.31+ readiness:** PASS. The active Clusters, ScheduledBackups, on-demand Backup example, and recovery examples use the Barman Cloud plugin. The 2026-07-28 production drill restored tenant and control-plane data at explicit PITR targets, and the 2026-07-31 production drill restored Kratos from its new auth archive. The historical 2026-07-14 drill below used CNPG's former fields; do not copy that record as a current manifest.

## Drill records

### ADR050 Tier A encryption enable + restore — 2026-08-04 (w5/m63, production)

Enabled [ADR050](ADR050-encrypted-platform-backups.md) Tier A age encryption for etcd + OpenBao in prod (out-of-band `bex-backup-age` ConfigMap; private key in `.env`/CI). Manual `etcd-backup` and `openbao-backup` runs both completed on the prod nodes — the `encrypt` step (`apk add age` + `age -r`) exited 0 — and uploaded `etcd-…​.db.gz.age` / `openbao-…​.snap.gz.age`. A real `scripts/restore-etcd.sh` run then fetched the fresh `.age` object from Wasabi, decrypted it with the out-of-band private key, passed `etcdutl` status, restored into a throwaway etcd, and extracted 9 CRs (7 App, 1 Database, 1 KeyValue). Tier B SSE (`encryption: AES256`) confirmed live on all three `ObjectStore`s with backups still `completed`. KV encrypt step reconciled into the live `kvbak` CronJob. The pre-existing snapshot-connectivity issue is **closed** ([`.pm/w5/done/039.md`](../.pm/w5/done/039.md)): nightly `kvbak-red-…` Jobs have succeeded since 2026-08-17. The write-only per-store credential rotation (§3) remains built-but-not-enabled.

### Scripted all-store restore — 2026-07-31 (w7/m69, production)

Fresh backups and `scripts/restore-*.sh` passed for etcd, OpenBao, `bex-db`, a PostgreSQL 16 tenant Database, Kratos, and a paid Valkey 8 KeyValue. Marker-based freshness passed for `bex-db`, tenant Postgres, OpenBao, and KeyValue; auth verification used only an aggregate count. Every recovery target was new, every target/source drill resource was removed, the tenant Postgres and KeyValue object prefixes were empty after source finalization, and the platform ended healthy. See [2026-07-31-scripted-restore-e2e.md](drills/2026-07-31-scripted-restore-e2e.md).

### Paid KeyValue backup, AOF-aware restore, and delete purge — 2026-07-31 (w7/m68, production)

A paid throwaway KeyValue captured a marker through eight manually triggered CronJob runs. The eighth run deleted exactly the oldest object, the bucket retained the newest seven, and the newest RDB passed Valkey checksum validation. A fresh-PVC restore loaded the marker with AOF disabled for the seed; after AOF was enabled and rewritten, the marker survived a workload restart. Deleting the source removed its CronJob, backup/purge Jobs and Pods, StatefulSet, PVC/PV, Secrets, and complete `keyvalue/<id>/` prefix. See [2026-07-31-keyvalue-restore.md](drills/2026-07-31-keyvalue-restore.md).

### Auth database Barman Cloud restore — 2026-07-31 (w7/m67, production)

All three auth clusters completed plugin base backups and continuously archived WAL under unique `*-pg18` server names. `kratos-db` restored from the bucket alone into a new one-instance Cluster in 62 seconds; a known identity was present exactly once, 35 total identities were present, and all throwaway compute/storage resources were deleted. Kratos, Hydra, OpenFGA, and all three source clusters stayed fully Ready. See [2026-07-31-auth-dbs-restore.md](drills/2026-07-31-auth-dbs-restore.md).

### Barman Cloud plugin PITR — 2026-07-28 (w1/m56, production)

Fresh plugin backups and explicit point-in-time restores passed for both a disposable tenant Database and `bex-db`. Each new restore contained a pre-target marker and excluded a later marker; the `bex-db` restore also retained the expected control-plane schema and row counts. Sources stayed healthy and all throwaway compute/storage resources were removed. See the credential-free evidence record: [2026-07-28-barman-plugin-pitr.md](drills/2026-07-28-barman-plugin-pitr.md).

### etcd restore — 2026-07-14 (w7/m29, CAPD 2-node mock cluster)

**Date:** 2026-07-14 · **Environment:** CAPI CAPD cluster (2 nodes: 1 control-plane + 1 worker, kindest/node:v1.36.1, kubeadm etcd 3.6.8-0) · **Duration:** ~8 min · **Outcome:** ✅ pass

**Steps executed:**

1. Applied test App CR (`drill-test-app`, label `bex.co/drill: w7-m29-2026-07-14`) to the CAPD cluster.
2. Took an etcd snapshot using `registry.k8s.io/etcd:3.6.8-0` reaching etcd via the control-plane container's IP (192.168.158.4:2379) with TLS certs copied from the node. Snapshot: 5.4 MB uncompressed, 550 KB gzipped.
3. Ran `etcdutl snapshot restore` (same image) → started throwaway `etcd` container from the restored data dir.
4. Ran `etcdctl get /registry/app.bex.co/apps/default/drill-test-app --print-value-only | jq ...` — the App CR including the drill label was present and the `jq` pipeline produced clean JSON ready for `kubectl apply`.
5. Cleaned up.

**Deviations from runbook (corrected in [ADR011](ADR011-etcd-backup-restore.md)):**

- _Image version_: The CronJob pinned `registry.k8s.io/etcd:3.5.15-0` but the CAPD cluster runs `3.6.8-0`. More critically, `etcdutl` (required for `snapshot restore`) is **not** in `3.5.15-0` — only in `3.6.x+`. Runbook updated to use the cluster's actual image. CronJob image pinned to `3.6.8-0`.
- _CAPD vs prod_: On prod, the CronJob runs on the node itself via `hostNetwork` + `hostPath` certs. On CAPD (Docker-in-Docker), etcd is in a container, so the snapshot was taken by reaching the container IP directly. This difference is CAPD-specific; prod is unaffected.
- _`etcdctl` path_: In the `registry.k8s.io/etcd` image, `etcdctl` and `etcd` are at `/usr/local/bin/` — must use `--entrypoint /usr/local/bin/etcdutl` (or `etcd`) with `docker run` since the default entrypoint doesn't resolve them from PATH in `docker run --rm image <cmd>` without explicit entrypoint.

**`jq` pipeline:** Works exactly as documented — `--print-value-only` output IS valid JSON in etcd 3.6.x.

---

### OpenBao restore — 2026-07-14 (w7/m29, local Docker containers)

**Date:** 2026-07-14 · **Environment:** Two local `openbao/openbao:2.5.5` Docker containers (source + fresh restore target) · **Duration:** ~12 min · **Outcome:** ✅ pass (fresh-node path)

**Steps executed:**

1. Started "source" OpenBao container; initialized (1 unseal key, 1 threshold for simplicity); enabled KV v2 at `tenants/`; wrote test secret `tenants/default/drill-test-app/DATABASE_URL` with `drill_marker: w7-m29-2026-07-14`.
2. Took a Raft snapshot via `bao operator raft snapshot save` using root token.
3. Started a FRESH "restore" OpenBao container (different master key after its own init + unseal).
4. Ran `bao operator raft snapshot restore <snap>` → **FAILED** with `could not verify hash file... use the snapshot-force API to bypass`.
5. Ran `bao operator raft snapshot restore -force <snap>` with the FRESH instance's root token → success (no output = success for this command).
6. Restarted the container so Raft reloads from the restored log.
7. Unsealed with the ORIGINAL unseal key → instance came up active.
8. Read `tenants/default/drill-test-app/DATABASE_URL` with the ORIGINAL root token → `drill_marker: w7-m29-2026-07-14` and `value: postgres://drilltest:secret@db:5432/bex` confirmed present.

**Deviations from runbook (corrected in [ADR015](ADR015-openbao-backup-restore.md)):**

- _`--force` flag required for fresh-node restore_: The existing runbook step 3 (`bao operator raft snapshot restore <snap>`) works ONLY when restoring to the same running instance (same master key). For a fresh node (new init, different master key), `--force` is mandatory. The "from-scratch rebuild" section was ambiguous; it now has an explicit fresh-node path.
- _Restart required after force restore_: After `snapshot restore -force`, the Raft state isn't reloaded until the process restarts. The runbook omitted this step.
- _Unseal key confusion_: After force restore + restart, unseal with the ORIGINAL `.env` keys (the restored master key). The fresh instance's keys no longer work. Documented explicitly.
- _Secret path_: The planned drill steps said `bao kv get secret/<some-path>` but the actual production path mount is `tenants/` (KV v2 at `tenants/`). Corrected in the drill record.
- _`bao-init.sh` warning_: The from-scratch note previously suggested running `bao-init.sh` first; doing so would generate NEW keys and make the old snapshot unrestorable. Clarified: do NOT run `bao-init.sh` before a fresh-node restore.

---

### bex-db restore — 2026-07-14 (w7/m29, kind cluster + CNPG + minio)

**Date:** 2026-07-14 · **Environment:** Single-node kind cluster (kindest/node:v1.36.1), CNPG 1.30.0, local minio as S3 target · **Duration:** ~18 min · **Outcome:** ✅ pass

**Steps executed:**

1. Deployed CNPG 1.30.0 on a fresh kind cluster + minio (S3 target at `http://192.168.158.8:9000`).
2. Created `bex-drill/bex-db` CNPG Cluster with `spec.backup.barmanObjectStore` pointing to `s3://bex-tfstate/bex-db` on minio.
3. Verified WAL archiving started (`last_archived_wal: 000000010000000000000001`, `last_archived_time` non-null).
4. Created test schema: `CREATE TABLE workspaces (id UUID PRIMARY KEY ..., name TEXT ...)` + inserted `('drill-workspace-w7m29')` → recorded UUID `f8b8e049-0201-4580-ac0e-6f128541db0d`.
5. Triggered on-demand `Backup` CR → phase `completed` in ~10 s. Artifacts in minio: `bex-db/bex-db/base/20260714T230242/` + WAL segments under `bex-db/bex-db/wals/`.
6. Applied `bex-db-recover` Cluster with `bootstrap.recovery.source: bex-db` + `externalClusters` barman reference → phase `Cluster in healthy state` in ~40 s.
7. Verified: `SELECT id, name FROM workspaces WHERE id = 'f8b8e049-...';` → row present in the recovered cluster.
8. Deleted recovery cluster + kind cluster.

**Deviations from runbook (corrected below):**

- _Object storage path_: The runbook says to look for `.catalog.json` at `s3://$TF_STATE_BUCKET/bex-db/`. Actual barman structure has the server name as an extra subdirectory: `s3://bex-tfstate/bex-db/bex-db/`. Artifacts are `base/YYYYMMDDTHHMMSS/backup.info` (not `.catalog.json`) and WAL segments under `wals/` (not `data/wal/`).
- _psql authentication_: The runbook uses `psql -U bex -d bex` inside the pod, but peer authentication is not configured for the `bex` role within the pod's Unix socket. Use `psql -U postgres -d bex` (superuser) instead.
- _nodeSelector/tolerations_: The recovery YAML includes `bex.co/pool: platform` selectors. On the mock cluster these nodes don't exist and would prevent scheduling. Omit for drills; keep for prod.
- _CNPG 1.30.0 deprecation warning_: Both `spec.backup.barmanObjectStore` and `spec.externalClusters[*].barmanObjectStore` emit a deprecation warning in CNPG 1.30.0: "Native support for Barman Cloud backups and recovery is deprecated and will be completely removed in CloudNativePG 1.31.0." Migration to the Barman Cloud Plugin is required before upgrading CNPG to 1.31+.

## Alerting

- **`BackupCronJobStale`** (prometheus.yaml `bex` group): fires if active `etcd-backup`, `openbao-backup`, or `kvbak-*` CronJobs have no success in >26h, including never-successful CronJobs older than 26h. Deliberately suspended KeyValue CronJobs are excluded. Severity: critical.
- **`PlatformDatabaseBackupStale`** (prometheus.yaml `bex` group): one alert instance per `bex-db`, `kratos-db`, `hydra-db`, or `openfga-db` if its Barman Cloud plugin-backed WAL archiver (`cnpg_pg_stat_archiver_last_archived_time` from the primary-only `cnpg-platform-db` scrape) has not archived in >26h. The scrape stamps namespace and cluster labels from Kubernetes discovery. Debugging starts with `pg_stat_archiver`, the cluster's ObjectStore, the `barman-cloud` deployment and instance-sidecar logs; credential presence is checked without printing Secret data. Severity: critical.

## Recovery objectives (RPO/RTO)

What bex actually promises per store — internal engineering objectives, not customer SLAs. Recovery is deliberately human-initiated (no automatic failover or cutover), so total time-to-recovery = detection + operator engagement + script execution + deliberate cutover. The **RTO objective** below covers only the scripted part: from an operator with `.env` credential custody starting a `scripts/restore-*.sh` run to a verified recovery target. The **observed** column is the evidence from the drill records above.

| store | RPO objective (max data loss) | basis | RTO objective (script → verified target) | observed |
| --- | --- | --- | --- | --- |
| **etcd** (App/Database/KeyValue CRs) | ≤ 24 h | daily 03:15 UTC snapshot, 7 rolling | ≤ 1 h to re-applied CR manifests on a healthy cluster (cluster rebuild itself is a separate procedure) | minutes — scripted extraction of 9 CRs from a fresh production snapshot (2026-07-31; decrypt-then-restore re-proven 2026-08-04) |
| **OpenBao** (tenant env-var secrets) | ≤ 24 h | daily 03:45 UTC Raft snapshot, 7 rolling | ≤ 1 h to a restored, unsealed, verified instance | ~12 min end-to-end in the 2026-07-14 drill; scripted production restore 2026-07-31 |
| **paid KeyValue** | ≤ 24 h — nightly snapshot only, **no PITR** | per-id RDB slot 03:20–03:39 UTC, keep 7 | ≤ 1 h to a verified fresh-PVC instance | single-session fresh-PVC restore incl. the AOF transition + workload restart (2026-07-31) |
| **Free KeyValue** | **unbounded — PVC-only, data loss on volume loss is accepted** | no off-cluster copy by design | none | — |
| **tenant Postgres** | ≤ ~5 min of unarchived WAL (CNPG `archive_timeout`); **PITR within 30 d** | continuous WAL + daily base backup | ≤ 1 h to a verified recovery Cluster | minutes — explicit-PITR marker restore (2026-07-28), scripted PostgreSQL 16 restore (2026-07-31) |
| **bex-db** | ≤ ~5 min of unarchived WAL; **PITR within 7 d** | continuous WAL + 04:00 UTC base backup | ≤ 1 h to a verified recovery Cluster | minutes — PITR restore with schema/row verification (2026-07-28), scripted marker restore (2026-07-31) |
| **auth DBs** (kratos/hydra/openfga) | ≤ ~5 min of unarchived WAL; **PITR within 7 d** | continuous WAL + 04:15/04:30/04:45 UTC base backups | ≤ 1 h to a verified recovery Cluster | 62 s to a Ready recovery Cluster (Kratos, 2026-07-31) |

The nightly-snapshot RPOs hold when last night's job succeeded; the `BackupCronJobStale` / `PlatformDatabaseBackupStale` alerts (>26 h, above) bound how long a silently failing backup — and therefore a silently growing RPO — can go unnoticed. The WAL-based RPOs assume archiving is healthy, bounded by the same alert.

## Re-drill cadence

**Owner:** the platform operator. **Next scheduled all-store re-drill: 2027-07-31** (annual anniversary of the scripted baseline). When that date arrives — or when any per-store trigger below fires earlier — file a `.pm` inbox note to schedule the drill; a drill that deviates from the scripts re-earns its record here.

| store | last drilled | next drill trigger | reason |
| --- | --- | --- | --- |
| etcd | 2026-07-31 | Annually or after any control-plane topology/script change | Scripted Path A tested against a fresh production snapshot |
| OpenBao | 2026-07-31 | Annually or after an OpenBao/image/script change | Scripted fresh-node force-restore and original-key unseal verified in production |
| paid KeyValue | 2026-07-31 | Annually or after Valkey major/image, snapshot-job, or restore-script changes | Snapshot recovery is AOF-sensitive; re-drill any load-order or transfer change |
| tenant Postgres | 2026-07-31 | Annually or after a CNPG/plugin major or restore-script change | PostgreSQL 16 marker restored through the shared ObjectStore driver |
| bex-db/auth DBs | 2026-07-31 | Annually or after a CNPG/plugin major, backup-transport, or restore-script change | Fresh production archives recovered through the generic driver |

Once [ADR050](ADR050-encrypted-platform-backups.md) encryption/credential-scoping is enabled, every Tier A (etcd/OpenBao/KeyValue) re-drill must prove **decrypt-then-restore** (not just restore), and every store's re-drill must run against its **write-only writer credential** for the backup and its **read-only reader credential** for the restore — enabling either is itself a re-drill trigger.
