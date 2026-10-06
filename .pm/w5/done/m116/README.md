# w5 · m116 — Dry-run is the real call stopped before its first write, for create and update of every kind

**Worker:** worker5 **Goal:** Every `dryRun` or preview runs exactly the real call's read-only checks (billing, protection, names, hosts, registry, quota, CRD admission) because both run the same plan step. **Status:** done — 2026-10-05

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Inventory every preview path and decide the preview relation — **DONE** | 30m | — |
| t002 | Give apps create and update one plan step — **DONE** | 1h | t001 |
| t003 | Give Postgres and Key Value one plan step and delete the Preview twins — **DONE** | 1h30m | t002 |
| t004 | End the plan with a server-side dry-run create — **DONE** | 45m | t003 |
| t005 | Render parity — **DONE** | 20m | t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 45m | t005, t006 |
| t008 | Closeout — **DONE** | 10m | t007 |

## Definition of done

- For service, Postgres and Key Value create, update and plan change, a dry-run returns exactly the real call's error, and writes nothing, for each of:
  - the count cap;
  - an unpaid plan with billing enforced;
  - a protected environment;
  - a domain held by a pending-verification host;
  - a CRD-invalid field.
- The five `Preview*` functions are gone, and every billing check goes through `RequirePlanBilling`.
- `docs/ADR006-bex-api.md` no longer says count caps aren't previewed, and the stale `apps/service.go` comment claiming `create` calls `createNewApp` is fixed.

## Source + Goal linkage

- **Source:** Last-24h code review, 2026-10-05, of w8/045 (`10827a8cd`) and w8/046 (`abe8f0eb5`). Both fixed create dry-run drift by copying checks. The update side still drifts: `PATCH /v1/services/{id}?dryRun=true {"plan":"standard"}` answers 200 where the real call answers 402. The store-mode host preview also misses pending-verification domains (preview 200, real 409).
- **Goal linkage:** ADR006/ADR018 Render-compatible `dryRun`; ADR046/ADR075 billing gates.
- **Expected outcome:** A successful dry-run means the real call passes its checks.
- **Why now:** The class recurred twice in a day, the update side is still open, and w5/m117 and w5/m118 build on the plan seam.
- **Render parity included:** dry-run answers change on REST/GraphQL/MCP.
- **Depends on w5/072:** the minimal Blueprint card-gate fix lands first.

## Inventory and decisions (t001, 2026-10-05)

What each preview skips that its real call checks before its first write, read from the code (file:line in the t001 record):

| Preview | Relation (preview / real) | Skipped today |
| --- | --- | --- |
| Service create `dryRun` | `can_create` / `can_create` | `domains` claims still pending verification; store display-name conflicts; env and secret-file size quotas; CRD-only limits (commands ≤4096, maintenance URI ≤2048, hosts ≤100); the count cap is read from `Spec.Hard`. The dry-run also records an allowed `Create` audit row, because the authorization is not deferred. |
| Service plan (`PreviewSetPlan`) | `can_view` / `can_operate` | billing (`RequirePlanBilling`); a service being deleted. |
| Service PATCH `dryRun` | `can_view` / per-row `can_operate`/`can_create` | Every field but `plan`. REST drops them, MCP refuses them, and `?confirm=` is read only after the dry-run branch, so protection guards never run. |
| Postgres create `dryRun` | `can_create` / `can_create` | CRD admission (duplicate read-replica names); the count cap is read from `Spec.Hard`. |
| Postgres plan and update (`PreviewSetPlan`, `PreviewUpdatePostgres`) | `can_view` / `can_operate` | billing; protection (rename, version upgrade); CRD admission (immutable identifiers). The real update calls `RequirePaymentMethod` (paid plans only) and `RequireBillingMutation` separately, so REST/MCP skip the all-plans payment gate GraphQL's `SetPlan` applies. `SetVersion` (GraphQL `updateDatabaseVersion`) skips both protection and billing. |
| Key Value create `dryRun` | `can_create` / `can_create` | CRD admission (version enum); the count cap is read from `Spec.Hard`. |
| Key Value plan and update (`PreviewSetPlan`, `PreviewUpdateKeyValue`) | `can_view` / `can_operate` | billing; protection (rename, durability, eviction). REST PATCH never passes `?confirm=`, so these cannot be confirmed over REST at all. |

Postgres and Key Value previews refuse a resource being deleted, and their real calls don't. Services are the reverse. Blueprint preview is out of this milestone's DoD.

**Decisions.**

- **Who can preview is unchanged,** so t001's user gate doesn't trigger. Plan and update previews keep `can_view`. Of the checks a preview gains, only billing discloses anything new: a 402 reveals payment state that otherwise needs `can_manage_billing`. Protected status and workspace limits are already `can_view`-readable. So billing gates run in a preview only when the caller holds the real verb's relation, probed without an audit row, the rule action capabilities already follow. Such a caller gets exactly the 402 the real call would give. Every other check runs for every previewer.
- **Admission judges the preview.** Each plan ends with a server-side dry-run (`client.DryRunAll`) of the exact object the real call writes: Create for creates, Patch for datastore updates. An admission `Invalid` maps to a 400 carrying only the field causes, for the real call as well as its preview. Today the real call surfaces it as a redacted 500.
- **A workspace with no namespace yet** dry-runs in the bootstrap namespace, unlabeled, which the app admission policy accepts. Every plan's count caps are at least 1 (`store.QuotaCapsForPlan`), so no cap can bind on a workspace's first resource. That retires `CheckQuotaCap`.
- **Service PATCH** has no single object to dry-run, because each row writes separately. So each row's check folds its own change into the preflight's scratch App, exactly as its verb would make it, and the preflight ends in a server-side dry-run patch of that App. A real PATCH meets an invalid field before any row lands, and a dry-run answers with the service as the patch would leave it. This replaces the first plan, which was to mirror the CRD's length limits in Go: admission judges instead.
- **One plan step per verb.** Preview and real call share it, and the five `Preview*` functions go. `SetVersion` and `UpdatePostgres` share Postgres's update plan step, and every billing check goes through `RequirePlanBilling`/`RequireBillingFor`.

## Evidence — 2026-10-05

**Plan steps (t002–t004).** Every service, Postgres and Key Value dry-run now runs its real call's plan step and returns where the real call would start writing.

- **Postgres and Key Value.** `SetPlan`/`Update*` and their `*DryRun` share one step: `fetchToChange`, the checks, then a patch that a dry-run sends with `dryRun=All`. The `Preview*` twins are gone.
  - A plan change takes `RequirePlanBilling`, so under `BEX_REQUIRE_PAYMENT_METHOD=all` a free-plan change needs a payment method on REST and MCP too; only GraphQL's `SetPlan` enforced that before. Postgres's other billed dimensions (version, disk, autoscaling, high availability) stay on the dunning gate (ADR046).
  - `SetVersion` (GraphQL `updateDatabaseVersion`) runs the update's checks, so the protected-environment "upgrade" confirmation and the dunning gate apply. It skipped both before.
  - A resource being deleted reads not-found for real calls and dry-runs alike; before, only the preview refused it.
  - Key Value's REST PATCH reads `?confirm=`; before, a protected store could not be renamed, or its eviction or durability changed, over REST at all. Both datastores' MCP update tools pass `confirm` to dry-runs.
- **Services.** `SetPlanDryRun` runs the service patch dry-run. `ApplyServicePatchDryRun` runs the real preflight: every row's check folds its change into a scratch App exactly as its verb would, and the preflight ends in a server-side dry-run patch of it. So a real PATCH meets an invalid field before any row writes, and a dry-run answers with the service as the patch would leave it. REST and MCP dry-runs check and preview every field; REST used to drop all but the plan, and MCP refused them.
- **Service create.** `planNewApp` runs before the first write, for interactive and Blueprint creates, in the order the writes meet the refusals:
  - the store name, display names included;
  - the custom-domain gate, exempting the service's own `<name>.<base>`, which store.CreateApp mints unless another workspace holds that slug;
  - hosts another service claims, verified or pending;
  - the registry credential;
  - the create-time secret limits (`CheckCreateSecrets` on the seeder seam);
  - a server-side dry-run of the exact App.

  The writes keep their constraints as race backstops. A dry-run create writes no audit row.
- **Admission.** `core.Base.DryRunCreate` validates a workspace's first resource in the bootstrap namespace. `core.Base.PatchObject` and `core.InvalidFieldsError` turn a CRD refusal into a 400 naming the fields; before, it surfaced as a redacted 500. `core.CountCap.CreateError` maps creates, and each kind defines its count-key and noun pair once. `CheckQuotaCap` is retired.
- **Live, on the local cluster** (a scratch namespace, deleted afterwards):
  - a server-side dry-run App create under a zero cap answered `exceeded quota: tenant-quota, … limited: count/apps.app.bex.co=0`, the shape `QuotaCapError` parses;
  - a 5,000-byte `startCommand` answered `spec.startCommand: Too long: may not be more than 4096 bytes`, plus a fieldless note the 400 now drops;
  - nothing persisted.

  Admission on the three CRs has no side effects: the only webhook targets labeled Pods, and the app admission policy is CEL.

**Render parity (t005).** REST, GraphQL and MCP send dry-runs to the same verbs with the same confirmation, so they answer the same errors in each surface's existing envelope. GraphQL has no whole-service patch mutation; its plan mutation's `dryRun` runs `SetPlanDryRun`. Render's public API has no `dryRun` on these writes (a bex extension). Its comparable preview is Blueprint validate, and bex's still runs its own subset of checks, filed as w5/088. The dashboard sends no dry-runs, so it needs no change.

**`/simplify` (t006)**, three reviews. Applied:

- Display-name conflicts now reach the patch preflight. One store query serves the preflight and the in-transaction refusal; before, a REST or MCP dry-run rename answered 200 where the real call answered 409.
- Postgres's non-plan billed dimensions are back on the dunning gate alone. My first cut put them on the plan gate, so a paid database's disk change needed a card.
- The create plan exempts the service's own `<name>.<base>` host, as the real create always has. The first cut refused it, on the real create too.
- `SetPlanDryRun` gains the admission step by running the service patch dry-run, through an unexported method so its audit row names its own verb.
- `PreviewOf` is lazy, so no OpenFGA probe runs unless a billing gate is reached. Its marker is unexported, it probes the object the real gate authorizes against, and both billing primitives honour it.
- `core.Base.PatchObject` replaces the `dryRunPatch` twins and three copies of the invalid-field mapping. It also maps every apps CR write, so GraphQL's single-field setters no longer answer a redacted 500 for an invalid field.
- One memoized cluster-wide App sweep per create replaces the two the plan and the write each made.
- `stampAppIdentity`, `errServiceNameInUse` and `errClaimServiceDomains` are each shared by the plan and the write.
- The source fold clears the old origin's clone token, as its setter does. Every patch row must have a check, and a test pins every fold to its verb.
- The `mapServiceCapError` alias is removed, `fetchToChange`'s argument order matches `authorizeToChange`, and stale docs are fixed.

Skipped:

- A generic core `AuthorizeToChange`: an audit row names the nearest exported method, so a core function or closure in the stack would rename every verb.
- Sharing the registry-credential resolution between plan and write: it would mean plumbing a secret through for 1–5 ms.
- Skipping the patch dry-run when nothing changed: AppSpec's CEL rules re-run on any change, and an empty patch is rare.
- `DomainHostsClaimed` taking declarations, a `kvMCPSession` helper, and the datastore `setPlan`s delegating to `checkUpdate`, which would reorder `SetPlan`'s refusals.

**Tests (t007).**

- `api/dryrun_parity_test.go`:
  - `TestDryRunAnswersExactlyTheRealCall` covers 21 cells: create at the cap, unpaid and CRD-invalid; plan change unpaid; update unpaid, protected and CRD-invalid; for each kind. The dry-run runs first and must answer the identical error with nothing persisted.
  - `TestDryRunThatPassesWritesNothing` covers nine clean dry-runs.
  - `TestDryRunShowsBillingOnlyToACallerWhoMayPerformTheCall` uses the model-pinned viewer role.
- `apps`:
  - `TestPGCreateDryRunMeetsTheStoreRefusals` (real Postgres) covers a pending-verification host, a displayed name, and a rename to one.
  - `TestServicePatchFoldsMatchTheirVerbs` pins each of the 21 rows' fold to its verb.
  - `TestCreateExemptsTheServicesOwnPlatformHost`, and the MCP whole-patch dry-run.
- `core`: `TestInvalidFieldsErrorNamesTheFields` against the live status, and the bootstrap-namespace fallback.
- `postgres`: `SetVersion`'s guard and dunning gate, the plan gate's rules, and deleting resources.
- Key Value and Postgres `confirm` on REST and MCP.

Every new test was mutation-checked: each fix reverted fails its test. That includes a dry-run patching for real, admission skipped, billing shown to viewers, the claimed-host and display-name reads dropped, the fallback's label and name rules, folds drifting, confirm dropped, and `SetVersion` without its checks.

Replaced: the three `dryrun_quota_test.go` files, the two `TestDryRunCreate*RunsTheBillingGate` tests, and the three `atCapQuota` copies.

**Docs.** ADR006's dry-run section is rewritten, the verb names in ADR018 and ADR030 are updated, and the `createNewApp` comment is fixed.

**Deviation from the DoD's wording.** Every plan-bearing billing check (create, plan change, an update carrying a plan) goes through `RequirePlanBilling`. A Postgres update's other billed dimensions keep the dunning gate alone, as apps' disk and scale changes do: ADR046 counts only a create or plan change as paid intent. My first cut sent them through the plan gate too, and the review caught that it would have asked a paid database for a card on every disk change.

**Gates.**

- The full backend suite on fresh Postgres 17, OpenFGA and OpenBao is green (68 packages). The first run caught two things. The events vocabulary guard needed the six dry-run verbs excused (read-only `can_view`, they write nothing). And the Blueprint lifecycle test's emulated API server treated the plan's dry-run Create as a real one; it now honours `dryRun` as a real API server does.
- `make lint`: 0 issues in all four modules.

## Live check — 2026-10-06 (w4 `/qa-find-bugs` loop66, pin `a5011ed13`)

`POST https://api.bex.co/v1/services` (`bex-canary`, Free `traefik/whoami` image body), each case with and without `?dryRun=true`. Status and message were identical in every pair:

- name `hello-go` (in use) → **409** `CONFLICT` "name \"hello-go\" is already in use";
- `port: 70000` → **400** "port must be 1024-65535 — …";
- `healthCheckPath: "no-slash"` → **400** "health check path must start with /";
- name `Qa Bad/Name!!` → **400** "name must be a DNS label of 1-30 chars ([a-z0-9-])".

A valid dry run (`qa-20261006-l66-ok`) → **200** preview, and the service list afterwards contained no `l66` service. Not exercised: the count cap, unpaid-plan billing, a protected environment, a pending-verification host, Postgres/Key Value dry runs and GraphQL/MCP. Side note: the valid preview's `id` is the requested name (`"id":"qa-20261006-l66-ok"`), not a `srv-` id; a client that treats `id` as a resource id should not use it.
