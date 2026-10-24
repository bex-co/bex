# w6 · m154 — A failed nightly disk backup says why

**Worker:** worker6 **Goal:** when a disk's nightly snapshot Job fails, the reason survives the Job and reaches the owner and the operator, instead of only "overdue" **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Operator: record the last backup Job failure on the App | 60m | — |
| t002 | bex-api + dashboard: surface the failure through diskSnapshotHealth | 45m | w6/m154/t001 |
| t003 | Keep failed backup pods long enough to diagnose | 20m | — |
| t004 | Render parity | 15m | w6/m154/t002 |
| t005 | Simplify | 15m | w6/m154/t004 |
| t006 | Test coverage | 20m | w6/m154/t004 |
| t007 | Closeout | 10m | w6/m154/t005, w6/m154/t006 |

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
