# w6 · m108 — Env group Patch rollout permanently reports failure once a linked service is deleted

**Worker:** worker6 **Goal:** an Environment Group's `PatchEnvironment` (REST `PATCH /v1/env-groups/{id}/contents`, GraphQL `patchEnvGroupEnvironment`, MCP `patch_env_group_environment`) never counts a since-deleted linked service against `rolledOut`/`failedServiceIds`, and the group's persisted link list converges back to only real, existing services. **Status:** todo

## Background (found live, 2026-08-25/26 `/qa-find-bugs` hunt, 18th run of the day)

Workspace `tea-d98210cbbpdc73dcrkvg` ("bex"). Repro: created env group `qa-20260826-envgroup` (`evg-da7lkci9vh5c73aj90b0`) with var `QA_ENV_GROUP_VAR`, created a private service `qa-20260826-envgrouptest` (`srv-da7lkp29vh5c73aj90d0`, `nginx:alpine` image-backed, free plan), linked the group to it (Linked (1), redeploy triggered as promised), then deleted the service via its own Settings > Delete Service. The group's own detail page still lists the deleted service under "Linked Services" (dead link to `/services/srv-da7lkp29vh5c73aj90d0`, which 404s: `{"error":"app not found","id":"not_found"}`), rendered as a bare service id (no display name) because the dashboard cross-references `envGroup.serviceLinks` against the `services(ownerId)` list to resolve names and the deleted service is no longer in that list.

**Root cause.** `lego/backend/internal/apps/service.go:2080` (`Service.Delete`) never removes the deleted service's id from any Environment Group's persisted link list (`lego/backend/internal/envgroups/service.go` `meta.links`, stored at `env-groups/{gid}/meta`, mutated only by `linkFetched` at `service.go:968` (add) and `UnlinkService` at `service.go:1005` (remove) — no reciprocal cleanup on service deletion). This dangling reference is not just cosmetic: `lego/backend/internal/envgroups/patch.go:273-293` (`patchEnvironmentAuthorized`, the body of `PatchEnvironment` — the single shared implementation behind REST `PATCH /v1/env-groups/{id}/contents`, GraphQL `patchEnvGroupEnvironment`, and MCP `patch_env_group_environment`, confirmed via `lego/backend/internal/envgroups/{rest.go:115,graphql.go:302,mcp.go:203}` all calling `s.PatchEnvironment`) loops over `m.links` calling `s.rollOne(ctx, serviceID, s.now())` directly for `SaveModeDeploy`, or `s.RebuildService(ctx, serviceID)` for `SaveModeRebuild` — neither tolerates `core.ErrNotFound`. Any non-nil error (including `ErrNotFound` for a deleted service) is appended to `result.FailedServiceIDs`, and `result.RolledOut = len(result.FailedServiceIDs) == 0` becomes permanently false (`patch.go:289-293`). This is distinct from `rollLinked` (`service.go:1037-1046`), which explicitly skips `ErrNotFound` ("a since-deleted linked service is skipped") — but `rollLinked` is dead code, never called from anywhere in the non-test codebase (confirmed via repo-wide grep); `patch.go`'s loop was written independently (introduced whole by commit `4f3619d1` "feat(env-groups): add safe scoped editing parity") and never reused the tolerant helper.

**Live-verified via direct GraphQL probe** against the exact repro group/service:

```
Request: mutation Patch($id, $envVars, $saveMode: "deploy", $expectedRevision) { patchEnvGroupEnvironment(...) { envVarKeys secretFileNames revision affectedServiceIds failedServiceIds rolledOut } }
Response: {"affectedServiceIds":["srv-da7lkp29vh5c73aj90d0"],"envVarKeys":["QA_ENV_GROUP_VAR"],"failedServiceIds":["srv-da7lkp29vh5c73aj90d0"],"revision":"egr1_AAAAAAAAAAI","rolledOut":false,"secretFileNames":[]}
```

The variable value WAS saved (revision advanced) but `rolledOut` is false forever, citing a service that has not existed since it was deleted.

**Reproduced identically through the real dashboard UI:** Edit the group's variable, click "Save and deploy" → a persistent Alert renders: title "Configuration saved; rollout incomplete" (`dashboard/src/features/services/locales/en.ts:3885-3888`), body "Your environment changes are stored. Retry only the incomplete rollout when you're ready." (`en.ts:3889-3893`), with a "Retry rollout" button (`en.ts:3894-3897`). Clicking "Retry rollout" (which calls `retryRollout` → `save({envVars:[],secretFiles:[]}, saveMode)` → the same `PatchEnvironment` call over the same stale link) does **not** clear the alert — confirmed live, screenshot `.playwright-mcp/qa-envgroups-1-rollout-incomplete-stale-link.png`. This banner/button pair is shared verbatim by both a service's own env editor and an env group's own editor (`dashboard/src/features/env-groups/components/env-group-editors.tsx:27-41` renders the same `EnvironmentEditor` from `dashboard/src/features/services/components/service-environment-editor.tsx`, confirmed by reading both files).

This directly breaks the documented guarantee in `docs/ADR006-bex-api.md:533`: "each linked service receives zero or one selected action, and rollout-only failures are retryable" — for a stale link this is provably false: retry can never succeed because the failure is permanent (a deleted service), not transient.

**Blast radius (exhaustive, not estimated).** `patch.go:287` is the ONLY non-test call site of `rollOne` besides `rollLinked`'s own dead-code definition (`grep -rn "rollOne(" lego/backend/internal/envgroups --include=*.go`, 3 hits: the 2 named plus `rollOne`'s own func signature). Both `SaveModeDeploy` and `SaveModeRebuild` branches of the same loop are affected identically (both funnel into the same `if actionErr != nil { FailedServiceIDs = append(...) }`). `SaveModeOnly` (save without deploy) is **not** affected — it returns before the loop (`patch.go`: `if patch.SaveMode == SaveModeOnly || len(m.links) == 0 { return result, nil }`). `LinkEnvGroup` (the Blueprint seam, `service.go:1246`) is **not** affected — it does not loop over `m.links`, only adds one new link via `linkFetched`. `DeleteEnvGroup`'s own detach loop (`service.go:711-716`) and `validateLinkedServiceEnvironments` (`service.go:1075+`) both already tolerate `ErrNotFound` correctly via `detach`/`continue` — these are the correct control-case pattern this fix should match, not the broken one.

**Adjacent classes (must not regress).** A genuine, non-`NotFound` rollout failure on a service that still exists must continue to behave exactly as the existing test `lego/backend/internal/envgroups/patch_test.go:216-247` (`TestPatchEnvironmentReportsPartialRebuildForRolloutOnlyRetry`) already covers — an injected error on an existing `sampleApp("worker")` must still land in `FailedServiceIDs`, set `RolledOut=false`, and be clearable by a genuine retry once the real problem is fixed. The fix must distinguish "service no longer exists" (`core.ErrNotFound` — exclude from `FailedServiceIDs`, self-heal by pruning from links) from "service exists but its rollout genuinely failed" (keep exactly current behavior).

**Unverified this run:** whether any of the real, standing services/groups in the two workspaces this account can see already carry a stale link from a past deletion (not checked — this run's repro used only its own isolated `qa-` fixtures, cleaned up after diagnosis). Whether a background sweep vs. an on-read/on-write self-heal is the right prune mechanism is a design decision for the implementer, not decided here.

## Tasks (in order)

| id   | title                                                                             | est | depends_on |
| ---- | ---------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Tolerate a since-deleted linked service in `PatchEnvironment`'s rollout loop        | 30m | —          |
| t002 | Prune a discovered-stale service id from the group's persisted `meta.links`        | 30m | t001       |
| t003 | Regression tests for the stale-link case, without regressing genuine failures      | 30m | t002       |
| t004 | Render parity                                                                       | 15m | t003       |
| t005 | Simplify                                                                            | 15m | t004       |
| t006 | Test coverage                                                                       | 20m | t005       |
| t007 | Closeout                                                                            | 10m | t006       |

## Definition of done

- Live repro command: link a fresh `qa-<date>-` service to an env group, delete the service, then call GraphQL `patchEnvGroupEnvironment(id, envVars:[{key,value}], saveMode:"deploy", expectedRevision)` — response must show `rolledOut: true` and `failedServiceIds: []`, never citing the deleted service.
- Same live check with `saveMode` `"rebuild"` gives the same result.
- The dashboard's env-group Edit-and-Save flow (Save and deploy / Save and rebuild) no longer renders the "Configuration saved; rollout incomplete" / "Retry rollout" Alert when the only affected link is a since-deleted service.
- `GET /v1/env-groups/{id}` (REST), `envGroup(id).serviceLinks` (GraphQL), and the MCP equivalent all agree the deleted service's id is gone from `serviceLinks` after the fix's prune path runs, and the dashboard's own "Linked Services" list shows no dead-link entry for it.
- `go test ./lego/backend/internal/envgroups/...` passes including `TestPatchEnvironmentReportsPartialRebuildForRolloutOnlyRetry` unmodified and the new regression case from t003.
- `ADR006-bex-api.md:533`'s "rollout-only failures are retryable" is true again: a retry after a stale link is pruned succeeds instead of looping forever.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co`, 18th run of the day, 2026-08-25/26. Evidence: `.playwright-mcp/qa-envgroups-1-rollout-incomplete-stale-link.png`, plus the exact GraphQL request/response pair quoted above (workspace `tea-d98210cbbpdc73dcrkvg`, env group `evg-da7lkci9vh5c73aj90b0`, service `srv-da7lkp29vh5c73aj90d0`, both deleted during cleanup).
- **Goal linkage:** [ADR006-bex-api.md](../../../docs/ADR006-bex-api.md) (env groups core contract, the "retryable" guarantee this violates) and [ADR018-render-parity.md](../../../docs/ADR018-render-parity.md)'s Environment groups row (REST/GraphQL/MCP/dashboard parity, all of which must move together since they already share one implementation).
- **Expected outcome:** deleting a service that is still linked to an environment group no longer permanently pollutes that group's future save-and-deploy/save-and-rebuild results with a false, un-retryable failure.
- **Why now:** any workspace that deletes a service while it is still linked to an environment group gets a permanently-misleading "rollout incomplete, retry" prompt on every future edit to that group, for real-usage-shaped groups that link more than one service — masking whether the group's OTHER, currently-existing linked services actually rolled out or not, which is the entire purpose of the `failedServiceIds`/`rolledOut` fields.
- **Render parity task included:** yes (t004) — REST/GraphQL/MCP already share one implementation (confirmed this hunt) and the dashboard reads the same response fields; the fix must preserve that agreement across all three plus the UI.
