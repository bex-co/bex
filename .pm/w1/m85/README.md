# w1 · m85 — Persistent disks 3/4: daily snapshots + restore

**Worker:** worker1 **Goal:** Render's disk-snapshot semantics on a substrate with no block snapshots — nightly file-level age-encrypted backups to object storage with 7-day retention, a Render-shaped snapshot list with 24 h keys, and a full-disk restore verb that restarts the service — ADR082 D5. **Status:** todo

## Tasks (in order)

| id   | title                                                    | est | depends_on |
| ---- | -------------------------------------------------------- | --- | ---------- |
| t001 | Nightly per-disk backup CronJob → object storage         | 1h  | —          |
| t002 | Snapshot list API + HMAC-signed 24 h `snapshotKey`       | 45m | t001       |
| t003 | Restore verb: scale-0 → wipe+extract Job → redeploy      | 1h  | t002       |
| t004 | Render parity check (snapshot routes/shapes/warnings)    | 30m | t003       |
| t005 | Simplify pass over the changed code                      | 30m | t004       |
| t006 | Test coverage: backup/restore round-trip + key expiry    | 45m | t005       |
| t007 | Closeout                                                 | 15m | t006       |

## Definition of done

A disk-bearing App gets a nightly backup Job (same-node when the pod is live) that streams `tar | gzip | age` to the platform backup store, keeping exactly 7 dailies with purge-on-delete; `GET /v1/disks/{id}/snapshots` returns Render's `{createdAt, snapshotKey, instanceId}` with keys that stop working after 24 h; `POST /v1/disks/{id}/snapshots/restore` scales to 0, restores the PVC to the snapshot, redeploys, and discards post-snapshot writes — verified by an end-to-end write→snapshot→mutate→restore round-trip on the mock cluster; the dashboard/API docs carry Render's database-recovery warning verbatim. Snapshot storage emits no meter. Suites green.

## Source + Goal linkage

- **Source:** [docs/ADR082-persistent-disks.md](../../../docs/ADR082-persistent-disks.md) (D5, D11 stage 3); Hetzner has no volume snapshots at any level, so file-level is the only honest mechanism (ADR082 § Context); anti-goal re-opened 2026-08-22.
- **Goal linkage:** Render parity — snapshots are half the disk product (data safety is why users trust a disk with state); reuses the ADR050/ADR021 encrypted-backup machinery.
- **Expected outcome:** disk data survives operator error with Render-identical restore semantics; retention matches Render's ≥7 days exactly.
- **Why now:** stage 3 of ADR082 D11 — needs m83's PVC mechanism and m84's disk identity/verb plumbing; m86's dashboard snapshot panel consumes these APIs.
- **Render parity closing task included** (t004): new REST/GraphQL/MCP snapshot surfaces.
