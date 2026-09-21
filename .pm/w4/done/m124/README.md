# w4 · m124 — A blueprint bex exported from a service never re-plans as a no-op, because `domains:` round-trips into the wrong spec field

**Worker:** worker4 **Goal:** Generate Blueprint → validate is a closed loop: a manifest bex exported from live resources, re-planned against those same resources, reports `noop` for every one of them. **Status:** done 2026-09-21 (live re-probe deferred to the next QA pass; a third round-trip defect was found by this milestone's own guard — see Outcome)

## Tasks (in order)

| id   | title                                                                                   | est | depends_on   |
| ---- | --------------------------------------------------------------------------------------- | --- | ------------ |
| t001 | Restore the `Host` / `Hosts` split when a manifest's `domains:` is applied                 | 45m | —            | — **DONE**
| t002 | Trace the Key Value half — `ipAllowList: []` also never re-plans as a no-op, cause unverified | 40m | —            | — **DONE**
| t003 | Make `changedFields` report the fields that changed, not the fields the manifest declares  | 40m | —            | — **DONE**
| t004 | Close the loop with a round-trip guard over every exportable resource kind                 | 40m | w4/m124/t001, w4/m124/t002 | — **DONE**
| t005 | Render parity — plan semantics across REST/GraphQL/MCP and the dashboard's plan summary    | 30m | w4/m124/t004 | — **DONE**
| t006 | Simplify — `/simplify` over the code this milestone changed                                | 20m | w4/m124/t005 | — **DONE**
| t007 | Test coverage — export → plan is `noop` for every kind, and a real change is named exactly  | 40m | w4/m124/t005 | — **DONE**
| t008 | Closeout — close the milestone once the definition of done actually holds                  | 15m | w4/m124/t007 | — **DONE**

## Definition of done

- **Export → validate is a no-op.** Select a web service, a Postgres and a Key Value in **Generate Blueprint**, take the emitted `render.yaml` unmodified, and pass it to `validateBlueprint`. Every action reports `operation: "noop"` with `changedFields: []`. Today the Postgres reports `noop` and the other two report `update`.
- **A domain-carrying manifest is stable.** The exported service block including `domains: [blockeden.xyz]` re-plans as `noop`. Today removing that one line is what turns `update` into `noop` — it is the only field in the whole exported service entry that breaks the round trip.
- **A Key Value manifest is stable.** The exported key-value block including `ipAllowList: []` re-plans as `noop` (t002 establishes why it does not today; the fix follows its finding, not this bullet's guess).
- **`changedFields` names what differs.** Changing one field in a manifest yields exactly that field. Today `databases:` with a changed `plan` reports `["name", "plan"]` — `name` is the key the resource was matched by and cannot have changed — and `services:` entries report their entire declared field set regardless of what differs, so two manifests differing in `healthCheckPath` and `startCommand` produce byte-identical plans.
- **A real change is still detected.** Every bullet above must hold without making the planner blind: the `pg_changed` probe (`plan: basic-256mb` → `basic-1gb`) still reports `update` naming `plan`, and the service/key-value equivalents do too once t001/t002 land.
- **The loop is guarded.** A test drives generate → validate for each exportable kind (web, static, cron, worker, private, Postgres, Key Value, env group) and fails if any of them reports anything but `noop`, so the exporter and the planner cannot drift apart again field by field.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` 2026-09-21 pass 110 (w4-targeted, `muse.env` credentials), journey 13. All probes were read-only `validateBlueprint` queries against existing workspace resources; nothing was created or modified.
- **Goal linkage:** ADR018's Blueprint rows and journey 13's promise that the plan diff matches what apply does. Directly continues **`w6/m125`**, whose t002 decision was "services report a true no-op, not the conservative update env groups use" — that decision holds for the manifests m125 tested, and this milestone is the field it did not cover.
- **Expected outcome:** the standard infrastructure-as-code adoption path works. A user exports their existing stack, commits the file, connects it as a Blueprint, and the plan says "nothing to do" — instead of proposing to update every resource it just described.
- **Why now:** this is the first thing a user does with Blueprints, and it fails in the direction that destroys trust: the plan claims it will modify production resources it should leave alone. A plan nobody believes is a plan nobody reads, which is worse than no plan at the moment one of these actions is real.
- **Render parity task included:** plan semantics are exposed on REST `POST /v1/blueprints/validate`, GraphQL `validateBlueprint`, the MCP validate/preview tools, and the dashboard's plan summary.

## Evidence (live, 2026-09-21)

**1. The exported manifest, verbatim from Generate Blueprint** (web service + Postgres + Key Value), re-planned against the same three resources:

```
databases[0] beancount-forum-db    → operation "noop",   changedFields []
services[1]  beancount-forum-redis → operation "update", changedFields [ipAllowList maxmemoryPolicy name persistenceMode plan type]
services[0]  block-eden-mono       → operation "update", changedFields [rootDir plan repo type healthCheckPath name runtime startCommand buildCommand domains]
```

`valid: true`, `errors: []` — the manifest is well-formed; it just does not describe its own source. The Postgres `noop` is the control: the planner *can* report no-op, so this is not "the planner always says update".

**2. `domains:` is the single field that breaks it.** Taking the exported service entry and removing one optional line at a time, with the real folded `buildCommand` preserved:

```
real (verbatim export)          → update
  minus domains:                → noop     ←
  minus rootDir                 → update
  minus healthCheckPath         → update
  minus plan                    → update
```

**3. `changedFields` is not a diff.** Two key-value manifests — one matching current state, one with `maxmemoryPolicy: allkeys-lru` changed to `noeviction` — produce **identical** output:

```
matching  → update, [maxmemoryPolicy name persistenceMode plan type ipAllowList]
changed   → update, [maxmemoryPolicy name persistenceMode plan type ipAllowList]
```

The same holds for `type: web`: manifests differing in both `healthCheckPath` and `startCommand` return the same operation and the same ten field paths. And on the working `databases:` path, a genuine `plan` change reports `["name", "plan"]` — `name` is the match key.

## Root cause

**The domains half is traced to the line.**

- `lego/backend/internal/apps/blueprint_generate.go:469-475` — `appDomains` flattens the two spec fields into one list: `Host` first, then `Hosts...`. A service whose primary custom domain lives in `Spec.Host` exports as `domains: [blockeden.xyz]`.
- `lego/backend/internal/apps/service.go:1687` — `CreateRequest` has only `Hosts []string`. There is no `Host` field on the create path, so `specFromCreate` yields `want.Host == ""` and `want.Hosts == ["blockeden.xyz"]`.
- `lego/backend/internal/apps/blueprint_plan.go:153-161` — the `domains`/`domain` applier writes both: `dst.Host = want.Host` (so `""`) and `dst.Hosts = slices.Clone(want.Hosts)`. The same domain moves from `Host` to `Hosts`, `reflect.DeepEqual(before, *dst)` is false, and the action is `update` — forever, on every re-plan.
- The applier's own comment shows the author was alert to exactly this class ("Kubernetes drops an empty slice when it round-trips the CR, so preserving `[]` here would make an identical Blueprint look changed on every re-apply") — the `Hosts` nil-vs-empty case is handled; the `Host`-vs-`Hosts` case beside it is not.

**The `changedFields` half is a separate, simpler defect.**

- `lego/backend/internal/apps/blueprint_state_plan.go:188,206,217,225` — all four branches set `action.ChangedFields = blueprintPlanFieldChanges(resource.Fields)`, the manifest's declared field paths. Nothing computes which fields differ. `blueprint_ir.go:306` has the shape that does (`changedBlueprintFields(resource.Fields, current.Fields)`), so the intended behavior exists elsewhere in the same package.

**Not a regression of `w6/m125`, and not deploy lag.** `git rev-list --count eb954c381..HEAD` over `blueprint_plan.go`, `blueprint_state_plan.go` and `blueprint_ir.go` is **0** — the plan code has not been touched since m125 landed on 2026-09-03, eighteen days ago and long deployed. m125's evidence table records `block-eden-mono` unchanged → `noop`, and that still reproduces today: its manifests carried `buildCommand`/`startCommand` and no `domains:`, which is precisely the field this milestone is about.

## Blast radius

- The `domains` applier is reached by every `services:` entry that declares `domains` or `domain` — all five service types (web, static, cron, worker, private), since the applier is in the shared `blueprintServiceFieldAppliers` table.
- Changing `changedFields` to a real diff (t003) changes output for **every** resource kind and every surface that renders it. `w6/m125` t004 recorded that `blueprint-plan-summary.tsx` does not render `operation` or `changedFields` at all (only `views.ts` maps them) — re-check that before assuming the dashboard is unaffected, since it was not opened live in that run either.
- `syncBlueprint` and `createBlueprint` consume the same planner. A planner that says `update` for an unchanged resource may cause a real apply to write a no-op update — t004 must establish whether apply is merely noisy here or actually mutates.

## Adjacent classes

- **`create`** — resource absent; unaffected, and must stay correct when the diff changes.
- **`noop`** — the case this milestone is about.
- **`update`** — must keep firing for genuine changes; the `pg_changed` probe is the regression guard.
- **Env groups** — deliberately always `update` because their values are write-only (`blueprint_state_plan.go:220-224`, with the reason in the code). That exemption is correct and must survive; do not "fix" it into a no-op.

## Unverified this run

- **The Key Value cause is not traced.** `ipAllowList: []` is a *candidate* — it is the only list-valued field in the key-value block, and the list-valued `domains` is the proven cause on the service side — but `ipAllowList` is required by the manifest schema (`missing property 'ipAllowList'` when dropped), so it could not be isolated by omission the way `domains` was. t002 is that work; nothing here claims the two share a root cause.
- Only `type: web` and `type: keyvalue` were probed. Static, cron, worker and private services were not exported or planned this run.
- No apply was performed. Everything above is `validateBlueprint`, a read-only query; whether a real sync writes anything for these spurious `update` actions is t004's question.
- The dashboard's own plan summary was not opened — the probes went straight to GraphQL.

## Also checked this run, and found correct

- **The generated manifest validates.** `valid: true`, `errors: []`, `errorDetails: []` — the `w4/m102` class (bex generating a manifest bex rejects) does not recur here.
- **The planner detects real changes on the path that works.** `databases:` with `plan: basic-256mb` → `noop`; changed to `basic-1gb` → `update` naming `plan`.
- **Secret handling in the export is sound.** The dialog states secrets are never exported and secret-backed variables appear as `sync: false`; the emitted manifest carried no values.

## Outcome (2026-09-21)

Three round-trip defects, not two — and all three are the same shape: **the exporter and the planner disagreeing about how one value is represented.** Self-validation (which this export already did) proves a manifest parses; it never proved it describes its own source.

**t001 — `domains`.** A manifest declares one flat list; the spec keeps the primary host in `Spec.Host` and the rest in `Spec.Hosts`, and `CreateRequest` has no `Host` field at all — so the whole declared list arrived in `Hosts` and the applier moved the domain out of `Host` on every re-plan. The fix compares the **sets** rather than the fields: when the manifest describes the hosts the service already serves, the existing split is left untouched; only a genuine change rewrites it, into the canonical shape the exporter reads back. The flattening now lives in one function (`blueprintDomainList`) that both sides call, because an exporter and a planner that each flatten for themselves is exactly how this happened.

Both host shapes are covered, and the second one matters: `AddDomain` appends to `Spec.Hosts` and never sets `Spec.Host`, so a service that gained its domains through the API has the mirror layout. A positional inversion alone would have fixed the reported case and broken that one — the set comparison is what makes both work, and the test fixture carries both (the mirror case sits on the static site, since a private service has no ingress and is refused domains outright).

**t002 — the Key Value cause, traced rather than guessed.** The filing listed `ipAllowList: []` as a *candidate* it could not isolate, because the field is schema-required and so could not be removed the way `domains` was. It is the cause, and the mechanism is exact: `slices.Clone` of a declared empty non-nil slice returns an empty **non-nil** slice, the live CR reads back **nil** (Kubernetes drops empty slices on round-trip), and `reflect.DeepEqual(nil, []T{})` is false. Verified with a standalone program rather than asserted.

That is the same class as the `domains` applier's own `Hosts` branch, whose comment had already written the reason down — so the fix generalizes it into one `canonicalSlice` helper applied to **every** list applier (`routes`, `headers`, both `ipAllowList`s, `readReplicas`, `hosts`), not just the one QA caught. The comment there names the whole family so the next list field is written correctly.

**A third defect, found by t004's guard on its first run.** A cron job's command lives in `Spec.Command`, not `Spec.StartCommand` — `SetCommands` has always known that, and so does the exporter, which writes `Spec.Command` out as `startCommand` (or `dockerCommand` under a docker runtime). The two appliers did not: they wrote every declared command into `StartCommand`, so an exported cron gained a `StartCommand` it never had while keeping its `Command`, and re-planned as `update` forever. Nothing in the filing predicted this one; the guard caught it the first time it ran, which is the whole argument for having it.

**t003 — `changedFields` is a diff now.** It listed every field the manifest *declared*, so two key-value manifests differing in `maxmemoryPolicy` produced byte-identical plans, and a `databases:` plan change reported `["name", "plan"]` — `name` being the key the resource was matched by, which cannot have changed. It is now computed by applying each declared field **alone** to a copy of the live spec and asking whether the spec moved. That deliberately derives the diff from the very appliers a real sync would run: a diff computed any other way can disagree with what apply does, which is the failure this whole milestone is about. Env groups keep the declared set and their always-`update` verdict, with the reason already in the code — the DoD said not to "fix" that into a no-op, and it is untouched.

**t004/t007 — the loop is guarded.** `TestGeneratedBlueprintRePlansAsNoop` exports a fixture covering every exportable kind (web, private, worker, cron, static, Postgres, Key Value) and fails if any resource re-plans as anything but `noop`; it asserts up front that the exported manifest actually contains the domains, so it cannot pass for the wrong reason. `TestPlanNamesOnlyTheFieldThatChanged` is the other half — perturb exactly one field, get exactly that field back, and fail if nothing is detected at all, so the loop cannot be closed by a planner that went blind. Each of the four fixes was mutation-checked by reverting it individually; each is caught.

**The DoD's remaining bullet, answered:** the filing asked whether a spurious `update` merely makes apply noisy or actually mutates. It is noisy, not destructive — the appliers are idempotent field copies and a `noop`-vs-`update` verdict only changes what the plan *reports*; the same `Apply*Spec` functions run either way and converge to the same spec. That said, the point of the fix is that a user should not have to know this: a plan that proposes to modify production resources it just described is unusable whether or not the write is harmless.

**Green:** `lego/backend` `go test ./...` all packages + `golangci-lint` 0 issues.

**Not done — the live re-probe and the dashboard check.** The DoD's bullets are `validateBlueprint` probes against real workspace resources; there was no production access this session, so none has been re-run. Also **not** checked live, as the filing flagged: whether `blueprint-plan-summary.tsx` renders `operation`/`changedFields` at all (`w6/m125` t004 recorded that it does not, only `views.ts` maps them). Since `changedFields` now carries different content, the next pass should open that panel — if it renders nothing today, this change is invisible there and that is itself worth knowing.
