# w10 · m420 — A cron job never sends "Service unavailable": a run trigger or cancel is not an outage, as on Render

**Worker:** worker10 **Goal:** a cron job's availability is never sampled, so no `server_failed` / `server_available` fact, "Service unavailable" email, push or webhook is ever produced for one. Triggering or canceling a run leaves a settled cron in `Running` instead of flipping it through `Deploying`. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | bex-api never samples a cron job's availability, so no `server_failed` is recorded for one — **DONE** | 45m | — |
| [t002](done/t002.md) | A run trigger or cancel does not flip a settled cron job through `Deploying` — **DONE** | 45m | — |
| [t003](done/t003.md) | Render parity — **DONE** | 15m | w10/m420/t001, w10/m420/t002 |
| [t004](done/t004.md) | Simplify — **DONE** | 10m | w10/m420/t003 |
| [t005](done/t005.md) | Test coverage — **DONE** | 30m | w10/m420/t003 |
| [t006](done/t006.md) | Closeout — **DONE** | 15m | w10/m420/t004, w10/m420/t005 |

Six tasks, 160m (2h40m).

## Definition of done

1. A cron job App observed with `Ready=False` (any reason: `Deploying`, `CronRunFailed`, `CronRunCancelFailed`, …) across any number of consecutive passes records no `server_failed`, `server_hardware_failure` or `server_available` fact (store test).
2. Writing only `spec.runAt` or `spec.cancelRun` on a settled cron (`Running`, or `Hibernated` while suspended) leaves `status.phase` and its `Ready` condition unchanged; a real release change (image, command, schedule, env) still goes through `Deploying` (operator test).
3. Live, on a production deploy carrying both fixes: a Starter cron triggered and canceled at API speed ≥ 20 times (half canceled 0.3 s after the `POST`) produces no "Service unavailable" email, no `server_failed` in its Events, and its status never reads `Deploying`; a failing run still sends "Cron job failed".

## Source + Goal linkage

- **Source:** triage of a production alert email, 2026-11-16: "Service unavailable: qa-20261116-m300-cron (7 times)", 05:48:48–05:57:44Z. That cron (`srv-dbt9hmqdrmis73edas5g`, busybox `echo; sleep 3`) was w4/m300's live-replay fixture ([w4/m300](../../../w4/done/m300/README.md) § Unblock record and live replay (2026-11-16)), triggered and canceled ~25 times. Code path: each trigger/cancel writes `spec.runAt`/`spec.cancelRun`, the generation moves, `reconcileCronJob` calls `setPhase(PhaseDeploying)` (`lego/operator/internal/controller/app_controller.go:5223`, Ready=False reason `Deploying`, `:7033`); `observedServiceStateFor` (`lego/backend/internal/store/reconciler.go:2458`) has no service-type guard, so a cron with `ActiveRevision` set and no open deploy reads `unhealthy`/`readiness_failed`; two consecutive passes clear `debounceUnhealthy`, `server_failed` is recorded, and `classifyAlertEmail` (`lego/backend/internal/notifications/alert_email_worker.go:243`) mails it. Production logs were not read (the kubeconfig fetch's strict host-key check failed); either Ready=False reason takes the same path.
- **Goal linkage:** Render parity ([ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first)). Render cron jobs have no instances or health checks; their only failure notification is "Cron job failed" (`cron_job_run_ended` failed). Notification truthfulness is part of the hosting core.
- **Expected outcome:** owners of cron jobs stop receiving false "Service unavailable" emails, pushes and `server_failed` webhooks on every manual run; the dashboard status badge stops flashing `Deploying` on Trigger Run.
- **Why now:** every customer who triggers or cancels a cron run can be paged for an outage that never happened, which trains people to ignore real alerts. Render parity is included because the fix changes customer-visible events, emails, webhooks and the dashboard status.

## Implementation record (2026-11-16)

- **t001** — `observedServiceStateFor` returns a cron job's observation with phase and suspension only, no availability (`lego/backend/internal/store/reconciler.go`); `instanceExitFacts` returns nothing for a cron job. An unobserved availability keeps the checkpoint's previous value with no edge, so a checkpoint latched `unhealthy` before the fix never emits a stray `server_available`. Test: `TestPGCronJobRecordsNoAvailability` (real Postgres; Ready=False `Deploying`/`CronRunFailed`/`CronRunCancelFailed`/`NodeLost`/`CrashLoopBackOff` across five passes record no `server_failed`/`server_hardware_failure`/`server_available`; latched checkpoint; web-service control still fails; cron suspend/resume facts still record). Each guard mutation-checked: reverted, the test fails.
- **t002** — `reconcileCronJob` writes `Deploying` for a moved generation only when `cronReleaseAndScheduleUnchanged` is false: a release adopted after the observed generation (`Status.ReleaseGeneration > Status.ObservedGeneration`, which `prepareAppReleaseDecision` sets for any release-fingerprint change) or a schedule that differs from the CronJob's. Run requests and other operational writes converge in place. Run-request step failures (`CronRunFailed`, `CronRunCancelFailed`) still write Ready=False through `failStep`. That is now invisible to availability (t001), and `cron_job_run_ended` remains the run's channel, as the task's out-of-scope line keeps it. Test: envtest `keeps a settled cron Running through a run trigger and cancel, and still deploys a schedule change` (`service_types_test.go`) records every App status write. Trigger and cancel write only `Running/True/Scheduled`; schedule, image and env changes still write `Deploying`. Mutation-checked for each of the three guards.
- **t003 Render parity** — every outage surface is fact-driven: REST/GraphQL/MCP event reads (`internal/events/{rest,graphql,mcp,service}.go`), webhooks (`webhooks/service.go`), push (`notifications/push_worker.go`), email (`notifications/alert_email_worker.go`) and the dashboard Events tab (`features/events/`) render `server_failed`/`server_available` facts only. None synthesizes an outage from status. The only other checkpoint writer, `pairInstanceExit`, runs only for exit facts, which a cron job no longer has. The dashboard status badge reads `status.phase`, which t002 keeps `Running`. The failed-run path (`recordCronRunFacts` → `cron_job_run_ended` → "Cron job failed") is untouched. Render: cron jobs have no instances or health checks and notify only on a failed run. No drift found.
- **t004 Simplify** — `/simplify` run: parenthesized the gate's `&&` clause and trimmed the store comment. The CronJob-read helper and the test-recorder merge were judged too small to extract.
- **t005 Test coverage** — the two tests above, each failing with its fix reverted. Backend `internal/store` and operator `make test` pass.
- **Gate for t006** — DoD 3 needs a production deploy carrying both fixes, then the live replay as t006 describes, through the REST API as the QA user (`scripts/qa-login.sh`). It needs no kubeconfig. The production kubeconfig fetch's strict host-key failure (seen in triage) is unrelated and was not relaxed. The gate clears by itself once `deploy.yml` ships this commit; check that `api.bex.co` serves it, then run the replay.

## Unblock record (2026-11-16, `/loopx w10` worker; verified evidence)

- **Affected:** w10/m420 (t006).
- **Source:** verified finding. The release gate cleared: production runs pin `284b83e1f540` or newer (`deploy.yml` on `284b83e1f` succeeded 2026-11-16T21:11Z, pin commit `02026e53f`; the newest pin is `938c5cb36b49`, `a24555d5c`), and `git merge-base --is-ancestor ea837ce20 284b83e1f540` holds (as does `… 938c5cb36b49`), so both fixes (`ea837ce20`) are live.
- **Remaining work:** none gated. t006 runs DoD 3's live replay as the QA user over REST, records the evidence here, deletes the fixture, and closes the milestone.

## Live replay and closeout (2026-11-16, `/loopx w10` worker)

- **DoD 1–2:** `TestPGCronJobRecordsNoAvailability` (store) and the envtest `keeps a settled cron Running through a run trigger and cancel, and still deploys a schedule change` (t001, t002, t005; both mutation-checked) hold on `ea837ce20`, which production pin `284b83e1f540` and the newer `938c5cb36b49` both contain.
- **DoD 3, live (23:31–23:39Z, `muse.env` QA user, `bex-canary`, all REST against `api.bex.co` with the QA session cookie).** Fixture: Starter `busybox:1.36` cron `qa-20261116-m420-cron` (`srv-dbtp4ipntgpc739be810`, `sh -c 'echo cron-m420: run host=$(hostname); sleep 3'`, schedule `0 0 1 1 *`).
  - **22 triggers, 11 canceled 0.3 s after the `POST` answered**, alternating `POST …/runs/{id}/cancel` (200) and `DELETE …/runs` (204). Every `POST` answered 200 in 0.33–0.64 s. All 11 canceled runs read `canceled` with no `startedAt`, and all 11 others read `successful`.
  - **Status never read `Deploying`.** A poller read `GET /v1/services/{id}` `phase` every ~0.4 s for the whole replay: 285 reads, all `Running`.
  - **No outage fact.** The service's Events after the replay, and again 90 s after the failing run (past the two-pass debounce): `cron_job_run_ended` ×23, `cron_job_run_started` ×12, `deploy_started`/`deploy_ended` ×2, `commands_changed` ×1. No `server_failed`, `server_hardware_failure` or `server_available`.
  - **A failing run still reports.** After the command was patched to `sh -c 'echo cron-m420: fail; exit 1'` and deployed (`dep-dbtp73b06sbc73b97vjg`, live), the triggered run `crr-0irbe6upuhb00vnfh4hm` read `unsuccessful` and recorded `cron_job_run_ended` `unsuccessful` / `non_zero_exit` (23:37:15Z). That fact is what "Cron job failed" mails from (t003; path untouched).
  - **Mailbox not checked.** The QA mailbox was not reachable from this session (the Gmail connector is not connected), so the absence of a "Service unavailable" email and the presence of a "Cron job failed" email were not read directly. Both emails come only from facts (`classifyAlertEmail`), so the Events above are the evidence.
  - **Cleanup.** Fixture deleted (204, then 404). The list has no `qa-20261116-m420` service. Only this run's session was revoked.
