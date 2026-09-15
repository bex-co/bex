# w1 · m158 — A background worker whose newest release is held (a pre-deploy step or a build) ignores suspend, resume and scale

**Worker:** worker1 **Goal:** a background worker's replicas follow suspend, resume, manual scale and autoscale while a newer release waits on its pre-deploy step or its image. They move on the prior release's pod template, and the held release never rolls. **Status:** done (2026-09-15). All seven tasks are done and the definition of done is met live on production, on a paid worker the user approved for the check and that was deleted afterwards.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Worker replica convergence under both holds: a scale-only patch of the prior Deployment plus the worker's parked or running status — **DONE** | 1h | — |
| t002 | Blast radius: every worker transition against the pre-deploy hold and the artifact hold — **DONE** | 30m | t001 |
| t003 | Live: suspend, resume and scale a background worker over a failed pre-deploy step and a failed build — **DONE** | 45m | t002 |
| t004 | Render parity — **DONE** | 20m | t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage — **DONE** | 40m | t004 |
| t007 | Closeout — **DONE** | 10m | t006 |

## Definition of done

Met (2026-09-15). t003 recorded the pre-fix result first; § Live verification has every transcript. Per-bullet verdict:

- **Resume over a failed pre-deploy step — live.** Pre-fix it stayed parked for 124 s; post-fix the same resume converged. **Suspend — live, and it already worked pre-fix** (§ Correction): what the fix changes there is the pod template the parked worker keeps.
- **Suspend and resume over a failed build — live,** 0.4 s and 0.9 s.
- **Manual scale — live over a _failed_ pre-deploy verdict** (1 → 2 instances in 15.7 s). Scaling while a step is still _running_, and release 2 rolling once the step passes, are pinned by `TestWorkerScaleWhilePreDeployRunsTakesEffect`, not by a live transcript: production's pre-deploy step is too short to act inside.
- **The held release stays held — live** (the deploy row stays `pre_deploy_failed` / `build_failed` and release 1 keeps running). "No pre-deploy Job runs again" is test-only: Jobs are not observable through the API.
- **Control — test-only on a worker.** A healthy worker's suspend, resume and scale are covered by the envtest suite; the live healthy controls in `w1/m156` and `w1/m157` are web services. The worker fixture was put into a held state before it was ever suspended.

- **Suspend and resume over a failed pre-deploy step.** A worker serving release 1 whose release 2 recorded `pre_deploy_failed`:
  - **Suspend.** Within one reconcile its Deployment scales to 0 and the service reads suspended (Hibernated).
  - **Resume.** It scales back to its instance count on release 1's pod template.
  - **The failed release.** Its deploy stays `pre_deploy_failed`, and no pre-deploy Job runs again.
- **The same over a failed build.** Release 2 recorded `build_failed`.
- **Manual scale while a pre-deploy step runs.** A new instance count takes effect on release 1 while release 2's step is still running, and release 2 rolls once its step passes.
- **Control (must not regress).** A healthy worker suspends, resumes and scales as before. Web and private services keep `w1/m156` and `w1/m157`'s behavior.

## Root cause (line numbers at `710ebe07e`)

- **The pre-deploy hold skips workers.** `holdUnpassedRelease` returns `held=false` for a worker (`lego/operator/internal/controller/app_controller.go:4588`). `reconcileKubernetes` therefore returns on the pre-deploy gate's halt (`:2053-2056`) before the Deployment write (`:2059`) and `reconcileWorkerStatus` (`:2093`).
- **The artifact hold skips workers.** `w1/m157` scopes its hold to web and private services. A worker whose newest build failed or is running halts in `resolveDeployImage` (`:692-696`) the same way.
- **Why it matters.** A worker has no Service, no Ingress and no auto-sleep, but its replica count still follows suspend, resume, manual scale and autoscale (`desiredReplicas`, `:2301-2324`).

## Blast radius

- **Who is hit.** Every background worker whose newest release is held. Suspend is silently ignored, so the worker keeps running and consuming. Resume leaves it at 0. Scale changes are dropped until the owner ships a new release.
- **Severity.** Moderate. Explicit user actions are ignored, but only while a newer release is held.

## Implementation (2026-09-15)

**Worker replica convergence under both holds (t001).** All changes are in `lego/operator/internal/controller/app_controller.go`, on top of `w1/m157`.

- **`holdUnpassedRelease`** no longer excludes workers. Its `worker` parameter is gone.
- **`holdPendingArtifact`** admits workers (`scalableRuntime`: every type except cron jobs and static sites) in place of `InternallyAddressable`.
- **`servingPriorRelease`** returns a worker's Deployment without the Service it never has.
- **`convergeServingRoute`** stops after the replicas patch for a worker: there is no Service, Ingress, URL or public-routing condition to converge.
- **Status.** A parked worker settles through `parkKubernetes`, whose suspended message now reads "worker suspended (scaled to 0)" for a worker, like `reconcileWorkerStatus`. An awake one settles Running (or the failed step's prior-release Running) through `settleHeldRuntime`.
- **Unchanged:** a first release, and a worker with no Deployment.

**Correction to the filed trace.** Suspending a worker over a held release already scaled it to 0 before this change, because the pre-deploy gate is skipped while suspended and a suspended App reuses its serving image instead of building. That suspended pass also wrote the held release onto the parked template. The frozen transitions were resume and manual scale. The mutations below confirm both, and the DoD's suspend half stays as a guard.

**Tests (t006).** Fake client; "Mutation" is the same file compiled in through `go test -overlay`:

| Test | Pins | Fails under |
| --- | --- | --- |
| `TestSuspendAndResumeWorkerOverFailedPreDeployKeepPriorRelease` (`wake_over_failed_release_test.go`) | Release 2's pre-deploy step failed. Suspend parks the worker at 0 on release 1's template and reads Hibernated. Resume brings it back to 1 and reads Running. The template stays on release 1, and no Job runs | Workers excluded from the pre-deploy hold. First the suspended pass: `suspended worker template revision = "rev-2", want the prior release's "rev-0"`. Before the template assertion existed, the same mutation also failed the resume: `resumed worker replicas = 0, want 1` |
| `TestWorkerScaleWhilePreDeployRunsTakesEffect` (`wake_over_failed_release_test.go`) | While release 2's step runs, a scale to 2 lands on release 1's template. Once the step passes, release 2 rolls at 2 | Workers excluded from the pre-deploy hold: `worker replicas while the step runs = 1, want 2` |
| `TestSuspendAndResumeWorkerOverFailedBuildKeepPriorRelease` (`wake_over_failed_build_test.go`) | Release 2's build failure is recorded. Suspend, then resume, brings the worker back to 1 and reads Running. The template, `status.image` and Build condition stay, and no build Job runs | Workers excluded from the artifact hold: `resumed worker replicas = 0, want 1` |

**Simplify (t005).** One combined review pass (reuse, quality, efficiency) over the m158 diff.

- **Applied:**
  - `hasPreDeployStep` reuses `scalableRuntime`;
  - the worker checks use `InternallyAddressable()` (a worker has no Service or Ingress);
  - one `workerSuspendedMessage` shared by the held and normal worker paths, so the Ready message does not change when a step passes;
  - stale comments fixed on the prior-release lookup, both converge functions, `parkKubernetes`, `hibernated` and the pre-deploy gate;
  - the `w1/m156` tests reuse `storeFailedPreDeploy` and a `jobsIn` helper;
  - the worker pre-deploy test also pins the template while suspended.
- **Declined:**
  - **A held worker reads Running before a pod is ready.** Web and private holds have behaved the same way since `w1/m156`.
  - **Skipping the autoscaling read on build polls.** Declined for the same reason as in `w1/m157`.
- **Filed as `w1/105`:** autoscaled background workers never end their scaling transition outside a hold. `reconcileWorkerStatus` never calls `completeAutoscalingTransition`, so they stop re-reading metrics after their first scale. The bug predates m158.

**Suites.**

- **`make test`** (from `lego/operator/`) passes, with `internal/controller` at 63.0 s and 84.1 % coverage.
- **`make lint`** reports only the three findings already on main:
  - `publish.go:611` minmax;
  - `writeGraphQLErrors`, reported both as unused and as unreachable.
- **Neighbouring tests:** the `w1/m156` and `w1/m157` hold tests pass unchanged.

**Blast radius (t002).** Worker transitions against each hold state.

| Hold state | Suspend | Resume | Manual scale / autoscale |
| --- | --- | --- | --- |
| Pre-deploy step pending or running | Covered (gate skipped while suspended; the hold keeps the template) | Covered | Covered: `TestWorkerScale…` |
| Pre-deploy step failed | Covered | Covered: `TestSuspendAndResumeWorker…PreDeploy…` | Covered (same converge path) |
| Build queued or running | Covered (suspended reuse through the hold) | Covered (in flight: Building kept, errors not recorded) | Covered (same converge path) |
| Build failed | Covered | Covered: `TestSuspendAndResumeWorker…Build…` | Covered (same converge path) |

Autoscale follows `desiredReplicas` like manual scale; workers never auto-sleep.

## Live verification (2026-09-15, production)

**Fixture.** `qa-20260915-m158w` (`srv-dakst9031mas7389obrg`), a background worker from `examples/hello-go` on the `starter` plan, created 23:03:01Z with the user's approval for this check and deleted afterwards. Workers are paid-only (`errWorkerFreePlan`), which is why the check waited for that approval.

**Held release.** A PATCH of `serviceDetails.preDeployCommand` to `echo qa-m158; exit 3` at 23:11:47Z opened `dep-dakt1cpcin2c7382cjc0`, which read `pre_deploy_failed` at 23:12:08Z ("the pre-deploy command exited with code 3"). The phase stayed Running: release 1 keeps running.

**Before the fix**, on the `w1/m157` operator (`82fed6791`), which still excluded workers from both holds:

```text
23:12:46.1  POST /suspend → 202; phase Hibernated
23:12:48.4  POST /resume → 202
23:14:52.9  phase still Hibernated — 124 s after the resume, the worker never came back
```

- **The filed symptom, reproduced.** A resumed worker stays parked while a failed pre-deploy verdict stands.
- **Caveat.** Suspend and resume were 2.2 s apart, so the suspend had not fully settled. What makes it conclusive is the 124 s with no change after the resume.
- **What is observable.** `serviceDetails.numInstances` is the spec value and stays 1 throughout, so the phase is the signal for a worker's live replica state.

**After the fix (t003).** The worker was deliberately left resumed-but-parked, so the `w1/m158` operator (pinned `eb035151a`, 23:30:41Z) had to converge it with no further action.

```text
23:15:31Z  phase Hibernated (resume already issued at 23:12:48, ignored by the m157 operator)
23:30:41Z  images pinned to 3be66b1d0259 (eb035151a); the deploy job rolls the operator
23:36:11Z  phase Hibernated → Running, with no request, no second resume and no new deploy
```

- **The resume finally lands.** The same resume that sat unhonoured for 124 s on the old operator is converged by the new one as soon as it reconciles the App.
- **The held release stays held.** `dep-dakt1cpcin2c7382cjc0` is still `pre_deploy_failed`, and release 1 is what runs.


**Manual scale over the held verdict (t003).** With `dep-dakt1cpcin2c7382cjc0` still `pre_deploy_failed`:

```text
23:37:23.5  instance-count = 1; phase Running
23:37:23.9  POST /scale {numInstances: 2} → 202
23:37:39.6  instance-count = 2 (+15.7 s) — the scale landed while the verdict stood
23:37:59.2  deploys unchanged: dep-dakt1cpcin2c7382cjc0 pre_deploy_failed, dep-dakst9031mas7389obs0 live
23:37:59.6  POST /scale {numInstances: 1} → 202
```

- **Why the metric, not `numInstances`.** `serviceDetails.numInstances` is the spec value the API echoes back, so it flips instantly and proves nothing. `GET /v1/metrics/instance-count` is the live count, and it reached 2 fifteen seconds later.
- **A first attempt did not count.** It read `numInstances` and scaled back down 1.7 s later, before any second pod could exist. It is not evidence and is not counted here.
- **Blemishes.** One mid-run metric read returned empty (a transient read), and the read after the scale-down still showed 2 because the metric had not caught up; the spec was back to 1.

**Suspend and resume over a failed build (t003).** The second DoD bullet, on the same worker and the same `w1/m158` operator:

```text
23:41:38.2  before: Running / not_suspended; held deploy dep-dakt1cpcin2c7382cjc0 pre_deploy_failed
23:41:38.6  PATCH serviceDetails.envSpecificDetails.dockerfilePath = ./Dockerfile.qa-missing → 200
23:42:10.0  dep-daktfcgb3bfc73eq7udg build_failed ("build failed in the docker build step: … load build definition from …")
23:42:10.4  over the failed build: phase Running — release 1 still runs
23:42:10.7  POST /suspend → 202
23:42:11.1  phase Hibernated (0.4 s)
23:42:21.5  POST /resume → 202
23:42:22.4  phase Running (0.9 s) → resumed over the failed build
23:42:22.7  deploys: dep-daktfcgb3bfc73eq7udg build_failed · dep-dakt1cpcin2c7382cjc0 pre_deploy_failed · dep-dakst9031mas7389obs0 live
```

- **The contrast with the baseline.** The same resume on the `w1/m157` operator was still unhonoured 124 s later. Here it lands in under a second, with two held releases stacked (a failed build on top of a failed pre-deploy step) and release 1 serving.
- **No rebuild.** The failed build's deploy row stays `build_failed`; no new deploy was opened by the suspend or the resume.

**Cleanup.** `DELETE /v1/services/srv-dakst9031mas7389obrg` → `204` at 23:44:30Z, `GET` → `404`. The paid worker existed 23:03:01–23:44:30Z (~41 minutes) and was the only resource created for this milestone.

## Render parity (t004)

**Live (REST).** Every step above was read through REST: `phase`, `suspended` (`suspended` / `not_suspended`), `serviceDetails.numInstances`, the deploys list, and `GET /v1/metrics/instance-count`. They stayed consistent with each other across the suspend, the resume, the scale and the failed build.

**The other three surfaces read the same object, by construction.**

- **MCP** `get_service` returns `toRenderService(app)` (`lego/backend/internal/apps/mcp.go:1110`) — the identical struct the REST handler serialises (`render.go:309`), so the two cannot disagree.
- **GraphQL** projects the same `AppView`: `suspended` → `core.SuspendedEnum(a.Suspended)` (`graphql.go:312`), `phase` → `a.Phase` (`:328`), and `replicas` just below it. The field name differs from REST's nested `serviceDetails.numInstances`; the value is the same one.
- **The dashboard** scales through the same verb the REST and MCP `scale` calls use (`dashboard/src/features/services/hooks/use-scale-service.ts` → `scaleService`) and renders the GraphQL fields above.

`w1/m157` t004 exercised all four surfaces live on a web service in the same held state; for the worker the fixture was deleted on schedule, so the three non-REST surfaces rest on the shared projection rather than on their own transcripts.

**What every surface reports is the desired count, not the running one.** `view()` sets `Replicas: a.Spec.Replicas` (`lego/backend/internal/apps/service.go:943`), so `numInstances` / `replicas` flips the instant a scale is accepted, whatever the pods are doing. The live count is `GET /v1/metrics/instance-count`. This matches Render, whose `numInstances` is also the configured count, so **no divergence is recorded in ADR018** — but it is written down here because it invalidated a first scale attempt (§ Manual scale).

## Dedupe

- `w1/done/103.md`: this milestone's source, promoted.
- `w1/done/m156`: the web and private pre-deploy hold. Its `convergeServingRuntime` gives the replica half.
- `w1/m157`: the web and private artifact hold. Its shared prelude and serving-prior lookup are reused.

## Source + Goal linkage

- **Source:** `w1/103`, promoted 2026-09-15 by user decision during `/loop-worker w1`. `w1/m156` t002 traced it from the code.
- **Goal linkage:**
  - `docs/ADR004-app-deployment.md`: a failed deploy leaves the previous revision serving;
  - `docs/ADR018-render-parity.md`: suspend, resume and scale act on a background worker's running instances.
- **Expected outcome:** suspend, resume and scale always take effect on a worker, whatever state its newest release is in.
- **Why now:** it is the last type left in the held-release class after `w1/m156` and `w1/m157`, and explicit user actions on paid services are silently ignored. It sequences after `w1/m157`, whose shared helpers make it small.
- **Render parity is included** because the phase and instance count surface through REST, GraphQL, MCP and the dashboard.
