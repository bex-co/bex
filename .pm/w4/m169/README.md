# w4 · m169 — Blueprint by-id verbs resolve the Blueprint's own workspace

**Worker:** worker4 **Goal:** a Blueprint id works like a service or env-group id: every by-id verb finds it in whichever of the caller's workspaces owns it, with no `ownerId` needed **Status:** todo（2026-10-31 解除阻塞；已完成任务保持 DONE，详见文末“解除阻塞记录”）

## Tasks (in order)

| id   | title                                                          | est | depends_on                 |
| ---- | -------------------------------------------------------------- | --- | -------------------------- |
| t001 — **DONE** | Resolve the owning workspace from the Blueprint id             | 1h  | —                          |
| t002 — **DONE** | Dashboard Blueprint deep links across workspaces               | 30m | w4/m169/t001               |
| t003 — **DONE** | Render parity across Blueprint by-id surfaces                  | 30m | w4/m169/t001, w4/m169/t002 |
| t004 — **DONE** | Simplify changed code                                          | 20m | w4/m169/t003               |
| t005 — **DONE** | Test coverage for shipped behavior                             | 45m | w4/m169/t003               |
| t006 | Closeout                                                       | 15m | w4/m169/t005               |

## Definition of done

Probes run from the dashboard origin (page `fetch`, `credentials:'include'`). The signed-in QA user belongs to `bex` (default workspace), `tian-personal` and `bex-canary`; Blueprint `blp-db136288mmqc73d4hpug` belongs to `bex-canary`:

- REST `GET /v1/blueprints/blp-db136288mmqc73d4hpug` with **no** `ownerId` returns 200 and the Blueprint (today: 404 `blueprint not found`; with `?ownerId=tea-daif693dqjvc73e7as3g` it is 200). The same holds for `GET /v1/blueprints/<id>/syncs` (today 404 vs 200 `[]`), and for `PATCH`, `DELETE` and `POST /sync` on an owned `qa-` fixture Blueprint.
- GraphQL `blueprint(id:)` without `ownerId` returns the Blueprint (today: "blueprint not found"), and the MCP blueprint read tool agrees.
- Control, kept: `GET /v1/services/srv-daif6dsmg29s73d1umvg` and `GET /v1/env-groups/<id>` without `ownerId` already resolve across the caller's workspaces, and still do.
- Adjacent classes: a Blueprint id in a workspace the caller is **not** a member of returns the same 404 as a nonexistent id, not a 403 (no existence oracle). A viewer-role member still gets the manifest gated by `can_view_sensitive` (round-7 #11 unchanged).
- Dashboard: with `tian-personal` selected, opening `/blueprints/blp-db136288mmqc73d4hpug` shows that Blueprint, or an explicit "belongs to bex-canary — switch?" state (target behavior chosen in t002). Today it silently redirects to `/`; a bex-canary service URL opens normally.

## Source + Goal linkage

- **Source:** infinite `/qa-find-bugs` loop7 2026-10-04 UTC (muse.env, `bex-canary`), journey 13. The workspace was switched to `tian-personal` for the deep-link check, then restored. Probes as listed in the DoD. Root cause:
  - `lego/backend/internal/apps/blueprint.go:1052-1073` `GetBlueprintByID` scopes the store read to `resolveTenantID(ctx)` (`blueprint.go:1642-1655`), which is the request's `ownerId` or else the caller's **default** workspace. It never looks up which workspace owns the id.
  - The REST routes forward an optional `?ownerId` (`internal/apps/rest.go:1278-1300`, `1392`).
  - Env groups do it right: `envgroups.fetchGroup` (`internal/envgroups/service.go:1667-1679`) loads the group by id, then `AuthorizeLabeled` on the group's own workspace.
- **Blast radius:** the five shared verbs `GetBlueprintByID`, `ListBlueprintSyncs`, `UpdateBlueprint`, `DisconnectBlueprint` and `SyncBlueprint` (enumerated by `w6/m96` t004), each served on REST, GraphQL and MCP.
- **Goal linkage:** ADR006/ADR018 Render-compatible API. Render's `GET /blueprints/{blueprintId}` takes no owner parameter, and the Render CLI/API keys span every workspace a user belongs to.
- **Expected outcome:** an API, CLI or MCP caller with several workspaces can read and operate their Blueprints by id, and shared or bookmarked Blueprint links work like service links.
- **Why now:** today a multi-workspace user's own Blueprint is "not found" on every by-id call unless they already know its workspace id. That's a silent 404 the dashboard turns into an unexplained redirect. Render parity task INCLUDED (t003): REST/GraphQL/MCP/UI surfaces change.
- **Unverified:** whether the pinned Render CLI calls these endpoints without an owner (not probed); MCP tools (same service functions).
- **Severity:** major.

## 解除阻塞记录（2026-10-31，`/unblock w4`）

- **涉及：** w4/m169（同组 D4：w4/m229）
- 来源：用户明确决定（2026-10-31，`/unblock w4`）——“i approve you to update bex-canary's subscription to paid and operate anything there but please clean up afterwards”；以及 2026-10-30：“qa-* 的数据库都是暂时的可以清除”。
- **结论（D4，非本次测试的旧 Blueprint）：** 允许删除 bex-canary 的 Blueprint `blp-db136288mmqc73d4hpug`（"bex"，2026-10-04 创建，autoSync，跟踪 `bex-co/bex@main:render.yaml`——该文件在 main 上已不存在；不管理任何资源，2026-10-29 只读核实）。删除后用 worker 自建的 Blueprint 跑验收，完成后删除自建 Blueprint 及其资源。不需要 QA 分支。 bex-canary（`tea-daif693dqjvc73e7as3g`）计费状态为 `operator_excluded: structural exclusion`，不走 Stripe 收款（2026-10-29 只读核实 `workspaceBillingReadiness`），付费 fixture 与付费套餐不产生真实费用。
- 生产已部署 `25b99d8a373a`（`deploy/gitops/base/bex.yaml`，2026-10-30 pin），本项实现均早于此版本；部署不等于验收，以下仍是待验证工作。
- **worker 下一步：** 删除该 Blueprint 并确认 404，再按本项原 DoD 重放剩余检查。
- **清理（用户要求，验收的一部分）：** 验收结束删除本次在 bex-canary 创建的全部资源；若为此把 bex-canary 切到付费套餐，所有依赖付费档的项做完后切回 Hobby；在本条记录清理结果（删除后 404 / 列表为空、套餐读回 Hobby）。
- **剩余阻塞：** 无（门槛已核实解除，交给 `/loopx w4`）。
- **验收：** 原 DoD / Acceptance 不变，未删减。
