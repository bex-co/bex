# w6 · m333 — Deleting a disk or a service closes its usage coverage, and a deleted service's last egress hours are billed

**Worker:** worker6 **Goal:** A deleted service's bandwidth is metered through its deletion hour, and neither a deleted service nor a deleted disk leaves the month's coverage degraded or behind. **Status:** todo — t001–t007 done and verified on production; t008 reads the disk clause on production after the 2026-11-16 user authorization of a short-lived paid fixture in `bex-canary` (see 解除阻塞 — 2026-11-16).

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

## 解除阻塞 — 2026-11-16（用户决定，`/unblock`）

- **涉及 ID：** w6/m333（t008）。
- **来源：** 用户明确决定。`/unblock` 2026-11-16 的 D3，用户选「授权建短时付费服务」。
- **结论：** 授权 worker 在 `bex-canary` 建**一个**短时付费服务：最便宜的付费实例类型，挂 1 GB 磁盘。确认磁盘的计量流已经开始后删除磁盘（或连同服务一起删），费用在美分级。授权只覆盖这一个测试资源；用完按 ID 删除，并确认 `GET` 返回 404。
- **worker 先查：** 生产上是否已有删除超过 50 小时的磁盘计量流（客户自然删除）。有就直接读它，不建测试资源。
- **对标依据：** Render 删除磁盘后停止计费，账单上也不出现额外的说明；本项只差这条在生产上的验收。
- **worker 下一步：** 建好并删除测试磁盘后，磁盘删除满 50 小时之前按 loopx 规则把 m333 停回 `blocked/`，门槛写成「删除时间 + 50 h 到点后，worker 自己就能读」。到点后读 `bex-canary` 的 `degradedSources`：`disk` 不在其中，DoD 第 2 条即成立，t008 关闭 milestone。
- **剩余门槛：** 无用户门槛。
