# w5 · m126 — Blueprint preview runs each declared resource's create plan

**Worker:** worker5 **Goal:** A Blueprint preview (Render's validate) refuses what its apply would refuse before the first write, because each new resource's preview runs the plan its apply runs. **Status:** done — 2026-10-06

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Inventory the apply's refusals the preview skips, and decide each one — **DONE** | 30m | — |
| t002 | Preview each new service through `planNewApp` — **DONE** | 1h | t001 |
| t003 | Give stack Postgres and Key Value creates one plan step, and preview through it — **DONE** | 1h15m | t002 |
| t004 | Preview the stack-wide count cap — **DONE** | 1h | t003 |
| t005 | Render parity — **DONE** | 20m | t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 45m | t005, t006 |
| t008 | Closeout — **DONE** | 10m | t007 |

## Definition of done

- A Blueprint declaring one new resource per refusal class gets each refusal from preview, located at its resource, with the code its apply refuses with. The classes:
  - the count cap, counted across the whole stack;
  - a store name or display-name conflict;
  - a host another service claims;
  - an unknown registry credential;
  - a CRD-invalid field;
- A preview writes nothing and records no allowed audit row.
- `fromGroup`, the protected-environment confirmation and the payment gate stay apply-only, as ADR018's Blueprint row records.
- ADR018 and ADR049 say which checks a preview runs.

## Source + Goal linkage

- **Source:** promoted from inbox w5/088, found in w5/m116's t001 inventory: Blueprint preview was outside that milestone's DoD, and it still runs its own subset of checks. Skipped today (`apps/blueprint.go` `blueprintValidationFor` vs the apply in `deploy.go` `deployParsedStack`):
  - the stack billing gate (`requireStackBilling`);
  - the plan's count caps;
  - store name and display-name conflicts;
  - the protected environment's `deploy` confirmation on changed services (`applyCreateWithFields`);
  - the registry credential, resolved only from name to id.

  Stack Postgres and Key Value creates (`applyDatabase`, `applyKeyValue`) build and create their objects directly, without the plan steps interactive creates use.
- **Goal linkage:** ADR006/ADR018 Render-compatible validate and dry-run; ADR049 `render.yaml` parity; ADR046/ADR075 billing gates.
- **Expected outcome:** A Blueprint that previews clean applies clean, short of races and external state. The dashboard's create review and pre-sync dialog stop letting a user find refusals by pressing Deploy.
- **Why now:** w5/m116 made every single-resource dry-run exact. The Blueprint preview is the last preview that runs its own subset.
- **Render parity included:** the preview's errors change on REST, GraphQL and MCP, and in the dashboard's Blueprint review.

## Inventory and decisions (t001, 2026-10-06)

What a Blueprint apply (`deployParsedStack`) refuses before or at each resource's first write, and whether preview (`blueprintValidationFor`) returns it:

| Refusal | Where the apply meets it | Preview today |
| --- | --- | --- |
| Registry credential name → id | `resolveBlueprintRegistryCredentials` (stack preflight) | same call |
| Service spec, maintenance mode, custom-domain gate | `validateBlueprintServices` (stack preflight) | same call |
| Resource owned by another Blueprint | `preflightBlueprintOwnership`, then `claimBlueprintResourceName` per resource | `previewOwnershipConflicts` |
| Unpaid paid plan | `requireStackBilling` (stack preflight) | apply-only (decision 1) |
| `fromDatabase` / `fromService` → Key Value | `resolveExistingBlueprintReferences` | `validateWorkspaceReferences` |
| `fromGroup`, env-store wiring | `preflightBlueprintEnv` | deferred by design (uncached `can_view_sensitive`) |
| Database admission (`CheckDatabaseAdmission`) | `applyDatabase` | `checkBlueprintDatabaseCreates` / the plan |
| Datastore display name used twice | `applyDatabase` / `applyKeyValue` | checked in t003 |
| Datastore CRD rules and count cap | `Client.Create` in `applyDatabase` / `applyKeyValue` | **skipped** |
| New service: store name and display names, claimed hosts (pending included), registry credential existence, create-time secret limits, CRD rules and count cap | `createFromStack` → `planNewApp` | **skipped** (the host gate alone runs) |
| Count cap across the stack | the k-th create's admission | **skipped**: a per-resource dry-run sees today's count |
| Changed service in a protected environment | `applyCreateWithFields` → `requireUnprotected(…, "deploy")` | apply-only (decision 5) |

**Decisions.**

1. **Billing stays apply-only** (revised during t004). The plan was to run `requireStackBilling` in preview under `core.PreviewOf`. But the dashboard resolves a 402 through payment onboarding, which `paymentGate.run` opens from the create's or sync's refusal. As a preview error it would disable Deploy (`blueprints.new.tsx`) and Sync (the reviewed pin needs a valid preview), so a user without a payment method could never reach onboarding. Agents get the apply's coded 402, which names `create_billing_checkout_session`. This is decision 5's reasoning.
2. **New services** run `planNewApp`, the plan their apply runs (t002).
3. **New datastores** get one plan step shared with the apply, ending in `DryRunCreate` with the interactive creates' count-cap mapping (t003).
4. **Count caps** are checked across the stack: the workspace's `tenant-quota` enforced hard and used counts, against the resources the stack creates. The first resource of a kind past the cap is refused with the message its create would get (t004). A workspace with no quota yet has none to bind; every plan's caps are at least 1, so each create's own dry-run still covers the first resource.
5. **The protected-environment confirmation stays apply-only,** like `fromGroup`. It authorizes the apply rather than refusing the manifest, and preview has no confirmation to carry. As an error it would mark the preview invalid and disable the dashboard's Deploy, whose refusal is what opens the confirmation dialog. ADR018's Blueprint row says so (t005).

## Evidence — 2026-10-06

**Preview (t002–t004).** `blueprintValidationFor` serves validate (REST, GraphQL, MCP) and preview. For each resource the current-state plan would create, it now runs the plan that create runs before its first write (`apps/blueprint_preview_creates.go`):

- **New services** run `planStackApp`, the plan half of the stack create (`createFromStack` = memo + `planStackApp` + `materializeNewApp`). Its checks are the store name and display names, the host gate and claimed hosts, the registry credential, create-time secret limits, and a server-side dry-run of the exact App.
- **New Postgres and Key Value** run the plan step the apply now shares (`planStackDatabase`, `planStackKeyValue`), dry-run through admission with the interactive creates' cap mapping.
- **The count cap across the stack** (`previewStackCountCaps`) reads the workspace's `tenant-quota` only when some kind has two or more creates. It refuses the first resource of a kind past the room.

Each refusal is located at its resource and carries the apply's message and `CodedError` code. A claimed host now names the host (`CUSTOM_DOMAIN_IN_USE: "<host>"`), so it is located at its `domains[i]` entry; the interactive create's message improves with it. One request memo serves the whole validation, so the cluster-wide App sweep runs once rather than about twice per new service.

**Apply fixes found on the way.**

- A Blueprint's Postgres and Key Value creates now share the interactive creates' tail (`postgres.CreateResource`, `keyvalue.CreateResource`). That tail maps the cap and invalid fields to the API's terms, where the stack apply used to pass Kubernetes' raw admission error through. It reports an id collision as the documented conflict. And it ensures the workspace namespace first: datastores apply before services, so a new workspace's first Blueprint with a database could fail with a namespace NotFound 500 (w2/026's class).
- `postgres.CountCap` and `keyvalue.CountCap` are exported, and `core.CountCap` gains `Room` and `Exceeded`.

**Decisions revised in flight.** Billing stays apply-only (decision 1), as do the protected-environment confirmation and `fromGroup`. ADR018's Blueprint row and ADR049 D4 record which checks validate and preview run.

**Tests.** `internal/api/blueprint_preview_parity_test.go` runs on the w5/m116 admission emulator, which now counts per quota key.

- A nine-case matrix. Each case's preview refusal equals the apply's message, code, and pinned path:
  - a service at the cap;
  - a service with a CRD-invalid field;
  - a display-name conflict;
  - a claimed host;
  - a Postgres at the cap;
  - a Postgres with a CRD-invalid field;
  - a Key Value at the cap;
  - a Key Value with a CRD-invalid field;
  - a registry credential that cannot pull the image.
- The stack-wide cap with three services: preview refuses the second, and the apply creates one and refuses the second.
- Billing is left to the apply.
- A passing preview writes nothing and plans its three creates.
- A Blueprint's datastore creates ensure the namespace.

Mutants are caught: preview creates off, code propagation off, the apply's cap mapping off, an unresolved credential, the wrong resource past the cap, no stack cap, billing in preview, and a direct create. The apps, api, postgres, keyvalue and core suites pass, and lint is clean.

**Not done.**

- A service's CRD refusal is located at the resource, not the field: `blueprintErrorField`'s keyword list has no `startCommand`. That is an existing heuristic.
- `materializeNewApp` still takes four parameters rather than the plan struct.
- The create plans call no `Authorize`, so they record no audit row. That holds by construction; no test asserts it.

**Follow-up.** w5/105: two new services in one manifest declaring the same host preview clean. The apply creates the first and refuses the second.
