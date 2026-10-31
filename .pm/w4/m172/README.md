# w4 · m172 — By-id REST verbs resolve the resource's own workspace, not the caller's default

**Worker:** worker4 **Goal:** every by-id verb on every resource family finds the resource in whichever of the caller's workspaces owns it, as services, env groups and projects already do. A multi-workspace user no longer gets a 404/403 for their own resource unless they guess `?ownerId=`. **Status:** todo（2026-10-31 解除阻塞；已完成任务保持 DONE，详见文末“解除阻塞记录”）

## Tasks (in order)

| id   | title                                                           | est | depends_on                               |
| ---- | --------------------------------------------------------------- | --- | ---------------------------------------- |
| t001 | Webhooks: by-id verbs resolve the endpoint's workspace — **DONE**          | 45m | —                                        |
| t002 | Sweep the remaining families and fix each that scopes by default — **DONE** | 1h  | —                                        |
| t003 | Shared guard so a new by-id verb can't regress — **DONE**                  | 30m | w4/m172/t001, w4/m172/t002               |
| t004 | Render parity across by-id surfaces — **DONE**                             | 30m | w4/m172/t003                             |
| t005 | Simplify changed code — **DONE**                                           | 20m | w4/m172/t004                             |
| t006 | Test coverage for shipped behavior — **DONE**                              | 45m | w4/m172/t004                             |
| t007 | Closeout                                                        | 15m | w4/m172/t006                             |

## Definition of done

The QA user belongs to `bex` (default), `tian-personal` and `bex-canary` (`tea-daif693dqjvc73e7as3g`). Probes run from the dashboard origin against owned `qa-` fixtures in `bex-canary`:

- **Webhooks:** `GET`, `PATCH` and `DELETE /v1/webhooks/<id>` and `GET /v1/webhooks/<id>/events` without `ownerId` return 200/204, matching their `?ownerId=` responses. Today: `GET` 404 and `DELETE` 404 without the owner; `GET` 200, events 200 (six 200 deliveries) and `DELETE` 204 with it, on `whk-db1h3s68inbs73f0v75g`.
- **Every family enumerated in t002** (at least registry credentials, sandboxes, agent sessions, Postgres/Key Value by-id sub-verbs, environments) shows the same: no `ownerId` needed for the caller's own resource. Each family's result is recorded with the probe that proved it.
- **Controls, unchanged:** services, env groups and projects keep resolving by id; Blueprints (w4/m169) and API keys (w4/194) keep their shipped behavior.
- **Adjacent classes:** per ADR072 #8, a typed-id non-member gets **403** ("typed-id 403 stays"), and a genuinely missing id gets 404. A supplied, mismatched `ownerId` answers like the non-member case. (Corrected 2026-10-05: the original bullet asked for 404 for non-members, contradicting ADR072 #8 and breaking `TestEnvGroupReadSideOwnerIDTargetingE2E`. See w4/199.)

## Source + Goal linkage

- **Source:** infinite `/qa-find-bugs` loop23 2026-10-05 UTC (muse.env, `bex-canary`). A webhook delivery journey into an owned echo sink: deliveries arrived as `POST /hook` 200 from `Go-http-client/2.0`. Cleanup then hit the defect on `whk-db1h3s68inbs73f0v75g` (deleted with `?ownerId`). Third family of the same class after w4/m169 (Blueprints, by-id resolved via `resolveTenantID`) and w4/194 (API key revoke).
- **Root cause (webhooks):**
  - `lego/backend/internal/webhooks/rest.go:200-254` reads `q.Get("ownerId")` and passes it to the service verbs, which scope to it or else to the caller's default workspace.
  - The working pattern is `envgroups.fetchGroup` (`internal/envgroups/service.go:1667-1679`): load the resource by id, then `AuthorizeLabeled` on its own workspace.
  - Other families whose REST handlers read an optional `ownerId` for by-id routes: `internal/registrycreds/rest.go:150-183`, `internal/sandbox/rest.go:103-165`, `internal/agentsessions/rest.go:71-88`. These are candidates, not yet probed.
- **Goal linkage:** ADR006/ADR018 Render-compatible API. Render's by-id endpoints take no owner parameter, and API keys span all of a user's workspaces.
- **Expected outcome:** CLI/API/MCP callers with several workspaces can read, update and delete their own resources by id across every family. One shared resolver makes the class impossible to reintroduce.
- **Why now:** three families have now shipped the same defect one at a time (m169, 194, this). A sweep plus a guard is cheaper than a fourth live finding. Render parity task INCLUDED (t004): REST/GraphQL/MCP surfaces change.
- **Unverified:** registry credentials, sandboxes, agent sessions and datastore sub-verbs (named from code, not probed live); GraphQL and MCP webhook by-id behavior.
- **Severity:** major (multi-workspace users' own resources are unreachable by id; a "not found" on DELETE leaves a webhook live).

## 解除阻塞记录（2026-10-31，`/unblock w4`）

- **涉及：** w4/m172（同组 D2：w4/m237、w4/m219、w4/m220、w4/m212、w4/295）
- 来源：用户明确决定（2026-10-31，`/unblock w4`）——“i approve you to update bex-canary's subscription to paid and operate anything there but please clean up afterwards”；以及 2026-10-30：“qa-* 的数据库都是暂时的可以清除”。
- **结论（D2，Pro+ 门槛）：** 允许 worker 用 `ChangeWorkspacePlan` 把 bex-canary 切到 Pro（或该功能所需最低付费档），在一个窗口内集中重放全部 Pro 门槛项（w4/m237、m219、m220、m212、m172、295），删除自建 webhook 等资源后切回 Hobby，并在各项记录升/降级时间。窗口期内 bex-canary 读作 Pro，其他会话的 Hobby 限制类测试读数可能不同，因此窗口尽量短。不使用 `bex`（Scale，真实计费）workspace。 bex-canary（`tea-daif693dqjvc73e7as3g`）计费状态为 `operator_excluded: structural exclusion`，不走 Stripe 收款（2026-10-29 只读核实 `workspaceBillingReadiness`），付费 fixture 与付费套餐不产生真实费用。
- 生产已部署 `25b99d8a373a`（`deploy/gitops/base/bex.yaml`，2026-10-30 pin），本项实现均早于此版本；部署不等于验收，以下仍是待验证工作。
- **worker 下一步：** 与同组其他项排在同一个 Pro 窗口内，按本项原 DoD 重放剩余的 Pro 门槛检查。
- **清理（用户要求，验收的一部分）：** 验收结束删除本次在 bex-canary 创建的全部资源；若为此把 bex-canary 切到付费套餐，所有依赖付费档的项做完后切回 Hobby；在本条记录清理结果（删除后 404 / 列表为空、套餐读回 Hobby）。
- **剩余阻塞：** 无（门槛已核实解除，交给 `/loopx w4`）。
- **验收：** 原 DoD / Acceptance 不变，未删减。
