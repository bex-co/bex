# w6 · m12 — Workspace plan change: upgrade · downgrade · plan-gated roles

**Worker:** worker6 **Goal:** the workspace plan becomes mutable after create — upgrade Hobby→Pro actually unlocks invites, downgrades that would violate the target plan's caps are refused with Render-shaped errors, and the role picker matches the plan. **Status:** done 2026-07-12 — `changeWorkspacePlan` (GraphQL-only, matching Render's REST-read-only/MCP-mutation-less shape) ships with downgrade guards for members/services/per-user-workspace-cap/out-of-plan-roles; `store.RolesFor` was folded into `PlanLimits.AllowedRoles` + a `RoleAllowedOnPlan` predicate during the `/simplify` pass (t007), shared by `members.guardPlanRole` and `workspaces.rolesOutsidePlan`; the per-user-workspace-cap guard was deduped into `Service.guardPerUserWorkspaceCap`, shared by `Create` and `ChangePlan`. Dashboard: a plan section + change-plan dialog (reusing `/new/workspace`'s `PlanPicker`) on workspace settings, verified live against the local dev stub (dialog opens, plan selection enables submit, mutation fires, success toast). `docs/render-artifacts/workspace-plan-change.md` captured live from `dashboard.render.com` (selection flow observed; submit deliberately not clicked — real billing account) with bex's decided guard copy recorded up front and verified verbatim against the shipped code. `go test ./...` (backend) and `yarn typecheck`/`yarn vitest run` (dashboard, 685 tests) green; `golangci-lint` clean.

## Tasks (in order)

| id   | title                                                                                                                              | est | depends_on | status |
| ---- | ----------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- | ------ |
| t001 | Capture Render's change-plan flow → `docs/render-artifacts/workspace-plan-change.md`; decide surface shape + guard copy             | 30m | —          | — **DONE** |
| t002 | Backend: `Service.ChangePlan` verb — `can_manage` scope, `NormalizePlan`, downgrade guards (members/services/per-user caps)         | 45m | t001       | — **DONE** |
| t003 | GraphQL `changeWorkspacePlan` mutation + `definitions.ts` sync (REST absent = parity; MCP stays read-only = parity)                 | 30m | t002       | — **DONE** |
| t004 | Plan-gate roles: invite/change-role reject roles outside the plan's set; downgrade refuses members holding roles the target lacks   | 45m | t002       | — **DONE** |
| t005 | Dashboard workspace settings: plan section (current plan, change-plan dialog with plan cards, guard errors surfaced)                | 45m | t003, t004 | — **DONE** |
| t006 | Render parity: sweep GraphQL/UI vs the capture; confirm REST/MCP absence is the correct shape                                       | 30m | t005       | — **DONE** |
| t007 | Simplify: `/simplify` over the code this milestone changed                                                                          | 30m | t006       | — **DONE** |
| t008 | Test coverage: meaningful tests for guards, role gating, and the settings flow                                                      | 40m | t006       | — **DONE** |
| t009 | Closeout: verify the DoD holds, mark done, move to `w6/done/m12/`                                                                    | 15m | t008       | — **DONE** |

## Definition of done

On a Pro workspace with 2 members, `changeWorkspacePlan(plan: "hobby")` is rejected with the member-guard error; hobby→pro succeeds and the dashboard settings page reflects the new plan; inviting a `VIEWER` on a Pro workspace is rejected (Scale+ role) while `DEVELOPER` succeeds; `docs/render-artifacts/workspace-plan-change.md` exists and the shipped guard copy matches it.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w6` 2026-07-12; `RESEARCH-workspaces.md` findings 1 (plan lineup), 4 (Hobby constraints — invites require Pro+), 5 (roles are plan-gated); verb inventory of `lego/backend/internal/workspaces/service.go` (Create/Rename/Delete only — no plan change).
- **Goal linkage:** w6's founding theme — user-initiated workspace lifecycle (the workstream README's own scope line names "plan limits"); GOAL.md #5 (multi-tenant).
- **Expected outcome:** the plan field stops being write-once — a Hobby workspace can be upgraded and then actually invite members; the five-role enum stops silently diverging from Render's plan gating.
- **Why now:** it's the last missing lifecycle verb and every ingredient shipped in w6 itself (plan catalog + limits in `internal/store/plans.go`, `can_manage` gating, the settings page) — the wiring cost will never be lower. Billing stays out per "Not in w6": the plan flips with no payment step, exactly like create.
- **Render parity:** **included** (t006) — feature dev touching GraphQL + dashboard UI; the parity task also confirms that REST/MCP *absence* matches Render (research finding 9: the REST owners surface is read-only; Render's MCP has no workspace mutations).
