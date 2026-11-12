# w6 · m333 — Deleting a disk or a service closes its usage coverage, and a deleted service's last egress hours are billed

**Worker:** worker6 **Goal:** A deleted service's bandwidth is metered through its deletion hour, and neither a deleted service nor a deleted disk leaves the month's coverage degraded or behind. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Close a deleted disk's coverage stream](t001.md) | 45m | — |
| t002 | [Keep a service's egress identity past its deletion](t002.md) | 1h | — |
| t003 | [Meter a deleted service's egress through its deletion hour](t003.md) | 1.5h | t002 |
| t004 | [Coverage reads healthy after a disk or service deletion](t004.md) | 30m | t001, t003 |
| t005 | [Render parity](t005.md) | 30m | t004 |
| t006 | [Simplify](t006.md) | 15m | t005 |
| t007 | [Test coverage](t007.md) | 30m | t006 |
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
