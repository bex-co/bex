# w6 · m125 — The Blueprint plan says "create" for services that already exist, because the planner keys them by tenant-prefixed CR name

**Worker:** worker6 **Goal:** the plan distinguishes creating a resource from updating one, so a user can tell whether applying a manifest will provision new infrastructure or modify what they already have **Status:** done — root cause already fixed by `643d1869` (round-21 finding 7, 2026-08-28) and live on production; this milestone closed the gaps around it (services duplicate guard, apply-then-replan regression suite) and verified the DoD on production with read-only plans

## Tasks (in order)

| id   | title                                                                                   | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Key the plan resolver by the public service name, and add the missing duplicate guard       | 40m | —          | — **DONE** (key: `643d1869`, round-21 finding 7; duplicate guard: this milestone) |
| t002 | Verify the update branch — it has never run for services — produces a truthful diff         | 45m | t001       | — **DONE** (apply-then-replan test + live read-only probe) |
| t003 | Check whether `estimatedPricing` is wrong for the same reason (unverified claim)            | 30m | t001       | — **DONE** (not affected: priced from the parsed stack, never from the plan) |
| t004 | Render parity                                                                                | 25m | t002, t003 | — **DONE** (REST validate/preview, GraphQL `plan`, MCP `validate_blueprint` all serve `blueprintValidationFor`) |
| t005 | Simplify                                                                                     | 20m | t004       | — **DONE** (nothing to flatten: one guard, one helper reuse) |
| t006 | Test coverage                                                                                | 40m | t004       | — **DONE** (4 tests, see closeout) |
| t007 | Closeout                                                                                     | 15m | t005, t006 | — **DONE** |

## Definition of done

- **A second plan of an existing stack is not a create.** Apply the fixture manifest below once (service created), then re-plan the **identical** manifest via `POST /v1/blueprints/validate` (multipart, `file=render.yaml`). The action for `qa-20260827-bp` is **not** `create`. Today it is exactly `{"operation":"create","kind":"service","name":"qa-20260827-bp","sourcePath":"#/services/0"}` with `totalActions: 1`.
- **A changed manifest plans differently from an unchanged one.** Re-plan with `buildCommand: npm ci` instead of `npm install` and get a plan that differs. Today the two are **byte-identical**.
- **The plan predicts the apply.** Apply that modified manifest and confirm what the plan said: same service id, `buildCommand` updated in place, no second service. Apply is already correct today — this bullet exists so the fix cannot "succeed" by breaking apply instead.
- **A genuinely new service still reports `create`,** so the fix does not merely invert the answer.
- **Databases and key-values keep planning correctly** — they are already correct (`:97`, `:111`) and are exactly what a careless refactor of this loop would break.
- **The duplicate-name case reports the same conflict for services** that datastores already report.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co`, **59th run**, 2026-08-27, journey 13 (Blueprints). Workspace `tea-d98210cbbpdc73dcrkvg`. Every probe is re-runnable.

  Fixture manifest — a free node web service on the public repo `github.com/render-examples/express-hello-world`:

  ```yaml
  services:
    - type: web
      name: qa-20260827-bp
      runtime: node
      plan: free
      buildCommand: npm install
      startCommand: npm start
      repo: https://github.com/render-examples/express-hello-world
      branch: main
  ```

  **Step 1 — plan it** (`POST /v1/blueprints/validate`, multipart, before anything exists):

  ```json
  {"valid":true,"plan":{"mode":"current_state","services":["qa-20260827-bp"],"totalActions":1,
    "actions":[{"operation":"create","kind":"service","name":"qa-20260827-bp","sourcePath":"#/services/0"}]},
   "estimatedPricing":{"totalUsd":"0.00","lines":[],"variable":[]}}
  ```

  **Step 2 — apply it** (`POST /v1/blueprints/deploy`) → created `srv-da8a82fm2e9c73ft65fg`. Plan matched apply. Correct so far.

  **Step 3 — re-plan, now that the service exists** — both the identical manifest and one with `buildCommand: npm ci` return the **byte-identical** plan:

  ```json
  {"mode":"current_state","services":["qa-20260827-bp"],"totalActions":1,
   "actions":[{"operation":"create","kind":"service","name":"qa-20260827-bp","sourcePath":"#/services/0"}]}
  ```

  `operation: "create"` for a service that demonstrably exists, and a changed manifest indistinguishable from an unchanged one — while `mode` claims to be `current_state`.

  **Step 4 — apply the changed manifest and watch what actually happens:**

  ```
  GET  /v1/services/srv-da8a82fm2e9c73ft65fg   (before) -> buildCommand "npm install"
  POST /v1/blueprints/deploy  (changed yaml)            -> 200, services:["srv-da8a82fm2e9c73ft65fg"]   <- SAME id
  GET  /v1/services?limit=100                  (after)  -> exactly ONE qa-20260827-bp, buildCommand "npm ci"
  ```

  Apply performed an in-place **update** — same id, no duplicate, field changed — while the plan said **create**. Measured, not inferred.

- **Root cause:** `lego/backend/internal/apps/blueprint_state_plan.go:83`, inside `newBlueprintActionResolver`:

  ```go
  resolver.services[app.Name] = app
  ```

  with the lookup at `:119` (`app, ok := r.services[name]`, where `name` is the **manifest's** service name) and `:150-152` (`if !exists { action.Operation = BlueprintPlanCreate }`). `app.Name` is the Kubernetes object name; for a store-managed App that is `CRName(tenant, name)` — tenant-prefixed — so it never equals the manifest's public name, every lookup misses, and every service plans as a create.

- **The control is eleven lines below, in the same function.** The two sibling resource kinds key by the **public** name and plan correctly:

  ```go
  :97   resolver.databases[database.Spec.Name] = database
  :111  resolver.keyValues[keyValue.Spec.Name] = keyValue
  ```

  Both also carry a duplicate-name guard (`%w: database name %q is already used more than once in this workspace`) that the services loop lacks entirely. Within one function, datastores are right and services are wrong.

- **The repo already diagnosed this exact mistake in the sibling path.** `lego/backend/internal/apps/blueprint_generate.go:177-185`:

  > "The manifest-facing **PUBLIC** name, never the tenant-prefixed CR object name: a store-managed App's `a.Name` is `CRName(tenant, name)`, which overruns `ValidAppName`'s 30-char cap … and writes the workspace's tenant id into that repo. `appServiceName` reads `LabelServiceName`, falling back to `a.Name` only for the legacy hand-applied App that has no such label (its object name IS the public name). **Datastore entries already emit `Spec.Name`; this aligns services with them. (w6/m114)**"

  That is `w6/m114` (done 2026-08-26), which fixed the **exporter** and introduced the correct accessor `appServiceName` (`blueprint_ownership.go:165-170`). The **planner** has the identical defect and was never fixed. Corroborating that `a.Name` is prefixed: `core/base.go:1030` ("metadata.Name may be tenant-prefixed") and `apps/service.go:1738`. Note Apps, unlike Database/KeyValue, have **no** `Spec.Name` — `AppSpec.DisplayName`'s doc (`lego/types/v1alpha1/app_types.go:260-264`) calls it "intentionally distinct from the App object's immutable, DNS-safe Name" — so the fix is `appServiceName(app)`, **not** a `Spec.Name` swap.

- **Blast radius — counted.** `blueprintActionPlan` has **4** call sites (`grep -rn "blueprintActionPlan(" internal/ --include='*.go' | grep -v _test`):
  - `deploy.go:581` — **discards** the plan (`if _, _, err :=`); error gate only — the apply path
  - `blueprint.go:419` (`CreateBlueprint`) — discards, error gate
  - `blueprint.go:572` (`prepareSyncManifest`) — discards, error gate
  - `blueprint.go:277` (`blueprintValidationFor`) — **uses** it; this is the one the user sees

  `blueprintValidationFor` is reached by **both** `POST /v1/blueprints/validate` and `PreviewBlueprint` (`blueprint.go:370`), so one fix corrects both surfaces. Fixing the key also makes the three error-gate call sites evaluate a genuinely different plan for the first time — they have only ever seen creates.

- **Goal linkage:** [docs/ADR049-render-yaml-parity.md](../../../docs/ADR049-render-yaml-parity.md) and [docs/ADR006-bex-api.md](../../../docs/ADR006-bex-api.md). A Blueprint plan is bex's `terraform plan` equivalent; its entire value is telling you what apply will do before you let it.

- **Expected outcome:** the plan distinguishes create from update, so a user can tell whether applying will provision new infrastructure or modify existing infrastructure.

- **Why now:** the plan is shown before a bulk, multi-resource, cost-bearing operation. Reporting "create" for an update makes a user hesitate on a safe change and — more dangerously — hides that an existing production service is about to be modified. It is also a one-line inconsistency whose correct form already exists in the same function.

- **Precedent — extend, do not re-litigate.** `w6/m114` fixed this `a.Name`-vs-public-name mistake in the exporter; this milestone finishes the alignment in the planner. **Not** a regression of m114 — m114's scope was the exporter.

- **Render parity:** included (t004). The plan is served through REST `validate` and `preview`; confirm GraphQL and MCP expose the same plan and move with it. Render's Blueprint preview distinguishes create from update, so this restores parity rather than diverging.

- **Adjacent classes:** a **legacy hand-applied App** with no `LabelServiceName` (its object name IS the public name — `appServiceName` already falls back correctly); an **App in another workspace** (the `scoped`/`LabelTenant` filter at `:80` must keep excluding it, so the fix must not match across tenants); a service **mid-delete**; and **env groups**, whose doc at `blueprint_state_plan.go:30-33` deliberately reports an existing group as "conservatively an update because a no-op cannot be proved without revealing its values" — that convention stays.

- **Unverified this run — carried as work, not presented as observation:** whether `estimatedPricing` is wrong for the same reason (`t003` owns it — only the free tier was exercised, where the figure is `0.00` either way); whether **GraphQL and MCP** expose the plan at all, let alone the same one; whether **databases/key-values** genuinely plan `update` correctly in a live run (their correctness is read from `:97`/`:111`, not measured — the fixture held only a service); and what the **dashboard's** Blueprint page renders for this plan, which was never opened.

## Closeout (2026-09-03)

**Verdict: real, and already mostly fixed before this milestone was picked up.** The keying defect at `blueprint_state_plan.go` (`resolver.services[app.Name]`) was corrected to `appServiceName(app)` by `643d1869` — "fix(security): remediate codex-security round-21 findings 1,2,4-8", 2026-08-28, one day after this milestone was filed — with the regression test `TestValidateBlueprintCurrentStatePlanResolvesStoreManagedService`. That commit is contained in the last two successful `deploy.yml` runs (`c3a0a122` on 2026-09-03 and `a33b32c6` on 2026-09-04), so production runs the fix.

**What this milestone added** (`lego/backend/internal/apps/`):

- `blueprint_state_plan.go` — the services loop now raises `core.ErrConflict` (`service name %q is already used more than once in this workspace`) when two Apps resolve to one public name, the same shape the database/key-value loops raise. Without it, list order silently chose which spec the plan diffed against.
- `blueprint_test.go` — `TestValidateBlueprintCurrentStatePlanPredictsStoreManagedApply` drives the exact QA fixture (node web service, `npm install`) through `applyBlueprintCreate` so the persisted spec is the real one, then asserts: plan before apply = `create`; unchanged re-plan = `noop` on the store id; `npm ci` re-plan = `update` naming `buildCommand` with no value leak; a mixed manifest = `update` + `create` in one plan; re-applying the changed manifest updates the same App in place with no second App; a final re-plan = `noop`. `TestValidateBlueprintCurrentStatePlanLegacyAndForeignServices` pins the legacy no-label fallback and the `LabelTenant` exclusion (an App labelled for another workspace, even in the listed namespace, plans as `create`). `TestValidateBlueprintCurrentStatePlanRejectsDuplicateServiceName` pins the new guard.
- `cd lego/backend && go test ./internal/apps/` and `make lint-backend` pass.

**t002 decision — services report a true no-op, not the conservative update env groups use.** An App's spec is fully readable, so `ApplyBlueprintServiceSpec` on a probe copy proves a zero diff; env groups stay conservative because their values are write-only. The three error-gate call sites (`deploy.go`, `CreateBlueprint`, `prepareSyncManifest`) only inspect the error, so seeing `update`/`noop` actions instead of `create` changes nothing for them.

**t003 — `estimatedPricing` is not affected.** `blueprintEstimatedPricing(st)` prices the parsed stack (what the manifest declares) and never reads the plan; create-vs-update does not enter it. Its doc and the MCP tool description both call it the always-on monthly projection of the declared stack, which is what Render's panel shows.

**t004 — parity holds on every surface.** REST `POST /v1/blueprints/validate` and `PreviewBlueprint`, the GraphQL `BlueprintValidation.plan` field (`graphql.go`), and the MCP `validate_blueprint` / repo-preview tools all return the value of `blueprintValidationFor`, so they moved together with the fix. The dashboard's `blueprint-plan-summary.tsx` does not render `operation` or `changedFields` at all (only `views.ts` maps them), so the dashboard page had nothing to regress; it was not opened live this run.

**Live verification — read-only, against `api.bex.co`, workspace `tea-d98210cbbpdc73dcrkvg`, 2026-09-03.** Only `POST /v1/blueprints/validate` (a dry run) and `GET /v1/services` were called; nothing was created, modified, or deleted, so there is no fixture service or Blueprint record to clean up. The mutating two-apply recipe in t007 was therefore not re-run; the apply half is covered by the apply-then-replan unit test above and by the original hunt's step 4 (apply already updated in place).

| manifest                                                                                   | result                                                                                         |
| ------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------- |
| existing `beancount-forum` (docker), unchanged                                             | `noop`, `resourceId: srv-d9nqg9dcavls73fp8m2g`                                                 |
| existing `block-eden-mono` (node, build+start commands), unchanged                          | `noop`, `resourceId: srv-d9ndt8hmcglc739fkp50`                                                 |
| same, `startCommand` changed                                                               | `update`, same `resourceId`, `changedFields` present                                           |
| fresh name `qa-m125-never-created`                                                          | `create`, no `resourceId`                                                                      |
| `name: eden-dash-v3` — the **display name** of `block-eden-mono`                           | `create` — truthful: `GetApp` resolves ids and immutable names only, so apply would create too |

**Two observations recorded, not fixed here (neither is this milestone's defect):**

- A Blueprint `name` is matched against the immutable name (`immutableName` on the API, `LabelServiceName` on the CR). A service renamed in the dashboard shows its display name as `name` on REST/GraphQL/MCP, and a manifest written from that field plans and applies a **second** service. Plan and apply agree, so the plan is honest, but it is a footgun worth a docs note or a validation hint.
- `changedFields` on an `update` lists every **declared** field (`blueprintPlanFieldChanges(resource.Fields)`), not the fields that actually differ — the live `startCommand` change listed all eight declared paths. Same convention for databases and key-values; the type doc says "changed field path". A precise per-field diff for services would need to handle `buildFilter`'s clear-on-omission and the `scaling`/`numInstances` coupling.

**Not measured live:** database/key-value planning (the QA workspace's manifest held only services; unit-covered by `blueprint_ir_test.go`), and any App still in the shared pre-ADR043 namespace (the planner lists `AppNamespace(tenant)` exactly as the canonical services `List` does, so plan and list agree by construction).
