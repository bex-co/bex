# w4 · m118 — Scheduled cron runs never reach the events feed: a failed run is invisible in Activity

**Worker:** worker4 **Goal:** a cron run that starts and fails on its schedule appears in the service Activity feed as `cron_job_run_started` / `cron_job_run_ended`, the way manual triggers/cancels, webhooks, and push already treat it. **Status:** todo

## Tasks (in order)

| id   | title                                                                                 | est | depends_on |
| ---- | ------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Feed the reconciler's cron-run facts into the events query (verbs ∪ facts)            | 45m | —          |
| t002 | Render parity + docs                                                                  | 20m | t001       |
| t003 | Simplify                                                                              | 20m | t002       |
| t004 | Test coverage                                                                         | 40m | t003       |
| t005 | Live re-probe: a failing scheduled run lands started + ended in Activity              | 30m | t004       |
| t006 | Closeout                                                                              | 10m | t005       |

## Definition of done

Each bullet is a click the next person can repeat on production and watch succeed.

- **Started lands.** A scheduled cron run's start appears in the service Activity feed as `cron_job_run_started` (today: absent).
- **Failed lands with status.** When the scheduled run fails, `cron_job_run_ended` appears with failed status (today: absent — Activity shows only the 2 deploy events).
- **No duplicates.** Manual Trigger Run / Cancel still show exactly once each (today: they show via audit verbs; the fix must union, not double-emit).
- **Type filter works.** Filtering the feed by `cron_job_run_started` / `cron_job_run_ended` returns the fact rows, not just the audit rows.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` pass 26 on `https://dashboard.bex.co`, 2026-09-19 (w4-targeted run, `muse.env` credentials). Fixture: every-minute cron `qa-20260919-p26-cron` (`srv-dan8iors0ils73bgp430`, `busybox:stable`, `echo p26-boom && exit 3`, deleted after the pass). First run failed after 10m 54s of crash-loop backoff and Recent Runs showed Failed with duration — but Activity still showed only Deploy started/ended. The facts exist (`recordCronRunFacts`, `lego/backend/internal/store/reconciler.go:439`), webhooks map them (`lego/backend/internal/webhooks/service.go:211-212`), push lists "Cron run failed", and the dashboard already renders both types (`dashboard/src/features/events/lib/timeline.ts:8-9`, `service-event-catalog.ts:35-36`) — only the feed query drops them: `allFactTypes` (`lego/backend/internal/events/service.go:351`) lists `job_run_ended` but neither cron type, and `pushDown` has no fact case for them (`service.go:397-403`), so the fallback returns only the trigger/cancel audit verbs.
- **Goal linkage:** cron-job operability — a failing schedule is currently silent on the service's own history surface; the operator already wrote the data, the API just never serves it.
- **Expected outcome:** scheduled cron runs are first-class Activity entries with terminal status, matching what webhooks and push already emit.
- **Why now:** mechanism nailed in code and verified live; the dashboard side is already built, so the fix is a backend-only query change plus tests.
- **Explicitly out:** this pass's verified-clean failure UX (Recent Runs Failed + duration, failed-build header/list/events consistency, crash-loop log ticks, Trigger-Run disabled reason) — not re-filed. Distinct from `w4/m114` (cancel resurrection / terminal delay), which is about run lifecycle, not feed visibility.
