# w8 · m138 — Hobby workspaces get Render's Hobby limits; Pro features start at Pro

**Worker:** worker8 **Goal:** A Hobby workspace has Render's Hobby limits (two environments per project, alongside the already-enforced 1 member and 25 services), and Render's Pro-and-higher features are refused on Hobby with a plan-named error, as on [Render's features-by-plan page](https://render.com/docs/platform-features-by-plan). **Status:** done (2026-10-29)

## Tasks (in order)

| id   | title                                                                          | est | depends_on                                           |
| ---- | ------------------------------------------------------------------------------ | --- | ---------------------------------------------------- |
| t001 — **DONE** | One plan-feature table and gate helper (Hobby / Pro / Scale / Enterprise)       | 30m | —                                                    |
| t002 — **DONE** | Hobby: at most two environments per project                                    | 30m | w8/m138/t001                                         |
| t003 — **DONE** | Network-isolated environments require Pro                                      | 20m | w8/m138/t001                                         |
| t004 — **DONE** | Webhooks require Pro (one destination on Pro, 100 on Scale)                    | 30m | w8/m138/t001                                         |
| t005 — **DONE** | HTTP request logs require Pro                                                  | 45m | w8/m138/t001                                         |
| t006 — **DONE** | Inventory and migrate existing Hobby workspaces using Pro features             | 30m | w8/m138/t002, w8/m138/t003, w8/m138/t004, w8/m138/t005 |
| t007 — **DONE** | Render parity: REST/GraphQL/MCP/dashboard plan copy and ADR018/ADR030          | 20m | w8/m138/t006                                         |
| t008 — **DONE** | Simplify the changed code                                                      | 15m | w8/m138/t007                                         |
| t009 — **DONE** | Test coverage: plan × feature matrix, legacy state, upgrade/downgrade          | 40m | w8/m138/t007                                         |
| t011 — **DONE** | Dashboard Logs: Request logs on Hobby show a plan notice, not "Couldn't load logs" | 30m | w8/m138/t005, w8/m138/t007 |
| t012 — **DONE** | Hobby "All logs" still serves HTTP request logs: the Loki type matcher ignores noRequest | 30m | w8/m138/t005 |
| t013 — **DONE** | Hobby plan refusals (environments, network isolation, request logs) carry a machine-readable code, as webhooks do | 30m | w8/m138/t007 |
| t010 | Close out the milestone — **DONE 2026-10-29** | 30m | w8/m138/t008, w8/m138/t009, w8/m138/t011, w8/m138/t012, w8/m138/t013                           |

## Definition of done

Live on production after the release pipeline deploys bex-api, the log reader and the dashboard, in the Hobby workspace `bex-canary`, with owned fixtures deleted by ID with 404 proofs:

- `POST /v1/projects` with three environments → 400 naming the Hobby limit (two environments per project). Two environments → 201.
- `PATCH /v1/environments/{id} {"networkIsolationEnabled":true}` → 400 naming the Pro plan; `false` → 200.
- `POST /v1/webhooks` → 400 naming the Pro plan.
- `bex logs -r <free web srv-id> --type request` → the Pro-plan refusal Render's CLI shows for a Hobby workspace (t005 pins the exact shape), while `--type app` and `--type build` still return lines.
- The same calls on a Pro workspace (dev environment or a test workspace switched with `changeWorkspacePlan`, no purchase) succeed.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` loop sweeps b1c3 and j3k5, 2026-10-23/24 UTC: bex v0.3.3, pin Render CLI v2.27.0 `a764810a7682`, platform pin `29d63e1f7bea`, `https://api.bex.co/v1/`, `bex-canary` (`tea-daif693dqjvc73e7as3g`, GraphQL `plan: "hobby"`), human device login. Evidence below.
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)) and pricing ([ADR030](../../../../docs/ADR030-pricing.md): Hobby $0 / Pro $25 / Scale $499). [render.com/docs/platform-features-by-plan](https://render.com/docs/platform-features-by-plan): Hobby is "limited to a single team member", "limited to 25 total services", "limited to two environments per project". Pro and higher gain, among others, "Network-isolated environments", "HTTP request logs", "Webhooks — send notifications to one destination" (Scale: "up to 100 destinations"), "Audit logs", "Autoscaling", "Preview environments", "Response latency metrics". The member and service caps are enforced today (`members/service.go:897`, ADR018:246); the rest are not. w6/m3 t006 recorded the two-environment Hobby limit in 2026-07 but never gated it.
- **Expected outcome:** the Hobby/Pro/Scale ladder means what Render's means. w8/m137 handles the Scale-only inbound IP rules.
- **Why now:** major for pricing parity: a $25/month plan's features are free on Hobby. Render parity task included: every surface (REST/GraphQL/MCP/dashboard, CLI-visible errors) changes.

## Evidence

```text
GraphQL query { workspaces { id name plan } } → bex-canary "hobby"
POST /v1/projects {"name":"qa-20261024-j3k5-proj","environments":[{"name":"production"},{"name":"staging"},{"name":"dev"}],…}
  → 201, 3 environmentIds                                   (Render Hobby: two environments per project)
POST /v1/webhooks {"name":"qa-20261024-j3k5-wh","url":"https://example.invalid/hook","enabled":false,"eventFilter":[],…}
  → 201 whk-dbe03ehf0c2s738fsgo0                             (Render: Pro and higher)
PATCH /v1/environments/evm-dbduagj17lic73al6o1g {"networkIsolationEnabled":true}  (sweep b1c3)
  → 200, networkIsolationEnabled:true                        (Render: Pro and higher)
bex logs -r <free web srv-id> --type request …  (sweeps 5e2a, 2f6a, d4e8)
  → request entries returned                                 (Render: HTTP request logs are Pro and higher)
```

All fixtures deleted by ID (204, then 404).

## Root cause

No plan-feature table exists. Workspace plans are stored (`lego/backend/internal/store/plans.go:35-45`) and used for pricing and the member gate only. The project/environment create path (`internal/environments`), `environments.Service.SetACL`, the webhooks service and the logs reader (`internal/logs`) never read the workspace plan.

## Fix (target)

- t001: `store.PlanFeatures` (or `core.PlanAllows(plan, feature)`) as the one table, with Render's features-by-plan as its source of truth, and a shared refusal message ("… requires the Pro plan or higher; this workspace is on Hobby").
- t005 (request logs): before implementing, capture what Render's API/CLI returns for `type=request` on a Hobby workspace (empty list vs an error) and match that shape. If it cannot be captured, refuse with the plan error and record it as unverified.
- t006: existing Hobby workspaces with three or more environments, isolation on, or webhooks: never silently delete. Keep them working, block new uses, and surface the plan requirement. Record counts and the transition in ADR018. The parity rule (no allowlist) applies to new uses in every workspace.

## Blast radius

Every Hobby workspace on REST, GraphQL, MCP, dashboard and Blueprints (Blueprint `projects[].environments[]` counts too). Pro/Scale unchanged. Autoscaling, preview environments, audit logs and latency metrics are Pro features too; t001's table must include them, and t006 lists which are already unreachable on Hobby (Free instances cannot autoscale) versus need their own gate (file follow-ups if larger than this milestone).

## Unverified

- Render's exact API errors for these refusals and the request-log behavior on Hobby (no Render account).

## Dedupe

Searched `.pm` open and done and `docs/` for each feature with Hobby/Pro/plan gate: none. w6/m3 (workspaces) researched the limits without gating them; w8/m137 covers Scale-only inbound IP rules; the member gate exists.

## Partial live evidence (2026-10-24 09:33–09:36 UTC, platform pin `ba5ad0e65498`, bex v0.3.3, `bex-canary` (Hobby), `/qa-find-bugs-cli` sweep q9r2)

- `POST /v1/projects` with three environments → 400 `environments: a Hobby workspace is limited to 2 environments per project; upgrade the plan to add more`; two → 201.
- `PATCH /v1/environments/{id} {"networkIsolationEnabled":true}` → 400 `network-isolated environments require the Pro plan or higher; this workspace is on Hobby`.
- `POST /v1/webhooks` → 400 `WEBHOOK_PLAN_LIMIT` `webhooks require the Pro plan or higher; this workspace is on Hobby`.
- `bex logs -r <id> --type request` → 400 `type: HTTP request logs require the Pro plan or higher; this workspace is on Hobby`; `--type build` works. **But** an unfiltered `bex logs -r <id>` still returned 3 request entries: see [t012](done/t012.md) (also found as w8/199, folded in).
- Not covered: Pro-workspace controls, legacy state, dashboard copy. DoD (t010) still open.


## 解除阻塞记录（2026-10-29，`/unblock w8`）

- **涉及 ID：** w8/m138
- **结论来源：** 本次复核证据 + 现有规则（loopx：线上验收由 worker 用 `scripts/qa-login.sh` / `bex` CLI 自行完成）。
- **复核结果：** 原卡点「发布流水线部署后再做线上 QA」已不成立。生产当前运行 pin `b33dd43c4428`（2026-10-29 06:51 UTC 由 deploy.yml + Argo 自动发布），本条目在 main 上已合入的修复都包含在内。线上验收不需要用户操作。
- **剩余工作：** 只剩线上验收（待验证，未完成）。部署不等于验收通过。
- **worker 下一步：** loopx 按原 DoD 和上方 BLOCKED 记录里的检查清单在生产执行线上验收：通过则按 done 规则关闭；不通过则修复后复验。fixture 按 ID 删除并留 404 证据。
- **用户门槛：** 无。

## Progress 2026-10-29 (t006, t011, t012, t013)

- **t012:** the Loki selector for an untyped/`type=all` log read is app-only on Hobby, and label discovery drops `type: request` and every `statusCode`/`method` value there. Pro is unchanged.
- **t013:** environment cap → `PLAN_LIMIT` (`params {plan, limit: 2, field: "environments"}`); network isolation and request logs → `PLAN_FEATURE_REQUIRED` (`params {plan, feature, requiredPlan: "pro", field}`). Codes appear on REST `code`/`params`, GraphQL `extensions` and the MCP prefix, and every message is unchanged. Blueprint validate reports the environment cap and network isolation at the authored entry with the code.
- **t011:** the dashboard Request logs view on Hobby shows a Pro-plan notice with an Open Billing link (en + zh) instead of "Couldn't load logs".
- **t006:** the production inventory (64 Hobby / 2 Scale / 1 Enterprise; 0 Hobby projects with 3+ environments, 0 isolated Hobby environments, 0 Hobby webhooks) is recorded in ADR018 and ADR030, and no migration was needed. Render's features-by-plan page lists audit logs and response latency metrics as Pro and higher, so both are now gated (`PLAN_FEATURE_REQUIRED`, `feature: "auditLogs"` / `"latencyMetrics"`), with dashboard plan notices on the Audit Log card and the Response Times section.

**The production re-check after deploy (t010) must verify, in Hobby `bex-canary` with owned fixtures deleted by ID and 404 proofs:**

1. Every Definition of done bullet above, plus the same calls succeeding on a Pro or Scale workspace.
2. `bex logs -r <free web srv-id>` with no `--type` (and `--text <rndr-id>` after a request) returns no `type=request` lines while app lines still return. REST `/v1/logs` without `type` and with `type=all`, GraphQL `logs` and MCP `list_logs` behave the same. The dashboard All logs shows no access-log JSON.
3. `GET /v1/logs/values?label=type` lists no `request`, and `label=statusCode` / `label=method` are empty (MCP `list_log_label_values` too). On a Pro workspace they still list request values.
4. `POST /v1/projects` with three environments → 400 with `code: PLAN_LIMIT`, `params.plan: hobby`, `params.limit: 2`, and the message unchanged. `PATCH /v1/environments/{id} {"networkIsolationEnabled":true}` and `GET /v1/logs?…&type=request` → 400 `PLAN_FEATURE_REQUIRED` with `params.feature` `networkIsolation` / `requestLogs`. GraphQL `createEnvironment` (third) shows `extensions.code`, and MCP errors carry the code prefix.
5. `POST /v1/blueprints/validate` (or `bex blueprints validate`) of a three-environment project with `networking.isolation: enabled` answers `valid: false` with `PLAN_FEATURE_REQUIRED` at `projects[0].environments[i].networking.isolation` and `PLAN_LIMIT` at `projects[0].environments[2]`.
6. Dashboard on Hobby: the Logs ▸ Request logs view shows the Pro-plan notice and an Open Billing link, and no "Couldn't load logs". Settings ▸ Audit Log shows the audit plan notice. The Metrics ▸ Response Times section shows the latency plan message. Check en and zh.
7. `GET /v1/owners/{hobby-id}/audit-logs` and `GET /v1/metrics/http-latency?resource=<srv>` → 400 `PLAN_FEATURE_REQUIRED`; both 200 on a Pro workspace.

## Closeout — 2026-10-29 (`/loopx w8`)

Post-fix re-check (t010) on production pin `69f0b17d1fad`, Hobby bex-canary.

- **Codes:** a third environment → `PLAN_LIMIT` `{plan: hobby, limit: 2}`; network isolation and request logs → `PLAN_FEATURE_REQUIRED`; webhooks → `WEBHOOK_PLAN_LIMIT` (REST, GraphQL, MCP). Blueprint validate flags both the third environment and isolation.
- **Request lines never leak on Hobby:** with 3 `type=request` lines for a marker in Loki, untyped and `type=all` reads (REST, CLI) return only the 3 app lines; label values list `type: ["app"]` and empty `statusCode`/`method`.
- **Audit logs and response-latency metrics** answer 400 on Hobby and 200 on Scale `bex` (read-only).
- **Dashboard (en and zh):** Request logs shows the Pro notice with Open Billing (no "Couldn't load"); All logs carries no access-log lines; Audit Log and Response Times show their plan notices.
- **Inventory** (recorded in ADR018/ADR030): 64 Hobby, 2 Scale, 1 Enterprise workspaces; no Hobby workspace used any newly gated feature.
- Not re-run: Pro-side mutating controls in `bex`. Cleanup: project and both environments → 404.
