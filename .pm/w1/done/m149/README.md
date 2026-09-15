# w1 · m149 — A failed pre-deploy command marks a still-serving service Failed, and explains the failure wrongly

**Worker:** worker1 **Goal:** when a pre-deploy command fails on a service that already has a healthy release, the service keeps reporting what is actually serving (Running) and only the deploy reads failed. That failed deploy says truthfully why it failed: the command's exit code and a pointer to its logs. It never says "did not finish within its window" about an immediate exit, and never shows raw Kubernetes Job text. **Status:** done (2026-09-15). Every task is complete, and the definition of done passed live on production (images pinned to `c4212ec71`). The wake gap found during verification is filed as w1/m156.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | A pre-deploy failure over a healthy prior release keeps the service phase on what is serving (mirror `settleFailedBuildOverPriorRelease`) — **DONE** | 60m | — |
| t002 | The failed deploy's reason is deterministic and human: the exit code plus the log pointer, never timeout text or `BackoffLimitExceeded` — **DONE** | 60m | — |
| t003 | Pre-deploy log records carry a `predeploy` type label (a line returned for `type=predeploy` is labelled `app`) — **DONE** | 20m | — |
| t004 | Render parity — **DONE** | 30m | t001, t002, t003 |
| t005 | Simplify — **DONE** | 20m | t004 |
| t006 | Test coverage — **DONE** | 45m | t004 |
| t007 | Closeout — **DONE** | 10m | t006 |

## Definition of done

Each bullet can be repeated on a throwaway free web service (`bex-co/bex` `examples/hello-go`, docker) that is live and returning `200`. Only states observed at filing time are listed:

- **The service stays Running.** Settings → **Edit Pre-Deploy Command** → `echo qa-pd-marker; exit 3` → **Save changes**. The resulting `config_change` deploy reaches `pre_deploy_failed`. The service's `phase` and the dashboard header read **Running**, and the public URL keeps returning `200`. At filing time the phase was **`Failed`**, the header read **"Failed · Latest deploy Pre-Deploy Failed"**, and `curl` returned `200` throughout.
- **No false Failed while the pre-deploy is still running.** While that deploy's `preDeployStatus` is `running`, the service phase is not `Failed`. At filing time it was `Failed` at 12:17:38Z with `preDeployStatus: running`, then flapped to `Deploying` after the deploy failed, and back to `Failed`.
- **The reason names the exit code.** The failed deploy's `failureReason` names the command's exit code (`3`) and points at the pre-deploy logs, on every run. At filing time, two identical failures got two different wrong reasons (Evidence 2 and 4).
- **The pre-deploy output is readable and labelled.** `GET /v1/logs?resource=<srv>&type=predeploy&text=qa-pd-marker` returns the marker line. That passed at filing time; this milestone must keep it and label the record `type: predeploy`. At filing time the returned record was labelled `app`.

## Evidence (probes run 2026-09-14, production, workspace `bex` / `tea-d98210cbbpdc73dcrkvg`)

Fixture: free web service `qa-20260914-pd` (`srv-dajua78gsm7s73f64cc0`), created 12:14:25Z. It went live at 12:16:47Z (first deploy `dep-dajua78gsm7s73f64ccg`) and was deleted inside the run. At 12:17:11Z its URL returned `200 OK`.

1. **Failing command.** Settings → Pre-Deploy Command `exit 3` → Save. `setPreDeployCommand` returned 200 and the toast said "Pre-Deploy Command updated." That opened `dep-dajubl26m8ac739r5rng` (`config_change`).
2. **What the API and header showed over time:**

   ```text
   12:17:38 phase=Failed    dep …rng: pre_deploy_in_progress, preDeployStatus=running
   12:17:49 phase=Deploying dep …rng: pre_deploy_failed, reason="the pre-deploy command did not finish within its window; check the pre-deploy logs"
   12:18:27 → 12:19:55 (9 samples) phase=Failed, header "qa-20260914-pd · Service · Failed · Latest deploy · Pre-Deploy Failed", same reason
   12:20:02, 12:20:03, 12:20:03  curl https://qa-20260914-pd.onbex.co/ → 200 ×3
   ```

   `exit 3` returns immediately. The reason is the **timeout** text (`timedOutDeployReason`) and it never corrected itself. `type=predeploy` logs returned 0 lines, which is expected because `exit 3` prints nothing.

3. **Second failing command.** `echo qa-pd-marker; exit 3` opened `dep-dajud8ggsm7s73f64cm0` (`config_change`). At 12:21:03Z it was `pre_deploy_failed`, `preDeployStatus=failed`, and the phase was `Failed`.
4. **Its reason** was `"pre-deploy command failed: BackoffLimitExceeded: Job has reached the specified backoff lim…"`: Kubernetes Job internals, with no exit code. `GET /v1/logs?…&type=predeploy&text=qa-pd-marker` returned `2026-09-14T12:20:52.214941886Z qa-pd-marker` with the record's `type` label reading **`app`**. The same text query without `type` returned 0 lines.
5. **Render's contract**, from `render.com/docs/deploys` (fetched 2026-09-14): "If any command fails or times out, the entire deploy fails. Any remaining commands do not run. Your service continues running its most recent successful deploy (if any), with zero downtime." Also: "The pre-deploy command is available for paid web services, private services, and background workers."

No screenshots were taken; the transcripts above are the evidence.

## Root cause

- **Phase.** `reconcilePreDeploy` → `failPreDeploy` → `r.fail(ctx, app, "PreDeployFailed", err)` sets `app.Status.Phase = PhaseFailed` unconditionally (`lego/operator/internal/controller/app_controller.go:4516-4524`, `:4327`). The build-failure path already refuses to do that over a healthy prior release: `r.fail` calls `settleFailedBuildOverPriorRelease` (`:4323`, defined at `:4342`, from `w6/m124`). The pre-deploy path never gets the same treatment. Meanwhile the pending/running branch sets `PhaseDeploying` (`:4502-4514`), which is what produces the flap.
- **Reason.** The store closes the row through `failureReasonFor` (`lego/backend/internal/store/reconciler.go:1310-1339`). It returns the operator's Ready-condition message only when that condition's `ObservedGeneration == app.Generation` (`:1313`); otherwise it falls through to `timedOutDeployReason`, whose `DeployPreDeployFailed` text is "the pre-deploy command did not finish within its window" (`:1341-1346`). `setNotReadyCondition` stamps `obj.GetGeneration()` (`app_controller.go:4262`), so a pure mismatch should not happen. The first probe's timeout text is most likely an **ordering race** (the row closed before the matching condition was observed). **This is unverified**, and t002 reproduces it first. When the message does get through, it is the Job condition's text (`BackoffLimitExceeded: …`). No code under `lego/operator/internal/predeploy/` reads the container's exit code.

## Blast radius

- **The phase rule** must distinguish "failed over a healthy prior release" (stay Running) from "first-ever release failed" (Failed, as `w4/m103` t002 records). The build and rollout paths are the controls: build failure over a prior release already stays Running (`w6/m124`), and rollout failure is handled by `w4/m103`.
- **The reason fix** reaches every surface that reads `deploys.failureReason`: REST `GET /v1/services/{id}/deploys`, GraphQL `deploys { failureReason }`, MCP, and the dashboard deploy header and list (which render `DeployFailureReason`). The store's timeout fallback is shared with build and rollout rows (`timedOutDeployReason`), and their honest timeout wording must not regress.
- **Pre-deploy log type** covers REST/GraphQL/MCP log reads and the dashboard log type filter.

## Adjacent classes

- **Pre-deploy timeout** (a command that really exceeds its window) must still say it timed out, with the window named.
- **Pre-deploy cancel superseded by a newer deploy** (the third `failPreDeploy` path) must not read as a command failure.
- **Free plan.** bex allows a pre-deploy command on free web services (`SetPreDeployCommand` refuses only cron jobs and static sites, `lego/backend/internal/apps/service.go:3413-3414`). Render reserves it for paid services. Record this as a deliberate divergence, or gate it, in t004; it is not a defect by itself.

## Unverified (reasoned, not probed this run)

- **Which of the two reason paths runs is timing-dependent.** Only two failures were observed, and the exact mechanism of the timeout text is not proven.
- **A first-ever deploy whose pre-deploy fails** (no prior release) was not exercised.
- **REST and MCP** reads of the same deploys were not compared, only GraphQL and the dashboard.

## Implementation (2026-09-14)

**Phase (t001).** `failPreDeploy` (`lego/operator/internal/controller/app_controller.go`) now applies w6/m124's rule: when a release has served (`status.activeRevision` set — not `status.image`, which a first release that crash-looped leaves behind without ever serving) it calls `settleFailureOverPriorRelease` — the build path's settle, generalized to name the failed step — so the phase is Running (Hibernated when parked) with Ready `True/PriorReleaseServing` ("the latest pre-deploy command failed; the previously deployed release keeps serving"). A first release still reads Failed with Ready `False/PreDeployFailed`. The pending/running branch is unchanged (`Deploying`, never `Failed`); the filing-time `Failed` sample at 12:17:38Z is most likely the CR already failed while the deploy row still read `pre_deploy_in_progress` (the store polls), which the settle removes.

**Reason (t002).** Two changes make it deterministic:

- **Operator:** `predeploy.FailureMessage` reads the Job pod's terminated `predeploy` container: "the pre-deploy command exited with code N; check the pre-deploy logs", "…ran out of memory (exit code 137)…", or, for a Job reaped by its deadline, "…did not finish within its 10-minute window…". With no terminated container (pod reaped, never started) it says "the pre-deploy command failed; check the pre-deploy logs". The Job controller's `BackoffLimitExceeded` text is never used. A step bex could not start (superseding an older run or the network policy failed) reads "the pre-deploy command could not be started: …". `status.preDeploy.message` carries it, and `Observe` now returns only the state.
- **Store:** `deployCloseFailureReason` reads that durable, release-generation-scoped verdict (`recordedPreDeployFailure`) before the Ready condition, as build rows read the Build condition (w6/m100). The timeout text came from `failureReasonFor` requiring `Ready.observedGeneration == metadata.generation`: once the generation moved past the condition (or Ready described the prior release) it fell through to `timedOutDeployReason`. `TestPreDeployFailureReasonSurvivesAGenerationBump` reproduces exactly that input. Whether production's generation bump came from a projector write was not traced; the fix removes the dependency either way.

**Log type (t003).** `collectPreDeployLogs` (`lego/backend/internal/logs/service.go`) stamps `type: predeploy` on each record (it went through `parseLogLine`, which hard-codes `app`). Decision on untyped queries: unchanged — they do not include pre-deploy lines. `predeploy` is a live Job-pod read that must be requested on its own (mixing it with app/request is already refused), and Render shows deploy output apart from the service's log stream.

**Parity (t004, docs).** ADR018 row 71 and ADR004 § Pre-deploy command record the phase rule, the reason wording and the log label. **Free plan: divergence recorded, not gated.** Render offers the pre-deploy command only on paid web services, private services and background workers; bex allows it on free web services too. Gating it is a pricing decision and would break free services already using it, so the behavior is unchanged and flagged for the owner. The live per-surface table (REST, GraphQL, MCP, dashboard) waits for the deploy.

**Simplify (t005).** Three review passes (reuse, quality, efficiency). Applied:

- `settleFailedRolloutOverPriorRelease` was a copy of the generalized settle and is gone; the rollout path calls `settleFailureOverPriorRelease(ctx, app, "the latest rollout failed")`.
- The prior-release test for pre-deploy is `status.activeRevision` only (see t001), with a test for the crash-looped first release.
- The persisted-verdict re-check returns a nil error after re-stamping, like the build verdict gate: a terminal pre-deploy failure used to requeue on error backoff forever. The first failing pass still surfaces the error once.
- The Job's pod is found by `batch.kubernetes.io/controller-uid`, not the Job name, so a same-named earlier Job's pod can't supply the exit code.
- `readContainerLogs` takes the log type, so pre-deploy and datastore records are labelled (type and container) when parsed instead of patched afterwards; `parseLogLine` is gone. Before this, pre-deploy records also carried `container: app`.

Declined:

- A shared Job-pod lookup across `build`, `publish` and `predeploy` — it would reach into two unrelated packages; each builds a different message.
- An `OOMKilled` constant shared with `build/signals.go`.
- Dropping the timeout-wording control test (t002 asks for it).
- The build path's own `status.image != ""` prior-release check has the crash-looped-first-release edge the review found; it is w6/m124's code, filed as `w1/101` rather than changed here.

**Tests (t006).**

- Operator: `controller/predeploy_prior_release_phase_test.go` (Running over a serving image; Running over an active revision; Hibernated when parked; first release Failed with the step's message; could-not-start wording) and `predeploy/failure_message_test.go` (exit code, OOM, deadline with its window, no terminated pod / waiting / other namespace / other Job's pod). The envtest spec "fails the deploy and leaves the previous revision serving" now stands in a Job pod exiting 1 and expects phase Running and "the pre-deploy command exited with code 1; check the pre-deploy logs", stable across a second reconcile.
- Store: `failed_predeploy_reason_test.go` (the verdict wins with the phase held Running, the generation-bump case, a verdict from another release is ignored, service state Running/healthy, build/pre-deploy/rollout timeout lines unchanged).
- Logs: `TestPreDeployLogsReadFromJobPod` pins `type: predeploy`.
- **Shown failing on pre-fix `HEAD` (`0299e0e70`) in a scratch worktree:** the store tests (reason = the timeout line, or empty), the logs test (`labelled type="app"`), and the envtest spec (phase not Running). The controller unit tests and `FailureMessage` tests call the new signatures and cannot compile pre-fix.

## Live verification (2026-09-15, production pinned to `c4212ec71`)

- **Fixture.** `qa-20260915-m149` (`srv-dakeca15v75s738uf84g`), a free web service from `examples/hello-go` (docker) with `MESSAGE=m149-predeploy`. Its first deploy went live at 06:42:54Z.
- **Before (still production `137a5186e`), 06:44:49Z.** `PATCH serviceDetails.preDeployCommand "echo qa-pd-marker; exit 3"` produced `dep-dakeio81e15c73bfgpv0`. At 06:44:56Z the phase read `Failed` while `pre_deploy_in_progress`. At 06:45:13Z the deploy was `pre_deploy_failed`, the phase `Failed`, and the reason "pre-deploy command failed: BackoffLimitExceeded: Job has reached the specified backoff limit". The URL answered 234 of 235 one-second samples with `200` between 06:44:42 and 06:51:06Z.
- **After.**

```text
07:30:11Z PATCH "echo qa-pd-marker2; exit 3" (phase before: Running)
07:30:12Z phase Deploying  dep-dakf80pvi8js739uild0 created (config_change)
07:30:19Z phase Running
07:30:26Z phase Running    pre_deploy_failed  reason "the pre-deploy command exited with code 3; check the pre-deploy logs"
07:31:35Z PATCH "echo qa-pd-marker3; exit 3"
07:31:36Z phase Deploying  dep-dakf8lpvi8js739uilfg created
07:31:39Z phase Deploying  pre_deploy_in_progress
07:31:42Z phase Running    pre_deploy_in_progress
07:31:57Z phase Running    pre_deploy_failed  same reason (deterministic across runs)
GET /v1/logs?type=predeploy&text=qa-pd-marker2 → "qa-pd-marker2" labelled type=predeploy, container=predeploy
07:32Z dashboard header: "Service Running" · "Latest deploy: Pre-Deploy Failed"
08:04Z GraphQL deploy(serviceId, deployId) and MCP get_deploy → status pre_deploy_failed, preDeployStatus failed, the same failureReason; MCP get_service → phase Running
```

- **Every DoD bullet passed.**
  - The service stays Running, and is never Failed while the pre-deploy runs.
  - The reason names exit code 3 and points at the logs, identically on two runs and on REST, GraphQL and MCP.
  - The pre-deploy record is labelled `predeploy`.
- **URL during the post-fix runs.** It answered the activator's `503 service hibernated`: the fixture had gone to sleep at 07:07:01Z, before the rollout, and the stored failed verdict kept the reconcile from ever waking it. The gate predates this milestone. m149 only changed the phase to Running, which makes that state misleading, so the gap is filed as w1/m156 with the evidence.

## Dedupe

- `w4/done/m103` states the invariant "a rollout failure over a healthy prior release does NOT change the phase", and its t002 table records the pre-deploy failure as landing on `PhaseFailed` via `failPreDeploy` → `fail`. That is recorded current behavior, not a decision that pre-deploy should differ from `w6/m124`'s rule for builds.
- `w6/done/m123` fixed failed **build** reasons showing Kubernetes internals and a 0s duration. Pre-deploy is the untouched sibling.
- `w6/done/m95` introduced `timedOutDeployReason` for timed-out rows, and does not cover an immediate exit being labelled a timeout.
- `w6/done/m51` made Settings changes (including pre-deploy) open deploy rows, which worked here.
- No open item covers any of this.
- `.pm/DO_NOT_DO.md`: no conflict.
- Not fixed on `main` as of `5c7435187`.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `https://dashboard.bex.co`, 2026-09-14 pass 10, journey 14 (break a deploy on purpose: the failure UX and reasons tell the truth), exercised through the Pre-Deploy Command (`docs/ADR018-render-parity.md` row 71, `w1/done/m33`).
- **Goal linkage:** pillar 1, Render-compatible deploy lifecycle (`docs/ADR004-app-deployment.md`), and the service/deploy status invariant `w6/m124` and `w4/m103` established.
- **Expected outcome:** a migration step that fails leaves the service reading Running while it keeps serving, the failed deploy says "exited with code N — see pre-deploy logs", and those logs are labelled so the filter finds them.
- **Why now:** pre-deploy commands exist for migrations, which is the most common reason a deploy is expected to fail safely. Today that safe failure paints a working service red, and the reason text sends the user looking for a timeout that never happened.
- **Render parity:** included. The fix changes service phase and deploy reason on REST, GraphQL and MCP and in the dashboard, and t004 records the free-plan difference.
