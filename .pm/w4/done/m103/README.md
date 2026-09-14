# w4 · m103 — A first deploy whose rollout fails leaves the service reading "Deploying" forever, while its deploy reads "Failed"

**Worker:** worker4 **Goal:** a readiness/crash-loop failure settles the service's own phase the way a build failure already does, so the header never contradicts the deploy row — the third case in the taxonomy `w6/m52` and `w6/m124` built. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Settle the App phase when the rollout budget expires with nothing ever served — **DONE** | 60m | — |
| t002 | Place the adjacent cases: prior release, hibernated, suspended, cancel, pre-deploy — **DONE** | 45m | w4/m103/t001 |
| t007 | Cover the image-pull class, closed by the 18m observer not the 900s budget — **DONE** | 40m | w4/m103/t001 |
| t003 | Render parity sweep over the changed surfaces — **DONE** | 30m | w4/m103/t002, w4/m103/t007 |
| t004 | Simplify pass over this milestone's changes — **DONE** | 25m | w4/m103/t003 |
| t005 | Test coverage for the shipped behavior — **DONE** | 40m | w4/m103/t003 |
| t006 | Closeout — **DONE** | 15m | w4/m103/t005 |

## Definition of done

- **A crash-looping first deploy settles the phase.** Create a free web service whose container cannot start (`dockerCommand: /bin/qa-nonexistent-binary` on `examples/hello-go`), wait out the 900s rollout budget, then `GET /v1/services/<id>`: `phase` is a terminal state consistent with "nothing is serving", and the dashboard header no longer shows **Service Deploying** beside **Latest deploy Failed**. Measured today: the deploy reached `update_failed` at 898s after rollout start and the phase was still `Deploying` 4 minutes later, on REST, GraphQL **and** the UI.
- **The deploy row's verdict is unchanged.** `failureReason` still reads exactly as it does today — verbatim: _"container exited shortly after start and is restarting repeatedly (last exit code 127) — check the service logs for the crash output. If the crash is a port bind: the process must listen on $PORT (3000), and tenant containers cannot bind ports below 1024 (all Linux capabilities are dropped)."_ This milestone changes the **phase**, not the diagnosis, which is already excellent.
- **A rollout failure over a healthy prior release does NOT change the phase.** The service keeps reading the state that describes what is actually serving, exactly as `w6/m124` established for build failures via `settleFailedBuildOverPriorRelease`. Asserted as the control case — this is the half that must not regress.
- **Every adjacent case is stated and tested**, not left implicit: cancel with no prior release (`PhaseCanceled`, `w6/m52`), build failure with no prior release (`PhaseFailed`, verified by `w6/m124`'s own control), pre-deploy failure (`failPreDeploy` → `fail`), suspended, and free-tier hibernated. One test per case, naming the phase each lands on.
- **The two clocks agree.** Whatever closes the deploy row at the rollout budget and whatever settles the phase reach their verdict for the same release, so no ordering leaves the pair contradictory for more than one reconcile.
- **A second failure class settles too, and a rollout row can close on either of two timers.** An **unpullable image** reproduces the same stuck phase: measured 2026-09-13 pass 20, `phase` stayed `"Deploying"` after the row reached `update_failed`. Its row closed at **1091s after `updatedAt`** — bex-api's 18-minute `DeployGateTimeout`, not the 900s rollout budget, because `deployTimedOut` (`store/reconciler.go:1047-1065`) gives an `update_in_progress` row the deploy gate. The crash-loop fixture above closed at 898s, so the two classes take different paths to the same contradiction and the phase must settle under both. The verdict text needs no change — `failureReason` already reads "image pull is failing: … failed to resolve image: … not found". See t007.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `https://dashboard.bex.co`, 2026-09-13 pass 5, journey 14. Fixture `qa-0913e-bad` (`srv-daj9umggsm7s73f63olg`), free web service from `github.com/bex-co/bex` `examples/hello-go`, docker runtime, created with `dockerCommand: /bin/qa-nonexistent-binary` and deleted at end of run. Timeline, responses and UI text are in t001.
- **Goal linkage:** `docs/ADR004-app-deployment.md` owns the deploy lifecycle; `lego/types/v1alpha1/app_types.go:1127-1149` is where the phase taxonomy and its rationale live. The shared goal of `w6/m52` ("a service's own status always agrees with its deploy history") and `w6/m124` ("a service that is serving reports that it is serving… while its failed deploy stays visible as a deploy fact") is the invariant this breaks.
- **Expected outcome:** a user whose first deploy crash-loops is told it failed — once — instead of watching a spinner-flavoured "Deploying" badge indefinitely beside a deploy row that already said Failed.
- **Why now:** it is the default outcome of the single most common first-time mistake (a start command that does not exist), it is silent and permanent, and the two milestones that built this taxonomy left exactly this branch uncovered because both were about build outcomes.
- **Render parity task included:** yes — `phase` is projected on REST, GraphQL and MCP and drives the dashboard header badge.

## Closeout evidence (2026-09-14)

Code + suite verification (production re-probe of the fixed phase rides this ship's deploy; pre-fix transcripts remain in t001 / pass 8 / pass 20):

- Operator: `ProgressDeadlineExceeded` → `settleFailedRollout` — first deploy (`ActiveRevision` empty) → `PhaseFailed` with CrashLoopBackOff / ImagePullBackOff Ready message (`TestFirstDeployRolloutDeadlineSettlesFailedCrashLoop`, `…ImagePull`); prior release → `Running` / parked → `Hibernated`.
- Deploying stamp skipped once the Deployment already reports the deadline, so the header does not flap.
- Docs: ADR004 three-timer paragraph + ADR018 Get-service row; phase taxonomy comment covers the rollout path.
- t007 decision: **(b)** — keep budgets; mid-rollout pull diagnosis already on Ready; open-row `failureReason` surface is a documented follow-up, not a timer change. **(a)** fail-fast needs a Render capture first.
- Residual: none filed — follow-up called out in ADR004 only.
