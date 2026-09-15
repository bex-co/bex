# w1 · m156 — A hibernated free service whose latest deploy failed never wakes: the failed-release gate halts the reconcile before replicas and routing

**Worker:** worker1 **Goal:** a free service whose newest release failed (its pre-deploy command, or its build) still sleeps and wakes on the release that is actually serving. A request gets the wake response and then the prior release's own reply, and the service never reads Running while its URL can only answer `503 service hibernated`. **Status:** done (2026-09-15). All seven tasks are complete. The pre-deploy hold shipped in `1cb1f2d27`. On production it recovered the stuck fixture, and it passed three live checks: a timed idle-sleep wake over a failed pre-deploy (`200` in 11.9 s), the healthy idle-sleep control (11.5 s), and the REST/GraphQL/MCP comparison. The failed-build variant reproduces live and is carved out to `w1/104`, which owns its fix.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Runtime convergence survives a terminal failed release: a serving prior release still wakes, hibernates, scales and routes while the failed verdict stands — **DONE** | 1h | — |
| t002 | Blast radius: every halt before the Deployment write, against every runtime transition that needs that write — **DONE** | 45m | t001 |
| t003 | Live: reproduce and then verify the wake over a failed pre-deploy (and probe the failed-build variant) on production — **DONE** | 40m | t002 |
| t004 | Render parity — **DONE** | 20m | t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage — **DONE** | 40m | t004 |
| t007 | Closeout — **DONE** | 10m | t006 |

## Definition of done

Repeat on a throwaway free web service (`bex-co/bex` `examples/hello-go`, docker) that is live, returns `200`, and receives no traffic. Only states observed at filing time are listed:

- **It wakes over a failed pre-deploy.**

  1. Set Pre-Deploy Command `echo qa; exit 3`, so the latest deploy reads `pre_deploy_failed`.
  2. Leave the service idle until `service_hibernated`.
  3. Request its URL every second.

  The first requests may get the activator's `503 {"error":"service hibernated","retryAfter":5}`. Within 60 s they get the prior release's own `200`. At filing time 188 of 188 requests over five minutes (07:30:02–07:35:01Z) got that `503`, and so did three more at 07:41:41–50Z, although `service_woken` and `server_available` had been recorded at 07:26:17Z.

- **The phase does not claim Running while the URL cannot serve.** At filing time `GET /v1/services/<srv>` read `phase: Running` and the dashboard header read "Service Running · Latest deploy: Pre-Deploy Failed" throughout those `503`s.
- **The failed release stays unrolled.** After the wake, the pod serves the prior release's response, the failed deploy stays `pre_deploy_failed`, and no new pre-deploy Job runs for that release.
- **Control (must not regress).** A hibernated free service whose latest deploy is live wakes on request. At filing time `qa-20260915-m151ws` answered `503` at 07:30:00 and 07:30:06 and `200` at 07:30:12.

- **It wakes over a failed build. Reproduced 2026-09-15 and owned by `w1/104`, not met by m156.**
  - **Setup.** A free service serves its first release while its newest deploys are `build_failed`.
  - **Observed.** After a suspend and resume it answered `503 {"error":"service hibernated","retryAfter":5}` to 112 of 113 requests over 180 s, and was still `503` 4.5 minutes later, while the phase read Running.
  - **Required.** The prior release's own `200` within 60 s, as the healthy resume control gets in 11.5 s.
  - **Scope.** m156 fixed the held pre-deploy step only; the build-path hold was withdrawn in review (§ Implementation, Simplify). Transcript: § Live verification.

## Evidence (probes run 2026-09-15, production, workspace `bex` / `tea-d98210cbbpdc73dcrkvg`)

- **Fixture:** `qa-20260915-m149` (`srv-dakeca15v75s738uf84g`), a free web service from `examples/hello-go` (docker) with `MESSAGE=m149-predeploy`. Its first deploy went live at 06:42:54Z. It was created to live-verify w1/m149.

```text
06:44:49Z  PATCH serviceDetails.preDeployCommand "echo qa-pd-marker; exit 3" → dep-dakeio81e15c73bfgpv0 pre_deploy_failed 06:45:11Z
06:44:42–06:51:06Z  URL every 1 s: 234/235 `200 m149-predeploy` (one curl connection failure)
07:07:01Z  service_hibernated (no traffic since 06:51:06Z)
07:22:30Z  production images pinned to c4212ec71 (w1/m146–m155)
07:26:17Z  server_available · service_woken — no later hibernation event
07:30:02–07:35:01Z  URL every ~1.6 s: 188/188 `503 {"error":"service hibernated","retryAfter":5}`
07:30:11Z  PATCH "echo qa-pd-marker2; exit 3" → dep-dakf80pvi8js739uild0 pre_deploy_failed 07:30:26Z, phase Running
07:31:35Z  PATCH "echo qa-pd-marker3; exit 3" → dep-dakf8lpvi8js739uilfg pre_deploy_failed 07:31:57Z, phase Running
07:32Z     dashboard header: "Service Running" · "Latest deploy: Pre-Deploy Failed"
07:41:03Z  GET /v1/services/srv-dakeca15v75s738uf84g → phase Running, numInstances 1
07:41:41Z / 07:41:46Z / 07:41:50Z  curl → 503 {"error":"service hibernated","retryAfter":5}
```

- **Controls in the same window.**
  - `qa-20260915-m151ws` (latest deploy live) hibernated at 07:26:17Z and woke on request: `503`, `503`, then `200` at 07:30:12.
  - `qa-20260915-m151` woke at 07:18:01Z and then served steady traffic.

## Root cause

- **The pre-deploy gate halts before the runtime writes.** `Reconcile` computes `desiredReplicas` at `lego/operator/internal/controller/app_controller.go:2022`, where a waking App is no longer `autoHibernating`. It then runs the pre-deploy gate only when `!app.Spec.Suspended && !autoHibernating` (`:2051-2055`), and returns on `halt` before the Deployment `CreateOrUpdate` (`:2057`) and `ingressBackend` (`:2118`).
- **A stored failure halts every pass.** The persisted `PreDeployFailed` branch (`:4395-4400`) calls `failPreDeploy`, which returns `halt=true` on every pass of that release generation (`:4515-4527`).
  - Hibernating works because the gate is skipped while `autoHibernating`, so replicas 0 are written.
  - Waking re-enters the gate and halts, so replicas stay 0 and the Ingress stays on the activator alias until a new release generation.
- **It predates w1/m149.** The same gate sat in the same place before it (`778bdd1aa^:app_controller.go:2042-2046`), and its `failPreDeploy` also halted, via `r.fail`. m149 only changed the reported phase from Failed to Running, which makes this state newly misleading.
- **The build path has the analogous gate.** `buildFromSource` halts on `terminalBuildFailureRecorded` (`:755-756`), and `resolveDeployImage`'s halt returns before `dispatchRuntime` (`:591-594`). A failed build over a hibernated serving release should block the wake the same way; this is traced, not probed.
- **Why the tests missed it.**
  - `TestPreDeployFailureOverParkedReleaseStaysHibernated` (`predeploy_prior_release_phase_test.go:115`) and `TestBuildFailureOverParkedReleaseStaysHibernated` (`failed_build_prior_release_phase_test.go:94`) call `failPreDeploy` or `r.fail` directly on a fake client and pin only the phase.
  - `TestWakeRestoresAppServiceAsIngressBackend` (`auto_hibernate_wake_test.go:199`) wakes an App that has no failed release.
  - No test wakes a parked App whose newest release failed.

## Blast radius

- **Who is hit.** Every free web service whose newest pre-deploy command or build failed, from its next sleep until the owner ships a new release. Auto-hibernation is the created default since `w6/m116`. The dashboard meanwhile reports the service as Running.
- **Other transitions.** Any runtime change that needs the Deployment or Ingress write while a failed verdict stands: resume after suspend, manual scale, autoscale, maintenance, custom domains and the IP allow-list. t002 enumerates them.
- **Paid services.** They do not auto-hibernate, so they keep serving; t002 still checks suspend and resume.

## Adjacent classes

- **Phase truth.** The prior-release settle derives Running or Hibernated from the Deployment's desired replicas. When routing is stuck on the activator, neither is true.
- **Registry credentials.** `convergeRegistryCredentials` (`:598-628`) already runs before the build gate for this reason: "a failed newer build deliberately leaves the previous Deployment serving". Replicas and routing need the same treatment.

## Unverified (reasoned, not probed this run)

- **The failed-build variant.**
- **The 07:26:17Z events.** Whether `service_woken` and `server_available` came from a real scale-up or from a projection of the phase after the operator restarted onto `c4212ec71`.
- **The failed rollout deadline.** Whether a service whose newest rollout failed its deadline is affected. Probably not, because `settleFailedRollout` (`:2698`) runs after the Deployment write.

## Implementation (2026-09-15)

**Runtime convergence over a held pre-deploy step (t001).** All in `lego/operator/internal/controller/app_controller.go`.

- **The invariant.** While a prior release serves, the pod template only advances to a release whose pre-deploy step passed. Every pass that holds a newer release, waking or parking, moves only replicas and routing on the Deployment that already serves the prior release.
- **`holdUnpassedRelease`** runs in `reconcileKubernetes` after `reconcileDiskLifecycle` and before the existing gate. It holds only a web or private service with an active revision, an existing Deployment and Service, and a step that has not passed for the current release (`preDeployPassed`, `hasPreDeployStep`).
  - **A stored failed verdict** converges directly, then settles the phase from the scale that pass wrote (`settlePriorRelease`). The wake pass therefore reads Running, not a stale Hibernated from the cache.
  - **A pending or running step** runs or observes `reconcilePreDeploy`. If the step passes on this pass, the normal rollout proceeds.
  - **While suspended or auto-hibernating** the step does not run, and the parked Deployment keeps the prior template.
- **`convergeServingRuntime`**:
  - patches only `spec.replicas` (a `MergeFrom` patch; the template is never written);
  - routes the Ingress through `ingressBackend` on the prior Service's port, and rewrites it, with its uncached middleware reads, only when the scale changed or `ingressRoutes` finds the live route or hosts differ;
  - parks with `status.image`, or completes the autoscaling transition and keeps the running requeue. It comes back after `wakeReadyPoll` (5 s) only while a woken free service waits on the activator.
- **Unchanged:**
  - a first release;
  - workers (`w1/103`);
  - a release still waiting for its image (`w1/104`);
  - a fresh step failure, which still returns its reconcile error;
  - an App with no Deployment or Service yet.

**Simplify (t005).** Three review passes (reuse, quality, efficiency) over the first version.

- **Withdrawn: the build-path hold.** The first version also held a release still waiting for its image, from `resolveDeployImage`. The review found it unsafe:

  - a parked App with a build in flight lost the build's own requeue, and nothing watches build Jobs, so a finished build could go unobserved for hours while the phase flipped between Building and Hibernated;
  - it ran before `reconcileDiskLifecycle`, so it could scale a service back up during a disk restore;
  - on every 5 s build poll it ran autoscaling and routing without completing the transition or persisting status;
  - its status writes could overwrite a legacy Ready-only build-failure marker.

  The failed-build case stays traced, not fixed. t003 probes it, and `w1/104` records these constraints for the fix.

- **Applied:**
  - a stored failed verdict no longer passes through `failPreDeploy`'s cached settle before the scale; `settleFailureOverPriorRelease` delegates to `settlePriorRelease(…, parked)`;
  - the hold applies only when the prior Service exists (a worker changed to web has none);
  - the Ingress and middleware write is skipped when nothing changed (`ingressRoutes`);
  - the 5 s poll runs only while waking behind the activator, not forever for a prior release that never becomes ready;
  - `completeAutoscalingTransition` and one status write on the awake path;
  - `soonerRequeue` is reused, `hasPreDeployStep` is shared with `reconcileKubernetes`, and a `replicaPlan` struct replaces four adjacent bools;
  - stale comments on the gate and on the settle ("the reconcile quiesces") are fixed, and the new doc comments trimmed;
  - the tests use `predeploy.JobName`, assert the phase on the parking and wake passes, and no longer point at this README.
- **Declined:**
  - extracting the routing tail shared with `reconcileKubernetes`, because the held path adds the skip-when-unchanged check and parks with the prior image;
  - a shared test harness for the setup the three tests repeat;
  - `reconcilePreDeploy` running twice on the pass where a step succeeds, because the second call returns from memory with no I/O.

**Tests (t006).** `wake_over_failed_release_test.go`, on the fake client:

| Test | Pins | With the hold not called |
| --- | --- | --- |
| `TestWakeOverFailedPreDeployRestoresPriorRelease` | Release 1 serves; release 2's failed pre-deploy verdict is stored; parking reads Hibernated; the wake pass scales to 1 and reads Running; the route returns to the App's Service; the template stays on release 1; no Job; the verdict is kept | `woken replicas = 0, want 1` (also failed this way against `c4212ec71`, before any change) |
| `TestParkedPendingPreDeployServesPriorReleaseUntilTheStepPasses` | A pre-deploy command lands while parked: the template stays on release 1 and no Job runs; the wake starts the Job while release 1 serves; the Job completing rolls release 2 | `parked template revision = "rev-2", want the prior release's "rev-0"` |
| `TestSuspendAndResumeOverFailedPreDeployKeepPriorRelease` | Suspend, then resume, over a failed verdict: 0 then 1 replicas on release 1's template, route restored, no Job | `suspended template revision = "rev-2", want the prior release's "rev-0"` |

- **How "hold not called" was produced.** The call site was replaced with `if held, res, err := false, (ctrl.Result{}), error(nil); held {`, the tests were run, and the reworked file was restored and re-run green. An earlier mutation that still called the helpers was discarded as unfaithful.

**Suites.**

- After the rework, `go test ./internal/controller` with envtest passes (61.1 s), including the Ginkgo pre-deploy gate specs and the suspend, resume and hibernate envtests.
- The first version also passed the full `make test`.
- `make lint` reports only the two findings already on main: `backend/internal/api/scope_matrix.go:163` has an unused `writeGraphQLErrors`, and `operator/internal/publish/publish.go:611` has a `modernize` minmax.

**Render parity (t004, docs).**

- `docs/ADR004-app-deployment.md` § Pre-deploy command, "A held release does not freeze the serving one", states the invariant, the parking rule, what changed and what did not (`w1/103`, `w1/104`).
- `docs/ADR018-render-parity.md` row 71 records the same against Render's "continues running its most recent successful deploy".
- **Live cross-surface comparison (10:03:39Z, after the wake).** The m149 fixture over its failed release `dep-dakf8lpvi8js739uilfg`:

| Surface | Service | Failed deploy |
| --- | --- | --- |
| REST `GET /v1/services/srv-dakeca15v75s738uf84g` and `…/deploys/dep-dakf8lpvi8js739uilfg` | `phase: Running`, `suspended: not_suspended` | `status: pre_deploy_failed`, `preDeployStatus: failed`, `failureReason: "the pre-deploy command exited with code 3; check the pre-deploy logs"` |
| GraphQL `deploy(serviceId, deployId)` | — | same three fields, same text |
| MCP `get_service` / `get_deploy` | `phase: Running`, `suspended: not_suspended` | same three fields, same text |
| URL | `200 m149-predeploy` (the prior release) | — |

- **Result.** The surfaces agree, and Running now matches what the URL does. This is Render's "continues running its most recent successful deploy", so ADR018 records no divergence.
- **Not compared.** The dashboard header, because the Playwright browser was disconnected this run. Its header reads the same GraphQL service phase and deploy status.

**Blast radius (t002).** Every `Reconcile` path that returns before the Deployment, Service or Ingress write, crossed with the runtime transitions that need that write: wake, auto-hibernate, suspend, resume, manual scale, autoscale, maintenance on or off, custom domains, and the IP allow-list.

| Halt before the runtime write | Verdict | Evidence |
| --- | --- | --- |
| Pre-deploy step pending or running over a prior release (web, private) | Covered by `holdUnpassedRelease`. Replicas follow `desiredReplicas` (wake, hibernate, suspend, resume, scale, autoscale); routing follows `ingressBackend` (maintenance precedence) and `reconcileIngressWithMiddlewares` (domains, allow-list) | `TestParkedPendingPreDeployServesPriorReleaseUntilTheStepPasses` |
| Pre-deploy step failed over a prior release (web, private) | Covered: same path | `TestWakeOverFailedPreDeployRestoresPriorRelease`, `TestSuspendAndResumeOverFailedPreDeployKeepPriorRelease` |
| A release still waiting for its image: build queued, running, credential wait, or terminal failure (every type) | Needs work: `resolveDeployImage` still halts before the runtime. The first fix was withdrawn after review | `w1/104`; t003 probes the failed-build case live |
| A held pre-deploy step on a background worker | Needs work: suspend, resume and manual scale still freeze behind the halt | `w1/103` |
| Cron job, static site | Cannot occur: no Deployment; their releases dispatch through `reconcileCronJob` / `reconcileStaticSite` | — |
| First release (no active revision) | Unchanged by design: nothing serves yet | `predeploy_test.go`: "gates the rollout on the Job" |
| Canceled release (`settleCanceledRelease`) | Not a halt: it dispatches the runtime with `status.image`. Its config-restore gap is w1/m152, blocked | — |
| Disk lifecycle (`reconcileDiskLifecycle`, a restore in progress) | Unchanged, deliberately: the hold runs after it, and a restore needs the volume detached | — |
| Registry credential gate (`deployRegistryGate`) | Unchanged: transient, holds the whole runtime pass until zot accepts the App's credential | — |
| Namespace guard, finalizer, protected-secret refusal, registry credential errors | Unchanged: genuine errors that set Failed, or no-op guards | — |

## Live verification (2026-09-15, production)

**Rollout.** Deploy run 34950840699 for `1cb1f2d27` succeeded, and the images were pinned at `1df779f8a` (09:57:29Z).

**The stuck fixture recovers (t003).** `qa-20260915-m149` had answered `503 service hibernated` since 07:30Z. Its phase read Running and its latest deploy was `pre_deploy_failed`.

```text
09:23:00–09:58:16Z  URL every ~62 s: 35/35 `503 {"error":"service hibernated","retryAfter":5}`
09:57:29Z           production images pinned to 1cb1f2d27dda (1df779f8a)
10:02:18.524Z       URL (first 1 s sample after the operator rolled): `200 m149-predeploy`
10:02:20Z           GET /v1/services/srv-dakeca15v75s738uf84g → phase Running, numInstances 1
10:02:21Z           latest deploy dep-dakf8lpvi8js739uilfg still pre_deploy_failed; no new release
```

- The operator woke the prior release on its first pass after the rollout. No request was needed, because the stored wake left `desiredReplicas` at 1.
- The events list since 09:00Z is empty. The phase already read Running, so no `service_woken` was projected.

**DoD: a wake over a failed pre-deploy (t003).** The same fixture was left idle with no requests from 10:02:21Z. Its latest deploy was still `dep-dakf8lpvi8js739uilfg`, `pre_deploy_failed`.

```text
10:28:49Z     service_hibernated
10:29:04.600  GET /v1/services/srv-dakeca15v75s738uf84g → phase Hibernated
10:29:05.977  URL (every 1 s from here): 503 {"error":"service hibernated","retryAfter":5}
10:29:17.174  URL: 200 m149-predeploy  (8 requests, 11.9 s after the first; no other responses)
10:29:19Z     service_woken
10:29:22.8    phase Running
10:29:23.4    deploys: dep-dakf8lpvi8js739uilfg pre_deploy_failed, dep-dakf80pvi8js739uild0 pre_deploy_failed (no new release)
```

- **All three DoD bullets hold.** The prior release's own `200` arrives within 60 s. Running is claimed only once the URL serves. The failed release stays unrolled.
- **The pre-deploy Job was not observed directly** (no cluster access). No deploy row opened, and the pod served the prior release's `MESSAGE`.
- At filing time the same state gave 188/188 `503`s over five minutes.

**The failed-build variant reproduces (t003 probe; the fix belongs to `w1/104`).** Fixture: `qa-20260915-m156b` (`srv-dakhhdfr0t2c73fc63gg`), a free web service from `examples/hello-go` with `MESSAGE=m156-build`.

- **Why suspend and resume.** Waiting for an auto-sleep could not conclude, because outside scanner requests (`10.10.0.7`, at 10:24:33, 10:24:49 and 10:34:18Z) kept resetting the idle clock. Resume needs the same Deployment and Ingress write that the build halt returns before (§ Blast radius, the "release still waiting for its image" row).

```text
10:09:22Z     first deploy dep-dakhhdfr0t2c73fc63h0 live; URL 200 m156-build
10:09:23Z     PATCH envSpecificDetails.dockerfilePath ./Dockerfile.qa-missing → 200; that change opened dep-dakhikvqniac73emh130 → build_failed
10:09:25Z     POST /deploys → dep-dakhilfr0t2c73fc63i0 → build_failed 10:10:07Z ("failed to read dockerfile: open ./Dockerfile.qa-missing")
10:10:09Z     phase Running; URL 200 m156-build (the prior release keeps serving)
10:38:45.9    POST /suspend → 202; 10:38:46.9 phase Hibernated, suspended
10:38:57.4    URL 503 no available server
10:38:57.9    POST /resume → 202; 10:39:02Z service_resumed
10:38:58.5–10:41:59  URL every ~1.6 s: 112/113 `503 {"error":"service hibernated","retryAfter":5}`, 1 `503 no available server`; no 200 in 180.9 s
10:41:59Z     phase Running, not_suspended; deploys unchanged (both build_failed)
10:43:25Z     URL 503 {"error":"service hibernated","retryAfter":5}; phase Running (stuck, not slow)
```

- **DoD control: an idle sleep and a wake on a healthy service (must not regress).** Fixture `qa-20260915-m156c` (`srv-dakhuuvqniac73emh18g`); its only deploy went live at 10:37:49Z. The probe sent no requests from 10:37:57Z. Scanner requests kept arriving until at least 10:45:37Z, so the sleep came late.

```text
11:31:46Z     service_hibernated
11:32:04.230  URL (every 1 s from here): 503 {"error":"service hibernated","retryAfter":5}
11:32:15.099  URL: 200 m156-control  (8 requests, 11.5 s after the first)
11:32:20.7    phase Running; latest deploy dep-dakhuuvqniac73emh190 live
```

- **Resume control, same operator build.** The same sequence ran on a fresh healthy fixture, `qa-20260915-m156d` (`srv-daki2d7r0t2c73fc63p0`), whose only deploy was live:

```text
10:45:28.0    first deploy dep-daki2d7r0t2c73fc63pg live; URL 200 m156-resume-control
10:45:28.9    POST /suspend → 202; 10:45:29.9 phase Hibernated, suspended; 10:45:40.5 URL 503 no available server
10:45:41.2    POST /resume → 202
10:45:41.8    URL 503 {"error":"service hibernated","retryAfter":5}
10:45:52.8    URL 200 m156-resume-control  (8 requests, 11.5 s after the resume); phase Running
```

- **Conclusion.** Resume recovers in about 11 s on a healthy service, so `m156b`'s stuck `503`s come from the failed-build halt. That DoD bullet is not met by m156, which fixed the pre-deploy hold only (§ Implementation, Simplify). It is recorded as reproduced in `w1/104`, which owns the fix.

## Dedupe

- **Board search.** `grep -rliE hibernat .pm | xargs grep -liE 'pre-?deploy|failed build|build fail'` finds these, and none covers waking over a failed release:
  - `w4/done/m103`: a first deploy's rollout phase;
  - `w3/done/m19`: the activity timeline;
  - `w6/done/040`: live verification of `m94`'s hibernate routing;
  - `w6/done/m97`: the protected-secret guard;
  - `w6/done/m124`: the build phase rule;
  - `w1/done/074`: RBAC;
  - `w1/101`: a build failure after a first release that never served reads Running.
- **Neighbours.** `w1/m149` covers the phase only; this was found while verifying it live. `w6/m94` is the wake routing hold, and `w2/m82` t006 the terminal build gate.

## Source + Goal linkage

- **Source:** live verification of w1/m146–m155 on production, 2026-09-15 (`/loop-worker w1`), on the w1/m149 fixture.
- **Goal linkage:**
  - `docs/ADR004-app-deployment.md`: a failed deploy leaves the previous revision serving;
  - `docs/ADR018-render-parity.md`: Render keeps the previous deploy live after a failed deploy, and a free service spins up on the next request;
  - `w6/m116`: free-service defaults.
- **Expected outcome:** a free service with a failed newest deploy still wakes and serves its previous release, and its phase matches what its URL does.
- **Why now:** it is silent and permanent. Every free service whose newest pre-deploy or build failed becomes unreachable after its first sleep, while the dashboard says Running, and free is the default plan.
