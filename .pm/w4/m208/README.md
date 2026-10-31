# w4 · m208 — Datastore plan previews match their ready cards

**Worker:** worker4 **Goal:** Postgres and Key Value plan cards preserve their ready title/description, responsive tier grid and footer geometry through resource, catalog and lazy loading. **Status:** todo（2026-10-31 解除阻塞；已完成任务保持 DONE，详见文末“解除阻塞记录”）

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Define one inert datastore plan preview from ready geometry — **DONE 2026-10-14** | 30m | — |
| t002 | Use the shared preview in both Key Value loading branches — **DONE 2026-10-14** | 20m | t001 |
| t003 | Align the complete Postgres plan loading chain — **DONE 2026-10-14** | 35m | t001 |
| t004 | Render parity — **DONE 2026-10-14** | 15m | t002, t003 |
| t005 | Simplify — **DONE 2026-10-14** | 10m | t004 |
| t006 | Test coverage — local done; live geometry replay pending (Postgres checks under D1, 2026-10-31) | 40m | t004 |
| t007 | Closeout | 10m | t005, t006 |

## Definition of done

- On owned private Free Postgres and Key Value fixtures at 1440×1000 and 390×844, delay the unchanged resource read. The plan preview preserves its title/description, three tier slots and bordered Cancel/Save footer. For the exercised catalogs it has the same 237px desktop / 441px mobile bounds as ready.
- After each resource is visible, delay its catalog response with complete batch receipts. The plan content remains structurally reserved; no empty Key Value radio group creates a 70/234px expansion and no Postgres 80px/gap8 placeholder creates a 10/22px shrink. Tier slots match the 70px ready cards and 12px gap; header/footer/text wrapping stay consistent.
- Independently verify the Postgres plan's cold lazy-module fallback and offscreen-to-mounted reservation at both widths. They use matching plan geometry without changing unrelated DeferredMount behavior.
- During later reads keep an already loaded/cached grid visible. Preserve Free selection, disabled unchanged actions, actual API prices/specs, permission reasons, Postgres compatibility messages and catalog error/empty behavior. No fake resource, plan eligibility, actionable placeholder radio or API change.
- Preserve the ten unrelated ReservedCard call sites, the two PlanCardGrid create consumers, their three create-preview callers, other detail regions and existing alias redirects. Complete locale, permission, unknown-plan, changed-catalog and reduced-motion dispositions and relevant dashboard checks.
- Reproduce the fixes live, delete exact owned fixtures, restore baseline inventory ids and revoke the own session. Close only when the source-only/unprobed states and all earlier criteria are independently verified.

## Source + Goal linkage

- **Source:** user-requested infinite functional `$qa-find-bugs w4`, muse.env only through the login helper; a454/a465/a466, 2026-10-14. Promoted from w4/301: [historical source](source-301.md), [expanded finding](finding.md), [Key Value resource](../evidence/301-keyvalue-plan-skeleton.json), [Key Value catalog](../evidence/301-keyvalue-catalog-loading.json), [Postgres](../evidence/m208-postgres-plan-loading.json). Severity minor; filing only.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) pillar 1, usable hosting configuration; supplied skeleton-shape rule and w5/m79 t009's faithful datastore previews; [ADR009](../../../docs/ADR009-postgresql-management.md), [ADR021](../../../docs/ADR021-keyvalue-management.md).
- **Expected outcome:** data arrival replaces placeholders with the same plan-card anatomy rather than expanding/shrinking its grid and footer.
- **Why now:** fresh live evidence extends the original 60m Key Value note to two datastore families, four live loading branches and two remaining Postgres source-traced stages. This is one shared pending-chain fix with multiple implementation tasks, so it is promoted instead of duplicating the issue or keeping an artificially sub-hour note.
- **Scope:** dashboard preview/reservation only. [m207](../blocked/m207/README.md) owns radio naming/keyboard interaction; [298](../298.md), [299](../299.md) and [303](../303.md) own different detail regions. Source arrays, prices, eligibility, plan updates, authorization and readiness semantics are preserved.
- **Standing closing tasks:** parity preserves existing UI/API-driven behavior and conscious Render differences; simplify, meaningful tests and live closeout are included. No signed-in Render loading-layout comparison is claimed.

**Total estimate:** 2h40.

## 解除阻塞记录（2026-10-31，`/unblock w4`）

- **涉及：** w4/m208（同组 D1：w4/m223、w4/m215、w4/m214、w4/m179、w4/m167、w4/m170、w4/325、w4/177、w4/364）
- 来源：用户明确决定（2026-10-31，`/unblock w4`）——“i approve you to update bex-canary's subscription to paid and operate anything there but please clean up afterwards”；以及 2026-10-30：“qa-* 的数据库都是暂时的可以清除”。
- **结论（D1，Free Postgres 名额）：** 允许删除 `qa-20261017-64552e-pg14`（`dpg-db9r654flioc739mcpbg`，2026-10-29 只读确认仍 available、占用 bex-canary 唯一 Free 名额；w8/w2 只把它当只读基线）。DoD 明确要求 Free 的检查使用腾出的 Free 名额（同一时间只能一个项占用，验完立即删除再交给下一项）；DoD 未限定 Free、或需要多个库的检查可用短期付费 Postgres。 bex-canary（`tea-daif693dqjvc73e7as3g`）计费状态为 `operator_excluded: structural exclusion`，不走 Stripe 收款（2026-10-29 只读核实 `workspaceBillingReadiness`），付费 fixture 与付费套餐不产生真实费用。
- 生产已部署 `25b99d8a373a`（`deploy/gitops/base/bex.yaml`，2026-10-30 pin），本项实现均早于此版本；部署不等于验收，以下仍是待验证工作。
- **worker 下一步：** 先读 `GET /v1/postgres?ownerId=tea-daif693dqjvc73e7as3g` 确认 pg14 是否仍在（他人可能已删）；仍在则删除并确认 404；然后按本项原 DoD 重放。
- **清理（用户要求，验收的一部分）：** 验收结束删除本次在 bex-canary 创建的全部资源；若为此把 bex-canary 切到付费套餐，所有依赖付费档的项做完后切回 Hobby；在本条记录清理结果（删除后 404 / 列表为空、套餐读回 Hobby）。
- **剩余阻塞：** 无（门槛已核实解除，交给 `/loopx w4`）。
- **验收：** 原 DoD / Acceptance 不变，未删减。
