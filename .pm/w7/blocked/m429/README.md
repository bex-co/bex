# w7 · m429 — Workspace plans gate the rest of Render's plan table: autoscaling, rollback builds kept, included custom domains

**Worker:** worker7 **Goal:** the three rows of Render's plan table that bex does not apply yet behave as on Render: autoscaling needs Pro, the builds kept for rollback are 5, 15 and 30 by plan, and custom domains are included 2, 15 and 25 by plan and metered beyond that; a guard keeps the table and the code from drifting apart. **Status:** blocked (t001–t005 and t007–t009 done; t006's inventory recorded, its production walk waits for the pin to carry the milestone)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Autoscaling needs the Pro plan on every write path — **DONE** | 75m | — |
| t002 | The Scaling page says autoscaling needs Pro on a Hobby workspace — **DONE** | 45m | t001 |
| t003 | The builds kept for rollback follow the plan: 5, 15 and 30 — **DONE** | 120m | — |
| t004 | Custom domains: 2, 15 and 25 included by plan, then metered — **DONE** | 120m | — |
| t005 | A guard maps every row of Render's plan table to a gate, a meter or a recorded non-goal — **DONE** | 45m | t001, t003, t004 |
| t006 | Production inventory and live check | 45m | t002, t005 |
| t007 | Render parity — **DONE** | 20m | t006 |
| t008 | Simplify — **DONE** | 15m | t007 |
| t009 | Test coverage — **DONE** | 45m | t007 |
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

## Production inventory (t006, read-only, 2026-11-16)

Read from production's control-plane store (`bex-db`, `SET default_transaction_read_only = on`) and the App objects, through the kubeconfig `scripts/fetch-app-kubeconfig.sh` fetched with a one-run host-key pin built from the stable control-plane key's public half (`~/.ssh/bex_cp_host_ed25519.pub`, `SHA256:W/o3ED3…`, the CI pin; w6/m153). The laptop's `~/.ssh/known_hosts` still holds stale keys for all three control-plane addresses, so an unpinned run fails host-key checking.

- Workspaces: 74 Hobby, 4 Pro, 3 Scale, 1 Enterprise. Apps: 27 (19 in Scale workspaces, 7 in Hobby, 1 in `default`).
- Autoscaling: no App on any plan has a policy on, so the Hobby refusal stops nothing that runs today.
- Custom domains: one workspace has any (a Scale workspace, 7 counted of its 25 included; 9 rows with the bex-added `www` siblings). No workspace is over its count, so nothing is metered yet.
- Kept builds: `status.storedImages` lists at most 7 builds for any service. One Hobby service holds 7 (its new rule keeps 5 live builds plus the five newest pushes); no Scale service holds more than 5 of its 30.

## Live Stripe step (the user's, from t004)

No card is charged for custom domains until the user creates the live Price and the hold is lifted:

1. `python3 scripts/stripe-billing-setup.py --live` (adds the `custom_domain_hours` item; the agent ran no Stripe call).
2. Set `BEX_STRIPE_CUSTOM_DOMAIN_METER` to `"1"` in `lego/operator/config/api/deployment.yaml` (shipped as `"0"`) and ship it. Until then the meter is on no Subscription and its rows wait in the outbox; held rows inside the 34-day window ship once released.

This brings the Stripe catalog to 20 items, Stripe's maximum per Subscription, so the next metered item needs a design change first.

## Gate (2026-11-16)

Clears by itself: the production pin must carry this milestone's commit. Check `git merge-base --is-ancestor "$(git log -1 --format=%H --grep="w7/m429 t001" origin/main)" <the sha of the newest "pin platform images to" commit on main>`, then run t006's steps 3–6 (autoscaling refused on a Hobby workspace through REST, GraphQL, MCP and the Scaling page and accepted on Scale; `storedImages` and rollback targets for one Hobby and one Scale service; the custom-domain row on a workspace over its count, with a Hobby canary's third domain created, verified and removed), then t010. The live Stripe step above is the user's and does not gate the closeout: t010 files it as its own note if it is still open.
