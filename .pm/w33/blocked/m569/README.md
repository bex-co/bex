# w33 · m569 — A native build's release starts when its image is uploaded, not when its cache upload ends

**Worker:** worker33 **Goal:** a build whose `cache-save` outlasts its image push records its end, emits `build_ended` and starts its release at the push's `==> Build successful 🎉`, as Render's deploy goes straight from that line to `==> Deploying...`. The cache is still stored, and it never decides the build's verdict. **Status:** blocked (t001–t006 done in `44850069a9`; t007, the live check, waits for a production pin carrying it)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | `EnsureBuild` reports success once the image's last phase (`push`, or `sign` when signing is on) exits 0, at that container's finish time — **DONE** | 1h15m | — |
| [t002](done/t002.md) | A `cache-save` still running, or failing, after that point never changes the recorded verdict — **DONE** | 45m | t001 |
| [t003](done/t003.md) | `build_ended`, `BuildRun.finishedAt` and the deploy row's `update_in_progress` follow the push's end — **DONE** | 30m | t001 |
| [t004](done/t004.md) | Render parity — **DONE** | 20m | t002, t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 30m | t004 |
| [t007](t007.md) | Closeout (live check on production) | 40m | t005, t006 |

## Definition of done

On production, once the pin carries this milestone's commits, for a native build whose `cache-save` container finishes after its `push` container (the next build of `eden-dash-v3`, `srv-d9ndt8hmcglc739fkp50` in the first-party `bex` workspace, or a disposable `bex-canary` Node fixture with a dependency tree of 1 GB or more):

1. The service's `build_ended` event (`GET /v1/services/{id}/events`) is timestamped within 10 s of the build log's `==> Build successful 🎉`, and `bex deploys list` reads `update_in_progress` by then.
2. The release's first pod is created before the build pod's `cache-save` container finishes (`kubectl -n bex-build get pod … -o json` against the release pod's `creationTimestamp`).
3. The build after that one restores the cache `cache-save` stored (its `cache-restore` reports a hit).
4. A build whose `cache-save` fails after a successful push still goes live, and a build whose push fails still ends `build_failed` (operator tests; no production fault injection).
5. `make test` (from `lego/operator/`) and `cd lego/backend && go test ./...` pass.

## Implementation record (2026-11-22, t001–t006)

- **Verdict at the push:** `EnsureBuild` reads the Job's pods first and returns `PhaseSucceeded` once an attempt's image phase (`push`, or `sign` with signing) terminated 0, dated at that container's `finishedAt`; `Complete`/`Failed` decide only what the pods cannot (`lego/operator/internal/build/build.go`). The in-flight poll shares that one pod List with the queued check.
- **Verdict holds:** once the artifact is stored, later passes reuse it and never call `EnsureBuild` for that release, so nothing can flip it. Added beyond the plan: a third pod failure policy rule, `FailJob` on `cache-save` exiting non-zero (only when the build has the phase; the API server refuses a rule naming a missing container). Without it a `cache-save` OOM would retry the whole build and push the release's tag again after the release rolled out from the first image. A push that failed beside a killed `cache-save` loses its retry and is attributed to the platform, naming the push.
- **Timestamps:** `storeBuiltArtifact` stamps `status.buildRun` (start → push end) with the artifact; bex-api already dates `build_ended` from it. A static site keeps no success window, since its build ends with its publish (w4/493).
- **Metering kept:** a `cache-save` still running when the build is metered marks its Job `app.bex.co/cache-save-pending`, and the build census collector records the outcome once the Job finishes, so `BuildCacheStoreErrors` still sees save errors. `build_seconds` still bills to the Job's end: follow-up [w33/012](../../012.md).
- **Docs:** ADR060 D3 amendment (w33/m569); ADR018 Build logs and List / get deploy objects rows.
- **Tests** (each fails on the old behavior): `TestABuildSucceedsAtItsImagePushWhileCacheSaveRuns`, `TestASignedBuildSucceedsAtItsSignature`, `TestACompletedBuildWithNoPodLeftSucceedsFromItsJob`, `TestACacheSaveFailureAfterThePushLeavesTheBuildSucceeded`, `TestAFailedPushFailsTheBuildWhateverCacheSaveDoes`, `TestAPolicyFailureWithNoExitToReadStaysTheTenants`, `TestTheFailurePolicyFailsTheJobOnAFailedCacheSave`, `TestACacheSaveStillRunningAtTheVerdictIsMeteredOnceItEnds` (build); `TestABuildRollsOutAtItsImagePushWhileItsCacheSaveRuns`, `TestAFailedPushStillFailsTheBuild`, `TestTheBuildCensusMetersACacheSaveThatOutlivedItsBuild` and the envtests `A build's verdict is its image push (w33/m569)` and `build Job failure policy` (controller); `TestBuildEndedFollowsTheImagePushNotTheObservation` (backend store).
- **Not done here:** DoD items 1–3 need a live build on production (t007).

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w33` (the fixes found watching deploy `dep-dc1mrpt3vdpc73b0i0l0` of `eden-dash-v3` with the bex CLI, 2026-11-22), item 1; filed by `/pmx the fix to w33`. Re-checked against `origin/main` `e201eb552d`.
- **Evidence (production, read-only, 2026-11-22):**
  - Build pod `bld-tea-d98210cbbpdc73dcrkvg-block-eden-mono-gen-130-rm972`: `push` ran 22:51:05 → 22:51:50, and `cache-save` ran 22:51:05 → 22:52:55. The Job's `completionTime` was 22:52:58.
  - The build log printed `==> Build successful 🎉` at 22:51:50. `build_ended` is stamped 22:53:00, and the release pod was scheduled at 22:53:00. `bex deploys list` read `build_in_progress` between those times.
  - The same service's build on 2026-11-14 (`dep-dbs25gvnl79c738fltig`) had a 10 s lag (`Build successful` 09:19:08, `build_ended` 09:19:18). The lag is whatever `cache-save` takes beyond the push, and it grows with the cache.
- **Cause:** `cache-save` is a regular container beside `push` (`lego/operator/internal/build/build.go:1611-1621`). The comment there already says it "is NOT off the critical path — the Job is Complete only once every regular container exits". It was measured then at +0.7 s on a small fixture. `EnsureBuild` reports `PhaseSucceeded` only on the Job's `Complete` condition (`build.go:794-851`). The operator re-reads an in-flight build every 5 s (`buildObserveRequeue`, `lego/operator/internal/controller/app_controller.go:1202`), so a success read off the `push` container's status lands within one poll.
- **Render reference:** Render's deploy log goes `==> Uploading build...`, `==> Uploaded …`, `==> Build successful 🎉`, then `==> Deploying...` (render.com/tutorials/when-deploys-go-wrong/reading-render-logs, read 2026-11-22). Render publishes nothing that puts a build-cache upload between the verdict and the rollout.
- **Goal linkage:** ADR008 Render parity first; ADR060 §D1 (non-blocking build observation); ADR018 rows "Build logs" and "Deploys".
- **Expected outcome:** a cached native build starts its rollout at its verdict. `build_ended` and the deploy row stop running a minute or more behind the log on large builds.
- **Why now:** w32/m560 prints `==> Deploying...` at the rollout's recorded start. Once its pin is live, every large cached native build will show this gap in its own log: 70 s on `eden-dash-v3`.
- **Render parity:** included. The deploy status (`GET /v1/deploys`, GraphQL, MCP `list_deploys`), the `build_ended` event and the dashboard deploy page all report when a build ended.
