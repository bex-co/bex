# w10 · m420 — A cron job never sends "Service unavailable": a run trigger or cancel is not an outage, as on Render

**Worker:** worker10 **Goal:** a cron job's availability is never sampled, so no `server_failed` / `server_available` fact, "Service unavailable" email, push or webhook is ever produced for one. Triggering or canceling a run leaves a settled cron in `Running` instead of flipping it through `Deploying`. **Status:** blocked (t001–t005 done; t006 closeout waits on the production deploy and the live replay, DoD 3)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | bex-api never samples a cron job's availability, so no `server_failed` is recorded for one — **DONE** | 45m | — |
| [t002](done/t002.md) | A run trigger or cancel does not flip a settled cron job through `Deploying` — **DONE** | 45m | — |
| [t003](done/t003.md) | Render parity — **DONE** | 15m | w10/m420/t001, w10/m420/t002 |
| [t004](done/t004.md) | Simplify — **DONE** | 10m | w10/m420/t003 |
| [t005](done/t005.md) | Test coverage — **DONE** | 30m | w10/m420/t003 |
| [t006](t006.md) | Closeout | 15m | w10/m420/t004, w10/m420/t005 |

Six tasks, 160m (2h40m).

## Definition of done

1. A cron job App observed with `Ready=False` (any reason: `Deploying`, `CronRunFailed`, `CronRunCancelFailed`, …) across any number of consecutive passes records no `server_failed`, `server_hardware_failure` or `server_available` fact (store test).
2. Writing only `spec.runAt` or `spec.cancelRun` on a settled cron (`Running`, or `Hibernated` while suspended) leaves `status.phase` and its `Ready` condition unchanged; a real release change (image, command, schedule, env) still goes through `Deploying` (operator test).
3. Live, on a production deploy carrying both fixes: a Starter cron triggered and canceled at API speed ≥ 20 times (half canceled 0.3 s after the `POST`) produces no "Service unavailable" email, no `server_failed` in its Events, and its status never reads `Deploying`; a failing run still sends "Cron job failed".

## Source + Goal linkage

- **Source:** triage of a production alert email, 2026-11-16: "Service unavailable: qa-20261116-m300-cron (7 times)", 05:48:48–05:57:44Z. That cron (`srv-dbt9hmqdrmis73edas5g`, busybox `echo; sleep 3`) was w4/m300's live-replay fixture ([w4/m300](../../../w4/blocked/m300/README.md) § Unblock record and live replay (2026-11-16)), triggered and canceled ~25 times. Code path: each trigger/cancel writes `spec.runAt`/`spec.cancelRun`, the generation moves, `reconcileCronJob` calls `setPhase(PhaseDeploying)` (`lego/operator/internal/controller/app_controller.go:5223`, Ready=False reason `Deploying`, `:7033`); `observedServiceStateFor` (`lego/backend/internal/store/reconciler.go:2458`) has no service-type guard, so a cron with `ActiveRevision` set and no open deploy reads `unhealthy`/`readiness_failed`; two consecutive passes clear `debounceUnhealthy`, `server_failed` is recorded, and `classifyAlertEmail` (`lego/backend/internal/notifications/alert_email_worker.go:243`) mails it. Production logs were not read (the kubeconfig fetch's strict host-key check failed); either Ready=False reason takes the same path.
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

