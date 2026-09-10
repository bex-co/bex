# w1 · m138 — Deploy `failureReason` reaches every surface, and the last protected-marker drop

**Worker:** worker1 **Goal:** a failed deploy shows the same status and the same actionable `failureReason` on the Deploy page, the Deploys list, and the Events feed — and in every REST/GraphQL/MCP payload that describes it — and the build-namespace registry Secret keeps the protected-mount marker its source carries. **Status:** done

## Tasks (in order)

| id   | title                                                                                                                   | est | depends_on       |
| ---- | ----------------------------------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Events: carry the deploy object's own status vocabulary + `failureReason` in `events.Details` across REST/GraphQL/MCP — **DONE**   | 45m | —                |
| t002 | Dashboard Events feed: render the backend's deploy status + `failureReason`, drop the lossy `deployStatus` re-mapping — **DONE**    | 30m | t001             |
| t003 | Deploys list: add `failureReason` to `deploys.graphql` and render it in the list row — **DONE**                                     | 30m | —                |
| t004 | `prepareBuildRegistrySecret` / `copyBuildRegistryCredential`: carry `LabelProtectedFromTenantMount` onto the build copy — **DONE** | 30m | —                |
| t005 | Render parity — **DONE**                                                                                                           | 30m | t002, t003, t004 |
| t006 | Simplify — **DONE**                                                                                                                | 30m | t005             |
| t007 | Test coverage — **DONE**                                                                                                           | 45m | t005             |
| t008 | Closeout — **DONE**                                                                                                                | 15m | t007             |

## Closeout evidence (2026-09-09)

- Backend: `TestViewMapsEverySource` (build/pre-deploy/update/canceled/live rows), `TestFailedDeployDetailsAcrossSurfaces` (REST=GraphQL=MCP field-for-field, extras absent on success/cancel), `TestEventsNeverCarryValues` (the extra is gated on the closed `DeployFailureStatus` vocabulary — an unrecognized status never echoes). Both new tests proven red with the fill neutered.
- Operator: `TestBuildRegistrySecretsPreserveProtectedMarker` (both writers × protected/unprotected), proven red pre-fix; `artifactLabels(` audit: NetworkPolicy (not a Secret) and native-env-secret (refuses protected sources) need no change.
- Dashboard: route tests (build/pre-deploy badge + reason, legacy fallback, no reason on success), list test (reason on failed row, nothing extra on live), `deployEndedStatus` unit tests, `DeployFailureReason` component test; full suite 404 files / 3096 tests green; codegen reconciled from the backend schema dump (2-query hunk + 2-line schema type).
- Simplify: one `DeployFailureReason` component (3 surfaces), one `deployEndedStatus` helper, one `carryProtectedMarker` helper (3 writers).
- Suites: backend `go test ./...` green except pre-existing `TestRunPTYCommand` env failure (proven red on pristine HEAD too); operator suite incl. envtest green; `dashboard/yarn test` green; `go vet` clean; eslint clean; typecheck has 16 pre-existing errors in unrelated files, zero in touched files.
- Live dev-1 walk **not done**: the shared kind cluster API is down (`connection refused` on 127.0.0.1:32770) and the verification inventory reports no default StorageClass. Deferred to w1/m139, the live-verification sweep that immediately follows in this loop run.

## Definition of done

- A deploy that fails in **build**, in **pre-deploy**, or in **rollout** shows the same status badge on the Deploy detail page, the Deploys list row, and the service Events feed. `build_failed` / `pre_deploy_failed` are never badged as `update_failed` on the Events feed.
- The deploy's `failureReason` string (already exposed by `GET /v1/services/{id}/deploys/{deployId}` and GraphQL `Deploy.failureReason`) is visible on all three dashboard surfaces, and the `deploy_ended` event's `details` carries it on REST, GraphQL, and MCP with identical semantics (empty/absent when the deploy did not fail).
- Render's `deployStatus` enum on `deploy_ended` stays `succeeded | failed | canceled`; the richer status and `failureReason` ride as documented bex extras (same treatment as `preDeployStatus`), recorded in `docs/ADR018-render-parity.md`.
- The `bld-<app>-registry-auth` Secret written into the build namespace carries `execution.LabelProtectedFromTenantMount` whenever its source `<app>-registry-pull` Secret does; an envtest asserts it, and the F7 godoc warning about "relocated with its marker stripped" no longer describes a live path.
- `cd lego/backend && go test ./...`, `make test` (operator), `make lint`, and `dashboard/yarn test` are green.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w1` 2026-09-09 proposal 1, from `.pm/w6/done/m97/README.md` § "Follow-ups found, NOT fixed here" (2026-08-27). All three items re-verified open on 2026-09-09: `events.Details` (`lego/backend/internal/events/service.go:437-444`, filled at `:710-711`) carries only the 3-value `deployStatus`; the Events route (`dashboard/src/routes/services.$serviceId.events.tsx:229-239`) hand-maps `failed` → `update_failed`; `dashboard/src/features/deploys/api/deploys.graphql` stops at `preDeployStatus` while the sibling `deploy.graphql` already selects `failureReason`; `lego/operator/internal/controller/build_registry_secret.go:69,112` write plain `artifactLabels` while `app_controller.go:1843-1848` shows the marker-preserving pattern.
- **Goal linkage:** pillar 1 — one core, thin adapters, Render-compatible (ADR006); ADR004 deploy transparency. w6/m123 invested in actionable failure reasons; two of three dashboard surfaces cannot show them.
- **Expected outcome:** a user whose deploy failed learns _why_ from whichever surface they are looking at, and an agent reading events over REST/MCP gets the same reason a human sees. The last known place the protected-mount marker is dropped is closed.
- **Why now:** a confirmed REST/GraphQL/UI divergence has been unowned for two weeks; each item is file:line-located and small, so the cost is lowest now, and w1's queue is empty.
- **Render parity:** included — the change touches REST/GraphQL/MCP event payloads and dashboard UI. Render's own `deploy_ended.details` has no failure reason, so the check is that the bex extras are consistently shaped across all three adapters and documented as extras, not that Render has them.
