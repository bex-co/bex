# m197 verification — 2026-10-11

Code: `741b541d2` `fix(static): restore rollback from retained publication (w4/m197)`. deploy.yml run 38129299286 green, and the pin `775e23c18` was rolled out: `bex-api` and `bex-controller-manager` are on `bex-operator@sha256:5ecfda0a…`, and the live CRD carries `releaseConfig.staticPrefix`.

## Live replay of the a79 journey (production, owned fixture)

Fixture: `qa-20261011-m197-build` (`srv-db5m5s4f52sc73b2rskg`). Public Git `bex-co/bex`, branch main, root `examples/static-site`, auto-deploy off. Created through GraphQL `createService`.

| Time UTC | Action | Result |
| --- | --- | --- |
| 10:20:32 | Create with build `printf 'qa-m197-v1\n' > dist/index.html`, publish dir `dist` | `dep-db5m5s4f52sc73b2rsl0` live at rev-1. Public body `qa-m197-v1`. Record `…-static-r1` = `tea-d98210cbbpdc73dcrkvg/tea-d98210cbbpdc73dcrkvg-qa-20261011-m197-build/rev-1/` |
| 10:28:49 | `setBuildCommand` writing `qa-m197-v2` | `dep-db5m9ocf52sc73b2rsm0` live at rev-2. Body `qa-m197-v2` |
| 10:35:39 | `setPublishPath qa-m197-missing-output` | `dep-db5mcuo9n5bc73aqiihg` closed `update_failed`: `extract: cp: cannot stat 'qa-m197-missing-output/.': No such file or directory (exit 1)`. Body still `qa-m197-v2` |
| 10:37:13 | From a freshly loaded deploy page `/static/<id>/deploys/dep-db5m5s4f52sc73b2rsl0`, clicked "Roll back to 775e23c (dep-db5m5s4f52sc73b2rsl0)" and then Proceed | `RollbackService` returned 200 with `dep-db5mdmcf52sc73b2rsrg`, trigger `rollback`, rollbackOf `dep-db5m5s4f52sc73b2rsl0`, image `…:gen-1@sha256:9697d61a…`. The dashboard navigated to the new deploy |
| 10:37:25 | Rollback finished | `live` with no failureReason. Public body `qa-m197-v1`, HTTP 200, `content-type: text/html` |

After the rollback:

- Saved settings were unchanged: `buildCommand` still writes `qa-m197-v2`, `publishPath` is still `qa-m197-missing-output`, and `autoDeploy` is false (the dashboard policy, already off).
- CR `spec.releaseConfig`:
  - `{generation: 4, image: …gen-1…, sourceGeneration: 0, staticPrefix: …/rev-1/}`
  - status `activeRevision rev-4`, `staticPrefix …/rev-4/`
- Publish Jobs: `pub-…-rev-1/2/3` used `extract`. `pub-…-rev-4` used `fetch`, with no image extraction through the bad directory and no build.

Cleanup:

- `deleteService` returned true, and REST `GET /v1/services/srv-db5m5s4f52sc73b2rskg` returned 404.
- The six-service inventory differs only by a concurrent session's fixture. `qa-20261011-a177-log-time` (`srv-db5mcm89n5bc73aqiigg`) was created at 10:35, and `srv-db5m1g2ek99c73ahur40` disappeared. Both belong to another run and were left untouched.
- `qa-login.sh --logout` printed `ok logged-out`, the old session's GraphQL call then returned 401, the browser held 0 cookies and ended at `about:blank`. The jar and state files were removed.
- The console showed only those two expected 404/401 probes.

## t003 — shared rollback callers and publication modes

| Surface / type | Result | Evidence |
| --- | --- | --- |
| GraphQL `rollbackService` (`graphql.go:266`, `DisableAutoDeploy` arg) | Live: dashboard call above | payload unchanged (Deploy with id/status/trigger/rollbackOf/image) |
| REST `POST /v1/services/{id}/rollback` (`rest.go:293`) | Source/unit: same `Service.Rollback`, no `RollbackOptions` → auto-deploy kept | `TestStaticRollbackHonorsDashboardAutoDeployOption` (dashboard=false/true) |
| MCP `rollback_deploy` (`mcp.go:182`) | Source/unit: same verb, API semantics | not live-replayed |
| Shared static publisher (`reconcileStaticSite` → `publishStaticRevision`, single caller) | Built mode: live. Direct (no-build) mode: unit/envtest only | `TestRollbackRestoresANoBuildStaticSitesRetainedPublication`; the operator restore branch precedes `directStaticPublish` |
| Legacy targets (no record, GC'd past 20 generations, foreign same-name record, malformed prefix) | Refused with a named 409 ("no retained static publication to restore") before any row is created; the projection returns `no_eligible_rollback_target` | `TestStaticRollbackRefusesAnUnretainedPublicationBeforeDispatch`. Not live-replayed: every new fixture has records. The outgoing served revision is backfilled at the next publish |
| Empty/missing/unreadable retained source | The `fetch` step fails with a fixed named message, the prior prefix keeps serving, and the rollout condition is `PublishFailed` | `retained_test.go` runs the real fetch script with an aws stub; envtest "keeps the prior publication serving…". Not live-replayed |
| Failed/canceled targets | Ineligible (unchanged `RollbackEligible`) | existing tests |
| Ordinary publishes | Unchanged extract/clone. Each now records its prefix | live rev-1..3; envtest |
| Non-static (web/private/worker/cron) | Unchanged `targetReleaseConfig`. A static selection on a non-static App fails `BadSpec` in `resolveDeployImage` before runtime planning | `rollback_config_test.go` suite; operator suite green |
| Dashboard entry points | Canonical `/static/<id>/deploys/<dep>` was live. The generic `/services/<id>/…` route canonicalizes to static (unchanged). The confirm dialog wording ("published again from this deploy … saved settings stay unchanged") is accurate; no dashboard change was needed | live above |
| Postgres / Key Value | No App rollback publication | n/a |

## t004 — Render parity

REST, GraphQL and MCP share one verb, so the retained-artifact selection, the named refusal and the finished status agree across all three. The dashboard consumes GraphQL. Render documents rollback as restoring a retained build artifact while saved settings stay current. Treating the publish directory as part of that artifact is an **inference**: Render does not document that field specifically. Headers, routes and domains stay current because they are read live from the CR. This is recorded in ADR029 §shape and ADR004. No public field changed.

## t005 / t006

- **Simplify.** One `revisionGeneration` parser, plus `Store.uri`/`envFrom` helpers. A GC failure is now logged rather than failing a successful publish. The method was renamed `restorableRollbackTarget`.
- **Not done:**
  - The projection still makes up to 20 Secret reads for a site with no records, but it stops at the first restorable target.
  - The object store keeps revisions older than the record window (pre-existing behavior).
- **Suites:** `make test` (operator, envtest) passed, `make lint` passed, and the backend real-deps suite passed via `ship-gates.sh`.
