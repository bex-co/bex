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
- **It must exist in the App's namespace — and since w2/033 the operator projects it there.** The Job runs beside its App (ADR043 D8). The canonical pair lives in the **apps namespace** (`BEX_APPS_NAMESPACE`, default `default`; the script seeds it), and the reconciler copies both Secrets into each disk-bearing App's own namespace at reconcile time — the same `BackupSourceNamespace` pattern the KeyValue credential uses — refreshing on rotation. New workspace namespaces therefore no longer need a script re-run (previously their first backup Job died in `CreateContainerConfigError` until an operator re-ran `install`); the script's tenant-namespace fan-out remains for immediate coverage and rotation repair. A missing canonical pair surfaces as the `SnapshotCredentialUnavailable` disk condition rather than a scheduled-but-doomed CronJob.
- **The age Secret holds the bare key**, not the file `age-keygen` writes. Its two comment lines make the restore fail with `parse identity: malformed secret key: mixed case` — discovered only when restoring a snapshot that had already been taken.
- **`restoreSnapshot` takes the full object key** — `<workspace>/<app>/<timestamp>.tar.gz.age` — not just the filename. A bare filename is refused by the prefix-confinement check that stops one disk restoring another's snapshot.
- **The restore Job is named per-App and is not recreated while it exists.** Changing `spec.disk.restoreSnapshot` after a failed attempt has no effect until the old Job is deleted.

The first **production** arming (w2/m86, 2026-09-02 — everything above came from the CAPD drill, whose `local-path` volumes are 777 hostPath directories) added four more, each also invisible until it failed late:

- **A first-ever `provision` used to die between minting and persisting the IAM keys.** The age-keypair probe (`kubectl get secret | jq`) exits non-zero on a cluster that has never held the Secret, and `set -o pipefail` aborted the whole run there — after Wasabi had shown the new access keys the one time it ever will. Recovery is the script's own message (delete the keys in Wasabi IAM, re-run); the probe now tolerates absence.
- **The operator's copy of `bex-disk-snapshot` must ALSO exist in `bex-system`, carrying `BEX_DISK_SNAPSHOT_AGE_PUBLIC_KEY`.** The manager manifest arms the recipient key through a `secretKeyRef` on that Secret, and the script previously installed it into tenant namespaces only, with the AWS keys only — so on any manifest-armed cluster `DiskSnapshots` never reported configured and **no backup CronJob was ever created**. The script now installs the bex-system copy with all three entries.
- **The snapshot Job must node-publish the claim read-write** (the container's bind stays read-only). A `ReadOnly` claim source becomes a direct `mount -o ro` of the LUKS mapper device, which the kernel refuses (`EBUSY`) while the service's pod holds the same device rw — a running app could never be backed up on the encrypted class.
- **The snapshot/restore container must run as root with `DAC_OVERRIDE`/`CHOWN`/`FOWNER`** (all PSS-baseline-allowed; the operator image itself is `USER 65532`). A real ext4 volume has a root-owned 0700 `lost+found` and tenant files carry arbitrary uids, so a non-root backup dies with `permission denied` on its first object — on **any** class, not just LUKS.

### If your object store is on a private network

Tenant namespaces carry an `allow-internet-egress` policy that permits `0.0.0.0/0` **except** RFC1918 ranges. Wasabi is public so production is unaffected, but a self-hosted MinIO on a private address is unreachable from the backup Job by design (ADR043). Either give the store a publicly-routable address or add an explicit egress allowance for it.

## Rotation

Re-running `provision` keeps existing keys. To rotate the S3 credentials, delete the access key in Wasabi IAM, clear the matching `.env` values, and re-run `provision`.

**Rotating the age key is different and needs care:** snapshots already in the bucket were encrypted to the old recipient and can only be decrypted by the old private key. Rotating without keeping it makes every existing snapshot permanently unreadable. Keep the old key until its snapshots have aged past the 7-day retention window.

## If it goes wrong

Deleting `bex-system/bex-disk-snapshot` reverts to the fail-closed state: no CronJobs, snapshot routes 503, disks and their data untouched. That is a safe rollback — it stops new snapshots and changes nothing about the volumes themselves.
