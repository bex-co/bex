# w6 · m100 — Fix App CR generation bookkeeping so a deploy queued behind a build-failed sibling is never permanently stranded at `queued`

**Worker:** worker6 **Goal:** when a service's second deploy is triggered while its first deploy is still `queued`/`build_in_progress`, and the first deploy later fails its build, the second (queued) deploy always advances to a real terminal state (`build_in_progress` → `live`/`build_failed`/`canceled`) within the same normal build-pickup window a solo deploy gets — never stuck at `status: "queued"` with empty `startedAt`/`finishedAt`/`failureReason` forever, recoverable only by a human noticing and clicking Cancel. **Status:** todo

## Background (found live, 2026-08-26 `/qa-find-bugs` hunt, 9th run of the day)

**Repro, live on production, workspace `tea-d98210cbbpdc73dcrkvg`:**

1. Created web service `qa-20260825-rollback-test` (`srv-da77gif1sa8c73d7r8r0`, Docker runtime) from a Public Git URL (`https://github.com/puncsky/qa-20260825-rollback-test`, temporarily made public for this hunt, restored private afterward — same throwaway repo run 8 left behind, undeletable, `delete_repo` scope missing on the local `gh` token). First deploy `dep-da77gif1sa8c73d7r8rg` (`trigger=create`) started building against the wrong branch (`main`; the repo's actual default is `master`).
2. While deploy 1 was still shown `Building` (log: `==> Build queued`, not yet pod-placed), clicked **Manual Deploy → Deploy latest commit**, creating deploy 2 (`dep-da77gov1sa8c73d7r8tg`, `trigger=api`), ~26s after deploy 1.
3. Deploy 1 failed fast (git can't find `main`): the build Job's `PodFailurePolicy` fired a `FailJob` rule. GraphQL confirms:
   ```
   query { deploy(serviceId:"srv-da77gif1sa8c73d7r8r0", deployId:"dep-da77gif1sa8c73d7r8rg") { status startedAt finishedAt failureReason } }
   => {"status":"build_failed","startedAt":"2026-08-26T05:24:34.840929Z","finishedAt":"2026-08-26T05:24:34.84093Z",
       "failureReason":"build failed: PodFailurePolicy: Container clone for pod bex-build/bld-tea-d98210cbbpdc73dcrkvg-qa-20260825-rollback-test-gendt6f7 failed with exit code 90 matching FailJob rule at index 1"}
   ```
4. Deploy 2 never moved. Re-queried repeatedly over **185+ seconds** (well past the ~40-90s a solo deploy takes to leave `queued`, confirmed independently with a 3rd deploy later in this same hunt):
   ```
   query { deploy(serviceId:"srv-da77gif1sa8c73d7r8r0", deployId:"dep-da77gov1sa8c73d7r8tg") { status startedAt finishedAt failureReason updatedAt } }
   => {"status":"queued","startedAt":"","finishedAt":"","failureReason":"","updatedAt":"2026-08-26T05:23:47.565892Z"}
   ```
   `updatedAt` never advanced from creation time. The backend's own `serviceEvents` audit trail confirms zero `build_started`/`build_ended` events were ever recorded for `dep-da77gov1sa8c73d7r8tg` in that entire window — only `deploy_started` (at creation) then, 6.5 minutes later, `deploy_ended` (`canceled`, from the manual Cancel below). The dashboard's own Events feed nonetheless labeled this row **"Deploy started / In Progress"** the whole time — an honest reflection of a `deploy_started` event that carries no live-status check, not evidence the row was actually progressing.
5. **Manual recovery exists**: clicking **Cancel** on the stuck deploy 2 correctly transitioned it to `status: "canceled"` (`finishedAt` set, `startedAt` still `""`) — so this is not unrecoverable, but nothing does this automatically.
6. **Control case, confirmed working**: after fixing the branch to `master` and saving (which auto-triggered a 3rd deploy, `dep-da77k4v1sa8c73d7r990`, which itself began `build_started` immediately since nothing was ahead of it), a 4th manually-triggered deploy (`dep-da77ka1goibs73ah06ag`) landed **while the 3rd was actively `build_in_progress`** (21s into its build). This time the queue correctly advanced: the 3rd was **canceled** and the 4th's `build_started` fired ~44s later, reaching `live` — `curl -sSI https://qa-20260825-rollback-test.onbex.co` → `200`, body `qa-rollback-v2`. This isolates the bug to the specific case where the blocking deploy resolves via **its own build failure**, not via being explicitly super­seded/canceled by a fresh trigger.

Evidence: `.playwright-mcp/qa-deploys-1-stuck-queued.png` (deploy 2's detail page mid-hunt, `Queued`/empty timestamps); the GraphQL request/response pairs above are the durable artifact.

## Root cause

**`lego/operator/internal/controller/app_controller.go:3761-3765`** (`setNotReadyCondition`, the shared body behind `r.fail`, called at `app_controller.go:3805-3809`) stamps the Ready condition it writes with:

```go
ObservedGeneration: obj.GetGeneration(),
```

— the App's **raw `metadata.generation`** at write time — instead of the release generation the build that actually just failed was pinned to (`app.Status.ReleaseGeneration`). That mis-attributed marker is read back by the gate at the top of `buildFromSource`:

```go
// app_controller.go:881-886
func terminalBuildFailureRecorded(app *appv1alpha1.App) bool {
    cond := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionReady)
    return cond != nil &&
        appv1alpha1.IsBuildFailureReason(cond.Reason) &&
        cond.ObservedGeneration == app.Generation
}
```

invoked at `app_controller.go:681-683`, which halts the reconcile with `return halt(ctrl.Result{}, nil)` — no error, no requeue — before the next deploy's build is ever dispatched.

**Mechanism, step by step (all citations verified by reading the source directly, not just the research pass):**

1. Deploy 1 (`trigger=create`) starts a build for release generation **G1**; `App.Status.Phase = PhaseBuilding`, `App.Status.ReleaseGeneration = G1` (`release_identity.go:252-259`).
2. Deploy 2 (`trigger=api`) is created via `deploys.Service.triggerFetched` (`lego/backend/internal/deploys/service.go:465-539`); its `patchApp` (line 500) bumps `App.metadata.generation` to **G2** and stamps `app.bex.co/release-generation=G2` (`stampReleaseGeneration`, lines 549-554) **before** deploy 1's build resolves. `store.prepareDeployCreate` (`lego/backend/internal/store/store.go:1739-1772`) correctly opens deploy 2 as `DeployQueued` with `OverlapPending=true` (line 1718) — this part is working as designed.
3. The next reconcile sees `buildRunning(app)==true` (`release_identity.go:196-204`) and a changed release identity, so `prepareAppReleaseDecision` **pins** the build to G1's identity and records only `PendingReleaseGeneration=G2` (`release_identity.go:230-247`, ADR060 §D1a "run-to-completion" — correct, documented behavior). `Status.ReleaseGeneration` stays **G1**.
4. Deploy 1's build Job fails via `PodFailurePolicy`/`FailJob` (`buildPodFailurePolicy`, `lego/operator/internal/build/build.go:1227-1249`); `build.EnsureBuild` (`build.go:535-573`) turns the `JobFailed` condition into `Observation{Phase: PhaseFailed, ...}` via `faultFromJob`/`execution.JobFailureMessage` — this is exactly the message this hunt observed. **Not specific to `PodFailurePolicy`**: `faultFromJob` (`build.go:469-480`) treats `PodFailurePolicy`, `BackoffLimitExceeded`, and `DeadlineExceeded` identically for control flow; only the metering `Fault` label differs. The actual trigger is a pure timing race — whichever failure class resolves before a second trigger lands reproduces this.
5. `buildFromSource`'s `case build.PhaseFailed:` (`app_controller.go:814-840`) calls `r.fail(ctx, app, ...)`, which sets `Status.Phase=PhaseFailed` and calls `setNotReadyCondition` — stamping `ObservedGeneration: G2` (the App's *current* raw generation) onto a condition actually describing **G1's** failure. This is persisted via `Status().Update` with conflict retry (`app_controller.go:3725-3747` / `3766-3803`).
6. The next reconcile computes, in memory, `app.Status.ReleaseGeneration = G2` correctly inside `prepareAppReleaseDecision`'s non-pin branch (`release_identity.go:248-262`, called from `Reconcile` at line 521) — **but `buildFromSource`'s very first statement is `terminalBuildFailureRecorded(app)`** (line 681), and the persisted condition's `ObservedGeneration (G2) == app.Generation (G2)` now matches (a coincidence created entirely by step 5's mis-stamp), so the gate halts the pass with no further status write. The in-memory `ReleaseGeneration = G2` advance is discarded — on etcd the App is stuck forever at `Status.ReleaseGeneration = G1`, `Status.Phase = Failed`, `App.Generation` never changes again (no further trigger), so **no future reconcile is ever scheduled** for this App via the generation-changed predicate either.
7. On the backend, `store.supersededDeployStatus` (`lego/backend/internal/store/reconciler.go:1036-1089`) trusts `app.Status.ReleaseGeneration` as the sole "which row is active" signal. Deploy 1 (`open.Generation=G1`) matches and correctly closes `build_failed`. Deploy 2 (`open.Generation=G2`) never matches (`G1 != G2` forever), and by explicit, deliberate design:
   ```go
   // reconciler.go:1058-1060
   if open.OverlapPending && open.Status == DeployQueued && generation < open.Generation {
       return "", true // "the intentional latest-pending slot ... elapsed time is not evidence it is orphaned"
   }
   ```
   this check runs — and returns "settled, no status change" — **before** (and therefore bypasses) the gate-timeout backstop (`timedOutDeployStatus`/`deployTimedOut`, `reconciler.go:942-960`/`1093-1101`) that would otherwise fail the row out after `BuildGateTimeout`. This branch is a **correct, deliberate design** built on the assumption that the operator will eventually advance `ReleaseGeneration` to G2 — the assumption step 6 breaks.

There is **no Job watch/informer** (`AppReconciler.SetupWithManager`, `app_controller.go:3997-4047`, has no `Owns(&batchv1.Job{})`); the only re-observation is the 5s polling requeue (`buildObserveRequeue`, used at `app_controller.go:846-848`/`850-851`) while `Phase==PhaseBuilding`. That poll **does** detect the `PodFailurePolicy` failure correctly and promptly — the bug is not in detection, it is entirely in which generation the terminal-failure marker is attributed to.

## Blast radius

The "queue" is not a distinct data structure — it is a property of `App.metadata.generation` + `app.bex.co/release-generation` + `Status.ReleaseGeneration`, shared by every code path that patches an App's spec to open a new release while a previous one may still be building. All reconcile through the identical `prepareAppReleaseDecision → buildFromSource → terminalBuildFailureRecorded` (operator) and `recordDeploy → supersededDeployStatus` (backend) code, so **all 8 are exposed** whenever the same race lands:

1. `deploys.Service.triggerFetched` (manual "Deploy latest commit" + deploy hook) — `lego/backend/internal/deploys/service.go:465-539` — **live-reproduced this hunt**.
2. `deploys.Service.Rollback` — `lego/backend/internal/deploys/service.go:704-755` (stamp at line 742) — identical `patchApp`/`stampReleaseGeneration` shape; not independently live-reproduced.
3. `apps.Service.materializeNewApp` (first deploy on Create) — `lego/backend/internal/apps/service.go:1929-2013` (stamp at line 1995).
4. `apps.Service.redeployFetched` — reached from the public `Redeploy` verb (`apps/service.go:2643`) **and** the signed git-push webhook auto-deploy (ADR017) via `apps/webhook.go:620` — `apps/service.go:2651-2677` (stamp at line 2677).
5. `apps.Service.patchChangedStackService` (Blueprint/`render.yaml` sync) — `lego/backend/internal/apps/deploy.go:2595-2659` (stamp at line 2632).
6. `apps.Service.Restart` — `lego/backend/internal/apps/service.go:2733-2735`.
7. `secrets.Service.bumpRestart` (env-var/secret change forcing a redeploy) — `lego/backend/internal/secrets/service.go:674-675`.
8. `rollout.Snapshot.Stamp`, used by env-group batch attach/detach (`envgroups/service.go:611,964,1030`) and `secrets/batch.go:275` — `lego/backend/internal/rollout/rollout.go:93-121` (stamp at line 104).

Plus `apps.Service.SetRootDir`/`SetDockerfilePath`/`SetPublishPath` (`apps/service.go:3056,3088,3984`), which bump `RestartedAt` the same way.

**Any two of the above firing back-to-back on the same App, where the first's build/rollout fails after the second has already landed its spec patch, reproduces this exact stuck-queue bug** — it is not specific to "Manual Deploy," git-based services, or `PodFailurePolicy`. Cron manual-runs, disk, and Key Value backups are unaffected (different CRs/reconcilers, no shared code).

## Adjacent classes

- **Cancel** (`deploys.Service.Cancel`, `service.go:620-689`) routes through a structurally different branch — `prepareAppReleaseDecision`'s `canceled` gate (`release_identity.go:211-218`) → `Reconcile`'s `releaseDecision.canceled` branch (`app_controller.go:522-524`) → `settleCanceledRelease` — which never touches `terminalBuildFailureRecorded`. Looks safe from this specific bug; **not independently verified this hunt** against a third trigger racing a Cancel.
- **Pre-deploy and rollout failures** use different Ready-condition reasons not covered by `IsBuildFailureReason` (`lego/types/v1alpha1/app_types.go:124-130`), so `terminalBuildFailureRecorded` itself does not gate them — but t001 must check whether an analogous generation-mis-stamp exists on **their** own `r.fail` call sites, since `setNotReadyCondition`'s bug (stamping `obj.GetGeneration()` unconditionally) is shared by every caller of `r.fail`, not just the build-failure path.
- The dashboard Events feed's "Deploy started → In Progress" label (`dashboard/src/features/deploys/`) is a **direct, honest symptom** of this bug, not an independent defect — it renders from the `deploy_started` event type without checking live status. Once t001 lands, a deploy that used to get permanently stuck will actually progress, and this label will stop being wrong for it. No separate UI task needed.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Fix the generation-mis-attribution in `setNotReadyCondition`/`r.fail` (or the `terminalBuildFailureRecorded` gate) so a pending/queued release generation can advance once the blocking generation's build failure is durably recorded — without reopening the w2/m82 double-metering/infinite-refail loop that gate exists to prevent | 60m | — |
| t002 | Regression tests: live-shaped repro (two deploys, first fails via build failure, second must reach a terminal state within the normal build-pickup window); the w2/m82 anti-regression (a recorded generation's failure is never re-failed/re-metered); at least one more call site sharing the identical `patchApp`/`stampReleaseGeneration` shape (Rollback) proven not to hit the same stuck-queue bug | 45m | t001 |
| t003 | Render parity | 20m | t002 |
| t004 | Simplify | 15m | t003 |
| t005 | Test coverage | 20m | t003 |
| t006 | Closeout | 10m | t005 |

## Definition of done

- **Live-verifiable repro, fixed:** create a web service from a Public Git URL with a deliberately wrong branch so its first deploy fails via a build Job failure; while it is still `queued`/`build_in_progress`, trigger a second deploy. The second deploy's own `deploy(serviceId, deployId) { status startedAt finishedAt failureReason updatedAt }` GraphQL query (same query used in this hunt) shows it leaving `queued` and reaching a real terminal state (`live` or `build_failed`) within the same window a solo deploy takes to leave `queued` (this hunt measured ~40-90s) — never staying at `status:"queued"`, `startedAt:""`, frozen `updatedAt` past that window.
- **w2/m82 anti-regression holds:** once a generation's build failure is durably recorded, the reconciler does not re-enter an error loop re-failing/re-metering that same generation (no repeated `Reconciler error` for one generation; `buildOutcomeAlreadyRecorded` still gates metering exactly once).
- **Operator state actually advances:** `App.Status.ReleaseGeneration` on the App CR advances to the newer pending generation once the blocking build's terminal state is durably recorded — verifiable in an envtest (or a dev cluster with `kubectl get app <name> -o jsonpath='{.status.releaseGeneration}'`), not just inferred from the deploy row.
- **Backend consumer verified, not just the producer:** a regression test proves `supersededDeployStatus`'s `OverlapPending` branch (`reconciler.go:1058-1060`) actually promotes a queued row once `appReleaseGeneration(app)` advances — today it is only tested/observed to wait forever.
- **Rollback covered:** a regression test drives `deploys.Service.Rollback` racing a still-building deploy that then fails, and shows the rollback's own deploy row reaches a terminal state instead of the same stuck `queued`.
- **Cancel keeps working:** manually canceling a deploy stuck in the affected state still transitions it to `canceled` (already true pre-fix — must not regress).
- **Unverified, carried forward (not claimed fixed by this milestone unless independently re-tested):** webhook auto-deploy racing a manual deploy, Blueprint/`render.yaml` sync racing a manual deploy, env-var/secret-triggered restart racing a manual deploy, and a chain of 3+ racing triggers — all reasoned to share the identical mechanism (see Blast radius) but not independently live-reproduced this hunt.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co`, 9th run of the day, 2026-08-25/26. Evidence: `.playwright-mcp/qa-deploys-1-stuck-queued.png`; the exact GraphQL request/response pairs quoted above (workspace `tea-d98210cbbpdc73dcrkvg`, service `srv-da77gif1sa8c73d7r8r0`, deploys `dep-da77gif1sa8c73d7r8rg`/`dep-da77gov1sa8c73d7r8tg`/`dep-da77k4v1sa8c73d7r990`/`dep-da77ka1goibs73ah06ag`, all deleted with the service during cleanup; `serviceEvents` timeline pasted in Background). Root cause independently verified by direct file reads of `app_controller.go` (lines 670-683, 855-886, 3750-3809), `release_identity.go` (190-268), and `reconciler.go` (1030-1089) — not taken solely from the research pass.
- **Goal linkage:** [ADR004](../../../docs/ADR004-app-deployment.md) (deploy queueing/generation identity is documented there — §"Queueing is not building" and the `queued`/`build_in_progress` projection) — this is the platform's single most fundamental promise (a triggered deploy eventually resolves) breaking silently and permanently. Related sibling: `w6/m95` (done) fixed a different manifestation of "a deploy row can hang forever" (no backing build job at all); this is a distinct code path (generation-identity bookkeeping in the build-failure branch specifically), not a duplicate or regression of that fix.
- **Expected outcome:** any of the 8 enumerated triggers, fired while a prior build is still resolving and that prior build then fails, leaves the newer deploy able to actually run — not permanently stuck requiring a human to notice and manually Cancel.
- **Why now:** live, currently-reproducing bug on production. Ordinary user behavior (clicking Deploy twice, a webhook racing a manual deploy, a quick settings change that auto-redeploys followed by a manual deploy) can permanently strand a service's deploy pipeline with zero automatic recovery and no user-visible explanation (the stuck row shows blank timestamps and no error) — only the dashboard's own Cancel button breaks the strand, and only if the user finds it. **Severity: blocker** for the affected race window (a real, ordinary-usage race, not a contrived edge case), scoped to the second-and-later deploy in the race.
- **Render parity included (t003):** the fix touches operator status (`App.Status.ReleaseGeneration`, the Ready condition) and the backend's REST/GraphQL/MCP deploy-status projection (`internal/deploys`, `internal/store/reconciler.go`) plus the dashboard's Deploys/Events views that read them — a genuine cross-surface change (REST `GET /v1/services/{id}/deploys/{id}`, GraphQL `deploy(...)`, MCP's deploy tools, and the dashboard deploy detail/list/events pages all read the same projected status and must keep agreeing post-fix, exactly as they already do pre-fix per this hunt's REST/GraphQL cross-check).
