# w1 · m158 — A background worker whose newest release is held (a pre-deploy step or a build) ignores suspend, resume and scale

**Worker:** worker1 **Goal:** a background worker's replicas follow suspend, resume, manual scale and autoscale while a newer release waits on its pre-deploy step or its image. They move on the prior release's pod template, and the held release never rolls. **Status:** blocked (2026-09-15). The fix and its tests are shipped: t001, t002, t005 and t006 are done. The live check (t003) needs your judgement; see § Blocked.

## Blocked — needs your judgement (2026-09-15)

The fix and its tests shipped in the same commit that moved this milestone here: t001, t002, t005 and t006 are done. Only the live check (t003) is left, and it needs a decision.

- **Why it is blocked.**
  - **Background workers are paid-only.** bex-api refuses the free plan on a background worker (`errWorkerFreePlan`, `lego/backend/internal/apps/service.go`), and an omitted plan defaults to `starter`.
  - **The loop may not buy anything,** so no worker was created.
- **The question.** May I create one `starter` background worker from `examples/hello-go` in the QA workspace (`bex` / `tea-d98210cbbpdc73dcrkvg`) for about 20 minutes, run t003 on it, and delete it?
  - The check: suspend, resume and scale over a failed pre-deploy step and over a failed build.
  - Whether that bills anything depends on the workspace's `tenants.billing_excluded` flag (ADR040, Mode A), which the loop cannot see.
- **Alternatives.**
  - Close m158 on the test evidence: three tests fail with workers excluded from either hold, and the envtest suite is green.
  - Keep it blocked.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Worker replica convergence under both holds: a scale-only patch of the prior Deployment plus the worker's parked or running status — **DONE** | 1h | — |
| t002 | Blast radius: every worker transition against the pre-deploy hold and the artifact hold — **DONE** | 30m | t001 |
| t003 | Live: suspend, resume and scale a background worker over a failed pre-deploy step and a failed build | 45m | t002 |
| t004 | Render parity | 20m | t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage — **DONE** | 40m | t004 |
| t007 | Closeout | 10m | t006 |

## Definition of done

Traced, not yet observed. t003 probes each bullet first and records the pre-fix result.

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

## Live-verification constraint

- **Plan.** Background workers may require a paid plan. t003 creates one only when the QA workspace can do so without a purchase.
- **Otherwise.** The live step moves to `w1/blocked/` with the exact question, and the unit and envtest coverage stands in for it.

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
