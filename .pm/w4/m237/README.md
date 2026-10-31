# w4 · m237 — Live request logs arrive in seconds, not on the range-poll clock

**Worker:** worker4 **Goal:** with Live tail on, a request line reaches the Log Explorer within ~5 s of hitting the service in All and HTTP Requests views — the log store becomes a producer of the subscribe stream alongside pod stdout. **Status:** todo（2026-10-31 解除阻塞；已完成任务保持 DONE，详见文末“解除阻塞记录”）

## Tasks (in order)

| id             | title                                                                 | est | depends_on                 |
| -------------- | --------------------------------------------------------------------- | --- | -------------------------- |
| [t001](done/t001.md) | Backend: log-store poll producer for request logs in the subscribe stream — **DONE** | 60m | —                          |
| [t002](done/t002.md) | Dashboard: offer live tail for request views and send their filters — **DONE**  | 30m | w4/m237/t001               |
| [t003](done/t003.md) | Render parity — **DONE**                                                         | 15m | w4/m237/t002               |
| [t004](done/t004.md) | Simplify — **DONE**                                                              | 15m | w4/m237/t003               |
| [t005](done/t005.md) | Test coverage — **DONE**                                                         | 30m | w4/m237/t003               |
| [t006](t006.md) | Closeout                                                              | 20m | w4/m237/t004, w4/m237/t005 |

Six tasks, 170m (2h50m).

## Definition of done

- With Live tail on, a request line appears in the Log Explorer within ~5 s of the request in the **All** view and the **HTTP Requests** view, at a human request rate.
- Live request lines carry the same fields as the history rows for the same request (status pill renders); each underlying request appears exactly once across history, live frames and the range refetch, in timestamp order — including requests sharing one timestamp.
- Subscribing with `type=request` (with or without its structured filters) is accepted and streams only matching request lines; a mixed app+request subscription merges pod and store lines on one stream. Producerless subscriptions are still refused before a slot is taken.
- With no log store wired, a request subscription fails closed before streaming headers; the historical path is unchanged.
- Checks pass: backend `go test ./...` (real dependencies in CI), dashboard `yarn lint`, `yarn typecheck`, `yarn test`.

## Source + Goal linkage

- **Source:** w4/367 (live `/qa-find-bugs w4` pass 305022, 2026-10-19), promoted in-run by loopx w4: the backend producer route is the parity fix and spans backend + dashboard + tests. The live capture: 7 curls, app lines ≤1 s, first request line at ~30 s; the closed reason string is `http metrics have not appeared yet in this sampling window (1/2m)` on the same seconds-old requests' Metrics cards.
- **Goal linkage:** the logs surface of the Render-compatible hosting product; ADR010 (observability) owns the "tail reads pod logs" line this milestone extends with a second, store-backed producer — it does not change the pod tail's semantics.
- **Expected outcome:** Live tail means the same thing in every log view; `render logs --tail` and SSE/NDJSON clients get request lines live too, since the producer sits behind `followLogs`.
- **Why now:** the only live producer gap left beside `type=predeploy`; the dashboard's store-only gating and the server refusal both rest on it.
- **Render parity closing task:** included — REST subscribe dialects all ride the stream this changes. GraphQL/MCP expose no log tail.

## 解除阻塞记录（2026-10-31，`/unblock w4`）

- **涉及：** w4/m237（同组 D2：w4/m219、w4/m220、w4/m212、w4/m172、w4/295）
- 来源：用户明确决定（2026-10-31，`/unblock w4`）——“i approve you to update bex-canary's subscription to paid and operate anything there but please clean up afterwards”；以及 2026-10-30：“qa-* 的数据库都是暂时的可以清除”。
- **结论（D2，Pro+ 门槛）：** 允许 worker 用 `ChangeWorkspacePlan` 把 bex-canary 切到 Pro（或该功能所需最低付费档），在一个窗口内集中重放全部 Pro 门槛项（w4/m237、m219、m220、m212、m172、295），删除自建 webhook 等资源后切回 Hobby，并在各项记录升/降级时间。窗口期内 bex-canary 读作 Pro，其他会话的 Hobby 限制类测试读数可能不同，因此窗口尽量短。不使用 `bex`（Scale，真实计费）workspace。 bex-canary（`tea-daif693dqjvc73e7as3g`）计费状态为 `operator_excluded: structural exclusion`，不走 Stripe 收款（2026-10-29 只读核实 `workspaceBillingReadiness`），付费 fixture 与付费套餐不产生真实费用。
- 生产已部署 `25b99d8a373a`（`deploy/gitops/base/bex.yaml`，2026-10-30 pin），本项实现均早于此版本；部署不等于验收，以下仍是待验证工作。
- **worker 下一步：** 与同组其他项排在同一个 Pro 窗口内，按本项原 DoD 重放剩余的 Pro 门槛检查。
- **清理（用户要求，验收的一部分）：** 验收结束删除本次在 bex-canary 创建的全部资源；若为此把 bex-canary 切到付费套餐，所有依赖付费档的项做完后切回 Hobby；在本条记录清理结果（删除后 404 / 列表为空、套餐读回 Hobby）。
- **剩余阻塞：** 无（门槛已核实解除，交给 `/loopx w4`）。
- **验收：** 原 DoD / Acceptance 不变，未删减。
