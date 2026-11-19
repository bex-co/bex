# w6 · m333 — Deleting a disk or a service closes its usage coverage, and a deleted service's last egress hours are billed

**Worker:** worker6 **Goal:** A deleted service's bandwidth is metered through its deletion hour, and neither a deleted service nor a deleted disk leaves the month's coverage degraded or behind. **Status:** done — verified on production 2026-11-19: the fixture disk deleted from a live service closed its stream healthy, and no deletion since the fix left a coverage hole (see Done — 2026-11-19).

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
| t008 | [Closeout](done/t008.md) — **DONE** | 10m | t007 |

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

## 解除阻塞 — 2026-11-16（用户决定，`/unblock`）

- **涉及 ID：** w6/m333（t008）。
- **来源：** 用户明确决定。`/unblock` 2026-11-16 的 D3，用户选「授权建短时付费服务」。
- **结论：** 授权 worker 在 `bex-canary` 建**一个**短时付费服务：最便宜的付费实例类型，挂 1 GB 磁盘。确认磁盘的计量流已经开始后删除磁盘（或连同服务一起删），费用在美分级。授权只覆盖这一个测试资源；用完按 ID 删除，并确认 `GET` 返回 404。
- **worker 先查：** 生产上是否已有删除超过 50 小时的磁盘计量流（客户自然删除）。有就直接读它，不建测试资源。
- **对标依据：** Render 删除磁盘后停止计费，账单上也不出现额外的说明；本项只差这条在生产上的验收。
- **worker 下一步：** 建好并删除测试磁盘后，磁盘删除满 50 小时之前按 loopx 规则把 m333 停回 `blocked/`，门槛写成「删除时间 + 50 h 到点后，worker 自己就能读」。到点后读 `bex-canary` 的 `degradedSources`：`disk` 不在其中，DoD 第 2 条即成立，t008 关闭 milestone。
- **剩余门槛：** 无用户门槛。

## BLOCKED — 2026-11-17: the fixture disk's 50 h read

- **No organic case.** Read-only on production's `bex-db` at 02:55Z. Every deleted disk had gone with its service: each closed `disk_gb_seconds` stream ended in the same hour as its service's other streams, and no disk stream had ever closed while its service's `instance_seconds` stream stayed open.
- **The fixture (authorized 2026-11-16).** In `bex-canary`, `POST /v1/services` created the `starter` image service `qa-m333-disk` (`srv-dbts58fa12vs7398m010`, `traefik/whoami`) at 02:57:38Z with the 1 GB disk `dsk-dbts58fa12vs7398m020`. Its deploy went live.
  - The disk stream started at about 03:43Z with window 02:00Z healthy.
  - `DELETE /v1/disks/dsk-dbts58fa12vs7398m020` answered 204 at 03:43:14Z and the disk's `GET` answered 404. The service stayed live (`disk: null`).
  - Window 03:00Z metered the disk's last 43 min (2,595 GB-s). At about 05:40Z the disk meter's pass for window 04:00Z closed the disk stream at 04:00Z through `CloseIdleUsageSourceStreams` (the w4/575 path), while the instance stream stayed open and was metered through 04:00Z.
  - Then `DELETE /v1/services/srv-dbts58fa12vs7398m010` answered 204 at 05:41:41Z. The service's and the disk's `GET` both answer 404, `usage_retired_resources` holds it, and no App CR is left. Cost: under 3 h of `starter` plus 46 min of 1 GB.
- **`bex-canary`'s November `degradedSources` already lists `disk`, so the workspace read alone cannot pass this month.** The cause is an older stream, `srv-dbpcja9pjd2c73f0qqjg`. Its disk lived for 30 s on 2026-11-10 07:40Z, and its service was deleted the next hour. The stream expects [07:00Z, 09:00Z) and has no row at 08:00Z, a hole settled since 2026-11-12. It predates both w4/575 (2026-11-11) and m333's rule that a close never extends a stream that already ended (2026-11-12), and it is the only disk hole in `bex-canary`. Coverage is month-scoped, so it leaves on 2026-12-01, but so does the fixture. The read therefore checks the fixture's own stream and attributes the workspace's `disk` entry.
- **Found on the way: w6/132.** A disk meter pass cut off mid-window (bex-api rolled out three times between 02:49Z and 03:48Z) left window 2026-11-17 02:00Z unmetered for three live disks in `bex`. The cursor is the newest disk row overall, so that window is never retried. The fixture's stream is not exposed to it: its windows 02:00Z and 03:00Z were both written, and the idle close ends a stream after its last written window.
- **Gate (no user gate; clears by itself): at or after 2026-11-19 06:00Z.** That is the disk deletion plus 50 h, rounded up to the first hour at which the stream's first disk-less window (2026-11-17 04:00Z) is past the 49 h settled edge (`usageSettledBefore`). A stream left open would read `disk` degraded from then on. Then t008:
  1. Reads `GET /v1/usage?ownerId=tea-daif693dqjvc73e7as3g` signed in as the QA user, and records `coverage.degradedSources`.
  2. On `bex-db`, read-only, checks that the fixture's stream `(service, srv-dbts58fa12vs7398m010, disk_gb_seconds, disk)` still expects [2026-11-17 02:00Z, 04:00Z) with both windows `healthy`. It also checks that the only missing or non-healthy window in any `bex-canary` disk stream this month is `srv-dbpcja9pjd2c73f0qqjg`'s 2026-11-10 08:00Z.
  3. If both hold, the deleted disk did not make `disk` a degraded source: DoD clause 2 holds. Verify clauses 1 and 3 again, then close the milestone.

## Unblocked — 2026-11-19

- **Affected IDs:** w6/m333 (t008).
- **Source:** verified finding. The gate `at or after 2026-11-19 06:00Z` (the fixture disk's deletion at 2026-11-17 03:43:14Z plus 50 h, rounded up) has passed: it is 2026-11-19 22:35Z. No user gate was left.
- **Next:** t008 runs the read the BLOCKED — 2026-11-17 gate lists: `bex-canary`'s `coverage.degradedSources` on `GET /v1/usage` as the QA user, then, read-only on `bex-db`, the fixture's disk stream and every other `bex-canary` disk stream's windows this month. If the fixture's stream reads [02:00Z, 04:00Z) healthy and the only disk hole is the pre-fix `srv-dbpcja9pjd2c73f0qqjg` 2026-11-10 08:00Z, DoD clause 2 holds; verify clauses 1 and 3 again and close the milestone.

## Done — 2026-11-19

- **Clause 2 holds on production.** It is 64 h after the fixture's disk deletion. The fixture's disk stream expects [2026-11-17 02:00Z, 04:00Z), and both windows are healthy.
- **`disk` is on `bex-canary`'s November list for one reason only:** the pre-fix hole of `srv-dbpcja9pjd2c73f0qqjg` at 2026-11-10 08:00Z. It is the only missing or non-healthy window in any of the workspace's 8 November disk streams.
- **The 2026-11-16 decision's literal check cannot pass this month.** That check was `disk` absent from the list, and the pre-fix hole keeps it there until 2026-12-01. The attribution read stands in for it, as the 2026-11-17 record set out.
- **No `bex-canary` stream of any source has a missing window** from 2026-11-12 12:00Z to the 49 h settled edge. Its other degraded names come from health-gated `degraded` readings, not from deletions (t008).
- **Clauses 1 and 3 still hold.** The probe's deletion-hour `egress_bytes` row reads on `GET /v1/usage`, and `through` is the latest closed hour (2026-11-19T22:00Z). ADR023 and ADR082 D9 record the rules.
- **Details:** [t008](done/t008.md).
