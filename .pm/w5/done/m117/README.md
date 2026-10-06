# w5 · m117 — Every create field reaches the service through REST, GraphQL and MCP, and Blueprint export round-trips

**Worker:** worker5 **Goal:** A guard test fails whenever a transport silently drops or mis-maps a create field, and Blueprint Generate → parse reproduces what a real create stored. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Give GraphQL service create its own mapping function — **DONE** | 30m | — |
| t002 | Guard that every create field is used or refused per transport — **DONE** | 1h | t001 |
| t003 | Fix whatever the guard finds — **DONE** | 45m | t002 |
| t004 | Drive the Blueprint round-trip from real creates — **DONE** | 1h | t001 |
| t005 | Render parity — **DONE** | 20m | t003, t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 30m | t005, t006 |
| t008 | Closeout — **DONE** | 10m | t007 |

## Definition of done

- For REST, GraphQL and MCP × runtime (docker, image, native, builder-only), every service-create wire field reaches `CreateRequest` or is refused with 400, and adding an unmapped field fails the guard.
- Create → GenerateBlueprint → parse → `specFromCreate` reproduces every exportable field for the probe set and the type × runtime × builder matrix; non-exportable fields are listed.
- Reverting w4/188 or w4/193 locally makes the respective guard fail.

## Source + Goal linkage

- **Source:** Last-24h code review, 2026-10-05. w4/188 (`4fe79515b`) fixed REST dropping `dockerCommand`, but MCP still drops it (w5/073). w4/193 (`24482892a`) fixed Generate Blueprint for plain Dockerfile services, after w6/m114 had shipped a similar bug past the hand-built round-trip test.
- **Goal linkage:** ADR006/ADR018 Render-compatible API; ADR049 render.yaml parity.
- **Expected outcome:** No transport silently loses a field, and Generate Blueprint exports what was created.
- **Why now:** Two transport/export bugs in 24 hours, and w5/m116's plan seam makes the guard cheap.
- **Render parity included:** this milestone is a parity guard for create and export.
- **Depends on w5/073** (MCP `dockerCommand`); best done after w5/m116.

## Evidence — 2026-10-05

**Transport guard (t001–t003).**

- Each create transport's wire-to-request step is now one function:
  - REST's `decodeCreateService`: decode, the `previews` refusal, `?dryRun`, then `toCreateRequest`.
  - GraphQL's `gqlCreateRequest`, extracted from the resolver with no behavior change.
  - Each MCP tool's `createRequest`: the tool's refusals, the call's workspace, and Render's two allowlist spellings. `createWithAllowList` is gone.
- `TestCreateWireFieldsReachTheRequest` enumerates every field a transport accepts from its request type (JSON tags, list elements and hand-decoded fields included) or its schema (graphql-go arguments and input objects).
  - It drives each field on 15 create shapes: web and cron × the docker, image, bare-image, native, `builder: dockerfile`, buildpack and auto builds, plus a static site. Each shape is a create `specFromCreate` accepts.
  - A field must change exactly the `CreateRequest` fields its rule names, answer 400, or be named inert on that shape with the reason.
  - GraphQL runs through graphql-go's own coercion, and MCP through the tool's inferred input schema, as the SDK checks a real call.
  - A field without a rule fails, and so does a rule no field uses.
- What it found, fixed in t003:
  - REST dropped a docker web service's `envSpecificDetails.startCommand`; only a cron kept it (w9/m165). PATCH, GraphQL and MCP always kept it.
  - REST dropped `dockerCommand`, `dockerContext` and `dockerfilePath` when a create named no runtime (bex's `builder: dockerfile` or auto build). MCP dropped `dockerContext` and `dockerCommand` there.
  - The fix is one rule for every transport and service type, `dockerDetails` in `build_strategy.go`:
    - a Dockerfile build reads all three;
    - a prebuilt image, with or without `runtime: image`, reads only the command;
    - a native runtime and buildpacks read none, as on Render.
  - `TestRESTCreateDockerWebServiceIgnoresNativeStartCommand` (w9/m165) pinned the drop as "unchanged behavior". Renamed, it now pins the start command kept.
- The guard lists three kinds of field as deliberately inert, each with its reason in the rule table:
  - Render's `serviceDetails.region` (bex is single-region);
  - `serviceDetails.env` beside a named runtime (Render's deprecated spelling);
  - Render's docker details where the build does not read them.

  Deviation from the DoD's "reaches CreateRequest or is refused with 400": Render compatibility keeps these inert rather than refused (ADR006).

- Mutation checks:
  - making images ignore `dockerCommand` (reverting w4/188 and w5/073) fails 8 cases;
  - dropping docker details on no-runtime builds fails 23;
  - an unmapped new MCP field fails as having no rule;
  - dropping GraphQL's `preDeployCommand` fails on every shape.

**Blueprint round trip (t004).**

- `TestGenerateBlueprintRoundTripsRealCreates` drives 73 creates through Create, Generate Blueprint, the parse a sync applies, and `specFromCreate`. They are the create probes, every type × runtime × builder combination create accepts, and four extras for values the probes leave at their defaults.
  - Every stored field must come back, apart from spellings the export rewrites by design (`exportSpelling`) and the gaps `blueprintNotExported` lists with where and why.
  - Syncing each export back must re-plan as `noop`.
- What it found, fixed:
  - The export dropped `buildFilter`, `maxShutdownDelaySeconds`, `maintenanceMode`, `ipAllowList`, `routes`, `headers`, `disk` and a disabled `renderSubdomainPolicy`, all render.yaml fields the parse already reads.
  - A buildpack service's export answered 503 `BLUEPRINT_GENERATE_FAILED` ("missing property 'runtime'"). It now exports as `runtime: docker` with `x-bex.builder: buildpack`, and the parse drops that runtime beside a buildpack builder.
  - A Dockerfile-built static site exported as a no-build site. It now carries `x-bex.builder: dockerfile`.
  - A Dockerfile or buildpack build with a stored `buildCommand` also answered 503 ("buildCommand requires a native runtime"). Only a native build exports its build command now.
  - Since w4/193 the export writes a service's effective runtime, but the planner compared the raw field. Syncing the export of any plain Dockerfile service or bare image therefore re-planned as `update` (35 of the 73 creates). The planner now judges runtime and builder together as one build strategy (`buildStrategy`), after every other field, and keeps an omitted builder.
- Not exported, listed with reasons:
  - the port and `notifyOnFail`, which render.yaml has no field for;
  - an autoscaled service's manual instance count;
  - a static site's native runtime and its start command;
  - a build command that a non-native build never runs.
- The registry-credential binding is not exported either. The round trip runs without a credential store, so it is filed as w5/089.
- Mutation checks:
  - reverting w4/193 (the export writes `spec.runtime`) fails 13 creates;
  - comparing runtime spellings in the planner fails 35 re-plans;
  - keeping `runtime: docker` beside buildpack fails 5;
  - exporting every build command fails 3.

**Render parity (t005).**

- Each surface keeps its own create field set, now pinned per transport:
  - REST is Render's `servicePOST` plus bex extensions, with `previews` refused and `region` normalized;
  - MCP tracks Render's MCP tools plus `image`, `builder` and the docker arguments;
  - GraphQL's `createService` is a bex extension with a narrower set: no `domains`, `dockerCommand`, `dockerContext`, `renderSubdomainPolicy` or `disk` at create. The dashboard sets those after create through their own mutations.

  The GraphQL difference is recorded as a divergence, not drift; no follow-up.

- Render compatibility of the transport rule:
  - The pinned CLI sends docker details only for `runtime: docker` (with its registry credential) and native details otherwise, so no CLI-shaped create changes meaning.
  - Docker details stay inert on a native runtime, as Render's docker-or-native `envSpecificDetails` and upstream MCP's "Applies when runtime is docker" have them.
  - The new mappings, a docker web service's `startCommand` and docker details with no runtime, cover only inputs Render clients never send.
- Generate Blueprint is one Core verb behind REST `POST /v1/blueprints/generate`, GraphQL `generateBlueprint` and MCP `generate_blueprint`, so the export changes reach all three byte-identically. The dashboard renders the returned manifest as is.
  - Every new field is a render.yaml field except `x-bex.builder`, ADR049's namespaced extension. It is emitted only for the two build strategies render.yaml cannot express.
- Docs: ADR006 gained a paragraph on create transports. It also had a stale sentence: service IP allow lists and a Docker source-build registry credential no longer answer 400, both are supported now. ADR018's Blueprint row is updated.
- Follow-ups:
  - w5/089: export the registry credential;
  - w5/090: create should refuse build settings its build never reads, as update does.

**`/simplify` (t006)**, three reviews. Applied:

- One `dockerDetails` type, with `buildStrategy`, in the new `build_strategy.go`. It replaces REST's docker branches and MCP's `resolveMCPDockerArgs`, `checkMCPDockerArgs` and `mcpDockerArgs`. It also replaces my first cut's runtime-only predicate, which treated buildpack and bare-image creates as Dockerfile builds.
- The quality review caught a regression in my first planner cut. Grouping runtime and builder cleared an omitted builder, so a hand-written sync of a Dockerfile-built static site would have turned it into a no-build publish.
  - The comparison now runs after the other fields and keeps an omitted builder.
  - `TestBlueprintSyncKeepsAnOmittedBuilder` pins it; reverting fails both its cases.
- `buildStrategy` treats a static site with a Dockerfile path as a Dockerfile build, as the operator does.
- The parse fold covers buildpack only, through one named `blueprintBuildpackRuntime` that both the export and the parse use. `runtime: docker` beside `x-bex.builder: dockerfile` is still refused, as before.
- MCP: the dead `OwnerID` fields are gone, and so is the allowlist `toCreateRequest` built only to discard. The agent-facing descriptions of the docker arguments state the new rule.
- The export reuses `serviceDiskView`, and `allowListEntries` is shared with the datastore export.
- Tests:
  - one generic `changedFields` (Kubernetes semantic equality) serves both new tests;
  - the round trip uses `parseBlueprintStackForTest` and `strconv.Itoa`;
  - one build table spells every transport's shapes;
  - MCP arguments pass the tool's input schema, with schema-required siblings filled.

Skipped:

- A generic `addCreateTool[A]` for the three five-line MCP handlers, and a helper for the two allowlist resolutions.
- Typed runtime and builder constants in `lego/types`. The codebase spells them as literals throughout; this diff adds build-strategy constants only.
- Holding the rule map on the transport rather than a table name. The name is what lets one stale-rule check span the three MCP tools.
- Efficiency: nothing to change. Both new tests together add about 0.3s.

**Tests (t007).**

- New: `TestCreateWireFieldsReachTheRequest`, `TestGenerateBlueprintRoundTripsRealCreates` and `TestBlueprintSyncKeepsAnOmittedBuilder`.
- Updated:
  - the w9/m165 docker web test;
  - the export self-check test. Its buildpack trigger is exportable now, so it stores a runtime the CRD enum would refuse; no App that a create or the CRD accepts is known to fail the check any more.
- Every fix is mutation-checked, as listed above.

**Gates.**

- The full backend suite on fresh Postgres, OpenFGA and OpenBao is green (68 packages); `/ship`'s gate runs it again before the push.
- `make lint`: 0 issues in all four modules, including the whole-program dead-code pass.
