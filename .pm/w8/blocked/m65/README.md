# w8 · m65 — Retain PostgreSQL process logs during bursts

**Worker:** worker8 **Goal:** successful database statements retain their distinct diagnostic records before hosting history search. **Status:** blocked — t001/t002 done；原升级门槛已由 w7/m156 消除；待 D3 的自有 QA Postgres fixture，之后验证 300/1005 条 burst，保留 strict source-access 条件

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Resolve the CNPG manager identity and compatible patched release — **DONE 2026-10-13** | 25m | — |
| t002 | Pin the patched manager and define a controlled rollout — **DONE 2026-10-29（由 w7/m156 的生产证据确认）** | 35m | t001 |
| t003 | Verify the process-log source and affected datastore siblings | 25m | t002 |
| t004 | Render parity | 20m | t003 |
| t005 | Simplify | 10m | t004 |
| t006 | Test coverage | 25m | t004 |
| t007 | Closeout | 10m | t005, t006 |

## Definition of done

Fresh owned Free PostgreSQL statements through published/current Bex and the unmodified2.27 pin emit300 and1005 distinct numbered LOG records in short bursts. All numbers are retained and retrievable through honest supported cursor pages, with exact original bytes/IDs and no silent sampling. Both native clients agree with the complete source. Pacing is a comparison, not the fix. Confirm the installed operator/instance managers contain the selected upstream correction; retain normal-rate Unicode, SQL-error propagation, primary/replica/pooler/backup compatibility and relevant REST/GraphQL/MCP/dashboard checks. Old-manager runtime coverage fails before the correction. Required tests/validation pass and every owned fixture is deleted with detail/list/endpoint/baseline/quota reconciliation before closeout.

## Source + Goal linkage

- **Source:** user-requested continuous CLI QA in w8, sweep218; [two-client source-loss finding](finding.md) and [durable evidence](evidence/cli-sweep218/README.md).
- **Goal linkage:** [ADR008 core hosting](../../../../docs/ADR008-vision.md), [ADR009 managed Postgres](../../../../docs/ADR009-postgresql-management.md) and [ADR010 observability](../../../../docs/ADR010-observability.md).
- **Expected outcome:** customers can retrieve distinct database diagnostic records from real bursts through the unchanged hosting interfaces.
- **Why now:** two successful native statements retain109/1005 and102/300, while the paced1005 control is complete. A released upstream correction exists, but the actual pinned/deployed manager identity still needs resolution.
- **Parity included:** the source feeds tenant-facing REST/GraphQL/MCP/dashboard logs even though the correction belongs to infrastructure.
- **Sizing:**150m /2h30 across seven tasks, including compatibility inventory, artifact/rollout work, source/sibling verification and standing closing tasks.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m65/t002–t007。
- **结论来源：** 本次代码与跨 workstream 已完成记录复核。`aa393fe29851` 已锁定 chart 0.29.1 / CNPG 1.30.1，并启用 `ENABLE_INSTANCE_MANAGER_INPLACE_UPDATES: "true"`。w7/done/m156 的 2026-10-26 生产记录证明 13 个 instance manager 均为 1.30.1，原 Pod UID / postmaster start time 保留，连续 1,828 s 监测及备份通过。只读取 w7，未修改它。
- **结论：** 升级批准、镜像 mirror 和全库重启均不再是 m65 门槛；不再进行第二次升级。m65 的 300 / 1005 条 LOG burst 保留验收仍未验证，不能用 WAL 指标验收替代，也不能直接关闭。
- **剩余门槛：** bex-canary 的唯一 Free Postgres 槽仍被 `dpg-db9r654flioc739mcpbg` 占用。D3 待用户决定提供临时 QA workspace / 最小数据库 fixture；不删除或升级既有数据库。当前本机 SSH trust pin 不匹配，read-only kubeconfig 获取失败，未放宽校验；worker 如需直读 source，应从受信任的生产 CI / runbook 恢复 strict access。
- **worker 下一步：** 资源条件明确后恢复 t002–t007：t002 记录已经生效的供应/升级证据，t003 重放两客户端 burst 与 paced 控制，再完成共享面和精确清理。

## 用户决定 D3（2026-10-29，接续本轮 unblock）

- **结论来源：** 用户明确回复「D3 允许，但是最终状态要干净和正确」。
- **授权范围：** 一个本轮临时 QA workspace、一份新的最小 Free Postgres；先用于 m65 的 Free burst 和 m126 的 fresh expiresAt 验收，再做一次 m126 必需的最低付费档短时升级。复用同一 fixture，尽快删除，不触碰原有数据库。允许由此产生的短时数据库费用，不授权添加/替换支付卡、修改其他 workspace 计费或绕过产品权限/付款检查。
- **前置复核：** 已获准创建/付费不是资源已就绪的证明；worker 仍须核实普通产品入口的创建权限、workspace cap、payment policy 和可用额度。能自行核查的事实由 worker 查，遇真实外部门槛再精确记录。未实际提供资源前不伪称线上验收已经开始或完成。
- **干净且正确：** 开始前保存资源/配额和本轮凭据基线；按创建返回的确切 ID 记自有资源账本。结束时数据库和 workspace 经正常路径删除且查询/列表不可见，确认异步关联资源和 quota/terminating 收敛、没有继续运行的付费实例；只撤销本轮 session/grant、删除本轮临时 config/CA/日志凭据并回收进程。原有资源和其余会话保持原状态。保留正确的审计/已发生用量记录，不能为“干净”删除计费事实。无法证实任何一项时明确列出残留和责任人，不能关闭 milestone。
- **worker 下一步：** 先准备最小资源及清理保证，再恢复并完成本项原 DoD；无新增 commit/push 授权。本轮 D2 仍是用户提问，未获公开上游提交许可。
