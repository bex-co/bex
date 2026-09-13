# w4 · m103 — A first deploy whose rollout fails leaves the service reading "Deploying" forever, while its deploy reads "Failed"

**Worker:** worker4 **Goal:** a readiness/crash-loop failure settles the service's own phase the way a build failure already does, so the header never contradicts the deploy row — the third case in the taxonomy `w6/m52` and `w6/m124` built. **Status:** todo

## Tasks (in order)

| id   | title                                                                              | est | depends_on   |
| ---- | ---------------------------------------------------------------------------------- | --- | ------------ |
| t001 | Settle the App phase when the rollout budget expires with nothing ever served        | 60m | —            |
| t002 | Place the adjacent cases: prior release, hibernated, suspended, cancel, pre-deploy    | 45m | w4/m103/t001 |
| t003 | Render parity sweep over the changed surfaces                                        | 30m | w4/m103/t002 |
| t004 | Simplify pass over this milestone's changes                                          | 25m | w4/m103/t003 |
| t005 | Test coverage for the shipped behavior                                               | 40m | w4/m103/t003 |
| t006 | Closeout                                                                            | 15m | w4/m103/t005 |

## Definition of done

- **A crash-looping first deploy settles the phase.** Create a free web service whose container cannot start (`dockerCommand: /bin/qa-nonexistent-binary` on `examples/hello-go`), wait out the 900s rollout budget, then `GET /v1/services/<id>`: `phase` is a terminal state consistent with "nothing is serving", and the dashboard header no longer shows **Service Deploying** beside **Latest deploy Failed**. Measured today: the deploy reached `update_failed` at 898s after rollout start and the phase was still `Deploying` 4 minutes later, on REST, GraphQL **and** the UI.
- **The deploy row's verdict is unchanged.** `failureReason` still reads exactly as it does today — verbatim: _"container exited shortly after start and is restarting repeatedly (last exit code 127) — check the service logs for the crash output. If the crash is a port bind: the process must listen on $PORT (3000), and tenant containers cannot bind ports below 1024 (all Linux capabilities are dropped)."_ This milestone changes the **phase**, not the diagnosis, which is already excellent.
- **A rollout failure over a healthy prior release does NOT change the phase.** The service keeps reading the state that describes what is actually serving, exactly as `w6/m124` established for build failures via `settleFailedBuildOverPriorRelease`. Asserted as the control case — this is the half that must not regress.
- **Every adjacent case is stated and tested**, not left implicit: cancel with no prior release (`PhaseCanceled`, `w6/m52`), build failure with no prior release (`PhaseFailed`, verified by `w6/m124`'s own control), pre-deploy failure (`failPreDeploy` → `fail`), suspended, and free-tier hibernated. One test per case, naming the phase each lands on.
- **The two clocks agree.** Whatever closes the deploy row at the rollout budget and whatever settles the phase reach their verdict for the same release, so no ordering leaves the pair contradictory for more than one reconcile.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `https://dashboard.bex.co`, 2026-09-13 pass 5, journey 14. Fixture `qa-0913e-bad` (`srv-daj9umggsm7s73f63olg`), free web service from `github.com/bex-co/bex` `examples/hello-go`, docker runtime, created with `dockerCommand: /bin/qa-nonexistent-binary` and deleted at end of run. Timeline, responses and UI text are in t001.
- **Goal linkage:** `docs/ADR004-app-deployment.md` owns the deploy lifecycle; `lego/types/v1alpha1/app_types.go:1127-1149` is where the phase taxonomy and its rationale live. The shared goal of `w6/m52` ("a service's own status always agrees with its deploy history") and `w6/m124` ("a service that is serving reports that it is serving… while its failed deploy stays visible as a deploy fact") is the invariant this breaks.
- **Expected outcome:** a user whose first deploy crash-loops is told it failed — once — instead of watching a spinner-flavoured "Deploying" badge indefinitely beside a deploy row that already said Failed.
- **Why now:** it is the default outcome of the single most common first-time mistake (a start command that does not exist), it is silent and permanent, and the two milestones that built this taxonomy left exactly this branch uncovered because both were about build outcomes.
- **Render parity task included:** yes — `phase` is projected on REST, GraphQL and MCP and drives the dashboard header badge.

## Dedupe

- **`w6/m52` (done)** introduced `PhaseCanceled` because reusing `PhaseFailed` for a user cancel "made a service contradict its own deploy history — the deploy read 'canceled' while the service header read 'Failed'". Same invariant, different branch; not a duplicate.
- **`w6/m124` (done)** fixed the inverse contradiction (serving service reading `Failed` after a failed build) and its DoD verified a **"first-build-fails control reports Failed"**. That is the *build* path on a first deploy; this is the *rollout* path on a first deploy, which neither milestone exercised. Not a regression of m124 — its own assertions still hold.
- `.pm/DO_NOT_DO.md` mentions `status.phase` only in the auto-hibernate anti-goal (#w6/m120), which is about `Hibernated ⇄ Running` wake flapping — unrelated, and this milestone must not disturb it.
- **Not already fixed on `main`:** `app_controller.go:4240` sets `PhaseFailed` only from `r.fail(...)`, which fires on reconcile **errors**. A crash-looping Deployment is not a reconcile error — the operator applied the Deployment successfully — so no path takes the phase out of `Deploying` when the rollout budget expires.

## Verified this run, and not claimed

- **Verified live:** the full timeline (`deploy_started` 13:04:26 → `build_started` 13:05:47 → `build_ended succeeded` 13:06:59 → `deploy_ended` + `update_failed` + `finishedAt` 13:25:17, i.e. 898s after rollout start, matching the documented 900s budget); `failureReason` text; `phase: "Deploying"` on REST and GraphQL at 13:28 and again at 13:29:33; the dashboard header rendering "Service Deploying" next to "Latest deploy Failed" and a deploys row reading "Failed … 19m 29s".
- **Traced in code, not observed:** that `r.fail` is the only writer of `PhaseFailed` and is unreachable for a crash loop. t001 must confirm by instrumenting or by reading the reconcile path for an App whose Deployment is progressing-false, rather than trusting this grep.
- **Not observed:** whether the phase settles eventually on a longer horizon than the ~4 minutes watched (the service was deleted at ~13:30). If it does settle later, this is a latency bug rather than a permanent one and the fix shrinks to closing the gap — t001 must establish which before building on the "forever" framing. **No `server_failed` event was emitted for this service at all**, unlike a crash loop on a service with a prior release, which emitted `server_failed / readiness_failed` within ~40s during pass 4 — that asymmetry is unexplained and worth understanding before choosing the fix's trigger.
- **Not exercised:** notifications and webhooks for this failure (journey 14's third clause), and the logs the failure reason points at.
