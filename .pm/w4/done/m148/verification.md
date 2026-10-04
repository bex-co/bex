# Complete first Blueprint deployment — verification

## Protocol and scope

New Blueprint services carry group names through a private create-request field.
The authorized envgroups seam resolves them in the acting workspace, checks
fresh sensitive permission and environment compatibility, reserves membership
with CAS, and adds ordered environment/file references before the first App CR
write. Service-local values retain precedence. Post-create revalidation fences
permission, scope, deletion and membership-pruning races.

The database row starts with internal `creation_pending=true` only for this new
grouped-create path. Migration 0137 defaults existing and ordinary rows to false.
The projector retains pending rows in its deletion inventory but cannot dispatch,
rewrite or observe their incomplete configuration. Completion clears the flag
after composition. An abandoned initialization times out with a truthful failure
without automatically unlocking the configuration; a process crash can require
operator recovery of the incomplete creation.

On failure, bounded cancellation-independent compensation removes only owned
membership additions and the created resources. Durable intent is removed
before deleting the exact owned App, preventing a projector fallback after an
ambiguous completion response. CAS retries track the successful attempt rather
than claiming another writer's reservation. Unrelated concurrent links and
metadata survive cleanup. Generated clone/pull credentials are removed only
after the fresh durable identity is removed or this request created the App;
shared group projections and unrelated credentials survive both preparation and
completion failures. Cleanup captures each generated Secret UID and resource
version before releasing the durable name reservation; delete preconditions
preserve a same-name replacement, including an upsert that retains the UID.
Seed compensation occurs before releasing the name reservation.

Existing-service link/unlink/content updates retain their auto-deploy policy.
Both ordinary and Blueprint create tails keep their release-generation stamp,
literal/file preparation and rollback. All five App kinds use this creation
path; managed Postgres and Key Value use separate appliers. Direct REST/MCP
deploy, connected create, manual sync and automatic sync share the executor.
The deferred host-reference pass and seed-once paths remain separate mechanisms.

## Behavioral evidence

An isolated PostgreSQL 17 container uses the same pinned image as backend CI.
The lifecycle regression uses real PGStore rows, the real envgroups service,
Secret projections and `Reconciler.ReconcileOnce`. Kubernetes generation
increments and operator status observations are controlled explicitly; no
backend-to-operator import is introduced. This is not a claim of real build Jobs
or HTTP serving.

Reverting only the initial group assignment through a Go overlay reproduces the
original failure: both declared-new and existing-group cases publish two specs,
the first lacks group references, and the returned generation-1 deploy is
Canceled as superseded. The literal control passes. With the fix, each grouped
case publishes one complete initial generation and its original deploy reaches
Live. A projector pass between database identity creation and CR creation
publishes nothing while the initialization flag is set.

Seven lifecycle cases pass, including genuine supersession/cancellation,
canceled-request compensation and completion that commits but returns an error.
The latter invokes the real projector at App deletion and verifies no partial
replacement is published. The full store suite also passes against real
Postgres. Additional store tests verify persisted/read-back flags, repeated
completion, concurrent deletion refusal, pending CR preservation and abandoned
initialization timeout.

The five App kinds with auto-deploy on/off preserve initial ordered group and
file references. Independent security/failure tests cover write-only OAuth,
fresh revocation before idempotency, workspace/environment mismatch, missing or
unversioned storage, CAS races, concurrent metadata, delete/scope changes,
post-commit locator errors and cancellation-independent cleanup. The full
envgroups suite passed after the CAS correction.

## Parity and review

REST, GraphQL and MCP deploy reads share the deploy service; the dashboard reads
its recorded status directly. No badge or adapter workaround is needed.
[Render environment-group docs](https://render.com/docs/configure-environment-variables)
describe linking, scope and service-value precedence; the
[Blueprint specification](https://render.com/docs/blueprint-spec) defines
envVarGroups/fromGroup. Authenticated Render first-create history was not
measured. Existing Bex deterministic ordering and auto-deploy-off policy remain
explicit differences, recorded in ADR049/ADR018.

Three simplify reviews completed. Cleanup reused the existing removeString and
row-rollback helpers, and one redundant test flag was removed. Repeated fresh
checks protect distinct races; no extra abstraction or operator dependency was
added. The unrelated superseded-event SQL constraint failure found by the
negative control is inbox 175, separate from this fix.

## Final checks

The complete backend suite passed with `GOWORK=off go test -p 2 ./... -count=1`.
The API reflection sweep explicitly excludes only the internal unpublished-App
callback; its public verb count remains 342 and dedicated sensitive-authorization
tests cover the callback. The full API suite also passed independently (13.024s).
Backend lint passed, and all seven real-Postgres lifecycle cases passed (0.888s).
The full store suite also passed against the isolated Postgres fixture. After the last seed-compensation ordering adjustment, the full apps package
passed again (5.479s), backend lint reported zero issues, and the focused
ownership review found no remaining issue.

An initial compiler/linker run exhausted local disk space before completing.
Evicting disposable Go build cache recovered space; the completed rerun used
package parallelism 2. No source or shared development stack was removed.
The isolated Postgres container, its anonymous volume and private connection
file were removed after verification.

## Remaining acceptance

The release pipeline must deploy the backend and migration. QA must then run
both native Go grouped creates and the literal control, verify real build
counts, external MESSAGE responses and the same first deploy through all API/UI
reads, then delete the three services/group and revoke its session. No hosted
fixture was created in this implementation run. Connected Git and runtime-family
claims above are source/local checks, not unperformed production probes.

## t002 shared-path evidence matrix

Paths below are repository-relative. “Existing test” identifies a regression
included in the backend suite; operator tests are identified as retained guards,
not as newly executed runtime acceptance. No row claims hosted verification.

| Scope | Concrete source / test evidence | Boundary of the evidence |
| --- | --- | --- |
| Direct REST and MCP apply | `apps/rest.go` registers `POST /v1/blueprints/deploy`; `apps/mcp.go` registers `deploy`; both call `DeployStack` → `deployParsedStack`. Existing `TestRESTDeployBlueprintUsesStackCore` (`apps/blueprint_test.go`) and `TestMCPDeployReportsEveryKind` (`apps/blueprint_env_test.go`). All backend paths here use prefix `lego/backend/internal/`. | Adapter/core wiring and local result coverage; there is no direct GraphQL DeployStack verb. New grouped first-deploy PG regression calls the shared service, not each transport. |
| Connected create and manual/automatic sync | `apps/blueprint.go`: `CreateBlueprint` and `runSync` call `deployParsedStack`; `apps/blueprint_auto_sync_worker.go` calls `runSync`. REST `create/sync`, GraphQL `createBlueprint/syncBlueprint`, MCP `create_blueprint/sync_blueprint` converge there. Existing `TestSyncBlueprintReappliesManifest`, `TestGraphQLSyncBlueprint`, `TestBlueprintAutoSyncPreservesTenantContext` and `TestBlueprintAutoSyncScopedMatchingIgnoresForeignResources` (`apps/blueprint_test.go`). | Shared executor and tenant isolation verified locally; no connected Git first-group lifecycle was separately exercised in this run. |
| Filename aliases | Existing `TestBlueprintFilenameDiscovery`, `TestBlueprintExplicitPathAllowsOnlyBlueprintFilenames`, `TestPreviewBlueprintWarnsOnImplicitLegacyFilenameFallback` (`apps/blueprint_test.go`). | `render.yaml` and legacy `bex.yml` discovery precede the same executor; filename choice does not select a separate composition protocol. |
| Web, static, cron, worker, private | New `TestInitialGroupsExistOnFirstAppWrite` (`envgroups/initial_links_test.go`) enumerates all five `AppSpec.Type` values and both auto-deploy settings, checking first-write ordered refs, unchanged service-owned values and no subsequent group-link rollout. `apps/service.go`: `createNewApp` → `materializeNewApp` → `writeInitialApp`. | Real envgroups/fake Kubernetes seam coverage for all five kinds. New PG lifecycle coverage is native Go web only; the kind matrix does not run builders or pods. |
| Postgres and Key Value | Separate `applyDatabase` / `applyKeyValue` through `applyStackDatastores` in `apps/deploy.go`. Existing `TestDeployStackAppliesDatabasesFirstThenServices`, `TestDeployStackKeyValueProvisionedAndValidationPlan` (`apps/stack_test.go`) and `TestDeployStackResultReportsEveryKind` (`apps/blueprint_env_test.go`). | Separate unchanged datastore appliers; they do not enter App initial-group composition. |
| Native versus buildpack | New `TestPGBlueprintInitialGroupDeployLifecycle` (`apps/blueprint_initial_group_lifecycle_pg_test.go`) uses native Go. `lego/operator/internal/controller/release_identity.go` includes group refs in artifact identity for both native and buildpack. Retained guards: `TestProjectNativeBuildEnvMergesGroupsAndOwn`, `TestNativeArtifactIdentityIncludesEnvFromSecrets` (`native_env_secret_test.go`), `TestBuildpackEnvSourcesAreArtifactIdentity` (`buildpack_env_test.go`). | Native initial identity/projector behavior is dynamically checked with real PG and controlled generation/status. Buildpack compatibility is shared-seam/source plus existing operator-guard evidence, not a new buildpack deployment. |
| Docker and prebuilt image | `writeInitialApp` and `WithInitialEnvGroups` do not branch on builder/runtime; `release_identity.go` includes group refs in release identity for all builders. Existing `TestGroupWriteStillDeploysAnImageBackedService` (`envgroups/autodeploy_gate_test.go`) and `TestDeployStackDisabledImageAutoDeploy` (`apps/image_autodeploy_test.go`). | The initial group protocol applies independently of artifact construction. These tests protect image auto-deploy semantics, not a new Docker/image grouped first-deploy PG or serving test. |
| Existing group writes, auto-deploy off/on | `envgroups/service.go`: `autoDeployGated` / `rollLinked` unchanged. Existing `TestGroupWriteSkipsTheDeployOnAnAutoDeployOffService` checks both on/off services; `TestLinkAndUnlinkSkipTheDeployOnAnAutoDeployOffService` checks link/unlink. Retained operator guard `TestLinkedServicesSnapshotASharedGroupSeparately` (`release_config_snapshot_test.go`) covers per-service snapshots. | Local restart/gating/result behavior is covered. Runtime retention of an already served snapshot remains the existing operator contract, not a live group-update probe in this run. |
| Deferred `fromService.host` | `apps/deploy.go`: `resolveServiceRefs` → `patchDeferredStackServices` → `applyBlueprintCreate`. Existing `TestDeployStackFromServiceHostResolvesToTheSlug` (`apps/stack_test.go`) exercises a forward reference and repeat-apply uniqueness; `TestDeployStackChangedServiceRedeploys` protects tracked changed apply. | Deferred host refs remain a separate second, tracked apply. Correct slug/value and repeat behavior are tested; no new complete first-build/terminal lifecycle test combines forward refs with groups. |
| `sync:false` / `generateValue` seeds | `apps/deploy.go` calls `SeedEnvVars` after create. `secrets/service.go`: `SeedEnvVars` → `materializeEnv` → `rollApp` → `Rollout.Patch` with `TriggerConfigChange`. Existing `TestBlueprintFiveFieldEndToEnd` (`apps/blueprint_e2e_test.go`), `TestSeedEnvVarsSeedsOnce` and `TestSeedEnvVarsAllPresentIsNoOp` (`secrets/generate_test.go`). | Real feature services verify seed-once values, preservation after a mutable edit and no-op reapply. Source tracing establishes a tracked rollout instead of the gated untracked group patch; no new seed-plus-group terminal lifecycle is claimed. |
| Ordinary create, service-owned env/files | Both ordinary create and Blueprint create still call `materializeNewApp` / `writeNewApp`; only private `initialEnvGroups` opts into composition. Existing `TestPrepareCreateEnvVarsSeedsStoreAndFirstAppRevision` (`secrets/files_test.go`); new `TestInitialGroupCompletionFailureCompensatesAppAndOwnSecrets` (`apps/blueprint_initial_group_compensation_test.go`). | First-write projection and compensation are exercised without exposing raw Kubernetes Secret names in public input. |
| Deploy reads and dashboard | `deploys/service.go` `List`/`Get` back REST (`rest.go`), GraphQL (`graphql.go`) and MCP (`mcp.go`). `dashboard/src/features/deploys/hooks/use-latest-deploy.ts` reads the newest recorded deploy; `service-detail-header.tsx` renders that ID/status. | Shared-reader source audit; no UI workaround or status reinterpretation. Real API/UI read agreement remains a hosted acceptance step. |
