# w1 · m85 — Persistent disks 3/4: daily snapshots + restore

**Worker:** worker1 **Goal:** Render's disk-snapshot semantics on a substrate with no block snapshots — nightly file-level age-encrypted backups to object storage with 7-day retention, a Render-shaped snapshot list with 24 h keys, and a full-disk restore verb that restarts the service — ADR082 D5. **Status:** todo (t001–t006 done; t007 closeout holds on the in-cluster drill)

## Tasks (in order)

| id   | title                                                    | est | depends_on |
| ---- | -------------------------------------------------------- | --- | ---------- |
| t001 | Nightly per-disk backup CronJob → object storage — **DONE** | 1h  | —          |
| t002 | Snapshot list API + HMAC-signed 24 h `snapshotKey` — **DONE** | 45m | t001       |
| t003 | Restore verb: scale-0 → wipe+extract Job → redeploy — **DONE** | 1h  | t002       |
| t004 | Render parity check (snapshot routes/shapes/warnings) — **DONE** | 30m | t003       |
| t005 | Simplify pass over the changed code — **DONE** | 30m | t004       |
| t006 | Test coverage: backup/restore round-trip + key expiry — **DONE** | 45m | t005       |
| t007 | Closeout                                                 | 15m | t006       |

## Definition of done

A disk-bearing App gets a nightly backup Job (same-node when the pod is live) that streams `tar | gzip | age` to the platform backup store, keeping exactly 7 dailies with purge-on-delete; `GET /v1/disks/{id}/snapshots` returns Render's `{createdAt, snapshotKey, instanceId}` with keys that stop working after 24 h; `POST /v1/disks/{id}/snapshots/restore` scales to 0, restores the PVC to the snapshot, redeploys, and discards post-snapshot writes — verified by an end-to-end write→snapshot→mutate→restore round-trip on the mock cluster; the dashboard/API docs carry Render's database-recovery warning verbatim. Snapshot storage emits no meter. Suites green.

## Progress notes (t001, 2026-08-23)

- **The byte pipeline is its own package and is fully verified without a cluster or a bucket.** `internal/disksnapshot` takes a directory in and produces one encrypted stream, and back. That separation is deliberate: this is the code that can silently lose a tenant's data, so it is tested as pure bytes against real temp directories — byte-exact round trip (nested dirs, empty file, 1 MiB incompressible blob, symlinks, permissions), post-snapshot writes correctly discarded, the mount point never removed, a crafted `../../etc/passwd` entry refused, and both a wrong key and a truncated object failing **without** clearing the volume first.
- **Streaming, not staging.** The KeyValue pipeline stages its RDB through an EmptyDir sized at 2× the data; that is fine for a 5 GB datastore and impossible for a 10 TB disk. One process walks the volume and streams `tar → gzip → age` into a multipart upload, so nothing is buffered and the plaintext never lands on a staging disk or crosses a container boundary — the same reasoning that made `/backup-encrypt` a first-party entrypoint (ADR068 #9).
- **Encryption is mandatory here, not opt-in.** `DiskSnapshotStore.configured()` requires the age recipient: a disk snapshot is a full copy of a tenant filesystem going to a third-party bucket, so bex takes no snapshot rather than an unencrypted one. The pair is **dedicated to disks** rather than the ADR050 platform pair, because restore needs the decrypt half in-cluster — and the data that key protects is already on the volume in that same cluster, whereas reusing the platform key would put etcd/OpenBao backups within reach of a cluster compromise.
- **Inert until configured.** With `BEX_DISK_SNAPSHOT_*` unset (everywhere today) no CronJob and no purge Job are created, so this stage is a no-op at runtime.

## Progress notes (t002–t006, 2026-08-23)

- **The data path is now verified against a REAL object store, not a mock.** `store_integration_test.go` runs the whole pipeline against MinIO: a directory becomes an encrypted object through a multipart upload of unknown length, comes back out byte-for-byte, retention deletes the three oldest of ten and keeps the newest, a purge removes one disk's objects while its neighbour keeps all of its own, and a non-snapshot object in the same prefix is never offered for restore. Gated on `BEX_TEST_S3_ENDPOINT` so `go test ./...` stays hermetic; the header documents the one-line `docker run` to reproduce.
- **Restore is gated so a bad key can never stop a service.** Verification (signature, 24 h expiry, and that the key names *this* disk) happens before anything is touched — proven by a table asserting that an expired, forged, cross-disk, empty, or garbage key leaves `spec.disk.restoreSnapshot` untouched. A well-signed key for another disk is refused here rather than range-checked in the Job.
- **The operator's restore order is forced by the volume and pinned in envtest**: scale to zero → wait for the pods to go → Job mounts the freed volume *writable* → only a SUCCEEDED Job records the snapshot and releases the service. A FAILED restore deliberately leaves the service down with the request still pending, because the alternative is serving a half-extracted filesystem. `backoffLimit: 0` — re-running a destructive restore automatically would repeat the wipe. With no decrypt key configured the Job is never created at all, since it could only fail *after* wiping.
- **The decrypt key never reaches bex-api.** It is mounted by reference into the restore Job alone. bex-api only lists objects and signs a 24-hour *reference* to one with the shell-ticket secret — it cannot read a snapshot's contents.

## Remaining

- **t007 only: the in-cluster drill.** Everything m85 builds is implemented, unit/envtest/real-S3 verified, and lint-clean. What has not been demonstrated is the orchestration on a live cluster: operator deployed, a real PVC, a real pod scale-down, a real Job run — i.e. the DoD's literal `write file A → snapshot → write file B → restore → A present, B gone, service back up`. That needs the mock cluster running the operator against a reachable S3 endpoint. The MinIO container this run used (`bex-test-minio`, port 59000) plus `BEX_DISK_SNAPSHOT_*` on the operator Deployment is the shortest path to it.

## Source + Goal linkage

- **Source:** [docs/ADR082-persistent-disks.md](../../../docs/ADR082-persistent-disks.md) (D5, D11 stage 3); Hetzner has no volume snapshots at any level, so file-level is the only honest mechanism (ADR082 § Context); anti-goal re-opened 2026-08-22.
- **Goal linkage:** Render parity — snapshots are half the disk product (data safety is why users trust a disk with state); reuses the ADR050/ADR021 encrypted-backup machinery.
- **Expected outcome:** disk data survives operator error with Render-identical restore semantics; retention matches Render's ≥7 days exactly.
- **Why now:** stage 3 of ADR082 D11 — needs m83's PVC mechanism and m84's disk identity/verb plumbing; m86's dashboard snapshot panel consumes these APIs.
- **Render parity closing task included** (t004): new REST/GraphQL/MCP snapshot surfaces.
