# w6 · m310 — Performance pipeline tier (16 CPU / 64 GB builds)

**Worker:** worker6 **Goal:** a Pro-or-above workspace can choose Render's Performance build pipeline: builds on 16 CPU / 64 GB, metered and priced separately from Starter minutes. **Status:** blocked (t001 done; t002–t004 wait on the Hetzner dedicated-vCPU limit)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Record Render's Performance tier surface](done/t001.md) — **DONE** | 30m | — |
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

## Render record (t001, 2026-11-10)

Read from [render.com/docs/build-pipeline](https://render.com/docs/build-pipeline) on 2026-11-10, and from the pinned public OpenAPI (`lego/backend/internal/api/openapi/render-public-api-1.json`):

- **Who:** "Pro workspaces and higher can choose between two pipeline tiers: Starter and Performance."
- **Where:** a workspace-wide setting: "In the Render Dashboard, go to your Workspace Settings page. In the Build Pipeline section, select a pipeline tier. Confirm your selection in the dialog that appears." The public API and MCP have no pipeline operation, so bex would offer it on GraphQL and Workspace Settings → Build Pipeline (the card `w6/m309` added).
- **Machine:** 16 CPU / 64 GB RAM, against Starter's 2 CPU / 8 GB.
- **Price:** $25 per 1,000 minutes, so $0.0175/min at bex's 30% off (ADR030 §2). No included minutes, and "pipeline minutes are specific to their associated tier": Starter's included minutes never pay for Performance builds.
- **Spend limit:** the workspace's one pipeline spend limit applies to whichever tier is active.

**Capacity:** 64 GB needs a dedicated-vCPU Hetzner type (ccx43: 16 dedicated vCPU / 64 GB). The largest shared types, cx53 and cpx62, have 16 vCPU but only 32 GB. The project's dedicated-vCPU limit is below 16 (`w2/blocked/m250`, `dedicated core limit exceeded` on 2026-11-02), so not one Performance build node can start today.

## Source + Goal linkage

- **Source:** this session's 2026-11-10 discussion of the tenant build experience against Render, then `/pm-brainstorm for w6` (same session) and `/pmx them all to w6`; ADR018 § Gap backlog row "Performance pipeline tier"; ADR060 non-goals (reworded 2026-11-10: a parity gap, not a non-goal).
- **Goal linkage:** ADR008 Render parity first; Render offers Pro-and-above workspaces a Performance pipeline (16 CPU / 64 GB, $25 per 1,000 minutes, no included minutes; [build pipeline](https://render.com/docs/build-pipeline)).
- **Expected outcome:** large builds can run on Render's larger build machine.
- **Why now:** the parity rule requires every gap to have its own scheduled follow-up rather than a parked note. **Expected gate:** the Hetzner dedicated-vCPU limit that blocks a ccx43 node (`w2/blocked/m250`, `w6/blocked/114`); t002's first step checks it, and `/loopx` parks the milestone there if it still holds.
- **Render parity task:** included (workspace setting, builds and billing are tenant-visible).

## BLOCKED — 2026-11-10

- **Gate (Hetzner / account owner): dedicated vCPUs.** A Performance build needs a 16 CPU / 64 GB node, which on Hetzner is a ccx43 (dedicated vCPU). The project's dedicated-vCPU limit is below 16, the same gate as `w2/blocked/m250` (Pro Ultra) and `w6/blocked/114`. Offering the setting before a node can start would accept Performance builds that never get a machine.
- **Release:** the account owner raises the dedicated-vCPU limit (Hetzner Console → Limits, or a support ticket) to at least 16 for one Performance build node, or at least 48 to also run m250's Pro Ultra pool at full size. Then t002 adds the pool (min 0) and its validator guards, t003 the workspace setting and operator shape, t004 metering and pricing. The rest of the build-pipeline parity (`m306`–`m309`) does not wait on it.

## 复核与用户决定 — 2026-11-16（`/unblock`）

- **涉及 ID：** w6/m310、w6/114（与 `w2/blocked/m250` 是同一个门槛）。
- **复核（Hetzner API `GET /server_types`，2026-11-16）：**
  - CX 机型（cx33、cx43、cx53）在 fsn1、nbg1、hel1 全部缺货；ARM 的 cax 也缺货。
  - 就算有货也装不下：共享 vCPU 机型最大只有 32 GB 内存（cx53、cpx62、cpx51、cax41）。m310 的 16 CPU / 64 GB 构建，以及 114 的四个独占规格（`4c-32g`、`8c-64g`、`12c-48g`、`12c-96g`），都只能跑在独占 vCPU 的 CCX 机型上。
  - CCX（ccx23–ccx63）在三个 EU 机房都有货，卡在项目的独占 vCPU 配额上。
- **用户决定：** 原话「你看一下 CX 的机器有吗？没有的话就没有了」。CX 没货，也装不下这些规格，所以暂不申请提高独占 vCPU 配额，接受等待。
- **接受的代价：** Pro 及以上 workspace 没有 Performance 构建可选；客户在 Blueprint 或 API 里写这些大规格时会被拒绝。
- **剩余门槛（用户）：** 用户以后决定申请提额（≥ 16 释放 m310；≥ 48 同时释放 m250 的 Pro Ultra 和 114 的独占规格），或 Hetzner 推出内存够大的共享机型。在此之前 worker 无事可做，不要重复问。
