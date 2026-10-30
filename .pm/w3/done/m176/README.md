# w3 · m176 — New workspaces get actionable empty states and a getting-started checklist (ADR075 D1+D2)

**Worker:** worker3 **Goal:** a new account that clears the email and card gates lands on an Overview that tells it what to do next and lets it do it — every new-user-path empty state has a working primary action, and a dismissible checklist tracks verify → card → first service → GitHub → teammate **Status:** done — 2026-10-30

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Give `EmptyState` an action slot and move the three new-user empties onto it — **DONE** | 30m | — |
| t002 | Primary actions on the Overview, Blueprints and Env groups empty states — **DONE** | 45m | t001 |
| t003 | Getting-started checklist on Overview for zero-resource workspaces — **DONE** | 1h30m | t001 |
| t004 | Pending state that matches the ready layout at desktop and narrow mobile — **DONE** | 40m | t003 |
| t005 | Live first-run walk on dev-3 with a fresh account — **DONE** | 45m | t002, t004 |
| t006 | Render parity — **DONE** | 20m | t005 |
| t007 | Simplify — **DONE** | 15m | t006 |
| t008 | Test coverage — **DONE** | 40m | t006 |
| t009 | Closeout — **DONE** | 10m | t007, t008 |

## Definition of done

- On dev-3, a freshly registered account's Overview shows the checklist; each item's state is correct before and after its action (create a service, connect GitHub, invite a teammate — verify-email/add-card/sample dropped, see ADR075 §D1 + D2 implementation), and the card disappears once dismissed — persisted across reload, per workspace, with no backend state.
- The Overview, Blueprints and Env groups empty states each show a primary action that opens the right create flow (en and zh).
- Pending and ready screenshots match at 1440 and 390 widths (AGENTS.md skeleton rule).
- Component tests cover every checklist item state, dismissal persistence and the empty-state actions; `yarn typecheck && yarn lint && yarn test` pass.

## Evidence (2026-10-19)

- [ADR075](../../../../docs/ADR075-user-onboarding.md):45-46 decides D1 (action slot + CTAs on Overview, Blueprints, Env groups) and D2 (dismissible checklist, client-side dismissal); `:78` says D1–D3 "remain pending `/pm` materialization"; `:69` non-goals: no tour framework, no per-user onboarding state in the control-plane DB.
- `EmptyState` takes only `title`, `description`, `iconName` (`dashboard/src/common/components/empty-state.tsx:24-28`) and is used only by deploy and log views; the three target empties are inline markup with no action: `dashboard/src/routes/index.tsx:256-260`, `routes/blueprints.tsx:114-118`, `routes/env-groups.tsx:160-165`.
- No checklist exists in `dashboard/src`. D3's sessionStorage `next` relay already ships (`features/auth/lib/auth-next.ts`); D4 is `w5/161`; D7/D8 shipped in `w6/m42`.
- "Deploy a sample" has no dashboard path: ADR075's "w9/m43–m44 funnel" is the CLI-verified `examples/` lifecycle, and `parseNewServiceSearch` (`features/services/lib/create-context.ts:103`) accepts only type/project/environment — t003 decides.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w3` 2026-10-19, item 5 (approved by the user the same day); [ADR075](../../../../docs/ADR075-user-onboarding.md) D1+D2.
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md) — a Render alternative a new user can adopt without help; ADR075's "working path to a first deploy without leaving the dashboard".
- **Expected outcome:** a new signup moves from the payment wall to a first deploy through the dashboard's own guidance; ADR075's funnel counters (workspaces at zero resources) have a lever to move.
- **Why now:** the open-signup gates (`w6/m42`) and the payment wall shipped, so every new account now clears verify + card and lands on dead-text empties; this is the unscheduled remaining half of ADR075, and `w5/161` already covers the auth-error dead end.
- **Render parity included:** dashboard UI change — compare against Render's new-workspace dashboard empty states and onboarding prompts.

## Unblock 记录（2026-10-29，/unblock w3，origin/main `e4d9c65d6`）

- **涉及：** t004、t005、t006、t009。
- **现有规则（AGENTS.md「Local dev environments are pre-approved」）：** t004/t005 需要的 `dev-3` 依赖 mock cluster，而 `scripts/mock-cluster.sh` 的完整重建不需要用户批准，不构成 blocked（2026-10-29 复核时仍只剩 `bex-tenant-0-…` 一个容器）。
- **用户明确决定 D4（2026-10-29）：** 由用户亲自登录 render.com 提供对比来源。用户已在本会话的 Playwright 浏览器里登录（workspace `Tian Pan's Workspace`，hobby 套餐）。
- **已采集的 Render 素材（1440 宽，存于 `.playwright-mcp/`，已 gitignore）：**
  - `m176-render-blueprints-1440.png`：空状态标题「You haven't created any Blueprint instances yet.」，说明文字带 infrastructure-as-code / Learn more 链接，下方一个「+ New Blueprint Instance」按钮。
  - `m176-render-env-groups-1440.png`：「You have no environment groups.」，一句说明，一个「+ New Environment Group」按钮。
  - `m176-render-overview-1440.png`：Overview 右上角有「Invite your team」入口；Projects 区是「Get organized with Projects」推广卡，带「+ Create your first project」和 Learn more，可关闭；**没有 getting-started checklist**。
  - 这个 workspace 有 2 个活跃服务，所以没截到「零服务 Overview」。新建空 workspace 可能涉及套餐，不在本次授权范围内，**没有创建**。
- **worker 的 t006 做法：** 用以上素材做对比，并写进 ADR018/ADR075：bex 的三个空状态按钮对应 Render 的「+ New …」模式；checklist 是 ADR075 D2 的有意增强，Render 没有，按 parity 规则要么记为已签字的 divergence，要么带着这个问题回来找用户（不能自行认定）。如果需要零服务 Overview 或 390 宽截图，且浏览器会话仍有效，可以只读地补截；会话失效就请用户重新登录，不要创建 Render 资源。
- **剩余阻塞：** 无（唯一可能的新问题：checklist divergence 需要用户签字）。
