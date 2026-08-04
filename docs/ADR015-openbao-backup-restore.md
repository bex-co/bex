# OpenBao backup & restore

**Why this exists:** OpenBao ([ADR013-secrets.md](ADR013-secrets.md)) holds every tenant credential in integrated Raft storage. Production runs a three-member Raft cluster; the disposable local overlay runs one member. Raft protects production from one member or volume failure, but it is not a backup: correlated loss or operator error can still destroy tenant credentials permanently. A nightly Raft snapshot shipped off-cluster is the recovery story — the mirror of [ADR011-etcd-backup-restore.md](ADR011-etcd-backup-restore.md) for the secret store.

Raft's `snapshot save` takes a point-in-time, internally-consistent copy of the **already-encrypted** store while OpenBao keeps serving — no seal, no downtime.

> Because the snapshot is already encrypted, [ADR050-encrypted-platform-backups.md](ADR050-encrypted-platform-backups.md) applies only its credential-scoping half (§3) here, not its Tier A encryption step.

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

  subgraph "disaster recovery — operator-triggered restore script"
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

## Node drain and rolling-maintenance unseal

Production uses three Raft members with Shamir/manual unseal. A rescheduled member starts sealed, so node maintenance must proceed one member at a time: keep the other two members unsealed, unseal the rescheduled member, wait for all three peers to be healthy again, and only then allow the next node to drain. Never continue while two members are sealed or unavailable; that loses Raft quorum.

Point `KUBECONFIG` at the target workload cluster before starting; do not copy or print its contents. Before the first drain, confirm all three pods are Ready and unsealed, and record which node hosts each member:

```sh
export KUBECONFIG=/path/to/app.kubeconfig
kubectl -n secrets get pods -l app.kubernetes.io/name=openbao -o wide
for pod in openbao-0 openbao-1 openbao-2; do
  kubectl -n secrets exec "$pod" -- bao status -format=json \
    | yq -e '.sealed == false' - >/dev/null
done
```

For each node in the roll:

1. Cordon/drain only that node (or let the CAPI `MachineDeployment` roll initiate the drain). Watch the OpenBao member leave and reschedule. Its `maxUnavailable: 1` PDB is the stop gate that prevents a second member from being evicted while this one is sealed.
2. Wait for the same ordinal to reach `Running`. It will not become Ready until it is unsealed:

   ```sh
   TARGET=openbao-N
   kubectl -n secrets get pod "$TARGET" -w
   ```

3. From the repository root, load the out-of-band keys without echoing them and run the idempotent unseal path. The script reaches every pod directly; already-unsealed members are left untouched, while the rescheduled member receives all three shares through request bodies rather than process arguments:

   ```sh
   set +x
   set -a; source .env; set +a
   bash scripts/bao-init.sh
   ```

   If the script is unavailable, the exact per-share interactive command is below. Run it three times and paste one distinct key at each hidden prompt; never put a key on the command line:

   ```sh
   kubectl -n secrets exec -it "$TARGET" -- \
     env BAO_ADDR=http://127.0.0.1:8200 bao operator unseal
   ```

4. Prove the member rejoined before touching another node. The token is sent over stdin into the pod shell, not placed in `kubectl`'s arguments:

   ```sh
   kubectl -n secrets wait \
     --for=condition=Ready "pod/$TARGET" --timeout=5m
   printf '%s\n' "$BAO_ROOT_TOKEN" \
     | kubectl -n secrets exec -i "$TARGET" -- sh -c \
       'read -r BAO_TOKEN; export BAO_TOKEN; bao operator raft list-peers -format=json' \
     | yq -e '.data.config.servers | length == 3' - >/dev/null
   kubectl -n secrets get pdb openbao -o wide
   ```

5. Confirm all platform workloads are Ready and the OpenBao PDB again permits one disruption. Only then continue to the next node.

For a rehearsal outside a real node roll, delete exactly one non-leader member pod, follow steps 2–5, and record the member, node change, sealed interval, peer count, and whether quorum stayed available. Do not use `rollout restart`; it can restart more than one member and defeats the one-at-a-time invariant.

> **Rehearsed 2026-07-15 (w1/m38):** Deleted non-leader `openbao-2`; its replacement scheduled on the same platform node, started sealed, and the leader continued serving with all three peers visible. `scripts/bao-init.sh` unsealed only the restarted member, which returned Ready within 42 seconds. The peer count remained three and the PDB returned to one allowed disruption before the rehearsal ended.

## Restore

A Raft snapshot is sealed with the **master key from when it was taken**. The supported executable path always restores into a new `restore-*` namespace and unseals the result with the original keys. Production keeps those keys only in GitHub Actions secrets, so use the manual workflow:

```sh
gh workflow run openbao-restore-drill.yml \
  -f target_namespace=restore-bao-incident \
  -f confirm=restore-bao-incident
```

An operator with separately custodied keys can run the same implementation directly:

```sh
DRY_RUN=1 scripts/restore-openbao.sh \
  --target-namespace restore-bao-incident --verify-path tenants/data/<known-path>
scripts/restore-openbao.sh \
  --target-namespace restore-bao-incident --verify-path tenants/data/<known-path> \
  --confirm restore-bao-incident --teardown-on-success
```

The script checks/downloads the snapshot, creates a one-node throwaway with a new PVC, initializes and unseals only that new target, force-restores with the fresh target token, restarts, unseals with the three **old** keys, and verifies the known `tenants/` path without printing its data. Credential values travel by environment/request stdin, never argv. The 2026-07-31 production workflow drilled this complete path and verified target teardown plus live-marker cleanup; see [the all-store drill](drills/2026-07-31-scripted-restore-e2e.md).

The following same-instance explanation is retained only for incident theory. `restore-openbao.sh --live` rejects the request: an executable tool must never overwrite production in place.

**Same-instance theory:** when the live PVC and master key are intact, OpenBao accepts a non-force snapshot restore after root authentication. That operation replaces every live secret immediately and has no isolation/verification boundary, so bex provides no executable shortcut. Treat it as an exceptional incident cutover requiring a separate change plan.

**Fresh-node mechanism** (implemented by `restore-openbao.sh`; new PVC / new pod — Raft data gone, different master key):

Do NOT run `bao-init.sh` against the recovery target: it writes keys to the operator's `.env` and can overwrite the original custody material. The script stores its disposable fresh credentials only in a protected temporary directory. The fresh token authorizes force-restore because the empty target begins with a different master key; after restart, only the original snapshot's keys/token are valid. That key transition is why both credential sets are short-lived in memory and never placed in argv.

> **Tested 2026-07-14 (w7/m29 drill):** Fresh-node path executed against two local Docker containers. Key findings: (1) `--force` is required for fresh-node restore; plain `snapshot restore` returns `could not verify hash file`. (2) A pod restart is needed after `snapshot restore -force` so the Raft state is reloaded. (3) After restart, unseal with the ORIGINAL `.env` keys — the new instance's keys no longer work because the master key was replaced by the snapshot. Full record in [ADR031-platform-data-backup.md](ADR031-platform-data-backup.md) §Drill records.
