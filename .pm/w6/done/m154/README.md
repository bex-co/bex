# w6 · m154 — A failed nightly disk backup says why

**Worker:** worker6 **Goal:** when a disk's nightly snapshot Job fails, the reason survives the Job and reaches the owner and the operator, instead of only "overdue" **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Operator: record the last backup Job failure on the App — **DONE** | 60m | — |
| t002 | bex-api + dashboard: surface the failure through diskSnapshotHealth — **DONE** | 45m | w6/m154/t001 |
| t003 | Keep failed backup pods long enough to diagnose — **DONE** | 20m | — |
| t004 | Render parity — **DONE** | 15m | w6/m154/t002 |
| t005 | Simplify — **DONE** | 15m | w6/m154/t004 |
| t006 | Test coverage — **DONE** | 20m | w6/m154/t004 |
| t007 | Closeout — **DONE** | 10m | w6/m154/t005, w6/m154/t006 |

## Definition of done

- When a `dskbak-` Job ends `Failed`, the App records it: a DiskReady reason such as `SnapshotFailing` plus an Event, carrying the failed container's termination message (sanitized, no credentials) and the time. A later successful snapshot clears it.
- `diskSnapshotHealth` reports a failure state (or `unavailable` with that reason), and the Disk tab shows a reason-specific warning in every locale.
- The last failed Job's pod and its log stay available for diagnosis until the next run.

## Source + Goal linkage

- **Source:** 2026-10-24 diagnosis of w6/089 (`disk-snapshot-drill` run 40065193707). The `dskbak-tea-daif693dqjvc73e7as3g-clickhouse` Jobs (schedule `45 2 * * *`) failed 3/3 attempts on 10-22, 10-23 and 10-24 (`BackoffLimitExceeded`, `lastSuccessfulTime` none), and no pod, event or log was left to say why. m152 can only say "overdue".
- **Goal linkage:** ADR082 managed disks: backups an owner can trust, and an operator can fix without a live repro.
- **Expected outcome:** the next failed backup names its cause on the Disk tab and in the App status.
- **Why now:** a production disk has had no backup for three nights, and its cause is unrecoverable from the evidence the platform keeps.
- **Render parity:** included. The Disk tab copy and `diskSnapshotHealth` are tenant-facing; REST keeps Render's shape.

## Outcome — 2026-10-26

**Design changed during t001.** Operator Events and kept pods would have raced the very pod cleanup that hid w6/089's cause, so the snapshot Job records its own failure instead.
- **The record:** on a failed backup, `cmd/disk-snapshot` writes `<prefix>last-failure-<stage>` beside the snapshots. The stage is `read`, `upload` or `config`; the record holds no tenant data. A success deletes it, and so does `PurgeAll`. The shared contract is `lego/types/v1alpha1/diskbackup.go` (`DiskSnapshotFailure*`).
- **API:** bex-api splits the one prefix listing into snapshots and failure records. `diskSnapshotHealth` reports `failing` with the stage when a record is newer, by store `LastModified`, than the newest snapshot. The operator's unschedulable verdict still outranks it. A record is never listed as a snapshot, so REST/MCP stay Render-shaped.
- **Disk tab:** stage-specific copy in en and zh, plus a generic fallback for an unknown stage.
- **t003 superseded:** kept pods aren't needed once the record survives them.

**Review fixes (t005):**
- a malformed `AGE_PUBLIC_KEY` is classified `config`, not `read`;
- `RecordFailure` writes before deleting stale stages;
- failure and snapshot times are compared on the same (store) clock;
- shared constants moved to `lego/types`;
- the dashboard has one typed stage list.

**Tests:**
- operator stage classification for config (no mount, no key, malformed key) and upload, without S3;
- against a real S3 API (SeaweedFS stand-in, since MinIO images are no longer pullable): an unreadable volume is recorded as `read` and the next success clears it; records replace one another and are purged;
- backend failing / history / no-snapshot / precedence / not-restorable;
- dashboard card and hook.

**Not exercised live:** a production failure was not induced. The success path is the same Job that produced the verified 2026-10-25 nightly snapshot.
