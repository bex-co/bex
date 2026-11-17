# w7 · m429 — Workspace plans gate the rest of Render's plan table: autoscaling, rollback builds kept, included custom domains

**Worker:** worker7 **Goal:** the three rows of Render's plan table that bex does not apply yet behave as on Render: autoscaling needs Pro, the builds kept for rollback are 5, 15 and 30 by plan, and custom domains are included 2, 15 and 25 by plan and metered beyond that; a guard keeps the table and the code from drifting apart. **Status:** done 2026-11-17 (production walk passed on pin `db48cde2bae9`; the live Stripe step is the user's, tracked as w7/080)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Autoscaling needs the Pro plan on every write path — **DONE** | 75m | — |
| t002 | The Scaling page says autoscaling needs Pro on a Hobby workspace — **DONE** | 45m | t001 |
| t003 | The builds kept for rollback follow the plan: 5, 15 and 30 — **DONE** | 120m | — |
| t004 | Custom domains: 2, 15 and 25 included by plan, then metered — **DONE** | 120m | — |
| t005 | A guard maps every row of Render's plan table to a gate, a meter or a recorded non-goal — **DONE** | 45m | t001, t003, t004 |
| t006 | Production inventory and live check — **DONE** | 45m | t002, t005 |
| t007 | Render parity — **DONE** | 20m | t006 |
| t008 | Simplify — **DONE** | 15m | t007 |
| t009 | Test coverage — **DONE** | 45m | t007 |
| t010 | Closeout — **DONE** | 10m | t008, t009 |

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

Still open at closeout (2026-11-17): tracked as [w7/080](../../blocked/080.md), the user's one command.

## Gate (2026-11-16)

Clears by itself: the production pin must carry this milestone's commit. Check `git merge-base --is-ancestor "$(git log -1 --format=%H --grep="w7/m429 t001" origin/main)" <the sha of the newest "pin platform images to" commit on main>`, then run t006's steps 3–6 (autoscaling refused on a Hobby workspace through REST, GraphQL, MCP and the Scaling page and accepted on Scale; `storedImages` and rollback targets for one Hobby and one Scale service; the custom-domain row on a workspace over its count, with a Hobby canary's third domain created, verified and removed), then t010. The live Stripe step above is the user's and does not gate the closeout: t010 files it as its own note if it is still open.

## Unblock record (2026-11-17, `/pm unblock w7/m429`, verified evidence)

- **Affected:** t006 (steps 2–6), t010. **Source:** verified evidence, not a user decision.
- **Gate cleared:** the newest production pin on `origin/main` is `fc4fbf28a` (`chore(deploy): pin platform images to db48cde2bae9`, 2026-11-17 12:08Z), and `git merge-base --is-ancestor 075eb5f6a db48cde2bae9` and `git merge-base --is-ancestor c1dc02202 db48cde2bae9` both succeed. Production serves it: both `bex-api` pods and `bex-controller-manager` run `ghcr.io/bex-co/bex-operator@sha256:ced89914…` and the dashboard runs `bex-dashboard@sha256:16bd3dea…`, the digests that pin wrote, all started 12:12–12:13Z.
- **Still the user's, not a gate:** the live Stripe step above. t010 files it as its own note if it is still open.
- **Next:** t006 steps 3–6 (the production walk), then t010.

## Production walk (t006, 2026-11-17, pin `db48cde2bae9`): PASS

Signed in as the QA user (`scripts/qa-login.sh`), in `tian-personal` (`tea-da2isimlm39c739m4ofg`, Hobby) and `bex-canary` (`tea-daif693dqjvc73e7as3g`, Scale). Read-only reads used the App objects and `bex-db` (`SET default_transaction_read_only = on`), through a kubeconfig fetched with the one-run pin built from `~/.ssh/bex_cp_host_ed25519.pub` (`SHA256:W/o3ED3/xJrcoF0ONPn3Cb1S98X32ubnDcTeYtSVRyU`). The dashboard was read with a headless Chromium (the Playwright MCP server did not connect).

**Fixtures.** `qa-20261117-m429-hb` (`srv-dbu7pj8gt2jc73djdsh0`), a Starter image service (`traefik/whoami:v1.10.1`) in the Hobby workspace, created 16:11:57Z. `qa-20261117-m429-sc` (`srv-dbu7puuggh5s73emen20`), the same in the Scale workspace.

**1. Autoscaling needs Pro.**

| Surface | Hobby | Scale |
| --- | --- | --- |
| REST create carrying `serviceDetails.autoscaling` | 400 `PLAN_FEATURE_REQUIRED`, `params {feature: autoscaling, plan: hobby, requiredPlan: pro}`; no service by that name | 201, the policy on (min 1, max 2) |
| REST `PUT /v1/services/{id}/autoscaling` | 400, same code; `GET` still reads `enabled: false` | 200 (max 3, CPU 60%) |
| GraphQL `setAutoscaling` | `PLAN_FEATURE_REQUIRED` in `extensions` | accepted (max 2, CPU 75%) |
| MCP `update_service(autoscaling:)` | `isError`, `PLAN_FEATURE_REQUIRED: … this workspace is on Hobby` | accepted (max 3, CPU 80%) |
| `POST /v1/blueprints/validate` with `scaling` | `valid: false`, `PLAN_FEATURE_REQUIRED` at `services[0].scaling` (line 9) | `valid: true`, the create planned |
| Scaling page | "Autoscaling is available on the Pro workspace plan and higher. This workspace is on Hobby. Open Billing"; the switch reads off and is disabled | the switch reads on and is enabled, with the range and targets |

`DELETE …/autoscaling` turned the Scale policy off (204; `GET` kept the range with `enabled: false`). No Hobby workspace has a policy on (t006 inventory), so "a policy already on keeps running and can be turned off" rests on `TestAutoscalingOnHobbyKeepsAnExistingPolicy` and the dashboard test "shows a running policy read-only, and turning it off still works".

**2. Builds kept follow the plan.** The registry's retention (`bex-registry/zot-config`) lists the repositories of every Hobby App under `^live-.*` = 5 and every Scale App under `^live-.*` = 30, each with `.*` = 5, ahead of the `**` catch-all of ten. Every App of a Hobby or Scale workspace carries `app.bex.co/kept-builds`: 5 on the 6 Hobby Apps besides the fixture, 30 on the 19 Scale Apps. The two others (`hello-static` in `default`, and `verify-iso-b`) carry none and keep ten, as for an unknown plan.

- **Hobby, `tianpan-v4-web`** (`srv-da40m1qii7bs73drbqlg`, 35 builds that went live): `status.storedImages` holds 5 digests, exactly the builds of its 5 most recent live deploys. GraphQL `buildAvailable` is true on those five and false from the sixth on, and the Deploys tab offers Rollback on exactly the four previous ones. `POST …/rollback` to the sixth (`dep-dba2alb4pcls73bt1pr0`) answered 409 "the build of deploy … is no longer stored — only a service's recent builds are kept. Deploy its commit again instead", and the live deploy did not change. `deployActions` offers `rollback` (`allowed`). No Hobby App holds more than 5 of its live builds.
- **Scale:** every Scale service with fewer builds than its count shows all of them (`twenty-crm` 3 of 3, with `buildAvailable` on all four deploys that went live; `hello-go` 4 of 4; `metrics-grafana` 2 of 2). Services with longer histories hold 3 to 6, because builds removed before this milestone do not come back. For example, `block-eden-mono-1` holds 3 of its 7. Its registry tags are `gen-13`, `gen-14` and `live-12`…`live-14`: the `live-` tags began with w8/m156 on 2026-11-11, so older builds were kept only as the five newest pushes. The same holds for the Hobby service `klivzo` (4 of 5; tags `gen-13`…`gen-16`, `live-16`). From the pin on, Scale keeps up to 30.

**3. Custom domains above the included count are metered.** `qa-m429-{1,2,3}.49-12-20-236.sslip.io` (sslip.io names resolving to the edge, `49.12.20.236`) were added to the Hobby fixture at 16:13:48Z, each answering 201 with nothing refused for the count. They were verified at 16:13:56Z (`cdm-dbu7qf6ggh5s73emen50`, `cdm-dbu7qf8gt2jc73djdsjg`, `cdm-dbu7qfeggh5s73emen60`).

- Right away, REST `GET /v1/usage`, GraphQL `usage` and MCP `get_usage` read `customDomains {used: 3, included: 2, unlimited: false}`. The Billing page read "3 of 2 included custom domains in use", with "Your plan's included custom domains are in use…".
- The 17:12:41Z rollup metered the 16:00 window: one `usage_hourly` row `custom_domain_seconds` = 2763. That is the third domain from 16:13:56.78Z to 17:00:00Z; the first two are included.
- REST, GraphQL and MCP `estimatedCost.resources` then carried the `custom_domains` resource "Custom domains" with the charge `custom_domain_seconds`: 0.00105 domain-mo at $0.175, $0.00. The Billing page's charge tree showed "Custom domains $0.00 › Custom domains beyond the plan $0.175/domain-mo 0.00105 domain-mo".
- `estimatedCost.meters` omits it, as it omits every meter below $0.01 (`pricing.go` "Meters whose rounded cost is below $0.01 are omitted"). The Stripe meter stays held, so the row waits in the outbox (see w7/080).

**Cleanup.** The Scale fixture was deleted at 16:13:43Z (204, then `GET` 404). At 17:14Z the three domains were deleted (204 each; the list read `[]`), then the Hobby fixture (204, then 404). Afterwards usage read `customDomains {used: 0, included: 2}`, no `domains` row or App of the fixtures remained, and the QA session was logged out (`GET /v1/owners` 401). The kubeconfig, cookie jar and state file were deleted. `.env`, `~/.ssh/known_hosts` and the pin files were not changed. The 2763 domain-seconds stay on `tian-personal`'s November estimate, worth $0.0002.

**DoD item 6.** Every CI suite passed on `075eb5f6a`: `test (backend)` with real Postgres, OpenFGA and OpenBao, `test (operator)`, `go lint`, `test (dashboard)` and the rest (16 of 16 workflows green). This closeout changes only markdown.
