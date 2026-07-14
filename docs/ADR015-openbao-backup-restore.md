# OpenBao backup & restore

**Why this exists:** OpenBao ([ADR013-secrets.md](ADR013-secrets.md)) holds every tenant credential in integrated Raft storage on a single PVC (`replicas: 1` — no quorum peer to heal from). Unlike the platform's other state, these secrets are **not** reproducible from git: a lost PVC = lost tenant credentials, permanently. A nightly Raft snapshot shipped off-cluster is the recovery story — the mirror of [ADR011-etcd-backup-restore.md](ADR011-etcd-backup-restore.md) for the secret store.

Raft's `snapshot save` takes a point-in-time, internally-consistent copy of the **already-encrypted** store while OpenBao keeps serving — no seal, no downtime.

```mermaid
graph LR
  subgraph cluster["app cluster (Hetzner)"]
    argo[Argo CD]
    secret["Secret openbao-backup-s3<br/>(inert, created once from .env)"]
    subgraph "ns secrets"
      cron["openbao-backup pod<br/>(nightly 03:45 UTC, exits when done)"]
      bao[("OpenBao<br/>(Raft on PVC)")]
    end
  end
  argo -->|creates CronJob + SA from git| cron
  cron -->|"k8s-auth login (role bao-snapshot)<br/>→ raft snapshot save"| bao
  cron -->|reads creds| secret
  cron -->|upload + prune, keep 7| bucket
  bucket[("Hetzner Object Storage<br/>(external, openbao-snapshots/)")]

  subgraph "disaster recovery — manual runbook"
    op@{ shape: tri, label: "human operator" }
  end
  op -->|"fetch snapshot"| bucket
  op -->|"bao operator raft snapshot restore"| bao
```

## What runs

A CronJob `secrets/openbao-backup` (Argo app `openbao-backup` in the gitops base — every non-disposable cluster gets it; the local overlay excludes it. Manifest in [`deploy/gitops/charts/openbao-backup/`](../deploy/gitops/charts/openbao-backup/cronjob.yaml)):

| step | container | what it does |
| --- | --- | --- |
| snapshot | `openbao/openbao` (matches the chart) | Kubernetes-auth login as SA `bao-snapshot` (role → `snapshot` policy: read on `sys/storage/raft/snapshot` only), then `bao operator raft snapshot save` against `http://openbao.secrets.svc:8200` |
| compress | `busybox` | `gzip -9` (the aws-cli image ships no gzip) |
| upload + prune | `amazon/aws-cli` (pinned < 2.23) | upload `openbao-<UTC timestamp>.snap.gz` to `s3://$S3_BUCKET/openbao-snapshots/`, then delete all but the newest 7 |

Schedule: **03:45 UTC daily** (staggered after etcd-backup's 03:15). Retention: **7 snapshots**, pruned by the job itself. The snapshot is already encrypted at rest by OpenBao; keep the bucket private regardless.

## One-time setup per cluster

Two out-of-band steps — the same trust boundary as `bao-init.sh` (nothing in git):

1. **The OpenBao snapshot role** (least privilege, read on the snapshot path only). `scripts/bao-k8s-auth.sh` provisions it idempotently alongside the `bex-api` role:

   ```sh
   scripts/bao-k8s-auth.sh      # writes the `snapshot` policy + `bao-snapshot` role
   ```

2. **The S3 credentials Secret** (reuses the Terraform-state Object Storage creds from `.env`, exactly like etcd-backup):

   ```sh
   source .env && kubectl -n secrets create secret generic openbao-backup-s3 \
     --from-literal=S3_ENDPOINT="$TF_STATE_ENDPOINT" \
     --from-literal=S3_BUCKET="$TF_STATE_BUCKET" \
     --from-literal=AWS_DEFAULT_REGION="$TF_STATE_REGION" \
     --from-literal=AWS_ACCESS_KEY_ID="$TF_STATE_ACCESS_KEY" \
     --from-literal=AWS_SECRET_ACCESS_KEY="$TF_STATE_SECRET_KEY"
   ```

Verify a backup landed (or trigger one now):

```sh
kubectl -n secrets create job --from=cronjob/openbao-backup openbao-backup-now
kubectl -n secrets logs job/openbao-backup-now -c upload -f
aws --endpoint-url "$TF_STATE_ENDPOINT" s3 ls "s3://$TF_STATE_BUCKET/openbao-snapshots/"
```

## Restore

A Raft snapshot restore overwrites the live store with the snapshot's contents. The snapshot is sealed with the **master key from when it was taken** — so after restore, OpenBao unseals with the **same** unseal keys already in `.env` (`bao-init.sh` never rotates them). Restore onto a running, unsealed OpenBao:

**Same-instance restore** (live OpenBao intact, same PVC, same master key — e.g. data corruption or runaway write):

```sh
# 1. fetch + unpack the latest snapshot
aws --endpoint-url "$TF_STATE_ENDPOINT" s3 cp \
  "s3://$TF_STATE_BUCKET/openbao-snapshots/<latest>.snap.gz" . && gunzip <latest>.snap.gz

# 2. reach OpenBao and authenticate as root (BAO_ROOT_TOKEN from .env)
kubectl -n secrets port-forward service/openbao 8200:8200 &
export BAO_ADDR=http://127.0.0.1:8200 BAO_TOKEN="$BAO_ROOT_TOKEN"

# 3. restore — the store (and all tenant secrets) is replaced by the snapshot
#    The master key hasn't changed (same instance), so no --force is needed.
bao operator raft snapshot restore <latest>.snap
```

**Fresh-node restore** (new PVC / new pod — Raft data gone, different master key):

Do NOT run `bao-init.sh` before restoring: it writes NEW unseal keys to `.env`, making the snapshot (sealed with the old keys) unrecoverable. Instead:

```sh
# 1. Let Argo bring up a freshly initialized OpenBao (it auto-inits with new keys).
#    Note the NEW root token for the API call in step 4.
kubectl -n secrets exec openbao-0 -- \
  bao operator init -key-shares=1 -key-threshold=1 -format=json > /tmp/fresh-init.json
NEW_ROOT_TOKEN=$(jq -r .root_token /tmp/fresh-init.json)
NEW_UNSEAL_KEY=$(jq -r .unseal_keys_b64[0] /tmp/fresh-init.json)
bao operator unseal "$NEW_UNSEAL_KEY"   # unseal with new keys

# 2. fetch + unpack the snapshot
aws --endpoint-url "$TF_STATE_ENDPOINT" s3 cp \
  "s3://$TF_STATE_BUCKET/openbao-snapshots/<latest>.snap.gz" . && gunzip <latest>.snap.gz

# 3. reach OpenBao
kubectl -n secrets port-forward service/openbao 8200:8200 &
export BAO_ADDR=http://127.0.0.1:8200

# 4. force-restore (bypasses master-key hash check — needed when instance has
#    a different master key than the snapshot's)
export BAO_TOKEN="$NEW_ROOT_TOKEN"
bao operator raft snapshot restore -force <latest>.snap

# 5. restart the OpenBao pod so it reloads from the restored Raft log
kubectl -n secrets rollout restart statefulset/openbao

# 6. unseal with the OLD .env keys (the restored data is sealed with the original master key)
#    Keep old .env; do NOT let bao-init.sh overwrite it.
export BAO_TOKEN="$BAO_ROOT_TOKEN"   # original root token from .env
bao operator unseal "$BAO_UNSEAL_KEY_1"   # original unseal key(s) from .env
```

> **Tested 2026-07-14 (w7/m29 drill):** Fresh-node path executed against two local Docker containers. Key findings: (1) `--force` is required for fresh-node restore; plain `snapshot restore` returns `could not verify hash file`. (2) A pod restart is needed after `snapshot restore -force` so the Raft state is reloaded. (3) After restart, unseal with the ORIGINAL `.env` keys — the new instance's keys no longer work because the master key was replaced by the snapshot. Full record in [ADR031-platform-data-backup.md](ADR031-platform-data-backup.md) §Drill records.
