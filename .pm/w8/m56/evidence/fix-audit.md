# w8/m56 fix audit (2026-10-10)

Source-level record for t001, t002, t004, t005 and t006. Live replay evidence (t003) is recorded separately after deployment.

## Mechanism (t001)

`ApplyServicePatch` (`lego/backend/internal/apps/settings.go`) now runs every present row's unchanged setter against `staged := s.withClient(stage)`. `stage` is a `rollout.Stage` (`lego/backend/internal/rollout/stage.go`), a `client.Client` scoped to the one App the preflight read.

- A read of that App returns the staged copy (`Get`, `List`). A spec-changing write to it (`Patch` merge/JSON, `Update`) is applied to the staged copy only.
- Dry-runs, status writes and every other object pass through unchanged. A write that leaves the spec alone also passes through, and the stage rebases onto the server's answer. This keeps the source row's pending-source marker durable before `Store.SetAppSource`, as `SetSourceAndRegistryCredential` requires.
- `Stage.Publish` writes the accumulated change once through `rollout.PatchAppLocked`, under an optimistic lock. A lost race re-reads, re-applies this request's merge diff on top of the winner, and re-stamps the release and pending-source generations against the new read.
- `refile` re-points the held `Batch` row at the generation the one write produced. A failed publish drops the held row (`drop`).
- After an unexpected mid-table failure, the rows before it are still published as one write, and the request returns the failure. This is the truthful partial outcome; no cross-store rollback was invented.

Annotation-only deferral was not used: the release annotation is still stamped by each setter's `Tracker.Patch`, and only the spec-bearing publication reaches the operator.

## Ownership census (t002)

Each row is listed with its setter, its write path, and what it owns.

| Row (fields) | Setter → write path | CR / store ownership |
| --- | --- | --- |
| DisplayName | SetDisplayName → writeThroughStoreFetched → patchFetched | store row + CR spec |
| Repo/Image/ImageOwnerID/Branch/RegistryCredentialID | SetSourceAndRegistryCredential → marker Patch (pass-through) → Store.SetAppSource → patchUntracked | store row (projector-owned) + CR; save-only, untracked |
| MaintenanceBeforeFreeDowngrade | ConfigureMaintenanceMode → SetMaintenanceMode → patch | CR |
| Plan | SetPlan → writeThroughStoreFetched | store tier + CR |
| IdleTTLSeconds | SetIdleTTL → writeThroughStoreFetched | store + CR |
| MaxShutdownDelaySeconds | recordedPatch | CR |
| RootDir | recordedPatch | CR |
| BuildFilter | recordedPatch | CR |
| AutoDeploy | patchFetched | CR |
| Schedule/Command | SetCronJob → patchFetched | CR |
| HealthCheckPath | patchFetched | CR |
| PreDeployCommand | recordedPatch | CR |
| PublishPath | recordedPatch | CR |
| BuildCommand/StartCommand | SetCommands → recordedPatch | CR |
| DockerfilePath | recordedPatch | CR |
| Port | SetPort → writeThroughStoreFetched | store + CR |
| NotifyOnFail | patch | CR |
| NotificationsToSend | patch | CR |
| RenderSubdomainPolicy | recordedPatch | CR |
| IPAllowList | patchFetched | CR |
| MaintenanceMode | ConfigureMaintenanceMode | CR |
| Autoscaling | SetAutoscaling → patchFetched, then Get | CR |

That is 22 rows covering all 28 fields. Every CR write goes through `s.Client`, which is the stage for the duration of the patch. `TestEveryRowWritesThroughTheStage` and `TestWideCompoundPatchIsOneAppWrite` exercise every row on the kinds that accept it: web, cron and static. Pre-deploy keeps its existing static/cron refusal. Managed Postgres and Key Value do not use this table.

Adapters and callers:

- The two production `ApplyServicePatch` adapters are REST at `rest.go:958` and MCP at `mcp.go:882`.
- There is one production `rollout.Batch` caller, `settings.go`.
- GraphQL and dashboard Settings call the individual setters with the real client. Each of those is already one write, and their behaviour is unchanged.
- The secrets and envgroups Tracker consumers are untouched.

Contracts kept:

- An empty patch is still a read-only `Get`.
- Dry-run still stops at the preflight and never constructs a stage.
- Deterministic invalid-patch preflight is unchanged.
- Operational-only patches open no deploy row (`TestOperationalOnlyPatchIsNoDeploy`).
- A save-only source update stays deferred (`TestSaveOnlySourceInACompoundPatchStaysDeferred`).

**Residual, not introduced here:** the store-backed rows (port, tier, idle TTL, display name, source) write Postgres before the CR. The App projector (`store/reconciler.go` `applyOwnedSpec`, 30s resync) can project a store-owned field during the request, as it could before this change. The window has grown from one setter to the remaining rows of the request, which is milliseconds.

## Parity (t004, source-level)

- The pinned client body is unchanged: `{"serviceDetails":{"healthCheckPath":"","preDeployCommand":""}}` (`pinned-body.json`).
- REST and MCP share `ApplyServicePatch`, so their fields and error shapes are unchanged. A conflict after repeated lost races returns the existing `core.ErrConflict` (409).
- The response body is still the rows' view of the requested configuration.
- GraphQL and dashboard single-field setters were not changed and no UI change was needed.
- The actual Render backend was not exercised.

## Regression coverage (t006)

`lego/backend/internal/apps/compound_publication_test.go` observes every App state the API server stores (dry-runs excluded) through an interceptor.

On the original production path (row setters on the real client), all of these fail:

- "stored state 0: health="" preDeploy="printf marker-old"" and 2 spec changes
- 9 spec changes for the wide patch
- 2–8 spec changes per kind
- a raw API conflict instead of `ErrConflict`

With the fix they pass. Also covered:

- rebase after a lost race: the racing writer's change is kept, and the release stamp and history row are filed under the raced generation's next
- repeated conflicts: 409, no change, no deploy row
- a mid-table store failure
- the single-field clear control

The operator was not changed. Its pre-deploy gate already runs only the command in the release it observes; the fake client does not model `metadata.generation`, so generation values in these tests use the existing monotonic fallback.

## Simplify (t005)

Three-agent review (reuse, quality, efficiency). Applied:

- `Publish` reuses `PatchAppLocked`.
- `refile` reuses `Snapshot.Stamp`.
- A failed publish drops the held row.
- Pass-through writes rebase onto the server's answer, so there is no self-inflicted conflict and no stale pending marker.
- `Update` and `Patch` follow one rule, and `List` and `Get` match the App the same way.
- The response keeps the rows' view.

Skipped: the package lock around the push-delivery cache pointer, which is uncontended and needed so `withClient` can copy `Service` without vet's copylocks error; and the four-value preflight return.
