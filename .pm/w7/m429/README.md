# w7 · m429 — Workspace plans gate the rest of Render's plan table: autoscaling, rollback builds kept, included custom domains

**Worker:** worker7 **Goal:** the three rows of Render's plan table that bex does not apply yet behave as on Render: autoscaling needs Pro, the builds kept for rollback are 5, 15 and 30 by plan, and custom domains are included 2, 15 and 25 by plan and metered beyond that; a guard keeps the table and the code from drifting apart. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Autoscaling needs the Pro plan on every write path | 75m | — |
| t002 | The Scaling page says autoscaling needs Pro on a Hobby workspace | 45m | t001 |
| t003 | The builds kept for rollback follow the plan: 5, 15 and 30 | 120m | — |
| t004 | Custom domains: 2, 15 and 25 included by plan, then metered | 120m | — |
| t005 | A guard maps every row of Render's plan table to a gate, a meter or a recorded non-goal | 45m | t001, t003, t004 |
| t006 | Production inventory and live check | 45m | t002, t005 |
| t007 | Render parity | 20m | t006 |
| t008 | Simplify | 15m | t007 |
| t009 | Test coverage | 45m | t007 |
| t010 | Closeout | 10m | t008, t009 |

## Definition of done

On production, as the QA user:

1. On a Hobby workspace, turning autoscaling on answers 400 `PLAN_FEATURE_REQUIRED` (`feature: "autoscaling"`, `requiredPlan: "pro"`) on REST `PUT /v1/services/{id}/autoscaling`, on a create carrying `serviceDetails.autoscaling`, on GraphQL `setAutoscaling`, on MCP `update_service(autoscaling:)`, and at the `scaling` path of a Blueprint. A policy that was already on keeps running and can be turned off. The Scaling page shows the plan notice. Pro and higher are unchanged.
2. A service's rollback targets are its 5 most recent builds on Hobby, 15 on Pro and 30 on Scale and Enterprise. `buildAvailable`, `deployActions` and the rollback 409 follow. The Free-instance rule (two most recent deploys) is unchanged.
3. Custom domains beyond the plan's included count (2, 15, 25) appear as a usage line and in `estimatedCost` at $0.175 per domain-month, prorated. Adding a domain is never refused for the count.
4. The guard test passes, and fails when a row of the captured plan table has no disposition.
5. ADR030 §9 and ADR018's Autoscaling, Rollback and Custom domains rows state the rules, and the production inventory is recorded.
6. `cd lego/backend && go test ./...`, `make test` and `make lint` in `lego/operator/`, and dashboard `yarn lint` + `yarn test` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for render parity`, 2026-11-16, item 4; filed by `/pmx them all to w7` the same day.
- **Render reference:** render.com/docs/scaling, read 2026-11-16: "Autoscaling: Available only for Pro workspaces and higher." render.com/pricing, workspace feature table, read 2026-11-16: "Instant rollbacks: 5 builds retained / 15 builds retained / 30 builds retained / 30 builds retained" and "Custom domains: 2 included then $0.25/domain/month / 15 included then $0.25/domain/month / 25 included then $0.25/domain/month / Custom" (Hobby, Pro, Scale, Enterprise). render.com/docs/platform-features-by-plan lists the same three as plan differences.
- **bex today:** `lego/backend/internal/core/plan_gate.go:55-61` gates six features and autoscaling is not one, so a Hobby workspace can turn it on. The registry keeps each service's ten most recent releases plus five recent tags on every plan; ADR018's Rollback row says "Difference, not signed off: for a paid service the number of builds kept does not vary by workspace plan." Custom domains have only abuse caps (`MaxCustomDomainsPerService`, `MaxCustomDomainsPerWorkspace`, `lego/backend/internal/api/server.go:735-740`): no included count and no price in `lego/backend/internal/pricing/`.
- **Goal linkage:** ADR008 Render parity first; ADR030 §9 (workspace plans gate Render's plan features, w8/m137 and w8/m138) and §2 (prices are Render's × 0.70).
- **Expected outcome:** bex's plans differ exactly where Render's do, and a new row on Render's plan table cannot go unnoticed.
- **Why now:** the Rollback row is an open, unsigned difference. w8/m138 applied six rows of the same table three weeks ago, so the gate, the refusal code and the "existing state keeps working" rule already exist. t003 builds on the retention and `storedImages` mechanism w8/m156 shipped (that milestone is parked only on a local-cluster check).
- **Decision state.** None of the following is a recorded user decision; each is this proposal's recommendation resting on ADR008 Render parity first.
  - t001 refuses a new autoscaling policy on Hobby and leaves existing ones running, by ADR030 §9's rule that existing state is never silently removed.
  - t003 lowers Hobby from ten kept builds to five and raises Pro and Scale to 15 and 30.
  - t004 starts a charge for something that is free today. $0.175 is Render's $0.25 at ADR030 §2's standing 30% discount. Keeping extra custom domains free would be a divergence that needs the user's sign-off in ADR018; if the user says so before t004 ships, t004 is dropped and that sign-off is recorded instead. The live Stripe item is created by the user (the classifier denies the agent live Stripe writes, as `w2/blocked/070` found), so no card is charged for domains until the user runs that step.
- **Not verified against Render:** how Render counts a `www`/apex pair or a wildcard domain toward the included count, and what happens to kept builds and domains on a plan downgrade.
- **Render parity:** included: plan refusals, rollback targets and usage lines change on REST, GraphQL, MCP and the dashboard.
