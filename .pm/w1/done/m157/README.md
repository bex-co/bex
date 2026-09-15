# w1 · m157 — A service whose newest build failed or is still building cannot wake, resume, sleep or scale its serving release

**Worker:** worker1 **Goal:** while a newer release has no image yet (its build is queued, running, waiting on a registry credential, or failed), the release that is actually serving keeps following the App. It sleeps when idle, wakes on a request, suspends and resumes, and scales. The pending or failed release never reaches the pod template, and a build in flight keeps its own polling. **Status:** done (2026-09-15). t001, t002, t004, t005 and t006 are done, and every definition-of-done bullet passed live on production after the `82fed6791` pin: resume (11.6 s), idle sleep and wake (11.7 s), a wake while a build runs (12.0 s), the rollout control, and the surface comparison. Both fixtures are deleted.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Runtime convergence over a release still waiting for its image: the prior release's replicas and routing follow the App while the build halts — **DONE** | 1h30m | — |
| t002 | Blast radius: every `buildFromSource` halt against every runtime transition, plus the legacy Ready marker and the disk restore — **DONE** | 45m | t001 |
| t003 | Live: resume, idle sleep and wake over a failed build, and a wake while a build runs, on production — **DONE** | 1h | t002 |
| t004 | Render parity — **DONE** | 20m | t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage — **DONE** | 45m | t004 |
| t007 | Closeout — **DONE** | 10m | t006 |

## Definition of done

Run on throwaway free web services from `bex-co/bex` `examples/hello-go` (docker, `dockerContext: examples/hello-go`) that serve `200`. Only states observed at filing time are listed as failing:

- **Resume over a failed build.**
  1. Set `dockerfilePath` to a missing file and deploy, so the newest deploys read `build_failed` while the first release keeps serving.
  2. Suspend, then resume.
  3. Request the URL every second.

  Within 60 s of the resume the URL answers the prior release's own `200`. At filing time 112 of 113 requests over 180 s got `503 {"error":"service hibernated","retryAfter":5}`, the URL was still `503` 4.5 minutes later, and the phase read Running throughout (`w1/done/m156/README.md` § Live verification, `qa-20260915-m156b`).
- **Idle sleep and wake over a failed build.** Left without requests, the same service records `service_hibernated`; a request then gets the prior release's `200` within 60 s. Traced at filing time: the idle check never runs while the build halts (§ Root cause). `qa-20260915-m156b` stayed Running for 28 idle minutes, but outside scanner requests also reached it, so that half was not isolated.
- **A wake while a build runs.** A hibernated free service whose next deploy is still building answers the prior release's `200` within 60 s of a request. The deploy keeps building and then rolls out normally.
- **The failed or pending release stays off the pod.** After each wake or resume the pod serves the prior release, a failed deploy stays `build_failed`, and no new build Job starts for that release.
- **Control (must not regress).** A healthy free service still resumes and wakes: at filing time `qa-20260915-m156d` answered `200` 11.5 s after resume and `qa-20260915-m156c` 11.5 s after its first request. A successful build still rolls out.

## Evidence (production, 2026-09-15, operator `1cb1f2d27`)

Reproduced during `w1/m156` t003; the full transcripts are in `w1/done/m156/README.md` § Live verification.

```text
qa-20260915-m156b (srv-dakhhdfr0t2c73fc63gg), MESSAGE=m156-build
10:09:22Z  first deploy live; URL 200 m156-build
10:09:23Z  PATCH envSpecificDetails.dockerfilePath ./Dockerfile.qa-missing → dep-dakhikvqniac73emh130 build_failed
10:10:07Z  dep-dakhilfr0t2c73fc63i0 build_failed ("failed to read dockerfile"); phase Running; URL 200 (prior release)
10:38:46Z  suspend → Hibernated
10:38:57Z  resume → 202; service_resumed 10:39:02Z
10:38:58–10:41:59Z  112/113 `503 service hibernated`, 1 `503 no available server`, no 200; phase Running
10:43:25Z  URL still 503; phase Running
```

Controls on the same operator build: `qa-20260915-m156d` (healthy, suspend and resume) `200` in 11.5 s; `qa-20260915-m156c` (healthy, idle sleep and wake) `200` in 11.5 s.

## Root cause (line numbers at `710ebe07e`)

- **The build halts before the runtime.** `Reconcile` returns on `resolveDeployImage`'s halt (`lego/operator/internal/controller/app_controller.go:591-594`) before `dispatchRuntime` → `reconcileKubernetes` (`:595`, `:709`). `resolveDeployImage` halts whenever `buildFromSource` does (`:692-696`). `buildFromSource` halts on:
  - a recorded terminal failure (`:755-757`, and `:923-925` on the quiesced pass);
  - a registry-credential wait (`:776-781`);
  - the build caps (`:804-808`, `:887-890`);
  - a build still waiting for capacity or running (`:941-951`).
- **Why suspend works and resume does not.** A suspended App reuses its serving image (`release_identity.go:316-318`), never reaches the build, and parks normally. Resume clears that. The release's artifact fingerprint still differs from the served one (`release_identity.go:264`), so the pass builds, meets the recorded failure, and halts with replicas at 0.
- **Why the idle check and scale never run.** `desiredReplicas` (`:2021`, `:2301-2324`) runs inside `reconcileKubernetes`. An awake service over a failed build is never checked for idleness, an asleep one is never woken, and manual scale and autoscale are ignored.
- **Terminal failures quiesce.** The recorded-failure halt returns no requeue (`:755-757`), so only a watch event re-enters, and it halts again.
- **Why the tests missed it.** `failed_build_prior_release_phase_test.go` calls `r.fail` directly and pins only the phase. `wake_over_failed_release_test.go` covers the pre-deploy hold only, because `w1/m156` withdrew its build hold in review.

## Blast radius

- **Who is hit.**
  - Any service, free or paid, whose newest build failed: unreachable after a suspend and resume, with manual scale and autoscale ignored.
  - A free service whose newest build failed: unreachable after its next sleep, which a failed build also stops from happening while it is awake.
  - A free service asleep while a push builds: a request gets `503` until the build finishes and rolls, which takes minutes.
- **Types.** Web and private services here. Background workers are `w1/m158`. Cron jobs and static sites have no Deployment to scale.
- **Every halt and transition.** t002 enumerates them.

## Design constraints (from the `w1/m156` review, `w1/done/104.md`)

- **Keep the build's requeue.** A parked App with a build in flight must still come back at the build's cadence, because nothing watches build Jobs. Do not overwrite Building or BuildQueued with Hibernated on every poll.
- **Respect the disk lifecycle.** Converge only after `reconcileDiskLifecycle`, which scales a service to 0 for a restore and must win.
- **Do not run the full runtime on every build poll.** Skip routing writes when nothing changed, complete autoscaling transitions, and persist status only when it changed.
- **Legacy markers.** `recordedBuildVerdict` (`:1046-1055`) falls back to a Ready-only build-failure marker. A status write that replaces Ready must not erase it, or the failed build is dispatched again.
- **The template never advances.** Only replicas and routing move. Writing the pending release's config onto the prior image is the class `w1/m152` (blocked) records for canceled releases.

## Implementation (2026-09-15)

**Runtime convergence over a release without an image (t001).** All changes are in `lego/operator/internal/controller/app_controller.go`.

- **`holdPendingArtifact`.** `resolveDeployImage` calls it in two places:
  - when `buildFromSource` halts without an error: a recorded failure, a registry-credential wait, a build cap, or a build that is waiting or running;
  - when a suspended App reuses its serving image for a release that has not built.

  It holds only web and private services in Kubernetes mode that have a serving prior release (`servingPriorRelease`: `status.activeRevision`, the Deployment, and a Service with a port). Only replicas and routing move, and the pod template is never written.
- **Build failure recorded.** The hold converges, then parks (Hibernated), or settles Running from the scale this pass wrote (`settleHeldRuntime`, then `settlePriorRelease`). The Build condition stays the verdict.
- **Build in flight.** A pass on the build's poll whose phase is Building. That phase pins the release to its build (`buildRunning`, ADR060 §D1a), so:
  - a parked pass only scales and routes, keeps Building, and returns the build's own requeue (or the routing grace, if sooner);
  - a routing or disk error comes back with its reason (`stepFailure`) and is returned without recording Failed, which would release the pin.
- **Polls stay cheap.**
  - **Asleep.** A poll of a service already asleep at 0 skips the plan entirely: no traffic query and no autoscaling or disk reads until a request stamps last-active.
  - **Awake.** Other polls run only the disk-restore gate (`planReplicas` with `poll`) and rewrite the Ingress only when the scale changed or the route differs.
- **Every other held pass.** This covers a recorded failure, a suspended App, and an event.
  - It first converges what `reconcileKubernetes` owns for every type (`convergeSharedChildren`: the slug Service on the serving port, and removal of a stale execution-egress grant).
  - It then always rewrites the Ingress with its middlewares, so domain and IP allow-list edits apply while a release is held. That includes `w1/m156`'s failed pre-deploy state, where such an edit used to wait for a new release.
- **Legacy Ready-only verdict.** Not held (`legacyReadyBuildVerdict`). The hold's status writes replace Ready, so erasing that marker would dispatch the failed build again.
- **`planReplicas`.** Extracted from `reconcileKubernetes`: `desiredReplicas`, `holdHibernateForRouting` and `reconcileDiskLifecycle`, in that order, so a restore always wins. The rollout and both holds scale from its plan.
- **Unchanged:**
  - a first release;
  - background workers (`w1/m158`);
  - cron jobs and static sites;
  - the opensandbox runtime;
  - the pass that first records a failure, which returns its error so the next pass holds;
  - the protected-environment NetworkPolicy and the ClusterIP Service port, which follow the release when it rolls. This is `w1/m156` parity: the prior pods still serve the old port.

**Simplify (t005).** Three review passes (reuse, quality, efficiency) over the first version.

- **Applied: bugs.**
  - A routing or disk failure on a pass observing a build recorded Failed. That releases the release's pin to its build, so a newer push could start a second build. Holds now get the failure back with its reason, and a build pass returns it without recording it.
  - A registry-probe error (15 s requeue, no phase write) counted as a build in flight. Being in flight is now decided by the Building phase.
  - The held pass skipped the per-type children, and skipped every Ingress rewrite while the route matched. An IP allow-list edit never reached a held service. Non-poll held passes now converge both.
  - A parked service polled Prometheus every 5 s while its build ran, because its phase stays Building. The asleep poll now skips the plan.
  - Disk upkeep (up to 5 uncached calls) ran on every build poll. Polls now run only the restore gate.
  - A restore during a build replaced the build's 5 s poll with 15 s. The two cadences now merge.
- **Applied: quality.**
  - `InternallyAddressable()` in place of a type switch.
  - `settleHeldRuntime` shared by both holds, and `priorRelease` in place of a four-value return.
  - `legacyReadyBuildVerdict` next to `recordedBuildVerdict`.
  - Stale comments fixed on `buildFromSource`'s halt and quiesce, and on `replicaPlan`.
  - Shared test helpers with the `w1/m156` tests.
  - Lint: the parameter that shadowed the `build` import (revive), and `errors.AsType` (modernize).
- **Declined.**
  - **Reading autoscaling metrics on each awake build poll.** The cost is bounded to autoscaled paid services during a build. `w1/m156`'s step poll already runs at the same cadence, and the read is a cached `metrics.k8s.io` list.
  - **The activity-reader error log during a Prometheus outage.** It is log volume only.
  - **Explicit in-flight parameters.** The hold derives this state from the halt and the phase instead.
  - **Deriving `holdUnpassedRelease`'s worker flag.** `w1/m158` changes that code.

**Tests (t006).** `wake_over_failed_build_test.go`, fake client. "Mutation" is the same file with the named change, compiled in through `go test -overlay`:

| Test | Pins | Fails under |
| --- | --- | --- |
| `TestResumeOverFailedBuildRestoresPriorRelease` | Release 2's build failure is recorded; suspend parks release 1; resume scales it to 1 and reads Running; the route returns once ready; template, `status.image` and Build condition stay; no build Job | Both holds removed: `resumed replicas = 0, want 1`. Only the suspended hold removed: `pod template revision = "rev-2", want the prior release's "rev-0"` |
| `TestIdleSleepAndWakeOverFailedBuildKeepPriorRelease` | Idle over a recorded failure: activator route, 0 replicas, Hibernated; a request scales to 1 and reads Running; route back once ready; prior release kept | Both holds removed: `idle replicas = 1, want 0` |
| `TestParkedServiceWithQueuedBuildKeepsBuildPollAndWakesOnPriorRelease` | Release 2 queued behind the workspace build cap. Awake: replicas 1, requeue ≤ 30 s. Idle: parks the prior release while the requeue stays at the build's poll and the phase stays Building; later asleep polls read no traffic. A wake scales up; the template stays | Both holds removed: `idle replicas = 1, want 0`. Asleep skip removed: `activity reads while parked = 3, want none`. Building phase ignored: `parked requeue = 0s, want the queued build's own poll` |
| `TestLegacyReadyOnlyBuildFailureIsNotHeld` | A Ready-only build-failure marker survives an idle pass, and no build Job is dispatched | Legacy check removed: `Ready condition = …Reason:AutoHibernated…, want the legacy build-failure marker kept` |
| `TestWakeOverFailedBuildWaitsForDiskRestore` | A wake during a running disk restore leaves the service at 0 | Disk lifecycle skipped: `replicas during the restore = 1, want 0` |
| `TestDeployWhileSuspendedKeepsTemplateOnServingRelease` | A repo release deployed while suspended leaves the parked template, `status.image` and phase alone; no build Job | Suspended hold removed: `parked template revision = "rev-3", want the parked release's "rev-2"` |
| `TestAllowListEditOverFailedBuildReachesIngress` | An IP allow-list edit over a recorded build failure creates the allow-list Middleware | The first version's skip-while-routed rule: `allow-list Middleware = … "web-ip-allow" not found` |
| `TestRoutingFailureWhileBuildQueuedKeepsBuildingPhase` | A refused activator-alias write while a build is queued is returned as an error and the phase stays Building | Building phase ignored: `phase after the routing failure = "Failed", want Building` |

**Suites.**

- **`make test`** (from `lego/operator/`) passes after the review rework: 24 packages, with `internal/controller` at 62.9 s under envtest and 84.4 % coverage.
- **Unchanged tests pass:** the `w1/m156` wake tests, the `w6/m124` failed-build phase tests, and the IP allow-list projection tests.
- **Mutations:** every mutation in the Tests table fails its tests, compiled in through `go test -overlay` (`scratchpad/m157-mutate.py`), and the unmutated build passes.

**Blast radius (t002).** Every `buildFromSource` halt, and the suspended reuse, against the runtime transitions: wake, idle sleep, suspend, resume, manual scale, autoscale, maintenance, custom domains and the IP allow-list.

| Halt or state | Verdict | Evidence |
| --- | --- | --- |
| Recorded build failure (Build condition) | Covered. Replicas follow `planReplicas` (wake, sleep, resume, scale, autoscale); routing follows `ingressBackend` (maintenance precedence) and `reconcileIngressWithMiddlewares` on every non-poll pass (domains, allow-list) | `TestResumeOverFailedBuild…`, `TestIdleSleepAndWakeOverFailedBuild…`, `TestAllowListEditOverFailedBuild…` |
| Suspended App reusing its serving image for an unbuilt release | Covered: parks through the hold; the template is not written | `TestDeployWhileSuspended…`, `TestResumeOverFailedBuild…` |
| Build cap (workspace or cluster), shed overshoot | Covered as a build in flight: the build's 30 s poll and the Building phase are kept | `TestParkedServiceWithQueuedBuild…` |
| Build waiting for capacity or running (5 s poll), registry-credential wait (10 s) | Covered as a build in flight (Building phase) | Same branch as the cap |
| Registry probe error (15 s, no phase write) | Covered as an ordinary held pass: parks or settles on the poll's cadence | The phase is not Building, so no pin to protect |
| A routing or disk error while a build is observed | Returned without recording Failed; the Building pin holds | `TestRoutingFailureWhileBuildQueued…` |
| The pass that first records a failure | Not held on that pass: `r.fail` returns its error, and the retry pass holds | Code path; controller-runtime backoff |
| Legacy Ready-only failure marker | Unchanged by design: today's halt keeps the marker | `TestLegacyReadyOnlyBuildFailureIsNotHeld` |
| Disk restore in progress | Unchanged, deliberately: the restore wins | `TestWakeOverFailedBuildWaitsForDiskRestore` |
| Missing repo (`BadSpec`) | Unchanged: a genuine error | — |
| Canceled release | Not a halt: dispatches with `status.image`. The config gap is `w1/m152` (blocked) | — |
| Background worker | Needs work | `w1/m158` |
| Cron job, static site, direct static publish | Cannot occur: no Deployment | — |
| First release, opensandbox runtime | Unchanged: nothing serves, or no Deployment | — |

## Live verification (2026-09-15, production)

**Rollout.** Deploy run 35021555678 for `4f0c2c3e2`; images pinned at `82fed6791` (21:27:20Z).

**Fixtures.** Free web services from `examples/hello-go` (docker) in the QA workspace:

- `qa-20260915-m157a` (`srv-dakqtorlse3s739rlnj0`, `MESSAGE=m157-failed-build`). First deploy live at 20:49:53Z. A PATCH of `envSpecificDetails.dockerfilePath` to `./Dockerfile.qa-missing` opened `dep-dakqusjlse3s739rlnmg`, which read `build_failed` at 20:50:57Z. The phase stayed Running and the URL kept answering `200 m157-failed-build`.
- `qa-20260915-m157b` (`srv-dakqtrjlse3s739rlnkg`, `MESSAGE=m157-inflight`). Live at 20:50:30Z, then left without requests.

**Render parity (t004, 21:30:01Z).** `qa-20260915-m157a`, serving its first release over the failed build on the pinned operator:

| Surface | Service | Failed deploy |
| --- | --- | --- |
| REST `GET /v1/services/srv-dakqtorlse3s739rlnj0`, `…/deploys/dep-dakqusjlse3s739rlnmg` | `phase: Running`, `suspended: not_suspended` | `status: build_failed`, `failureReason` "build failed in the docker build step: … load build definition from ./Dockerfile.qa-missing …" |
| GraphQL `deploy(serviceId, deployId)` | — | `status: build_failed`, `trigger: config_change`, same `failureReason` |
| MCP `get_service` / `get_deploy` | `phase: Running`, `suspended: not_suspended` | `status: build_failed`, same `failureReason` |

- **Agreement.** The surfaces agree, which is Render's "the previous deploy stays live". ADR018 records no divergence.
- **Not compared: the dashboard header.** It reads the same GraphQL fields.

**DoD: resume over a failed build (t003).** On `qa-20260915-m157a`, with its newest deploy `dep-dakqusjlse3s739rlnmg` at `build_failed`:

```text
21:33:05.0    phase Running; deploys dep-dakqusjlse3s739rlnmg build_failed, dep-dakqtorlse3s739rlnjg live; URL 200 m157-failed-build
21:33:05.4    POST /suspend → 202; 21:33:23 phase Hibernated, suspended; 21:33:33.6 URL 503 no available server
21:33:34.1    POST /resume → 202
21:33:34.7    URL (every 1 s from here): 503 {"error":"service hibernated","retryAfter":5}
21:33:45.7    URL: 200 m157-failed-build  (8 requests, 11.6 s after the resume)
21:33:49.4    phase Running, not_suspended; deploys unchanged (build_failed; the first release live)
```

- **Before the fix,** the same sequence on `qa-20260915-m156b` gave 112 of 113 `503`s over 180 s, still `503` 4.5 minutes later.
- **The failed release stayed off the pod.** The deploy stays `build_failed` and no new deploy opened. The URL answers the first release's `MESSAGE`.

**DoD: idle sleep and wake over a failed build (t003).** `qa-20260915-m157a` stayed awake over `dep-dakqusjlse3s739rlnmg` (`build_failed`) after the resume.

- **Scanner traffic.** Requests from the outside scanner (`10.10.0.7`) arrived at 21:36, 21:47, 21:50, 21:59, 22:02 and 22:06 and kept resetting the idle clock.
- **The check.** A 5 s phase poll caught the sleep and requested the URL at once:

```text
22:21:46.7    phase Hibernated (15 min after the 22:06 scanner request)
22:21:47.6    URL (every 1 s from here): 503 {"error":"service hibernated","retryAfter":5}
22:21:58.7    URL: 200 m157-failed-build  (8 requests, 11.7 s after the first)
22:22:04.3    phase Running; deploys unchanged (build_failed; the first release live)
```

- **Before the fix,** the idle check never ran while a build halted (§ Root cause), and a service parked over a failed build could not wake.

**DoD: a wake while a build runs (t003).** `qa-20260915-m157b`, on a clean run that made no request until the build was running:

```text
22:37:50.7    phase Hibernated
22:37:52.3    POST /deploys {clearCache: clear} → dep-dakshg031mas7389obo0
22:38:08.8    deploy build_in_progress; phase Building; still no request sent
22:38:09.5    URL (every 1 s from here): 503 {"error":"service hibernated","retryAfter":5}
22:38:20.4    URL: 200 m157-inflight  (8 requests, 12.0 s after the first)
22:38:21.2    the deploy is still build_in_progress at that first 200; phase Building
22:39:14.2    dep-dakshg031mas7389obo0 live; phase Running; URL 200 m157-inflight
```

The wake therefore starts while the newer release has no image, which is the state that halted every reconcile before the fix. The prior release answers well inside the 60 s the DoD allows.

**Control: a successful build still rolls out, over a waking service (t003).** `qa-20260915-m157b`:

```text
22:21:49.4    phase Hibernated
22:21:50.9    POST /deploys {clearCache: clear} → dep-daks9vg31mas7389objg (deploy_started 22:21:50; build_started 22:22:08)
22:21:51.5    URL (every 1 s from here): 503 {"error":"service hibernated","retryAfter":5}
22:22:01.0    URL: 200 m157-inflight  (7 requests, 10.4 s); the deploy still queued, phase Building
22:23:14.8    dep-daks9vg31mas7389objg live; phase Running; URL 200 m157-inflight
```

- **What this shows.** The prior release keeps serving while a newer release waits for its build, and the build then rolls out normally.
- **What it does not show.** A separate probe hit the URL at 22:21:47, before the deploy was created, and probably started the wake on the normal path. So this run does not prove a wake that starts during a build.
- **Rerun.** A clean run triggers the deploy first and makes no request until it reads `build_in_progress`.

## Adjacent classes

- **`w1/101`.** "A prior release exists" is decided by `status.image` in the failure-phase settle. This hold keys on `status.activeRevision`, as `w1/m156` does.
- **`w1/m152` (blocked).** A canceled config-change release still ships its config.

## Dedupe

- `w1/done/104.md`: this milestone's source, promoted.
- `w1/done/m156`: the pre-deploy analogue. Its `convergeServingRuntime` and `ingressRoutes` are reused.
- `w6/m124`: the phase over a failed build (Running or Hibernated), which stays.
- `w2/m82` t006: the terminal build gate that stops a failed build re-running, which stays.

## Source + Goal linkage

- **Source:** `w1/104`, promoted 2026-09-15 by user decision during `/loop-worker w1`, after it reproduced live in `w1/m156` t003.
- **Goal linkage:**
  - `docs/ADR004-app-deployment.md`: a failed deploy leaves the previous revision serving;
  - `docs/ADR018-render-parity.md`: Render keeps the previous deploy live after a failed deploy, a free service spins up on the next request, and suspend and resume act immediately;
  - `w6/m116`: free-service defaults.
- **Expected outcome:** a failed or in-flight build never takes the serving release offline. Sleep, wake, suspend, resume and scale keep working, and the phase matches what the URL does.
- **Why now:** the most common deploy failure, a broken build, silently and permanently takes a service offline after an ordinary suspend or sleep, and it reproduced on production. `w1/m156` fixed the pre-deploy half, so this closes the class.
- **Render parity is included** because the phase and wake behavior surface through REST, GraphQL, MCP and the dashboard header.
