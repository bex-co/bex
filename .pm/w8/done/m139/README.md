# w8 · m139 — Free web services share 750 instance hours per workspace per month, as on Render

**Worker:** worker8 **Goal:** Each workspace gets Render's 750 Free instance hours per calendar month, counted only while a Free web service is running (spun-down time is free). When they run out, the workspace's Free web services are suspended until the start of the next month, with warning and exhaustion emails and a "Monthly included usage" readout, as on Render. **Status:** done — 2026-10-29：dev-8 全生命周期通过；生产读取端复核一致；全部自有 fixture / 邮件 / session 已清理

## Tasks (in order)

| id   | title                                                                         | est | depends_on                 |
| ---- | ----------------------------------------------------------------------------- | --- | -------------------------- |
| t001 — **DONE** | Month-to-date Free instance hours per workspace from the existing usage meter  | 40m | —                          |
| t002 — **DONE** | Exhaustion: suspend Free web services until the month starts; resume on reset  | 60m | w8/m139/t001               |
| t003 — **DONE** | Notifications: approaching (e.g. 80%) and exhausted emails                     | 30m | w8/m139/t001               |
| t004 — **DONE** | Surfaces: monthly included usage on REST/GraphQL/MCP and the Usage page        | 40m | w8/m139/t001               |
| t005 — **DONE** | Render parity: ADR030/ADR018, suspension reason and resume rules               | 20m | w8/m139/t002, w8/m139/t003, w8/m139/t004 |
| t006 — **DONE** | Simplify the changed code                                                     | 15m | w8/m139/t005               |
| t007 — **DONE** | Test coverage: accrual, sleep exclusion, exhaustion, month rollover, upgrade   | 40m | w8/m139/t005               |
| t008 — **DONE** | Close out the milestone                                                       | 30m | w8/m139/t006, w8/m139/t007 |

## Definition of done

On a dev environment with a shortened budget (configurable, e.g. 0.05 h), and live on production for the read side:

- Two Free web services running in one workspace accrue Free hours at 2× wall time; a slept service accrues none (the activator's hibernation is excluded).
- Crossing the budget suspends every Free web service in the workspace with a suspension reason naming the monthly limit (`suspended` + suspender `billing`-like, not `user`); paid services keep running. The owner gets the exhaustion email; at 80% the warning email.
- At the month boundary the services resume (or become resumable, matching Render: t005 decides from Render's docs/behavior) and the counter resets.
- Upgrading a suspended Free service to a paid plan lets it run immediately.
- Production: `GET` usage (REST/GraphQL `usage`/MCP `get_usage`) shows `freeInstanceHours {used, included: 750}` for `bex-canary`, matching the instance-seconds meter.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` loop sweep p8q1, 2026-10-24 UTC, `bex-canary` (Hobby), platform pin `29d63e1f7bea`. Source-level finding (a 750-hour exhaustion cannot be produced live in a QA sweep).
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)); pricing ([ADR030](../../../../docs/ADR030-pricing.md)). [render.com/docs/free](https://render.com/docs/free): "Render grants 750 Free instance hours to each workspace per calendar month", consumed only while running; when exhausted, "Render suspends all of your Free web services until the start of the next month"; usage appears in the Billing page's "Monthly Included Usage", with emails when approaching and exceeding. ADR030 §6 caps a Free service at one instance but sets no monthly budget, so N Free services run without bound at $0.
- **Expected outcome:** the Free tier's cost ceiling matches Render's, closing an unbounded free-compute path, and tenants see the same monthly allowance.
- **Why now:** major (pricing/abuse parity). The meter already exists (below), so this is accounting plus enforcement. Render parity task included: usage surfaces, dashboard and emails.

## Evidence

- `rg -i "free.?instance.?hours|instance.hours|750"` over `lego/`, `dashboard/src`, `docs/`, `.pm/`: no implementation, ADR or board item. The only nearby rule is ADR030 §6 (one running instance per Free service).
- The meter exists: `lego/backend/internal/store/usage.go:33` (`UsageKindInstanceSeconds`, broken out by tier, `usage.go:102,153`), surfaced as month-to-date usage by `internal/usage/service.go:168,660` and MCP `get_usage` ("instance-seconds by tier", `usage/mcp.go:41`). GraphQL `usage(period, ownerId)` returns `UsageSummary {billing, coverage, estimatedCost, period, services, workspaceId}`, with no free-hours or included-usage field.

## Fix (target)

- t001: Free hours = month-to-date `instance_seconds` for tier `free` / 3600, per workspace (verify the meter excludes hibernated time; if it counts pods at zero replicas, fix that first).
- t002: a replica-safe sweeper (precedent: w8/m126's `FreeExpirySweeper`) suspends Free web services through the normal suspend path with a distinct suspender, and lifts that suspender at the month boundary. A user-suspended service stays user-suspended.
- t004: add `freeInstanceHours {used, included}` (or Render's "Monthly included usage" shape if the pinned OpenAPI has one) to usage on every surface, and a Usage-page meter.

## Blast radius

Every workspace's Free web services. Paid services, static sites (Render's Free static sites do not consume instance hours) and datastores are unaffected. Existing heavy Free users will be suspended in the first month after release, so the release note and the 80% email ship first.

## Unverified

- Whether Render resumes automatically at the month start or requires a manual resume (docs: "suspends … until the start of the next month").
- Whether the bex meter excludes hibernated (scaled-to-zero) time.

## Dedupe

Searched `.pm` open and done, `docs/` (ADR030, ADR040, ADR018) and DO_NOT_DO for free instance hours / 750 / included usage: none. w8/m126 (free Postgres 30-day expiry) is the closest precedent for a free-tier sweeper.

## Partial live evidence (2026-10-24 10:07 UTC, platform pin `ba5ad0e65498`, `/qa-find-bugs-cli` sweep r3s5)

GraphQL `usage(ownerId:"tea-daif693dqjvc73e7as3g") { period freeInstanceHours { used included } }` → `{"period":"2026-10","freeInstanceHours":{"used":516.41,"included":750}}` for `bex-canary`. Note for QA: this shared QA workspace has used ~69% with 7 days left. At the month-to-date rate it ends near 666 h, but the 80% warning (600 h) will fire, and heavy fixture churn could exhaust it and suspend every Free web service there until November, other runs' fixtures included. Not covered: exhaustion/suspension, emails, REST/MCP readouts, sleep exclusion.


## 解除阻塞记录（2026-10-29，`/unblock w8`）

- **涉及 ID：** w8/m139
- **结论来源：** 本次复核证据 + 现有规则（loopx：线上验收由 worker 完成）。
- **复核结果：** 生产 pin `b33dd43c4428` 已包含 migration 0161 和 sweeper。`newFreeHours`（`lego/backend/internal/api/free_hours.go`）无条件接入，750 小时上限**已经在生产执行**，不存在「上线前公告」的时间点。现有 workspace 全是自有的（`.pm/DO_NOT_DO.md` #PSL：17 个），影响有限。
- **结论：** 线上验收不需要等公告。是否给现有 Free 用户补发公告，单独作为可选事项问用户，不阻塞本条目。
- **用户门槛：** 无（公告为可选，见本次 `/unblock` 汇报）。
- **worker 下一步：** loopx 执行 t008：在生产验证读取端（REST/GraphQL/MCP `freeInstanceHours`、Usage 页卡片）；在 dev 环境用 `BEX_FREE_INSTANCE_HOURS=0.05` 验证用尽流程。fixture 按 ID 删除并留 404 证据。

- **补充（2026-10-29，用户明确决定 D5）：** 不补发公告。现有 workspace 都是自有的，80% 预警和用尽邮件已经够用；开放注册前再统一写对外说明。

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m139/t008。
- **结论来源：** 本次复核原 DoD 和既有用户决定 D5。原 DoD 明确「dev 缩短预算验证 enforcement，生产验证 read side」，t008 亦如此；不要求等待生产 750 h 或 11 月换月。
- **处理：** 恢复 t008 待验证；保留已有生产读数/80% claim 证据。worker 在隔离 dev-8 用 `BEX_FREE_INSTANCE_HOURS=0.05`，接入测试 meter / Prometheus 并用测试时钟完成 warning、exhaustion、免费与付费区别、人工 resume 拒绝、升级恢复、月界重置与一次性通知。修复本地 harness / 使用隔离复现环境属于预批准开发环境工作，不是用户或日历门槛。不得改生产时钟或降低生产预算。
- **验收：** 原 DoD 全部保留；SMTP claim 不等于邮件送达，应以 dev Mailpit 收件证明通知。生产自然跨界可补充观察，不能替代未做的 dev 验收，也不必以它阻塞现有验收路径。不补发公告的 D5 不变。
- **剩余门槛：** 无用户门槛；实现/运行隔离验收后才可 done。

## Completed — 2026-10-29 Pacific / 2026-10-30 UTC

All original DoD bullets are observed. The real dev-8 drill passed in 422.31 seconds with actual kubelet/cAdvisor/Prometheus metering, real Postgres/Kubernetes lifecycle writes, both Mailpit deliveries, upgrade and test-clock month rollover. Fresh production REST/GraphQL/MCP readouts agree, and the prior hosted Usage-card/meter evidence is retained. The fractional-budget threshold defect found during acceptance is fixed with a failing-before/passing-after boundary regression. [Full evidence and cleanup](evidence/2026-10-29-lifecycle/README.md).

## Final simplify review — 2026-10-29

Three independent reuse, quality and efficiency reviewers found no further changes needed in the seconds-based threshold fix or its behavioral boundary/deduplication regression.

## Live exhaustion observed (2026-10-30 09:2x–09:3x UTC, pin `dc80c29bf2bc`; `/qa-find-bugs-cli` loop sweep 3093)

`bex-canary` crossed its October budget naturally: `GET /v1/usage` → `freeInstanceHours: {used: 750.58, included: 750}`.

- All 13 of the workspace's Free web services read `suspended` with `suspenders: ["billing"]`. The one static site stays `not_suspended` and served 200.
- New Free web services created after the crossing (`qa-20261030-3093-py`, `-nd`) build and deploy (`live`, Python 3.14.8 / Node v24.22.1), but are suspended at once and answer `503 retry-after: 3600 x-render-routing: suspend` "This service is suspended."
- `POST /services/{id}/resume` → `400 FREE_INSTANCE_HOURS_EXHAUSTED`, as specified. Its text names the internal App name instead of the service; filed as [w8/225](../../225.md).
- Fixtures were deleted by ID with 404 proofs. Free-web-service QA in `bex-canary` is unavailable until the 2026-11-01 reset.
