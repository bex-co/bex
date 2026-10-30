# w8 · m126 — Free Postgres expires after 30 days and reports `expiresAt`, as on Render

**Worker:** worker8 **Goal:** A free managed Postgres reports Render's `expiresAt` on every surface, warns its owner before it expires, stops serving at expiry, and is deleted after Render's grace period unless upgraded. The ADR009 30-day TTL becomes real. **Status:** blocked — t001–t008 done；日历不是门槛，待 D3 的 fresh QA Postgres 槽及短时 paid-upgrade fixture，随后完成原 DoD

## Tasks (in order)

| id   | title                                                                         | est | depends_on                 |
| ---- | ----------------------------------------------------------------------------- | --- | -------------------------- |
| t001 — **DONE** | Contract: pin Render's free-Postgres expiry, grace and upgrade semantics       | 30m | —                          |
| t002 — **DONE** | Store + API: `expiresAt` on free databases (REST, GraphQL, MCP)              | 60m | w8/m126/t001               |
| t003 — **DONE** | Control plane: expiry sweeper (warn, suspend at expiry, delete after grace)   | 90m | w8/m126/t002               |
| t004 — **DONE** | Upgrade clears the expiry; existing free databases get a dated rollout window | 45m | w8/m126/t003               |
| t005 — **DONE** | Dashboard: expiry date, warning banner and upgrade action on the database page | 45m | w8/m126/t002               |
| t006 — **DONE** | Render parity: expiry fields and lifecycle across REST, GraphQL, MCP, UI       | 20m | w8/m126/t004, w8/m126/t005 |
| t007 — **DONE** | Simplify the changed code                                                      | 15m | w8/m126/t006               |
| t008 — **DONE** | Test coverage: dates, sweeper transitions, upgrade, rollout window           | 45m | w8/m126/t006               |
| t009 | Close out the milestone                                                        | 30m | w8/m126/t007, w8/m126/t008 |

## Definition of done

Live on production after the release pipeline deploys bex-api and the dashboard:

- `bex pg create --plan free` → `GET /v1/postgres/{id}` returns `expiresAt` = creation + 30 days. GraphQL, MCP `get_postgres`/`list_postgres` and the dashboard database page show the same instant. A paid database has no `expiresAt`.
- Upgrading a free database to a paid plan removes `expiresAt` on every surface.
- On a test clock (or a database backdated in a dev environment), the sweeper sends the warning, makes the database unavailable at `expiresAt`, and deletes it after the grace period with audit/events. Each step happens exactly once across bex-api replicas.
- Free databases that existed before this milestone get an announced expiry no earlier than the rollout date + 30 days, never an immediate deletion.

## Source + Goal linkage

- **Source:** inbox note `w8/145` (`/qa-find-bugs-cli` sweep 1247, 2026-10-20), [kept in done/145.md](../done/145.md). User decision 2026-10-20: implement Render's expiry (option A), not record a divergence.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) Render alternative; [ADR009](../../../docs/ADR009-postgresql-management.md) plan table ("30-day TTL … else 'free forever' eats the cluster"); [ADR018](../../../docs/ADR018-render-parity.md) Postgres rows. Render's API carries `expiresAt` on Postgres and PostgresDetail (pinned CLI `pkg/client/types_gen.go:2624–2625`, `2674–2675`).
- **Expected outcome:** free databases stop accumulating forever on the cluster, users see the same lifecycle as on Render, and the ADR stops promising something the code does not do.
- **Why now:** the user decided it on 2026-10-20. Every free database created in the meantime adds to the rollout window t004 has to announce.
- **Render parity included:** the change adds a field and a lifecycle to REST, GraphQL, MCP and the dashboard.

## Notes for t001

- Read render.com/docs/free (Postgres section) and the pinned spec's `expiresAt` description, and record the exact rules: the 30-day clock, the grace period before deletion, what "expired" serves, whether a suspended or upgraded database resets the clock, and what notifications are sent. Write them into ADR009 before t002.
- Deletion must go through the existing Postgres delete path (backups and audit as for a user delete), never a raw CR delete.

## Progress (2026-10-20)

- t001: Render's rules pinned in [ADR009 §Free-plan expiry](../../../docs/ADR009-postgresql-management.md#free-plan-expiry-w8m126) from render.com/docs/free and the pinned spec.
- t002: `expiresAt` derives from the Database CR (creation, or `app.bex.co/free-since` after a change to free, clamped to the 2026-10-27 rollout, + 30 days). No store migration: the CR is the only state. REST, GraphQL `database.expiresAt` and MCP agree (`TestExpiresAtParityAcrossSurfaces`).
- t003: `postgres.FreeExpirySweeper` (5-minute `core.Poll`, started in `cmd/api/main.go`) claims each stage with an optimistic-concurrency `Update` (`app.bex.co/free-expiry: <stage>@<expiresAt>`), suspends at expiry, deletes after 14 days through `deleteDatabase` (usage retirement first, delete preconditioned on UID + resourceVersion), writes `postgres.FreeExpiry*` audit rows and mails owners through `notifications.DatabaseExpiryNotifier`.
- t004: every plan writer (`setPlan`, PATCH via `patchDatabaseObj`, Blueprint `applyDatabase`) runs `SyncFreeClock`: paid clears the clock and resumes an expiry suspension; paid → free starts a new clock. Resume of an expired database is refused (`POSTGRES_FREE_EXPIRED`). The rollout clamp keeps every pre-existing database until at least 2026-12-10.
- t005: Details card Expires row with a 7-day/expired warning linking to `#plan`, the expiry reason beside the status, and a list hint in the resource table; Vitest covers free/paid/soon/expired. The Details skeleton keeps modelling only the always-present rows plus Region, as it already does for the optional external-host row, so a free database's ready card is one row taller than its skeleton. The side-by-side visual check was not run: this machine has no cluster tooling.
- t006: ADR018 Postgres CRUD row updated.
- t007: `/simplify` applied (shared owner-mailing helper with billing's no-address and permanent-rejection handling, named reason-code constants, one tone helper in the dashboard).
- t008: `expiry_test.go` (dates, stage boundaries, every stage once, collapsed overdue stages, two-replica claims, upgrade and user-suspension, downgrade, rollout window), `expiry_cross_surface_test.go`, `notifications/database_expiry_test.go`, dashboard `expiry.test.ts` and `database-expiry.test.tsx`. Each fails on the pre-milestone code (the field, sweeper and notifier did not exist).



## 解除阻塞记录（2026-10-29，`/unblock w8`）

- **涉及 ID：** w8/m126
- **结论来源：** 本次复核证据 + 现有规则（loopx：线上验收由 worker 用 `scripts/qa-login.sh` / `bex` CLI 自行完成）。
- **复核结果：** 原卡点「发布流水线部署后再做线上 QA」已不成立。生产当前运行 pin `b33dd43c4428`（2026-10-29 06:51 UTC 由 deploy.yml + Argo 自动发布），本条目在 main 上已合入的修复都包含在内。线上验收不需要用户操作。
- **剩余工作：** 只剩线上验收（待验证，未完成）。部署不等于验收通过。
- **worker 下一步：** loopx 按原 DoD 和上方 BLOCKED 记录里的检查清单在生产执行线上验收：通过则按 done 规则关闭；不通过则修复后复验。fixture 按 ID 删除并留 404 证据。
- **用户门槛：** 无。

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m126/t009。
- **结论来源：** 本次复核原 DoD、t009 与生产资源只读列表。生命周期验收原文允许测试时钟 / dev 回填日期；不要求等待 2026-11-19 至 12-10 的生产日历。已有生产读取端证据保留。
- **剩余门槛：** bex-canary 的唯一 Free Postgres 槽被 `dpg-db9r654flioc739mcpbg` 占用，未删除或升级它。D3 待用户决定临时 QA workspace 及一次最小短时付费 PostgreSQL fixture，用于 fresh-create 和 free→paid 的实际验收。现有 D4 仅批准 cron 验收付费 fixture，不扩大为数据库付款授权。
- **worker 下一步：** D3 明确且资源实际就绪后恢复 t009；在 dev-8 用测试时钟验证 warn/suspend/final-warning/delete/claim 幂等，生产验证 fresh creation+30d 和 upgrade 清除 expiresAt。记录邮件实际收件、删除和审计，只清理自有 fixture 并留 404。所有原 DoD 仍有效；自然日历观察是可补充证据。

## 用户决定 D3（2026-10-29，接续本轮 unblock）

- **结论来源：** 用户明确回复「D3 允许，但是最终状态要干净和正确」。
- **授权范围：** 一个本轮临时 QA workspace、一份新的最小 Free Postgres；先用于 m65 的 Free burst 和 m126 的 fresh expiresAt 验收，再做一次 m126 必需的最低付费档短时升级。复用同一 fixture，尽快删除，不触碰原有数据库。允许由此产生的短时数据库费用，不授权添加/替换支付卡、修改其他 workspace 计费或绕过产品权限/付款检查。
- **前置复核：** 已获准创建/付费不是资源已就绪的证明；worker 仍须核实普通产品入口的创建权限、workspace cap、payment policy 和可用额度。能自行核查的事实由 worker 查，遇真实外部门槛再精确记录。未实际提供资源前不伪称线上验收已经开始或完成。
- **干净且正确：** 开始前保存资源/配额和本轮凭据基线；按创建返回的确切 ID 记自有资源账本。结束时数据库和 workspace 经正常路径删除且查询/列表不可见，确认异步关联资源和 quota/terminating 收敛、没有继续运行的付费实例；只撤销本轮 session/grant、删除本轮临时 config/CA/日志凭据并回收进程。原有资源和其余会话保持原状态。保留正确的审计/已发生用量记录，不能为“干净”删除计费事实。无法证实任何一项时明确列出残留和责任人，不能关闭 milestone。
- **worker 下一步：** 先准备最小资源及清理保证，再恢复并完成本项原 DoD；无新增 commit/push 授权。本轮 D2 仍是用户提问，未获公开上游提交许可。

## D3 资源预检后的精确剩余条件（2026-10-29 当地时间）

- **已获授权：** 最小临时 QA 资源、一次短时最低付费 PostgreSQL 升级和完整清理，不再重复请求这部分许可。
- **事实：** 新 workspace 的生产 creation policy 为 `all` / `paymentRequired=true`，绑定付款方式是其正常产品门槛。本轮未创建 SetupIntent、workspace 或数据库，未修改任何 payment marker / exemption / 支付方式。
- **可行更小路径：** `tian-personal`（`tea-da2isimlm39c739m4ofg`）已有付款资格，Postgres used/freeUsed/terminating=0、freeLimit=1，调用者 admin；现有 2 个服务保持原状。已向用户询问是否允许将测试落点改到这里，仅创建和删除本轮 DB fixture；或者坚持新 QA workspace 并由用户在 dashboard 完成付款设置。
- **状态：** 本项继续 blocked 等这一次范围/实际环境条件确认。原 DoD 和 D3 清理要求全部保留；不是因缺少重复的付费许可而阻塞。
