# w4/m201 verification

Code: `34e63b751 fix(deploys): restart the release already serving, not the newest Live row (w4/m201)`. deploy.yml run 38174884122 green. Live replay ran 2026-10-11 21:47–21:54 UTC.

## t001: selection policy and shared callers

**Serving identity.** The operator's `markRunning` (`operator/internal/controller/app_controller.go`, the only writer) sets `status.activeRevision` = `rev-<releaseGeneration>` once a release serves. Deploy rows are filed under that same generation. Restart now resolves this identity, not the newest SQL-Live row and not `status.releaseGeneration` (which is the current attempt). See `servingDeploy` in `lego/backend/internal/deploys/rollback_config.go`.

| State | Outcome |
| --- | --- |
| `releaseGeneration > 0` + `rev-N` + row N Live | Restart row N. Existing record/image policy applies: a record gives snapshot config, no record gives image-only, empty ResolvedImage gives the legacy commit-pinned fallback. |
| row N open/canceled but serving, `releaseGeneration == N`, `status.image` set | Restart row N with `status.image` and record N. |
| row N not Live, record N missing | Named 409 before any write. Image-only would run the saved config. |
| no row at N | Named 409 before any write. |
| row N not Live, `releaseGeneration != N` (a newer release is in flight, so `status.image` may belong to it) | Named 409 before any write. |
| row N not Live, `status.image` empty | Named 409 before any write. |
| `releaseGeneration == 0`, or `activeRevision` empty/malformed (legacy status, or never served) | Unchanged newest-Live selection, then the existing image-only/commit-pinned fallback (refused when there is no Live row). |

**Serialization.** Selection now runs inside `openRelease`'s mutate. That happens after the in-lock App re-read, and again after each optimistic-conflict re-read in `rollout.PatchAppLocked`. The trigger lock is taken once. The unlocked pre-read only chooses snapshot Restart over the legacy fallback. If the locked selection becomes legacy, the result is a 409 with nothing written.

**Shared callers.**

- `openSelectedRelease` now takes a selector. Rollback passes its pre-validated fixed target, so explicit historical Rollback is unchanged (all `rollback_config_test.go` / `cancel_rollback_test.go` / `static_rollback_test.go` pass).
- `openRelease`'s mutate now returns an error. The normal trigger path returns nil, so behaviour is unchanged.
- `liveCommit` stays as the legacy fallback only.
- Families:
  - Web was replayed live.
  - Private/worker share the same App runtime path and are covered by the unit regressions, not run live.
  - Cron Restart is still refused (`TestRestartCronJobIsRefused`).
  - Static skips this path (`restartSelectedRelease` early return).
  - Postgres/Key Value use separate services.

**m107 carry-over.**

- Saved/runtime separation holds: a normal deploy after Restart applies saved V2, both live and in `TestRollbackRestartAndNextDeployKeepSeparateConfigurations`.
- Restart keeps `PreserveGroupValues=true`.
- Pod replacement, operator restart, linked env groups, and the health/replica/disk/domain field matrix were not changed by this diff. They were not re-probed live and are not claimed here.

## t004: Render parity and surfaces

- Render's Restart docs (https://render.com/docs/deploys#restarting-a-service) say a restart uses the running instance's commit/configuration and excludes undeployed environment edits. The fix brings the history-lag window in line with that. This is documentation evidence only; no authenticated Render timing reproduction was run.
- Every adapter reaches `deploys.Service.Restart`:
  - REST `POST /v1/services/{id}/restart` (`apps/rest.go`) and MCP `restart_service` go through `apps.Service.Restart` → `RestartDeploy`, wired once in `api/server.go`.
  - GraphQL `restartServer` calls it directly.
  - The dashboard's useTriggerDeploy and useServiceLifecycle both send `RestartServer`.
  - No adapter selects independently. The new 409 uses the existing `core.ErrConflict` mapping.
- The pinned Render CLI is a REST client of the same route. It was not invoked live.

## t006: tests (`lego/backend/internal/deploys/restart_test.go`)

- Tests added:
  - `TestRestartSelectsTheServingReleaseBeforeItsRowIsLive` covers rollback and api triggers: A Live, B open/serving, same image, different records → restarts B.
  - `TestRestartSelectsTheSettledServingRelease`
  - `TestRestartRefusesAnUnresolvableServingRelease` covers no row / newer release in flight / no image / evicted record, each a 409 with no CR or row write.
  - `TestRestartLegacyStatusSelectsTheNewestLiveRelease`
  - `TestRestartReselectsAfterWaitingForTheLock`
  - `TestRestartReselectsOnOptimisticConflict` uses a real conflict through the interceptor.
- Mutation checks:
  - Forcing the old newest-Live selector fails 4 tests.
  - Freezing the pre-lock selection fails both reselect tests.
- Suites:
  - Backend `go test -p 1 ./...` passed against real Postgres + OpenFGA, both locally and in `scripts/ship-gates.sh` (which also brings up OpenBao).
  - `make test` (operator) passed.
  - Types tests passed.
  - `make lint` reported 0 issues across all four modules.
- The `rev-N` parser moved to `appv1alpha1.RevisionGeneration`, shared by the operator and the backend.

## t003: production replay (owned Free image web service)

Fixture: `qa-20261011-m201-restart` (`srv-db607lvoikms739hpj1g`) on `python:3.13-alpine`, using the a209 start command unchanged. `QA_A209_MARKER` was `qa-m201-v1` at create and saved as `qa-m201-v2`. All mutations were the dashboard's GraphQL documents, sent with the signed-in QA session. Ledger with complete requests/responses and timestamped reads: `.playwright-mcp/qa-m201-ledger.json`. The first, aborted script run is in `-part1.json`.

| UTC | Observation |
| --- | --- |
| 21:48:58.880 | Rollback to V1 accepted: `dep-db608inoikms739hpj4g`. |
| 21:49:10.604 | **Window 1.** Public 200/V1, Running rev-5, rollback row still `created` (open). |
| 21:49:10.962 | Restart accepted: `dep-db608lhtodic73duut80`. |
| 21:49:40.977 | Restart Live, rev-6, public 200/**V1**, undeployedChanges=true (saved V2 pending). |
| 21:50:25.797 | Settled rollback `dep-db6091foikms739hpj6g` Live, public V1. |
| 21:50:55.950 | **Settled control.** Restart `dep-db6098htodic73duuta0` Live, rev-9, public 200/**V1**, V2 still pending. |
| 21:51:18.356 | **Window 3.** Deploy of saved V2 (`dep-db609g1todic73duutb0`): public 200/V2, Running rev-10, row still `update_in_progress`. |
| 21:51:58.089 | Restart `dep-db609lhtodic73duutc0` Live, rev-11, public 200/**V2**, undeployedChanges=false. |
| 21:52:26.173 | **Final deploy.** `dep-db609vhtodic73duutd0` Live, rev-12, public 200/V2, nothing pending. |

- The superseded intermediate rows (window rollback/deploy rows) closed `canceled`. They are reported as such, not as Live.
- No saved setting was rewritten.
- Public reads were discrete samples, not a continuous availability sampler.
- Window 1 used the rollback trigger; window 3 used the fresh-deploy trigger (api).

**Cleanup.**

- `deleteService` returned true.
- At 21:53:58, `server` returned NOT_FOUND and the public URL returned 404.
- The QA service inventory matched its pre-run baseline, apart from another session's `qa-…-a227` fixture, which that session removed itself. It was untouched by this run.
- `qa-login.sh --logout` succeeded, and the API then returned 401. The session files were removed.
