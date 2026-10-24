# w8 · m138 — Hobby workspaces get Render's Hobby limits; Pro features start at Pro

**Worker:** worker8 **Goal:** A Hobby workspace has Render's Hobby limits (two environments per project, alongside the already-enforced 1 member and 25 services), and Render's Pro-and-higher features are refused on Hobby with a plan-named error, as on [Render's features-by-plan page](https://render.com/docs/platform-features-by-plan). **Status:** blocked — t001, t002, t003, t004, t005, t007, t008, t009 done 2026-10-23; release pipeline deploys bex-api, the log reader and the dashboard; QA runs t006+t010 in bex-canary (Hobby): POST /v1/projects with three environments → 400 naming the Hobby limit (two → 201), PATCH environment networkIsolationEnabled true → 400 naming Pro (false → 200), POST /v1/webhooks → 400 WEBHOOK_PLAN_LIMIT naming Pro, bex logs --type request → plan error while --type app/build return lines, same calls succeed on a Pro workspace; read-only inventory of production Hobby workspaces with ≥3 environments, isolated environments or webhooks (counts into ADR018/ADR030); file follow-ups for audit logs, autoscaling, preview environments, latency metrics if not already unreachable on Hobby; delete fixtures by ID with 404 proofs

## Tasks (in order)

| id   | title                                                                          | est | depends_on                                           |
| ---- | ------------------------------------------------------------------------------ | --- | ---------------------------------------------------- |
| t001 — **DONE** | One plan-feature table and gate helper (Hobby / Pro / Scale / Enterprise)       | 30m | —                                                    |
| t002 — **DONE** | Hobby: at most two environments per project                                    | 30m | w8/m138/t001                                         |
| t003 — **DONE** | Network-isolated environments require Pro                                      | 20m | w8/m138/t001                                         |
| t004 — **DONE** | Webhooks require Pro (one destination on Pro, 100 on Scale)                    | 30m | w8/m138/t001                                         |
| t005 — **DONE** | HTTP request logs require Pro                                                  | 45m | w8/m138/t001                                         |
| t006 | Inventory and migrate existing Hobby workspaces using Pro features             | 30m | w8/m138/t002, w8/m138/t003, w8/m138/t004, w8/m138/t005 |
| t007 — **DONE** | Render parity: REST/GraphQL/MCP/dashboard plan copy and ADR018/ADR030          | 20m | w8/m138/t006                                         |
| t008 — **DONE** | Simplify the changed code                                                      | 15m | w8/m138/t007                                         |
| t009 — **DONE** | Test coverage: plan × feature matrix, legacy state, upgrade/downgrade          | 40m | w8/m138/t007                                         |
| t010 | Close out the milestone                                                        | 30m | w8/m138/t008, w8/m138/t009                           |

## Definition of done

Live on production after the release pipeline deploys bex-api, the log reader and the dashboard, in the Hobby workspace `bex-canary`, with owned fixtures deleted by ID with 404 proofs:

- `POST /v1/projects` with three environments → 400 naming the Hobby limit (two environments per project). Two environments → 201.
- `PATCH /v1/environments/{id} {"networkIsolationEnabled":true}` → 400 naming the Pro plan; `false` → 200.
- `POST /v1/webhooks` → 400 naming the Pro plan.
- `bex logs -r <free web srv-id> --type request` → the Pro-plan refusal Render's CLI shows for a Hobby workspace (t005 pins the exact shape), while `--type app` and `--type build` still return lines.
- The same calls on a Pro workspace (dev environment or a test workspace switched with `changeWorkspacePlan`, no purchase) succeed.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` loop sweeps b1c3 and j3k5, 2026-10-23/24 UTC: bex v0.3.3, pin Render CLI v2.27.0 `a764810a7682`, platform pin `29d63e1f7bea`, `https://api.bex.co/v1/`, `bex-canary` (`tea-daif693dqjvc73e7as3g`, GraphQL `plan: "hobby"`), human device login. Evidence below.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)) and pricing ([ADR030](../../../docs/ADR030-pricing.md): Hobby $0 / Pro $25 / Scale $499). [render.com/docs/platform-features-by-plan](https://render.com/docs/platform-features-by-plan): Hobby is "limited to a single team member", "limited to 25 total services", "limited to two environments per project". Pro and higher gain, among others, "Network-isolated environments", "HTTP request logs", "Webhooks — send notifications to one destination" (Scale: "up to 100 destinations"), "Audit logs", "Autoscaling", "Preview environments", "Response latency metrics". The member and service caps are enforced today (`members/service.go:897`, ADR018:246); the rest are not. w6/m3 t006 recorded the two-environment Hobby limit in 2026-07 but never gated it.
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
