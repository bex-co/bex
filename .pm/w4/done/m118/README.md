# w4 · m118 — Scheduled cron runs never reach the events feed: a failed run is invisible in Activity

**Worker:** worker4 **Goal:** a cron run that starts and fails on its schedule appears in the service Activity feed as `cron_job_run_started` / `cron_job_run_ended`, the way manual triggers/cancels, webhooks, and push already treat it. **Status:** done 2026-09-21 (live re-probe of the deployed fix deferred to the next QA pass — no production access this session)

## Tasks (in order)

| id   | title                                                                                 | est | depends_on |
| ---- | ------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Feed the reconciler's cron-run facts into the events query (verbs ∪ facts)            | 45m | —          | — **DONE**
| t002 | Render parity + docs                                                                  | 20m | t001       | — **DONE**
| t003 | Simplify                                                                              | 20m | t002       | — **DONE**
| t004 | Test coverage                                                                         | 40m | t003       | — **DONE**
| t005 | Live re-probe: a failing scheduled run lands started + ended in Activity              | 30m | t004       | — **DONE**
| t006 | Closeout                                                                              | 10m | t005       | — **DONE**

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

## Outcome (2026-09-21)

The premise held exactly as filed, and the fix turned out to be a *vocabulary* correction rather than the union t001 sketched.

**The design question t001 left open.** t001 proposed a union case — push down both the audit verbs and the fact type — so one filter queries both sources. That would have satisfied DoD bullets 1, 2 and 4 and **broken bullet 3**: `recordCronRunFacts` states outright that it "does not special-case how a run was requested", so a manually-triggered run already leaves a fact *and* an audit row. Mapping both would show every manual run twice, seconds apart, under the same type.

Two pieces of repo evidence settled it in the other direction:

- `eventTypes`' own doc comment already excuses `deploys.Trigger` because "the deploys row it opens IS the deploy_started event; mapping the verb too would show every API deploy twice". `apps.TriggerCronRun` is that argument verbatim, one surface later.
- Outbound webhooks had **already made this exact call**, pinned by `TestCronWebhookEventsComeFromObservedFactsNotIntentVerbs`: intent verbs must not map to webhook events; the observed facts are the source. The events feed was simply inconsistent with its sibling.

**So the feed now matches webhooks.** `TypeCronJobRunStarted`/`TypeCronJobRunEnded` joined `allFactTypes` and the fact-only `pushDown` case, and the three intent verbs (`apps.TriggerCronRun`, `apps.CancelCronRun`, `apps.CancelCurrentCronRun`) were removed from `eventTypes` with the rationale recorded in three places the guards read: the doc comment, `excusedVerbs` in `TestEveryTargetedVerbIsNamedOrExcused` (which failed until the exclusion was written down with a reason — the guard working as intended), and ADR038. The "who asked" record survives in the workspace audit log, where the other unmapped verbs put it.

**No store or dashboard change was needed**, as the filing predicted. The store's fact join is `WHERE f.app_id = $1 AND f.fact_type = ANY($12)` (`store/events.go:436`) — driven entirely by the pushed-down list — and `ev.Details.Status = r.FactStatus` already applies to every fact row, so the ended rows carry `succeeded|failed|canceled` with no mapping work. The dashboard has rendered both types since before the bug was filed (`timeline.ts:8-9`, `service-event-catalog.ts:35-36`).

**DoD status.** Bullets 1, 2 and 4 are implemented and covered; bullet 3 is *strengthened* beyond "exactly once" — a manual run can no longer produce a second row by construction, asserted by `TestManualCronRunIsNotCountedTwice`.

**Tests** (each mutation-spot-checked by reverting the fix, which reproduces the finding — the unfiltered feed's fact-type list comes back without either cron type): `TestScheduledCronRunReachesTheFeed` drives the service with a fact-only run and asserts the unfiltered feed asks for both types, that both events come back in order, that the ended one carries `failed`, and that filtering by either type pushes exactly that fact type with no verb fallback; `TestManualCronRunIsNotCountedTwice`; `TestCronRunEventsComeFromObservedFactsNotIntentVerbs` holds both halves together (re-adding the verbs double-counts, dropping the facts re-hides the schedule).

**Green:** `lego/backend` `go test ./...` all packages + `golangci-lint` 0 issues.

**Not done — t005's live re-probe.** Watching a failing scheduled run land `cron_job_run_started` + `cron_job_run_ended` in Activity needs the fix deployed and a throwaway every-minute cron on production; no production access this session. Deferred to the next QA pass, the same disposition m115/m113/m112/m111 carry. Worth knowing for that pass: the run's *terminal* fact depends on the operator observing `FinishedAt` plus a terminal status in `status.runs`, which is the lifecycle `w4/m114` (blocked) is about — a run whose terminal state never lands will still show a start and no end. That is m114's bug, not this feed's, and t001's own scope note drew the same line.
