# w1 · m160 — Release identity: a never-served first release must not read Running, and an autoscaled worker must keep autoscaling

**Worker:** worker1 **Goal:** the phase a service reports after a failed or canceled build reflects what it can actually serve — Running only when a release really served — and a background worker's autoscaling transition ends, so it keeps reading metrics after its first scale. **Status:** todo (t001, t002, t003, t006 and t007 done; t004 live and t005 parity wait on the deploy)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | `fail` and the canceled-release path decide "a prior release exists" from `status.activeRevision` — **DONE** | 30m | — |
| t002 | A worker's autoscaling transition ends, so autoscaling continues past the first scale — **DONE** | 45m | — |
| t003 | Blast radius: every "a prior release exists" decision, and every path that starts a transition — **DONE** | 30m | t001, t002 |
| t004 | Live: a never-served first release, then a failed build | 45m | t003 |
| t005 | Render parity | 20m | t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 45m | t005 |
| t008 | Closeout | 10m | t007 |

## Implementation (2026-09-15)

**The prior-release decision (t001).** Both sites in `lego/operator/internal/controller/app_controller.go` now read `status.activeRevision`, which `markRunning` writes only once a release has actually served:

- `fail`, on a build failure: `settleFailureOverPriorRelease` runs only when a release served. Otherwise the phase settles Failed.
- `settleCanceledRelease`: the canceled generation re-dispatches `status.image` only when a release served; otherwise it settles Canceled, the branch `w6/m52` wrote for "there is simply no release yet".

`status.image` is still what gets dispatched — it is the right _image_, just not proof that anything served.

**The worker's autoscaling transition (t002).** `reconcileWorkerStatus` now calls `completeAutoscalingTransition` on the refreshed Deployment, exactly where `reportKubernetesRunning` does for web and private services. Until then only those two paths (and the held-release path from `w1/m156`–`w1/m158`) ended a transition, so a worker's first scale left it `Started` forever and `applyAutoscaling` kept returning the recorded `ToReplicas` without reading metrics again.

**Blast radius (t003).**

| Site | Reads | Verdict |
| --- | --- | --- |
| `fail` (build failure) | `activeRevision` | **fixed here** |
| `settleCanceledRelease` | `activeRevision` | **fixed here** |
| `settleFailedRollout` | `activeRevision` | already correct |
| `failPreDeploy` (`w1/m149`) | `activeRevision` | already correct — the precedent this milestone generalizes |
| `servingPriorRelease` (`w1/m156`–`m158` holds) | `activeRevision` | already correct |
| `successfulReleaseGeneration` (`release_identity.go`) | `activeRevision`, falling back to `status.image` + `observedGeneration` | **deliberately unchanged — filed as `w1/106`.** The fallback has the same flaw (a cancel over a never-served release stamps a `releaseGeneration` nothing ever served), but it is also the legacy path for Apps predating `activeRevision`, and bex-api's deploy reconciler consumes the field. Changing it needs its own milestone with backend evidence. |
| `status.image` assignments (`reportKubernetesRunning`, `parkKubernetes`, `reconcileWorkerStatus`, the static/cron paths) | — | plain image writes, not identity |
| `registryHosted`, the `status.image != image` change checks, `reusableArtifactImage` | — | plain image references, not identity |

Transitions: started in `applyAutoscaling`; ended in `reportKubernetesRunning` (web, private), `convergeServingRuntime` (held releases) and now `reconcileWorkerStatus` (workers). Cron jobs and static sites have no Deployment and no autoscaling.

**Tests (t007).** Fake client; "Mutation" is the same file compiled in through `go test -overlay`.

| Test | Pins | Fails under |
| --- | --- | --- |
| `TestBuildFailureOverNeverServedReleaseReportsFailed` | A crash-looped first release (image stamped, no active revision) plus a failed build reports Failed | `fail` keyed on `status.image`: `phase = "Running", want "Failed"` |
| `TestCancelOverNeverServedReleaseSettlesCanceled` | A cancel with nothing ever served settles Canceled instead of re-dispatching the image | The cancel path keyed on `status.image` |
| `TestAutoscaledWorkerEndsScalingTransition` | A worker's Started transition reaches Ended with `finishedAt` once its Deployment has the target replicas ready | The completion call removed: `State:Started` stands |

**Simplify (t006).** One combined reuse/quality/efficiency pass over the diff.

- **Applied:**
  - extracted `releaseHasServed(app)` and pointed all five sites at it, collapsing the duplicated rationale into one doc comment;
  - dropped the control test this milestone had added — `failed_build_prior_release_phase_test.go`'s `TestBuildFailureOverServingReleaseStaysRunning` already covers a failed build over a serving release and asserts more (the Build condition and its generation attribution), so a second copy was pure duplication.
- **Declined:**
  - **Folding the never-served `fail()` case into `failed_build_prior_release_phase_test.go`'s `failedBuildApp`** (parameterizing it the way `preDeployApp` is). Reasonable, but it would scatter one milestone's four-test story across three files; the remaining tests stay together and the file points at the external control.
- **Confirmed clean:** the worker's `completeAutoscalingTransition` sits exactly where the web path's does (right after the Deployment refresh, before the readiness gate) and reuses that same refreshed object — no extra API call, and the same shape the hotter web path already pays.

**Suites.**

- **`make test`** (from `lego/operator/`) passes, `internal/controller` included.
- **`make lint`** reports **0 issues** across all four modules.
- **Mutations** (`go test -overlay`): reverting either decision to `status.image`, or removing the worker's transition completion, fails the matching test each time.

## Definition of done

Traced, not yet observed. t004 probes each bullet on production with a throwaway free service and records the pre-fix result first.

- **A never-served first release that then fails its build reads Failed.** A service whose first release crash-loops (so `status.image` is set but `status.activeRevision` is not), given a second release whose build fails, reports `phase: Failed`, not Running. The deploy row still reads `build_failed`.
- **The same for a cancel.** Canceling that second release leaves the phase Failed, not Running.
- **Control (must not regress).** A failed build over a release that did serve still reads Running with the prior release serving — the `w6/m124` behavior `w1/m157` verified live.
- **An autoscaled worker scales more than once.** Its scaling transition reaches Ended, and a later metric change moves replicas again. Autoscaling is a paid-only feature, so this bullet closes on the envtest evidence unless a paid worker is approved; the milestone records which.

## Live verification (2026-09-16, production)

**Fixture.** `qa-20260915-m160` (`srv-dakvffish60c73ao4m5g`), a free web service from `examples/hello-go` whose start command exits immediately, so its first release crash-loops: the operator stamps `status.image` and no revision ever becomes active. Its first deploy ended `update_failed` after 17 minutes at the progress deadline, with the phase correctly reading Failed — `settleFailedRollout` already keys on `activeRevision`, which is the precedent this milestone generalizes.

**Before the fix**, on the operator production is still running (images pinned at `eb035151a`, the `w1/m158` build — every later pin was superseded, so nothing since has rolled):

```text
02:16:55Z  before: phase Failed / not_suspended, latest deploy update_failed
02:16:56Z  PATCH serviceDetails.envSpecificDetails.dockerfilePath = ./Dockerfile.qa-missing → 200
02:17:38Z  dep-dakvo5r3hm6c73bir7rg build_failed (42.5 s)
02:17:59Z  phase over the failed build: Running / not_suspended
```

- **The filed symptom, reproduced.** A failed build over a release that **never served** reports the service **Running**. `fail` read `status.image` — which the crash-looped first release had already stamped — and settled "the previous release keeps serving" over a release that never served a single request.
- **The deploy row is unaffected either way**, as `w1/101` predicted: it reads `build_failed` from the Build condition, not from the phase.

## Root cause

- **The phase half (`w1/101`).** `fail` decides "a prior release exists" with `app.Status.Image != ""` (`app_controller.go`, around `:4294` at filing), and the canceled-release path does the same (around `:633`). `status.image` is set by a first release that never served. `settleFailedRollout` and, since `w1/m149`, `failPreDeploy` already key on `status.activeRevision`, which `markRunning` sets only once a release has served.
- **The autoscaling half (`w1/105`).** `completeAutoscalingTransition` (`autoscale.go`) is the only place a Started transition becomes Ended, and `applyAutoscaling` returns the recorded `ToReplicas` without reading metrics while one is Started. Its only callers are `reportKubernetesRunning` (web and private) and `convergeServingRuntime` (the held-release path). `reconcileWorkerStatus` never calls it, so a worker's first scale freezes its autoscaling for good.

## Blast radius

- **Who is hit.** Any service whose first release never served and whose next build fails or is canceled — it reads Running while serving nothing. Every autoscaled background worker, after its first scale event.
- **Severity.** Minor for the phase half (it needs a never-served first release); moderate for the autoscaling half, which is a paid feature that silently stops working.

## Source + Goal linkage

- **Source:** `w1/101` (code review during `w1/m149` t005) and `w1/105` (`w1/m158` t005). Both were traced from code and never probed; promoted together 2026-09-15 during the w1 triage because they are one question — which recorded fact a release's identity is read from.
- **Goal linkage:** `docs/ADR004-app-deployment.md` (a failed deploy leaves the previous revision serving, and the phase says what is running) and `docs/ADR018-render-parity.md` (Render reports a service that is not serving as failed, and keeps autoscaling live).
- **Expected outcome:** the phase never claims Running for a service that never served, and an autoscaled worker keeps following its metrics.
- **Why now:** both sit in the code `w1/m156`–`w1/m158` just reworked, so the context is loaded and the tests are already in place to extend.
- **Render parity is included** because the phase is read through REST, GraphQL, MCP and the dashboard header.
