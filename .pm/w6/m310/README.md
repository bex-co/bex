# w6 · m310 — Performance pipeline tier (16 CPU / 64 GB builds)

**Worker:** worker6 **Goal:** a Pro-or-above workspace can choose Render's Performance build pipeline: builds on 16 CPU / 64 GB, metered and priced separately from Starter minutes. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Record Render's Performance tier surface](t001.md) | 30m | — |
| t002 | [A Performance build pool](t002.md) | 1.5h | t001 |
| t003 | [The workspace setting and the operator's Performance shape](t003.md) | 2h | t002 |
| t004 | [Meter and price Performance minutes separately](t004.md) | 1.5h | t003 |
| t005 | [Render parity](t005.md) | 30m | t004 |
| t006 | [Simplify](t006.md) | 15m | t005 |
| t007 | [Test coverage](t007.md) | 30m | t006 |
| t008 | [Closeout](t008.md) | 10m | t007 |

## Definition of done

- t001's Render record is in this README.
- On production, a Pro workspace switched to Performance runs its next build on a 16 CPU / 64 GB node from a pool that scaled from zero, and that build's minutes are metered and estimated as Performance minutes, not Starter ones.
- A Hobby workspace is refused the setting.

## Source + Goal linkage

- **Source:** this session's 2026-11-10 discussion of the tenant build experience against Render, then `/pm-brainstorm for w6` (same session) and `/pmx them all to w6`; ADR018 § Gap backlog row "Performance pipeline tier"; ADR060 non-goals (reworded 2026-11-10: a parity gap, not a non-goal).
- **Goal linkage:** ADR008 Render parity first; Render offers Pro-and-above workspaces a Performance pipeline (16 CPU / 64 GB, $25 per 1,000 minutes, no included minutes; [build pipeline](https://render.com/docs/build-pipeline)).
- **Expected outcome:** large builds can run on Render's larger build machine.
- **Why now:** the parity rule requires every gap to have its own scheduled follow-up rather than a parked note. **Expected gate:** the Hetzner dedicated-vCPU limit that blocks a ccx43 node (`w2/blocked/m250`, `w6/blocked/114`); t002's first step checks it, and `/loopx` parks the milestone there if it still holds.
- **Render parity task:** included (workspace setting, builds and billing are tenant-visible).
