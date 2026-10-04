# w6 · m147 — A free service that auto-hibernates mid-rollout loses its crash diagnosis (generic health-gate reason) and its header disagrees with its phase

**Worker:** worker6 **Goal:** a failing rollout on a free web service reports the operator's specific diagnosis (crash exit code, probe, image pull) as its `failureReason` even when the service auto-hibernated before the progress deadline, and the dashboard header's phase agrees with the API after that failure. **Status:** in progress (t001, t002, t006, t007 done; t003 traced, live sampling with t004 after the deploy)

## Tasks (in order)

| id   | title                                                                                                     | est | depends_on             |
| ---- | --------------------------------------------------------------------------------------------------------- | --- | ---------------------- |
| t001 | Backend: keep the last real stall diagnosis on the open row and close with it instead of the gate line — **DONE** | 45m | —                      |
| t002 | Operator: settle (or defer the park of) an App whose current-generation rollout is unsettled at idle — **DONE** | 1h  | —                      |
| t003 | Trace the header "Deploying" vs API `Hibernated` disagreement after the failure, and fix the actual cause | 45m | t002                   |
| t004 | Live: reproduce and verify both diagnoses and the header/phase agreement on production                    | 40m | t001, t002, t003       |
| t005 | Render parity                                                                                             | 20m | t004                   |
| t006 | Simplify — **DONE** | 15m | t005                   |
| t007 | Test coverage — **DONE** | 30m | t005                   |
| t008 | Closeout                                                                                                  | 10m | t007                   |

## Definition of done

- On a free web service with a live prior release and no traffic, setting the start command to a missing binary and deploying ends `update_failed` with a `failureReason` naming the crash (`… (last exit code 127)`), not `the deploy did not become healthy within the health-gate window; check the service logs` — on REST, GraphQL, MCP `get_deploy` and the dashboard deploy list/detail, including when `service_hibernated` fires before the 15-minute progress deadline.
- The same with a failing health check (w8/m44 observation 2): the `failureReason` is the probe line (`… a TCP connect to port <n>`).
- After that failure, the dashboard header phase badge and GraphQL `server(id).phase` agree on every sample over 5 minutes (no "Deploying" while the API reads `Hibernated`) — either the root cause is fixed, or the divergence is shown to be a real phase flap and t002 / w1/m172 removes the flap.

## Evidence

- **w4/m142 live acceptance, 2026-10-02** (`.pm/w4/done/m142/README.md:43`): native service with a live prior release, start command `./qa-missing-binary`; `dep-db09vioehcmc739j12k0` closed `update_failed` with the generic health-gate line on REST/GraphQL/MCP/dashboard, though its open row had carried a stall reason with exit code 127. Afterwards the header badge read "Deploying" while GraphQL phase read `Hibernated`.
- **w8/m44 live closeout, 2026-10-03** (`.pm/w8/done/m44/README.md`, adjacent observation 2): `dep-db0a302tm2ss7389qnmg` (no traffic) was preempted by `service_hibernated` at 06:45:36Z before the progress deadline and closed at the 18-minute backend gate with the generic line, though its open row carried the TCP-probe stall reason.

## Mechanism (current `main`)

- **Two equal 15-minute clocks.** `defaultIdleTTL = 15 * time.Minute` (`lego/operator/internal/controller/app_controller.go:2088`) equals the 900 s rollout budget. A deploy does not refresh `app.bex.co/last-active` (stamped only on the first Running reconcile, `:2698-2705`, otherwise from traffic, `activity.go:143-164`), so with no traffic the park lands at or before the progress deadline.
- **Parking ignores a rollout in flight.** `desiredReplicas` sets `autoHibernating` from idleness alone (`:2557-2560`); `parkKubernetes` (`:2419-2445`) calls `hibernated()` (`:2727-2738`), which overwrites Ready with `Reason: AutoHibernated` for the current generation and sets phase Hibernated. `settleFailedRollout` / `ConditionRollout` never run, so no durable verdict exists.
- **The backend throws away what it had.** Each pass calls `SetDeployStallReason(open.ID, deployStallReason(cur))` (`lego/backend/internal/store/reconciler.go:706`); `deployStallReason` (`:1459-1474`) returns `""` for `AutoHibernated`, clearing the crash/probe text. At the gate, `deployCloseFailureReason` (`:808-836`) finds no `recordedRolloutFailure` (`:844-851`), `failureReasonFor` (`:1428-1449`) reads Ready=AutoHibernated, and falls to `timedOutDeployReason` (`:1493-1501`).
- **Header badge (cause unverified).** `ServiceStatusBadge` (`dashboard/src/features/services/components/service-detail-header.tsx:120`) → `deriveStatus` (`dashboard/src/features/services/lib/status.ts:239-250`) maps the raw phase only; the backend projects the CR phase verbatim (`lego/backend/internal/apps/service.go:1002,1043`); `useServer` polls every 3 s while converging (`dashboard/src/features/services/hooks/use-server.ts:65-71`). A stale cache should self-correct in 3 s, so the leading hypothesis is a real phase flap: the parked rollout never settled, any wake scales the broken template (Deploying), then it re-parks. Not traced to file:line — t003 owns it.

## Not fixed by

- `72da96bc6` (w8/039): `lastStallDiagnosis` (`app_controller.go:2990-3024`) recovers the stall only when the Deployment reaches `ProgressDeadlineExceeded` and Ready still carries it; once parked, Ready reads `AutoHibernated` and settle never runs.
- `78e410542` (w4/m156): `started_at` stamping only.

## Implementation (2026-10-04)

- **t001** (`lego/backend/internal/store/reconciler.go`): `deployStallObservation` skips the open row's `stallReason` write while Ready carries a park reason (`AutoHibernated`, `Suspended`), so a park no longer clears the crash/probe text. `deployCloseFailureReason` for `update_failed` reads the `Rollout` verdict, then a current Ready diagnosis, then the row's `stallReason`, then the generic line. Tests: `parked_rollout_reason_test.go` (crash exit 127 and TCP probe, each under both park reasons, plus the fallback order); 6 failures with the fix reverted.
- **t002, decision (a) defer the park.** A deploy in progress is not idleness: Render runs a deploy to completion and spins a free instance down only after 15 minutes without inbound traffic. Deferring keeps one code path for the verdict (`settleFailedRollout`) instead of a second settle-at-park diagnosis. `rolloutAwaitingVerdict` (`app_controller.go`) keeps an idle App awake while the current release, newer than the served one, is on its awake Deployment with no `Rollout` verdict for its generation. It is bounded by the progress deadline plus `rolloutVerdictGrace` (2 min). After the verdict, w1/m172's `holdFailedRollout` keeps the served template and the App parks on it. A manual suspend still parks mid-rollout and puts the served template back (w1/m172 `holdUnservedRelease`). Tests in `wake_over_failed_rollout_test.go`: `TestIdleMidRolloutWaitsForItsVerdict` (crash 127 and TCP probe; 3 failures with the deferral removed), `TestIdleMidRolloutDeferralIsBounded`, and `TestSuspendMidRolloutPutsServedTemplateBack` (replaces `TestParkMidRolloutPutsServedTemplateBack`, whose idle park is now deferred). `docs/ADR004-app-deployment.md` documents the rule.
- **t003 trace.** The header badge is a pure function of `server(id).phase` (`dashboard/src/features/services/lib/status.ts` `deriveStatus`). It polls every 3 s while converging and every 30 s otherwise (`dashboard/src/common/lib/polling.ts` `useConvergingPoll`), so no client cache can hold "Deploying" against a `Hibernated` read. The divergence was a real phase flap on the pre-m172 operator: the failed rollout had no verdict, so the parked Deployment kept the failed template. Any request woke it onto that template (phase Deploying, no ready pod, activator 503), and it re-parked after the idle window (Hibernated). Two samples taken at different moments disagreed. Now the verdict (t002) plus `holdFailedRollout` keep the served template on the parked Deployment, so a wake goes Hibernated → Running on the prior release. 5-minute live sampling is recorded under t004.
- **t006** `/simplify` (three reviewers): `progressDeadlineExceeded` returns the condition (and `reasonProgressDeadlineExceeded` replaces the literals); `currentFailureReason` splits the diagnosis from `failureReasonFor`'s generic fallback, so the close order is flat instead of comparing against the generic string; `parkReason` replaces the duplicated Suspended/AutoHibernated test in `observedServiceStateFor`; a shared `rollFailingReleaseTwo` test setup.
- **t007** Coverage: every new test fails with its fix reverted (backend 6, operator 3). The header/phase agreement has no client mechanism to test (t003); it is verified live in t004.
- Suites: operator `make test`, `make lint` (all four modules), backend `go test ./...` pass.

## Related

- [w1/m172](../../w1/m172/README.md) — a free service woken after a failed rollout over a prior release serves the broken template (503 while Running). Same family; t002 here must leave a settle verdict that w1/m172's template restore can act on. Coordinate, do not duplicate.

## Source + Goal linkage

- **Source:** w4/m142 live acceptance (2026-10-02) and w8/m44 live closeout observation 2 (2026-10-03); filed 2026-10-03 from today's live QA triage.
- **Goal linkage:** honest deploy failure reasons (w7/m79, w4/m103, w8/m44, w8/039 lineage); Render-compatible deploys and free-tier sleep (ADR006, ADR018).
- **Expected outcome:** free-tier users — the ones most likely to have no traffic during a deploy — see why their deploy failed instead of "check the service logs", and the header tells the same story as the API.
- **Why now:** w8/039 closed the last non-hibernation path to the generic line; this reproduced twice on production in two days.
- **Render parity included:** it changes the deploy `failureReason` on REST/GraphQL/MCP and the dashboard header/deploy views.
