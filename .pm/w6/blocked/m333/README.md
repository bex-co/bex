# w6 · m333 — Deleting a disk or a service closes its usage coverage, and a deleted service's last egress hours are billed

**Worker:** worker6 **Goal:** A deleted service's bandwidth is metered through its deletion hour, and neither a deleted service nor a deleted disk leaves the month's coverage degraded or behind. **Status:** blocked — everything is done and verified on production except the disk clause's production read, which needs a disk deleted from a live service and read 50 h later. None exists yet: the fix (w4/575) shipped 2026-11-11, and creating one needs a paid service with a disk, which is the user's call.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Close a deleted disk's coverage stream](done/t001.md) — **DONE** | 45m | — |
| t002 | [Keep a service's egress identity past its deletion](done/t002.md) — **DONE** | 1h | — |
| t003 | [Meter a deleted service's egress through its deletion hour](done/t003.md) — **DONE** | 1.5h | t002 |
| t004 | [Coverage reads healthy after a disk or service deletion](done/t004.md) — **DONE** | 30m | t001, t003 |
| t005 | [Render parity](done/t005.md) — **DONE** | 30m | t004 |
| t006 | [Simplify](done/t006.md) — **DONE** | 15m | t005 |
| t007 | [Test coverage](done/t007.md) — **DONE** | 30m | t006 |
| t008 | [Closeout](t008.md) | 10m | t007 |

## Definition of done

- On production, a service deleted in the QA workspace has its deletion hour's `egress_bytes` row on `GET /v1/usage`, and its streams stop holding the workspace's `through` once that hour is metered.
- A disk deleted from a live service never makes `disk` a degraded source (test; read on production 50 h after a deletion).
- ADR023 records it.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` 2026-11-11, item 2; `w4/575` (absorbed: moved to `w4/done/575.md` with a pointer here); the w4/496 production read, which found the deleted-resource pass meters only instance and build seconds, so every deleted service's last egress hours are never metered.
- **Goal linkage:** ADR008 Render parity first; Render bills outbound bandwidth for all of it ([outbound bandwidth](https://render.com/docs/outbound-bandwidth), ADR030 §14), and its billing shows no false caveat.
- **Expected outcome:** no undercounted bandwidth at deletion, and no coverage caveat a deletion caused.
- **Why now:** after w4/496 this is the only systematic cause left of a coverage that lags by up to 49 h, and it is a real undercount.
- **Render parity task:** included (usage and the billing caveat are tenant-facing).

## BLOCKED — 2026-11-12: the disk clause's production read

- **Verified on production:**
  - DoD clause 1: the probe service `srv-dbqmev0rcnqs73dmd100` was deleted at 07:18Z, its deletion-hour `egress_bytes` row reads on REST and GraphQL, and its streams close at 08:00Z with every source healthy (t005).
  - Clause 3: ADR023 records it.
- **Clause 2 is covered by tests only so far:** `TestDiskMeterClosesTheStreamOfADeletedDisk` and `TestCloseIdleUsageSourceStreamsKeepsCoverageHealthy`. `disk` is not a degraded source in `bex-canary`'s coverage now.
- **What is missing:** a disk deleted from a live service, read 50 h later. No production disk stream has ended that long ago (2026-11-12 query), because the fix shipped on 2026-11-11. Disks need a paid instance type, so creating a fixture is a paid resource.
- **Release:** the user agrees to a short-lived paid service with a disk in `bex-canary`, or an organic disk deletion happens. Then read the workspace's `degradedSources` 50 h after it, and t008 closes the milestone.
