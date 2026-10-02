# w5 · m106 — Prevent cron overlap during manual preemption

**Worker:** worker5 **Goal:** Keep the recurring schedule paused while a manual replacement cancels prior execution. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Research Render scheduling, preemption and retry behavior — **DONE** | 20m | — |
| t002 | Correlate live Jobs and Pods and reproduce overlap — **DONE** | 45m | t001 |
| t003 | Keep scheduled runs paused throughout manual preemption — **DONE** | 1h | t002 |
| t004 | Verify live preemption and cancellation across schedule ticks — **DONE** | 40m | t003 |
| t005 | Render parity and surface disposition — **DONE** | 20m | t004 |
| t006 | Simplify — **DONE** | 20m | t005 |
| t007 | Test coverage — **DONE** | 30m | t006 |
| t008 | Closeout — **DONE** | 10m | t007 |

## Definition of done

The reproduced cancel-and-catch-up race no longer overlaps actual container execution. Tests cover pending foreground deletion and scheduled runs absent from API history. Local replay verifies scheduled delay, manual preemption, cancel persistence and suspend/resume, with Job/Pod/API evidence and cleanup. Original production incident acceptance remains with w4/m114; no production or global race-freedom claim follows from a local pass.

## Source + Goal linkage

- **Source:** w5/069, promoted 2026-10-01 after its live verification reproduced a mechanism defect; original note retained in verification-origin.md.
- **Goal linkage:** ADR008 dependable hosting and ADR038/ADR018 Render cron behavior.
- **Expected outcome:** Manual preemption holds scheduling paused through old-Pod termination and replacement creation.
- **Why now:** In disposable dev-5, scheduled Job `tea-davcqapjg4r2nk99chtg-w5-cron-069-29848159` and manual Job `…-run-15622c09` both began execution at 2026-10-01T21:19:24Z after a 21:18:49Z trigger. `cancelPending` forced manualCronRunActive false, reopening the schedule during foreground deletion; catch-up started before replacement.
- **Render parity included:** User-facing cron execution semantics are shared across REST, GraphQL, MCP and dashboard; each uses the same backend intent/operator mechanism.

## Research and initial verification — 2026-10-01

[Render](https://render.com/docs/cronjobs#single-run-guarantee) documents one active run, manual cancel-then-replace, delayed scheduled runs and a twelve-hour limit; no retry policy is established. Existing w8/028 backoffLimit0 policy remains credited. [Kubernetes](https://kubernetes.io/docs/concepts/workloads/controllers/cron-jobs/) documents missed-tick catch-up. Local long run ended before the next scheduled Job started at21:18:32 for its original21:18:00 tick. A failing scheduled run executed once and reached Failed21:16:09 after exit23at21:16:05; API history matched. Then manual preemption reproduced real overlap, saved in `/tmp/w5-069-evidence/overlap-confirmed.json`. Direct one-App reconciliation bypasses the manager watch/cache delivery path; evidence is local mechanism proof, not production incident closure.

## Ship checkpoint — 2026-10-01 local / 2026-10-02 UTC

Implementation is present but uncommitted. Scheduling is paused before foreground deletion and held through replacement; fresh reads identify active Jobs owned by the current CronJob, extra cancellations are persisted before deletion, and canceled history is retained. Focused before/after regressions passed, including unrelated/terminal Job preservation. All three Simplify reviews completed; obsolete parameters/guards and the one-use test wrapper were removed. Full operator `make test` passed, including envtest. The all-module lint run found only excessive complexity in the new test (other modules and dead-code checks passed); its cancellation assertion was extracted into a helper, focused tests passed again, and final operator lint passed with zero issues. Markdown was formatted with the installed Prettier 3.4.2 binary after offline npx could not resolve its package metadata.

Live baseline evidence is retained in [evidence/before.json](evidence/before.json). The fixed replay resumed at 2026-10-01T21:26:15Z but the required manual-preemption/cancel/suspend-resume sequence was not completed before the environment changed. Do not mark live verification or closeout done. Local scheduled-run snapshots alone do not prove the fix's full live acceptance.

**Run-level ship block:** the newly restricted sandbox makes `.git` read-only. `git pull --rebase --autostash origin main` failed opening `.git/FETCH_HEAD` with `Operation not permitted`; no commit or push was attempted through another route. Browser cleanup was also denied (`MCP tool call requires approval, but approval policy is never`). Restore Git write/network access and authorized local-browser access to finish the replay, cleanup, board closeout and ship.

**Cleanup still required:** disposable service `srv-davcqdhjg4r2nk99ci50`, App `tea-davcqapjg4r2nk99chtg-w5-cron-069`, workspace `tea-davcqapjg4r2nk99chtg`, identity `40bcdebd-78a9-44ea-9c06-5f7da064670f`. The last stored CronJob snapshot (2026-10-01T22:04:35Z) has `suspend: false`; current live state could not be rechecked. Stop files for the scoped local reconcile/observer loops were written. Delete the service and workspace before deleting the identity, then verify Kubernetes residue. No other dev stack was touched.

## Completion — 2026-10-01 local / 2026-10-02 UTC

The earlier session-permission block and pending cleanup above are resolved. The resumed run had Git/network/browser access, completed the live replay, removed the disposable resources, and verified the current code. Historical checkpoint text is retained as an audit trail, not an active blocker.

- **Implementation:** pause scheduling before foreground deletion; keep it paused during termination and manual execution; read directly from the API for this handoff; preempt current CronJob-owned runs missing from API history; persist extra cancellation outcomes before deleting their Jobs. Terminal and foreign-owned Jobs remain untouched.
- **Simplify:** three independent reuse/quality/efficiency reviews completed. Reused the already-read CronJob for ownership checks and removed the redundant manual-name exclusion. The focused cancellation assertion checks persistence both during deletion and after the backing Job disappears.
- **Tests:** full operator `make test` (including envtest) and all-module `make lint` passed. The new regression fails against the original controller, then passes with the patch; final extra cancellation-history assertions and operator lint also pass. REST/GraphQL/MCP cron adapter and error-contract tests pass. See `evidence/operator-tests.txt`, `workspace-lint.txt`, `regression-before.txt`, and `api-adapter-tests.txt`.
- **Live verification:** scoped direct `AppReconciler.Reconcile` loop on dev-5, with real Kubernetes Jobs/Pods and authenticated REST/GraphQL/dashboard reads. The harness exited successfully after explicit stop. It bypasses manager watches/cache delivery, so this is local mechanism evidence, not production rollout acceptance.

### Observed sequence (UTC on 2026-10-02)

1. Scheduled container `…-29848523-f8xzg` ran **03:23:05–03:24:30**. The Job for the **03:24:00** tick was created at **03:24:32**, and its container `…-29848524-xpn47` ran **03:24:34–03:25:59**, demonstrating delayed execution after the prior container finished. Kubernetes catch-up chooses the latest eligible tick; this does not prove every missed tick is replayed.
2. Manual trigger at **03:26:28.598** canceled that scheduled run. The CronJob was observed suspended at **03:26:30.245** and remained suspended through the 03:27 tick. The replacement `…-run-9a52a106` was created at **03:27:00**, after foreground deletion, and its container ran **03:27:01–03:28:26**. No scheduled execution appeared during that manual run. The canceled Pod's final terminated sample was missed by polling; no exact finish time is invented.
3. Scheduling resumed at **03:28:32**; the delayed scheduled container began at **03:28:32**, six seconds after the manual container finished. User suspend was observed at **03:28:44**, through the 03:29 tick; resume was observed at **03:29:09**.
4. Dashboard **Trigger Run** confirmation at **03:29:11.893** preempted the scheduled container. That container finished at **03:29:43**; the replacement `…-run-2e8d0993` started at **03:29:46**. There is a measured three-second gap, not an overlap.
5. Dashboard **Cancel → Proceed** at **03:29:51.694** canceled the second manual run. After suspend at **03:30:05** and resume at **03:30:24**, the old manual Job/Pod stayed absent and its API/dashboard history stayed canceled. A new scheduled container started at **03:30:25**. Cancellation's API `finishedAt` records intent time, not actual container exit; the final terminated Pod sample was missed for this deletion too.

See [after.json](evidence/after.json) for Job ownership, scheduled timestamps, container intervals/logs and CronJob suspension transitions, and [api-after.json](evidence/api-after.json) for matching REST/GraphQL/dashboard history after resume. The baseline [before.json](evidence/before.json) retains the actual simultaneous-running snapshot. Earlier retry evidence remains credited to w8/028; Render retry policy is not established. Kubernetes has no suspension acknowledgement excluding an already-in-flight scheduler create, so the local pass is not a global race-freedom proof. Original production acceptance remains with w4/m114, outside this milestone.

### Cleanup

Deleted service `srv-davcqdhjg4r2nk99ci50` through GraphQL at 03:30:48, then workspace `tea-davcqapjg4r2nk99chtg` at 03:31:08, then the disposable Kratos identity (HTTP 204). Verified no App/CronJob/Job/Pod remains in that fixture namespace; scoped reconcile and observer loops stopped. No other dev environment was changed.
