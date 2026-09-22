# A blueprint sync silently orphans any resource the manifest stops declaring — the plan has no verb for it

Why: dropping a resource from a manifest is an ordinary IaC edit, and a rename is that edit twice over; today it leaves the old resource running and billing with nothing managing it, and the plan the author reads beforehand cannot say so because its operation vocabulary has no word for a resource leaving.

Found by live `/qa-find-bugs` 2026-09-21 pass 131 (w4-targeted, `muse.env` credentials), applying real Blueprints and reading the result. All fixtures deleted.

## Repro

```
1. createBlueprint(examples/static-site/render.yaml)
   → applies; blueprint.resources = [ srv-…n70  static-site  static_site ]; status in_sync

2. validateBlueprint(<manifest declaring only qa-20260921-keep>)
   → valid: true
     plan.actions = [ { kind: service, name: qa-20260921-keep, operation: create } ]
       ↑ one action. Nothing about static-site leaving.

3. syncBlueprint(bexYaml: <that manifest>)
   → services: [ { id: srv-…na0, name: qa-20260921-keep } ]
     blueprint.resources = [ srv-…na0  qa-20260921-keep  web_service ]   ← static-site gone from tracking
     blueprint.status    = in_sync
     workspace services  = […, qa-20260921-keep, static-site]            ← still running
```

`static-site` is still there, still costing money, and is now tracked by nothing. The blueprint reports `in_sync` — truthfully about what it now manages, and misleadingly about what it used to.

**Correction, pass 133: "tracked by nothing" is too kind — the orphan is also _locked_.** The blueprint's durable ownership claim on it is not released. `ReleaseBlueprintResourceClaims` has exactly one non-test call site, `apps/blueprint.go:1226`, inside `DisconnectBlueprint`; nothing runs at sync time. Meanwhile `resources[]` is computed from the current manifest's IR (`apps/blueprint.go:1404` `resolveBlueprintResourcesFromIR`), so the dropped resource cannot appear there. The two surfaces then contradict each other about the same blueprint id — observed live in pass 133, where a preview reported `service "static-site" is managed by blueprint blp-dan9g8rs0ils73bgp500` while `blueprint(id:"blp-dan9…").resources` listed only a different service. Until that blueprint is disconnected, no other blueprint can adopt the orphan without a confirmation phrase naming a blueprint that disclaims it. This also breaks `w8/m23`'s definition of done, which promised the ownership marker would be "visible on the blueprint's `resources[]`". Filed with its create-path sibling as [`w4/m125`](m125/README.md) t002.

**Not deleting it is right.** Render detaches rather than destroys, and silently deleting a production service because a line left a YAML file would be far worse. The defect is the _silence_, not the survival.

## Why the plan cannot warn you

`lego/backend/internal/apps/blueprint_ir.go:90-92` — the entire vocabulary:

```go
BlueprintPlanCreate BlueprintPlanOperation = "create"
BlueprintPlanUpdate BlueprintPlanOperation = "update"
BlueprintPlanNoop   BlueprintPlanOperation = "noop"
```

There is no `detach` / `remove` / `orphan`. The plan is built by walking the **manifest's** resources (`blueprint_state_plan.go` resolves each declared resource against current state), so a resource that is no longer declared is never visited and can never produce an action. The omission is structural, not a missing branch.

## The case that makes this sharp: renaming

Blueprint resources are matched by `name`. So changing

```yaml
- name: api        →     - name: api-v2
```

is not a rename — it is "orphan `api`, create `api-v2`". The author sees a plan with **one** `create` action, applies it, and ends up paying for two services, one of which no longer appears in the blueprint. Nothing in the plan, the sync result, or the blueprint status mentions the first one.

## Fix

Two separable pieces, and the first is most of the value:

- **Make the plan say it.** Add a removal operation and populate it by diffing the blueprint's currently-tracked resource set against the manifest's declared set — both are already in hand at plan time (`blueprint.resources` and the parsed stack). The word matters: `detach`, not `delete`, so the action reads as "stops being managed", which is what happens.
- **Decide what the sync result and blueprint status say afterwards.** `in_sync` immediately after orphaning something reads as "everything is accounted for". At minimum the detached ids belong in `SyncBlueprintResult`; the dashboard's blueprint page is the natural place to list "no longer managed by this blueprint".

Estimate: ~1h for the plan verb plus its wiring through REST/GraphQL/MCP and the dashboard's plan summary; that makes it milestone-sized if the status/result half is included — promote if so.

## Unverified

- ~~The **dashboard's** blueprint page was not opened during the orphaning~~ — **closed in pass 132.** With a resource orphaned, `/blueprints/<id>` shows a **Managed Resources** table listing only the currently-declared resources; the orphan appears nowhere on the page, and the header reads **In Sync**. So the UI reproduces the API's silence exactly. (Sync History on the same page is good: it lists the failed sync with its full error text.)
- ~~Whether datastores orphan the same way as services~~ — **closed in pass 132.** Identical. A Key Value declared by the manifest and then dropped: the plan showed one action (`noop` on the surviving service) and nothing about the Key Value; after sync `qa-20260921-d1` was still `available` and the blueprint tracked only the service. A datastore is the more expensive thing to leave running unmanaged.
- Whether `autoSync` (a git push that drops a resource) behaves identically was not tested — it uses the same sync path, but that is inference, not observation.

## Also checked this pass, and found correct

- **`w4/118`'s unverified line is now closed, live.** Applying a manifest with a dangling `fromDatabase` fails with exactly the message the code promised — `bad request: fromDatabase references unknown database "qa-no-such-database-anywhere" in this workspace` — and **creates nothing**: the workspace service list was byte-identical before and after. So the deferred check is correct and the apply is all-or-nothing; the only thing wrong is that it runs after validation has already said `valid: true`. That also makes `w4/118`'s proposed fix safer than it looked, since the resolver already runs before anything is written.
- **A failed sync leaves the blueprint in `status: error`**, which is honest, and a subsequent good sync returns it to `in_sync`.
- **Apply matches plan for the create path** — the plan's single `create` produced exactly that service, tracked with the right id and type.
