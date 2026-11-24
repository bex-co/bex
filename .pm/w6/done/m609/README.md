# w6 · m609 — A plan change through Render's Update Service is saved for the next deploy, and billing follows the plan the service runs

**Worker:** worker6 **Goal:** `PATCH /v1/services/{id}` and MCP `update_service` save a compute plan change without deploying, as Render's Update Service does. The service keeps running and being billed on its old plan until a deploy applies the new one. The dashboard's Compute page still deploys at once, as Render's dashboard does. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [REST and MCP save a plan change for the next deploy](done/t001.md) — **DONE** | 1h30m | — |
| t002 | [The operator keeps running the old plan until a deploy applies the saved one](done/t002.md) — **DONE** | 1h | — |
| t003 | [Billing and the Free instance hours follow the plan the service runs](done/t003.md) — **DONE** | 2h | t002 |
| t004 | [Record the saved plan in ADR018, ADR004 and ADR040](done/t004.md) — **DONE** | 30m | t001, t002, t003 |
| t005 | [Ship and check on production](done/t005.md) — **DONE** | 1h | t004 |
| t006 | [Render parity](done/t006.md) — **DONE** | 45m | t005 |
| t007 | [Simplify](done/t007.md) — **DONE** | 15m | t006 |
| t008 | [Test coverage](done/t008.md) — **DONE** | 30m | t006 |
| t009 | [Closeout](done/t009.md) — **DONE** | 10m | t007, t008 |

## Definition of done

- On production, as the QA user in `bex-canary` with a disposable Starter web service: `PATCH /v1/services/{id}` `{"serviceDetails":{"plan":"standard"}}` answers 200 and opens no deploy. The service's pods keep Starter's resources, it reads `undeployedChanges: true`, and the hour's `instance_seconds` row is billed at `starter`. `POST /v1/services/{id}/deploys` then applies Standard in one deploy, and the following hour bills `standard`.
- MCP `update_service` with `plan` does the same.
- The dashboard's Compute page (GraphQL `updateServicePlan`) still opens one `settings_change` deploy that applies the plan at once.
- ADR018 and ADR004 § Settings saves wait for a deploy describe the saved plan, and the w8/m135 plan tests assert it.

## Source + Goal linkage

- **Source:** inbox note [w6/172](../../done/172.md), filed by w6/159's Render check on 2026-11-23 and promoted by `/loopx w6` the same day.
- **Goal linkage:** Render parity ([ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first)). [render.com/docs/compute-plans](https://render.com/docs/compute-plans) § Changing a service's compute plan: "The Update service endpoint does not trigger a new deploy." "For Update service, follow up with a request to the Trigger deploy endpoint to apply the new compute plan." "Your service's compute plan does not change until the first successful deploy after you update it."
- **Expected outcome:** a client that follows Render's documented "update, then deploy" sequence gets one deploy, not two, and a client that only saves gets no restart it did not ask for. The bill matches the plan the instances run.
- **Why now:** w8/m135 made every other Update Service field save only and left plan out as an unverified exception. w6/159 read Render's page and found the exception is a divergence ADR018 never signed off. w6/159 (`f5c781bbc8`) and w6/163 (`14cf6f55fb`, `rollout.AwaitAdoptionBeforeHold`) have since settled what a save-only write and a plan release do around a deploy, so the plan row can join them.
- **Render parity task included:** the change is to REST, MCP and the dashboard's view of a saved plan.

## Triage findings (2026-11-23)

The note's "Check first" questions, answered from the code at `ddc314ffdb`:

- **The operator can hold a saved plan.** `Tier` is `IdentityRelease` (`lego/types/v1alpha1/app_identity.go`), so `releaseView` takes it from the adopted spec, and `pendingReleaseView` (`lego/operator/internal/controller/release_view.go`) renders the running release's tier while a save waits. The reconcile pass reads `app.Spec.Tier` for resources, serving pool, `WEB_CONCURRENCY`, autoscaling, the instance cap, free-tier hibernation and the container policy, and should see the view's tier in each. ADR004 says the settings fingerprint already counts plan for `undeployedChanges`. No test proves any of this for a plan yet: t002 does.
- **Billing would switch at the save.** `SetPlan` writes `Store.SetAppTier` (`lego/backend/internal/apps/service.go`, `SetPlan`), and the hourly meter bills `instance_seconds` at `store.App.Tier` (`lego/backend/internal/usage/service.go`, `processAppMeterWindowResult`). The Free instance hours enforcer reads `a.Spec.Tier` (`lego/backend/internal/freehours/freehours.go`). Today the plan deploys at once, so both are near enough. Once a plan can wait for a deploy, a saved upgrade bills the new plan for instances still on the old one, and a saved downgrade the reverse. A saved paid plan on a Free service would also exempt its still-free instances from the monthly hours. t003 moves both onto the plan the service runs.
- **Render's MCP server cannot change a plan.** The pinned upstream tool list (`lego/backend/internal/api/openapi/render-mcp-tools.json`, captured 2026-11-17) has no update tool for services: upstream removed its placeholder `update_web_service`, `update_static_site` and `update_cron_job` in `b5f93eb`. bex's `update_service` is a bex extension that mirrors REST Update Service, so it follows REST.
- **What changes in the backend.** `servicePatchTable`'s plan row is the only one marked `deploys: true` (`lego/backend/internal/apps/settings.go`), and `ApplyServicePatch` is `rollout.Batch`'s only caller. With the flag gone, every row runs under `rollout.WithSaveOnly` and the batch has nothing left to collapse. `SetPlan` has side effects that assume the plan applies at once: `resumeFromBilling` for a service the Free instance hours suspended, and the `plan_changed` row with its `edge_cache_enabled` / `edge_cache_disabled` beside it (`planEdgeCacheToggle`). t001 settles each against Render.

## Unblocked 2026-11-24 (w6 loopx, verified evidence)

- **Affected:** w6/m609 t005–t009. **Source:** verified evidence, no new decision.
- **The gate cleared.** The newest pin on `origin/main` is `ff5de0a130e6` (`d9dd27d687`), and `git merge-base --is-ancestor 05a4803948 ff5de0a130e6` holds. Production runs it: the `bex-api` and `bex-controller-manager` pods in `bex-system` run `bex-operator@sha256:93c96486…`, the controller digest that pin writes into `deploy/gitops/base/bex.yaml`, since 06:30Z, and the live `apps.app.bex.co` CRD carries `status.servedTier`.
- **Next:** t005 steps 2–7 on production as the QA user in `bex-canary`, then t006–t009. The acceptance checks are the definition of done above.
