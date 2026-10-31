# w4 · m212 — Keep webhook disable intervals consistent

**Worker:** worker4 **Goal:** disabled-period source events never become new webhook notifications after re-enable, while eligible backlog and admitted retries remain durable. **Status:** todo（2026-10-31 解除阻塞；已完成任务保持 DONE，详见文末“解除阻塞记录”）

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Specify source-time eligibility and migration boundaries — **DONE** | 30m | — |
| t002 | Persist eligibility through every endpoint state writer — **DONE** | 50m | w4/m212/t001 |
| t003 | Enforce interval eligibility during webhook admission — **DONE** | 45m | w4/m212/t002 |
| t004 | Render parity and surface consistency — **DONE** | 25m | w4/m212/t003 |
| t005 | Simplify changed code — **DONE** | 15m | w4/m212/t004 |
| t006 | Prove interval and retry behavior with meaningful tests — **DONE** | 50m | w4/m212/t004 |
| t007 | Replay live evidence and close out | 15m | w4/m212/t005, w4/m212/t006 |

## Definition of done

- On owned Free receiver and webhook A subscribed to Deploy Ended, disable A through Settings, restart the receiver, wait for Live and record the source event ID. A has no attempt while disabled. After enabling A and waiting at least 30 seconds, A still has no attempt or receiver POST for that disabled-period event. Repeat from a fresh Settings load.
- Repeat with a second owned enabled webhook B. B receives the event with HTTP 204; A skips it both before and after re-enable. A's result is the same as the preceding case.
- Restart while A is enabled: its new deploy-ended event receives HTTP 204 and appears in the real receiver and immutable history. Preserve the observed failing-route control: first attempt 503, restore URL, native Resend delivers the same source event/payload as attempt 2 with 204 while attempt 1 remains failed.
- Actual REST/GraphQL/MCP endpoint enabled-state inputs use the same interval decision, without changing their dialect or authorization/error shapes; record which adapters were replayed. Regression tests prove initially disabled creation, auto-disable, pre-disable undispatched eligible events, queued retries, two worker caches, exact cutovers and migration behavior; these classes are source-traced work rather than live claims from a675.
- Clean up every owned endpoint/receiver, restore baseline inventory ID sets, revoke only the QA session, and record results. Passing local tests alone does not close the live bullets.

## Source + Goal linkage

- **Source:** continuous functional `$qa-find-bugs w4`, 2026-10-16 UTC a675, muse.env through the QA login helper. [Finding](finding.md) and [durable requests/responses](evidence-a675.json).
- **Goal linkage:** reliable Render-alternative hosting automation (ADR008, ADR006, ADR052); event delivery must reflect endpoint configuration consistently.
- **Expected outcome:** enabling A cannot replay disabled-period events merely because another endpoint was absent or a dispatcher cursor was behind; enabled-period backlog and admitted retries keep their delivery guarantees.
- **Why now:** two live failures and an opposite passing control establish customer-visible nondeterminism in the existing shipped webhook feature. Resolve eligibility before users depend on disabling automation during changes.
- **Render parity:** included because endpoint state across REST/GraphQL/MCP/dashboard changes delivery eligibility. Render docs do not specify disabled-gap replay; preserve declared bex differences and record any newly chosen divergence. This ships a functional fix, not a new event bus.

## Implementation record (2026-10-16)

- **t001 contract:** each enabled stretch is an interval `[enabled_at, disabled_at)`, start inclusive and end exclusive, over the source row's full-precision time. An initially disabled create opens nothing. A no-op save, a refused write or a failed write leaves the intervals unchanged. The automatic disable closes the open interval like a manual one. Admitted notifications and queued automatic/manual attempts are never re-filtered: send already holds them while the endpoint is disabled and resumes them on enable. Migration 0148 opens enabled rows at `created_at` (the old CreatedAt guard) and closes disabled rows at `updated_at`, which is never earlier than the unrecorded disable time. Closed intervals are pruned at the 90-day delivery-retention horizon.
- **t002 state:** table `webhook_endpoint_eligibility` plus an `AFTER INSERT OR UPDATE OF enabled` trigger on `webhook_endpoints`. That covers create, `SetWebhookEndpointEnabled`, `UpdateWebhookEndpoint` and the auto-disable in `CompleteWebhookAttempt`, each in the writer's own transaction. Service code is unchanged.
- **t003 admission:** `ListWebhookDispatchEndpoints` replaces `ListEnabledWebhookEndpoints`. It returns every retained interval of each endpoint that is enabled, or whose last interval ended within a 24-hour late-recording allowance behind the feed position. Recording-time cursor versus source-time eligibility was found in review. `fanOutPage` admits only events inside an interval. The window end is fixed before the snapshot, so a toggle the snapshot misses lies after every event read, assuming a toggle commits within the feed's 3-second commit lag. `TailFeed`, push and the enqueue paths are unchanged.
- **t004 surfaces:** REST create/PATCH, GraphQL `setWebhookEndpointEnabled`/`updateWebhookEndpoint` and MCP `update_webhook_endpoint` were exercised over real Postgres. Dialects and refusal shapes are unchanged; there is no dashboard change. ADR052 §D3b and ADR018 record this as a bex guarantee: Render's docs do not specify disabled-gap replay, and no authenticated comparison was run.
- **t006 tests:** `TestDisabledWebhookIntervalNeverReplays` replays both a675 cases through the real feed, store, worker and HTTP receiver. It fails with the old CreatedAt rule (A received `[backlog disabled reenabled]`). Also: writer, no-op, refused-write, auto-disable, prune and cascade coverage (`TestPGWebhookEligibilityFollowsEveryWriter`); migration backfill and trigger in an isolated schema; exact boundaries; two workers with different cursors; late-recorded facts; and the adapter test. Full backend suite (fresh real Postgres, `-p 1`), lint and deadcode pass. **Not run:** families other than deploy events through the real feed (projection is unchanged and shared), and multiple live replicas.

## 解除阻塞记录（2026-10-31，`/unblock w4`）

- **涉及：** w4/m212（同组 D2：w4/m237、w4/m219、w4/m220、w4/m172、w4/295）
- 来源：用户明确决定（2026-10-31，`/unblock w4`）——“i approve you to update bex-canary's subscription to paid and operate anything there but please clean up afterwards”；以及 2026-10-30：“qa-* 的数据库都是暂时的可以清除”。
- **结论（D2，Pro+ 门槛）：** 允许 worker 用 `ChangeWorkspacePlan` 把 bex-canary 切到 Pro（或该功能所需最低付费档），在一个窗口内集中重放全部 Pro 门槛项（w4/m237、m219、m220、m212、m172、295），删除自建 webhook 等资源后切回 Hobby，并在各项记录升/降级时间。窗口期内 bex-canary 读作 Pro，其他会话的 Hobby 限制类测试读数可能不同，因此窗口尽量短。不使用 `bex`（Scale，真实计费）workspace。 bex-canary（`tea-daif693dqjvc73e7as3g`）计费状态为 `operator_excluded: structural exclusion`，不走 Stripe 收款（2026-10-29 只读核实 `workspaceBillingReadiness`），付费 fixture 与付费套餐不产生真实费用。
- m212 的 migration 0148 已随 `25b99d8a373a` 部署（实现早于该 pin）；webhook receiver 用自建 Free web service。
- 生产已部署 `25b99d8a373a`（`deploy/gitops/base/bex.yaml`，2026-10-30 pin），本项实现均早于此版本；部署不等于验收，以下仍是待验证工作。
- **worker 下一步：** 与同组其他项排在同一个 Pro 窗口内，按本项原 DoD 重放剩余的 Pro 门槛检查。
- **清理（用户要求，验收的一部分）：** 验收结束删除本次在 bex-canary 创建的全部资源；若为此把 bex-canary 切到付费套餐，所有依赖付费档的项做完后切回 Hobby；在本条记录清理结果（删除后 404 / 列表为空、套餐读回 Hobby）。
- **剩余阻塞：** 无（门槛已核实解除，交给 `/loopx w4`）。
- **验收：** 原 DoD / Acceptance 不变，未删减。
