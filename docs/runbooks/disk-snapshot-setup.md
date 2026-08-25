# Runbook — arm persistent-disk snapshots

Brings [ADR082](../ADR082-persistent-disks.md) D5's snapshot pipeline from shipped-but-inert to actually running. Until this is done, a tenant who attaches a disk gets **no backups**: the operator creates no CronJob, and `GET /v1/disks/{diskId}/snapshots` answers a coded `DISK_SNAPSHOTS_NOT_CONFIGURED` 503 that the dashboard renders as "Snapshots aren't set up".

That inertness is deliberate — a disk snapshot is a full copy of a tenant filesystem leaving the cluster, so bex takes none rather than an unencrypted one — but it means ADR018's disk row and ADR082 D5 describe a capability the running system does not yet have.

## What this creates

| Thing | Why it is separate |
| --- | --- |
| Bucket `bex-disk-snapshots` | **Dedicated.** Never `bex-tfstate` — that would put every tenant's filesystem beside the credentials that rebuild the platform. |
| IAM user `bex-disk-snapshot` | The **operator**: PUT/GET/DELETE, for backup, 7-day retention, and purge-on-detach. |
| IAM user `bex-disk-snapshot-read` | **bex-api**: LIST/GET only. bex-api must never write or delete a tenant's backups, and never holds the age key at all. |
| Secret `bex-system/bex-disk-snapshot` | Operator's credential + the age **recipient** (public) key it encrypts to. |
| Secret `bex-system/bex-disk-snapshot-read` | bex-api's read-only credential. |
| Secret `bex-system/bex-disk-snapshot-age` | The age **private** key, mounted only by the restore Job. |

The age keypair is **dedicated to disks**, deliberately not ADR050's `AGE_BACKUP_*` platform key: a restore has to decrypt _inside_ the cluster, and ADR050 exists precisely to keep the platform key out of it.

## Prerequisites

- Wasabi root (or equivalently privileged) S3 credentials in `.env` as `TF_STATE_ACCESS_KEY`/`TF_STATE_SECRET_KEY`, or `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY` in the environment.
- `docker`, `jq`, `kubectl` on PATH, with `kubectl` pointing at the target cluster (or set `BEX_KUBE_CONTEXT`).
- `BEX_DISK_SNAPSHOT_ENDPOINT` and `BEX_DISK_SNAPSHOT_REGION` set, or inheritable from `TF_STATE_*` / `BEX_STATIC_S3_*`.

## Steps

```bash
# 1. Bucket, both IAM users, the age keypair, .env, and all three Secrets.
#    Idempotent: re-running keeps an existing bucket, policy, and keypair.
scripts/disk-snapshot-secret.sh provision

# 2. Prove the separation actually holds against the live bucket.
scripts/disk-snapshot-secret.sh verify
```

`verify` asserts what the design depends on, not merely that credentials work:

- the operator can PUT under the prefix;
- bex-api can LIST;
- bex-api **cannot** write and **cannot** delete;
- neither identity can reach `bex-tfstate`.

Treat any `FAIL` as blocking. A read credential that can delete is worse than no snapshots at all.

Then let Argo apply the manifests (or `kubectl rollout restart deployment/bex-controller-manager deployment/bex-api -n bex-system`). Both read their coordinates from these Secrets with `optional: true`, so the Secrets may land before or after the env is live — whichever arrives second arms the feature.

## Verifying end to end

On a **scratch** service with a **scratch** disk — never a tenant's:

1. Attach a disk, write a known file under its mount path.
2. Confirm the CronJob exists: `kubectl -n <tenant-ns> get cronjob | grep disk-`.
3. Trigger it (`kubectl create job --from=cronjob/...`) and confirm exactly one object lands under `disks/<workspace>/<app>/`.
4. `GET /v1/disks/{diskId}/snapshots` lists it with a 24-hour `snapshotKey`.
5. Mutate the file, restore, and confirm the pre-snapshot content returns while the post-snapshot change is gone. **A restore is irreversible and scales the service down first.**
6. Delete the disk; confirm the purge Job leaves no objects under that prefix.

## Contract details the drill established

These are not obvious from the code and each one failed _late_ the first time — after a snapshot had been taken, or when a Job tried to start. They are now guarded by `scripts/disk-snapshot-secret.test.sh`.

- **The operator's Secret carries `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY`**, not `BEX_DISK_SNAPSHOT_*`. The backup Job hands the whole Secret to the AWS SDK via `envFrom`; the wrong names fail at upload with the misleading `no EC2 IMDS role found`.
- **It must exist in every tenant namespace.** The Job runs beside its App (ADR043 D8) and nothing projects this credential the way `BackupSourceNamespace` projects the KeyValue one. A Secret only in `bex-system` leaves every backup Job in `CreateContainerConfigError`.
- **The age Secret holds the bare key**, not the file `age-keygen` writes. Its two comment lines make the restore fail with `parse identity: malformed secret key: mixed case` — discovered only when restoring a snapshot that had already been taken.
- **`restoreSnapshot` takes the full object key** — `<workspace>/<app>/<timestamp>.tar.gz.age` — not just the filename. A bare filename is refused by the prefix-confinement check that stops one disk restoring another's snapshot.
- **The restore Job is named per-App and is not recreated while it exists.** Changing `spec.disk.restoreSnapshot` after a failed attempt has no effect until the old Job is deleted.

### If your object store is on a private network

Tenant namespaces carry an `allow-internet-egress` policy that permits `0.0.0.0/0` **except** RFC1918 ranges. Wasabi is public so production is unaffected, but a self-hosted MinIO on a private address is unreachable from the backup Job by design (ADR043). Either give the store a publicly-routable address or add an explicit egress allowance for it.

## Rotation

Re-running `provision` keeps existing keys. To rotate the S3 credentials, delete the access key in Wasabi IAM, clear the matching `.env` values, and re-run `provision`.

**Rotating the age key is different and needs care:** snapshots already in the bucket were encrypted to the old recipient and can only be decrypted by the old private key. Rotating without keeping it makes every existing snapshot permanently unreadable. Keep the old key until its snapshots have aged past the 7-day retention window.

## If it goes wrong

Deleting `bex-system/bex-disk-snapshot` reverts to the fail-closed state: no CronJobs, snapshot routes 503, disks and their data untouched. That is a safe rollback — it stops new snapshots and changes nothing about the volumes themselves.
