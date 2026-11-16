# w4 · m300 — A triggered cron run exists from the moment it is acknowledged: readable, cancellable, and canceled (not lost) when a second trigger replaces it, as on Render

**Worker:** worker4 **Goal:** the run object that `POST /cron-jobs/{id}/runs` returns is the run. Every read finds it as `pending` at once, Render's cancel call cancels it, and a second trigger records it as canceled instead of dropping it. **Status:** blocked — t001–t007, t009, t011 done (shipped); the t010 start-hold fix is live on `7aaa8700befa`; live replay on `7aaa8700befa` (2026-11-16 evening) held P1 and P3, but 3 of 19 rapid cancels still started because bex-api took up to 8 s to land them; the hold is now 10 s, and t010 and t008 close after a replay on a deploy carrying it

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Make an acknowledged run request readable on every surface — **DONE** | 60m | — |
| [t002](done/t002.md) | Cancel and replace a run request that has no Job yet — **DONE** | 90m | w4/m300/t001 |
| [t003](done/t003.md) | Recent Runs shows a triggered run as pending at once — **DONE** | 40m | w4/m300/t001 |
| [t004](done/t004.md) | Blast radius: every caller and alias of the cron run read model — **DONE** | 30m | w4/m300/t001, w4/m300/t002 |
| [t005](done/t005.md) | Render parity — **DONE** | 15m | w4/m300/t003, w4/m300/t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 10m | w4/m300/t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 40m | w4/m300/t005 |
| [t009](done/t009.md) | A run canceled before it started is announced as "not started yet" in Recent Runs — **DONE** | 15m | w4/m300/t003 |
| [t010](t010.md) | A run canceled or replaced within a second of its trigger still executes (its pod prints after the row reads `canceled`), and a replaced run reads `finishedAt` before `startedAt` | 60m | w4/m300/t002 |
| [t011](done/t011.md) | A triggered run that has not started yet reads "Running" in Recent Runs — **DONE** | 15m | w4/m300/t003 |
| [t008](t008.md) | Closeout | 15m | w4/m300/t006, w4/m300/t007, w4/m300/t009, w4/m300/t010, w4/m300/t011 |

Eleven tasks, 390m (6h30m); t010 added in round 171, t011 in round 244. One finding, major. t009 was added in round 81 of the `/qa-find-bugs w4` loop (2026-11-11). This milestone schedules the work only.

## Finding — a triggered run is invisible until the operator reconciles it

**Severity: major.** The API acknowledges a run, then cannot find it, cancel it or account for it. The dashboard never shows a short triggered run as active.

### Probes

All probes ran from the signed-in dashboard page with `fetch(…, {credentials: "include"})`. They can be re-run as written against any cron job ID.

- **P1, trigger then read by ID.**
  - `POST /v1/cron-jobs/srv-dbpceuf5g1is7398kqd0/runs` (no body), at +357 ms → `200 {"id":"crr-p6jc1a3nk2nkcvr9a2ki","status":"pending"}`.
  - `GET /v1/cron-jobs/srv-dbpceuf5g1is7398kqd0/runs/crr-p6jc1a3nk2nkcvr9a2ki`, at +878 ms → `404 {"code":"CRON_RUN_NOT_FOUND","error":"cron run \"crr-p6jc1a3nk2nkcvr9a2ki\" was not found"}`. At the same moment, `GET …/runs?limit=1` still lists the previous run, `crr-qlv72pge9i2625o0que4`.
  - Only at +3522 ms does the ID read `200 pending` (`startedAt 07:39:11Z`). It reads `successful` at +6811 ms.
- **P2, trigger then cancel (Render's own two calls).**
  - `POST`, at +1114 ms → `200 {"id":"crr-t8pbgb4bqkfkn22n8kvh","status":"pending"}`.
  - `DELETE /v1/cron-jobs/srv-dbpceuf5g1is7398kqd0/runs`, at +1631 ms → `409 {"error":"conflict: cron job \"srv-dbpceuf5g1is7398kqd0\" has no active run"}`.
  - The ID read 404 until +5648 ms, then `pending`. The run went on to `successful` (07:39:42Z → 07:39:46Z): the cancel was refused and the job ran anyway.
- **P3, two triggers back to back.**
  - `POST` #1 → `200 {"id":"crr-aj6vvc5q5jrb610o2f49","status":"pending"}`. `POST` #2 → `200 {"id":"crr-9dm4fr7dtvc2ucacjvbv","status":"pending"}`. Both answered by +642 ms.
  - `crr-9dm4…` ran: `pending` 07:41:11Z → `successful` 07:41:14Z.
  - `crr-aj6v…` answered 404 on every read for 40 s. It is missing from `GET …/runs?limit=4`, which reads `crr-9dm4…` (successful), `crr-drrumbasft62qs7d9rht` (scheduled, 07:40:00Z), `crr-t8pb…` and `crr-p6jc…`. It never ran, was never recorded as canceled, and its ID stays unknown.
- **P4, dashboard.** Events tab → Recent Runs → Trigger Run → confirm. The first table row was sampled every second, and the GraphQL responses were recorded.
  - At 0.7 s, `RunCronJob` answered `{"id":"crr-qlv72pge9i2625o0que4","status":"pending","startedAt":"","finishedAt":""}`.
  - At 1.0 s, the `CronJobRuns` refetch listed the previous run (07:36:37Z) first and did not contain `crr-qlv7…`.
  - The first row stayed on the previous run until 22.6 s. `crr-qlv7…` then appeared already Succeeded, with `startedAt 07:37:47Z`, nine seconds after the click.
  - With no active row, `hasActiveRun` is false. The table polls at the 25 s baseline instead of the 3 s converging poll, so a short triggered run is never seen pending, and its Cancel button never appears.
  - An earlier dashboard trigger, `crr-1078u863fdgtks2d9ln1`, instead showed a row with Started `—` for at least 15 s while the run was in flight. Its screen-reader toggle read "Run details, started crr-1078u863fdgtks2d9ln1", naming the run by its ID.

### Render

- [render.com/docs/cronjobs § single-run guarantee](https://render.com/docs/cronjobs#single-run-guarantee): one run is active at a time, and a manual trigger cancels the active run before its replacement starts.
- The pinned OpenAPI (`lego/backend/internal/api/openapi/render-public-api-1.json`):
  - `POST /cron-jobs/{cronJobId}/runs` returns `cronJobRun`: `id` and `status` required (`pending | successful | unsuccessful | canceled`), `startedAt` and `finishedAt` optional.
  - `DELETE /cron-jobs/{cronJobId}/runs` ("Cancel running cron job") returns `204`. It documents 401, 406, 429, 500 and 503, and no 409.
- bex's list and get-by-ID routes are documented extensions (w2/m36). They must still agree with the run that the trigger returned.

### Root cause

- **The read model has no entry for the request.** `lego/backend/internal/apps/service.go:4071-4104` (`TriggerCronRun`) patches `spec.runAt` and returns a synthesized `{ID: ids.Derive(CronRun, jobName), Status: pending}`. No read can see it:
  - `ListCronRuns` (`:4107-4119`) and `GetCronRun` (`:4122-4133`) read only the operator-written `a.Status.Runs`, through `cronRunViews` and `cronRunByID`.
  - The run lands there only after the operator reconciles `spec.runAt` and creates the Job (`lego/operator/internal/controller/app_controller.go:4895-4973`: `convergeCronRuntime` → `ensureManualRun` → `cronRuns` → `app.Status.Runs = runs`). Live, that took 2.6–9 s.
- **Cancel and replacement can't see it either.** `pendingCronRun` (`service.go:4174-4181`) scans only `status.Runs`. In that window:
  - `CancelCurrentCronRun` (`:4157-4172`) answers 409 "has no active run" (P2).
  - The `cron.cancelRun` capability (`actioncaps.go:86`) reads `no_active_run`.
  - A second `TriggerCronRun` finds nothing to put in `spec.cancelRun` and simply overwrites `spec.runAt` (`:4095-4100`). The first request's Job is never created, and no status entry ever names it (P3).
- **The operator cannot repair this.** `cancelRequestedCronRun` returns false for a Job that doesn't exist and records nothing. `manualRunSettled` looks only at the current `spec.runAt`.
- **Dashboard.**
  - `dashboard/src/features/services/hooks/use-cron-runs.ts` `trigger()` awaits `runCronJob`, then calls `refetch()`.
  - `hasActiveRun` is derived only from the list, so the mutation's pending run is dropped and polling stays at 25 s (`useConvergingPoll`, `common/lib/polling.ts:56`).
  - A run with no `startedAt` renders Started `—` (`absoluteTime`), and `runToggleWhen` in `cron-runs-section.tsx` then returns `run.id` as the row's accessible name. That is the "Run details, started crr-…" label. (`toRuns` also keeps the GraphQL `""` as `""`, because `??` does not catch the empty string.)

### Target behavior

- **The run object returned by `POST` is the run.** From the moment `POST` answers 200, these all return it as `pending`:
  - get-by-ID, and the list under both `/v1/cron-jobs/{id}/runs` and `/v1/services/{id}/runs`;
  - GraphQL `cronJobRuns` and `cronJobRun`;
  - the MCP list and get tools;
  - the service's `runs`.
- **Cancel finds it.** `DELETE …/runs` answers 204, and the run reads `canceled` and never executes. `POST …/runs/{runId}/cancel` works on it too.
- **Replacement cancels it.** A second trigger records the first run as `canceled`, Render's "cancels the active run before its replacement". The replacement runs exactly once.
- **A run that hasn't started has no `startedAt`.** Render's field is optional, and it is never the empty string: GraphQL answers `null`, not `""`.
- **Unchanged:** `LastSuccessfulRunAt` and the scheduled-run path.

### Blast radius

- `pendingCronRun`: `service.go:4093`, `:4165` and `actioncaps.go:86`.
- `cronRunViews`: `service.go:1165` (`AppView.Runs`, next to `LastSuccessfulRunAt` at `:1166`) and `:4115`.
- `cronRunByID`: `service.go:4130`, `:4146`.
- Entrypoints:
  - `TriggerCronRun`: `graphql.go:1555`, `mcp.go:727`, `rest.go:1281`.
  - `ListCronRuns`: `graphql.go:1252`, `mcp.go:739`, `rest.go:1285`.
  - `GetCronRun`: `graphql.go:1262`, `mcp.go:757`, `rest.go:1288`.
  - `CancelCurrentCronRun`: `rest.go:1296`.
  - `CancelCronRun`: `graphql.go:1565`, `mcp.go:765`, `rest.go:1291`.
- REST aliases: `/v1/cron-jobs/{id}/runs…` and `/v1/services/{id}/runs…` (`rest.go:1301-1310`).

### Adjacent classes

- A run ID that belongs to another service, and an unknown ID, stay `404 CRON_RUN_NOT_FOUND`, so IDs cannot be used to probe across resources.
- A suspended cron's trigger stays `409 CRON_SUSPENDED`.
- Cancel of a terminal run stays `409 CRON_RUN_TERMINAL`.
- Unauthenticated and forbidden callers are unchanged: authorization runs before any of this.

### Unverified this run

These are work to verify, not DoD facts:

- MCP tool output was reasoned from `mcp.go`, not probed.
- GraphQL `cancelCronJobRun` in the window was not probed.
- A reconcile slower than 9 s was not measured.

### Seen but not filed

- During the first deploy, the header read "Service: Building" while the deploy read Created, then Queued (07:28). This remains one untraced observation; its cause is unverified.
- `build_started` was stamped 07:27:53, the deploy's `createdAt`, although its `startedAt` was 07:28:57. **Reproduced in round 2 and filed as [w4/535](../535.md)**, a regression of w6/035: `buildStartedAt` treats a `created` row as already dispatched.

### Dedupe

- `grep -rlF` for "has no active run", `CRON_RUN_NOT_FOUND` and "Cancel running cron" across `.pm` returns nothing.
- Nearby work covers different guarantees: w5/m108 (a canceled run is not resurrected after history eviction), w4/m114 (one active slot, prompt failure), w6/039 (scheduled and manual concurrency) and w2/m36 (the run API's existence).
- `git log 1672bb379c0c..HEAD -- lego/backend/internal/apps lego/operator/internal/controller` touches no cron code, so nothing is waiting to deploy.
- No `.pm/DO_NOT_DO.md` entry applies. Item 24's one-off jobs are `/services/{id}/jobs`, not cron runs.

## Definition of done

Live on production after the release pins this build. The fixture is a Starter cron `qa-<date>-cron` from `examples/cron-demo` in `bex-canary`, deleted afterwards.

- **P1 replay.** `POST /v1/cron-jobs/{id}/runs` → `200 {id X, status pending}`.
  - Within 1 s, `GET …/runs/X` → `200 pending` with no `startedAt` (not 404), and `GET …/runs?limit=1` lists X first.
  - GraphQL `cronJobRun(serviceId, runId: X)` and the MCP get tool agree.
  - `startedAt` appears when the Job starts, and X reaches `successful`.
- **P2 replay.** `POST`, then `DELETE …/runs` within 1 s → `204`.
  - X reads `canceled` with `finishedAt`.
  - X never executes: no `cron-demo:` log line for it, and no `cron_job_run_started` for X.
  - The same holds through `POST …/runs/X/cancel` and GraphQL `cancelCronJobRun`.
- **P3 replay.** Two back-to-back `POST`s → IDs A and B.
  - A reads `canceled` (never 404), and B runs exactly once.
  - The list holds both.
  - Eleven runs later, A still reads `canceled`.
- **P4 replay.** At 1440 and 390 px, the dashboard's Trigger Run shows a pending row for the returned ID within 1 s, with Cancel offered and the 3 s poll running. The row's accessible name never contains the `crr-` ID. A run canceled before it started (P2) is named "Run details, never started (row N)", not "not started yet" (t009).
- **Adjacent classes unchanged.**
  - Another service's run ID and an unknown ID stay `404 CRON_RUN_NOT_FOUND`.
  - A suspended cron's trigger stays `409 CRON_SUSPENDED`.
  - Unauthenticated and forbidden calls are unchanged.
  - Scheduled `*/5` runs and Last successful run are unchanged.
- **Checks pass.** Operator `make test`, the backend `go test ./...` real-DB gate, and dashboard `yarn lint` and `yarn test`.
- **Cleanup.** No fixture remains, and only that run's session is revoked.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs w4` loop, round 1 of the 2026-11-10 loop, 07:27–07:42 UTC (`muse.env`, workspace `bex-canary` `tea-daif693dqjvc73e7as3g`, platform pin `1672bb379c0c`, research on main `3a12969ed`). Fixture: cron job `qa-20261110-cron` (`srv-dbpceuf5g1is7398kqd0`; Docker, `examples/cron-demo`, `*/5`, Starter). It was created through the dashboard and deleted at the end of the pass (GET 404, gone from the services list). The durable evidence is the probe record above. The local screenshot `.playwright-mcp/qa-cron-2.png` is gitignored.
- **Goal linkage:**
  - [ADR038](../../../../docs/ADR038-cron-jobs.md) (cron jobs).
  - [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first) and [ADR018](../../../../docs/ADR018-render-parity.md).
  - [ADR006](../../../../docs/ADR006-bex-api.md): REST, GraphQL and MCP move together.
- **Expected outcome:** Render's own trigger-then-cancel sequence works on bex. No acknowledged run ID is ever unknown. The dashboard shows a triggered run while it is pending and lets the user cancel it.
- **Why now:** P3 drops a user's run without a trace, and P2 runs a job its owner just canceled. Both happen at normal API speed, not under load.
- **Render parity included:** this changes REST, GraphQL, MCP and dashboard behavior, so t005 checks every surface against the Render contract above.

## Findings (2026-11-10, loopx w4)

- **t001, read model** (`lego/backend/internal/apps/service.go` `cronRunsOf`): every read serves `status.runs` plus two derived entries.
  - **The acknowledged run.** Pending while `spec.runAt`'s Job has no history entry, or canceled once `spec.cancelRun` names it. This holds while the operator has not acknowledged it, and for two minutes after it has, which covers the gap between acknowledgement and the history write in the same reconcile.
  - **A run canceled or replaced before its Job existed** (`spec.cancelRun.unmaterialized`), until the operator records it.
  - Get-by-id, both list aliases, GraphQL `cronJobRuns`/`cronJobRun`, the MCP tools, `Service.runs`, `DELETE …/runs`, `POST …/runs/{id}/cancel` and the `cron.cancelRun` capability all read through this.
  - `LastSuccessfulRunAt` and the scheduled path are untouched.
  - GraphQL `startedAt`/`finishedAt` answer `null` instead of `""`.
- **t002, cancel and replace:**
  - **Backend.** A cancellation of a run not yet in history carries `unmaterialized: true`, whether it comes from a cancel or a replacing trigger (CRD field on `spec.cancelRun`).
  - **Operator.** `recordUnmaterializedCancel` (`app_controller.go`) runs before the acknowledgement. It records the run `Canceled` exactly once when neither a Job nor a history entry carries it, and `status.cancelRunRecorded` keeps an entry evicted from history from being added back.
  - **Effect.** A run canceled before its Job exists never executes: the existing acknowledgement keeps `ensureManualRun` from creating it.
- **t003, dashboard:**
  - `useCronRuns` shows the run that `runCronJob` answered at the top until a read carries it, so it is pending at once, Cancel is offered and the 3 s poll runs.
  - `toRuns` maps `""` to `null`.
  - The row toggle's accessible name never contains the `crr-` id: a not-started run reads "Run details, not started yet (row N)" (en/zh), and a start-time tie adds the row number.
- **t004, blast radius:** all entrypoints listed above go through `cronRunsOf`/`pendingCronRun(a, now)`. The `/services/{id}/runs` aliases share the handlers.
- **t005, Render parity:** `POST` answers the run, `DELETE …/runs` cancels it (no 409 in the window), and a replacing trigger cancels the active run before its replacement (render.com/docs/cronjobs § single-run guarantee).
- **t006, Simplify:** one derived-history function serves every read; the operator change is a single guarded recording step.
- **t007, tests:**
  - **Backend.** `TestAcknowledgedCronRunIsReadableCancellableAndNeverLost` covers P1, P2 and P3, plus the adjacent 404 and 409 cases. `TestCronRunsOfDoesNotResurrectSettledRequests` covers settled requests.
  - **Operator.** `TestUnmaterializedCancelRecordsTheRunWithoutExecutingIt` and `TestReplacedUnmaterializedRunIsRecordedOnceAndReplacementRuns` cover cancellation, replacement and no resurrection after eleven later runs. Both fail with the recording disabled.
  - **Dashboard.** The toggle-name test now asserts no `crr-`, and the hook test shows the triggered run pending before the read includes it.
  - **Suites.** Operator `make test`, backend `go test ./...`, dashboard `yarn lint` (0 errors) and `vitest` (1241 passed) all pass.

- **2026-11-10 — parked (loopx w4):** the code is shipped; only the live DoD replay remains. Production deploys moved to 19:00/00:00/05:00 UTC plus manual dispatch (`4630a48a7`, 2026-11-09), so this waits for the 2026-11-10 19:00 UTC deploy. Then replay the DoD and close. No user action.

- **2026-11-11 — round 81 of the `/qa-find-bugs w4` loop:** production now runs `348bd77a5`, which contains `d4d105415`, so the deploy this milestone waited for has shipped. P1, P2 (`DELETE …/runs`) and P3 replays passed live; the probes are in [t009](done/t009.md). t009 was added for the canceled-before-start row's accessible name. t008's remaining replays (P2's two other cancel paths, "eleven runs later", P4 and the adjacent classes) still need running.

## Post-release probe (QA loop round 171, 2026-11-12): P1 passes; P2 and P3 fail (filed as t010)

Pin `e7a56b07e0d7` (`c5518dc24`, 02:23 UTC) carries t001–t007. The fixture and the full probe table are in t010.

- **P1 passes.**
  - `POST` → `200 {id, status: pending}`.
  - 0.7 s later, `GET …/runs/X` → `200 pending` with no `startedAt`, `…/runs?limit=1` lists X first, and GraphQL `cronJobRun` agrees.
  - X reached `successful`.
- **P2 fails.**
  - `DELETE …/runs` (204) right after the `POST`, and `POST …/runs/Y/cancel` (200) likewise, both leave the run `canceled`.
  - But each run started, emitted `cron_job_run_started`, and printed its `cron-demo:` line about 1 s later.
- **P3 fails in part.**
  - Two back-to-back `POST`s: B ran once, A reads `canceled`, and the list holds both.
  - But A's pod printed at 05:40:14, after A was recorded `canceled` at 05:40:11, and A reads `startedAt` 05:40:13 later than `finishedAt` 05:40:11.74.
- **Not run:** P4 (dashboard Trigger Run at 1440 and 390 px) and the eleven-runs retention check.

## Unblock record (2026-11-15, loopx w4)

- **Affected:** w4/m300 (t008–t011).
- **Source:** verified finding. Production's scheduled deploy at 2026-11-15 22:49Z ran on `06cc3b02c`, and `git merge-base --is-ancestor d4d105415 06cc3b02c` holds, so t001–t007 are live. The release gate this milestone was parked on has cleared.
- **Remaining work:** t009, t010 and t011 were filed after the park (rounds 81, 171, 244) and have no code on main. None of them has an external gate, so the milestone returns to active work.
- **Next steps:** implement t009 and t011 (dashboard Recent Runs labels) and t010 (a run canceled or replaced before its Job starts never executes), ship them, then replay the live DoD once a production deploy carries them. A deployed version is not proof the DoD passed.
- **Outcome of this run (2026-11-15).** t009 and t011 are done, and t010's fix is shipped in the commit `fix(cron): … (w4/m300 t009–t011)`; see [t010](t010.md) § Implementation. The milestone is parked again on a gate that clears itself: production's next deploy must run a commit that contains that one (`git log --grep 'w4/m300 t009' origin/main`, then `git merge-base --is-ancestor <sha> <deployed sha>`). Then a worker replays the whole DoD (P1–P4, the adjacent classes, eleven runs later) plus t010's and t011's live checks, and closes t010 and t008. No user action.

## Unblock record and live replay (2026-11-16, loopx w4)

- **Affected:** w4/m300 (t008, t010).
- **Source:** verified finding. The release gate cleared: production runs pin `14af7282d9ac` (`a518fa61f`; deploy run on `14af7282d` succeeded 05:06Z; QA round 486 confirmed it live), and `git merge-base --is-ancestor d47dbcb90 14af7282d9ac` holds, so t009–t011's fix is live.
- **Replay** (05:47–06:01Z, `muse.env`, `bex-canary`). Fixtures: Starter `busybox:1.36` crons `qa-20261116-m300-cron` (`srv-dbt9hmqdrmis73edas5g`, `sh -c 'echo cron-demo: run host=$(hostname); sleep 3'`) and `qa-20261116-m300-long` (`srv-dbt9hn40h0ts7389hbng`, same with `sleep 90`), schedule `0 0 1 1 *`. All REST from the signed-in page; `POST` has no body.
  - **P1 holds.** `POST` → `200 {crr-nrom6591vho7jjccrsce, pending}` at +309 ms. `GET …/runs/X` at +623 ms → `200 pending`, no `startedAt`; `?limit=1` lists X first; GraphQL `cronJobRun` → `pending`, `startedAt: null`; MCP `get_cron_job_run` (+2.1 s) → `pending` with `startedAt 05:47:35Z`. X reached `successful` 05:47:35–05:47:43.
  - **P2 (`DELETE …/runs`), P2c (GraphQL `cancelCronJobRun`) and P3 hold on the first pass.** `DELETE` 204 → `crr-3u2k…` `canceled`, no `startedAt`. GraphQL → `crr-03ct…` `canceled`, `startedAt: null`. P3: A `crr-plqe…` read `canceled` at once (never 404), B `crr-tfa8…` ran once (05:48:52–05:48:58), the list holds both. None of the three canceled runs has a `cron-demo:` line or a `cron_job_run_started`.
  - **P2b fails, and the race is not path-specific.** `POST`, then `POST …/runs/Y/cancel` 0.3 s later (200 `canceled`): Y `crr-8dppvdmdicgv45a2jp3e` reads `canceled` with `startedAt 05:48:12Z`, `finishedAt 05:48:12.46Z`, has a `cron_job_run_started` at 05:48:12, and its pod printed `cron-demo: run` at **05:48:13.83**, after it read `canceled`. Six more tries alternating `runs/{id}/cancel` and `DELETE`, each 0.25–0.3 s after the `POST`: five never started; `DELETE` on `crr-ntjor728t3o36l51rlop` emitted `cron_job_run_started` 05:51:04 (no `cron-demo:` line). So 2 of 9 cancels sent at API speed came after the operator had created the Job. `startedAt ≤ finishedAt` held on every row.
  - **Eleven runs later holds.** After 11 more successful manual runs, A still reads `canceled` and is listed (23 runs).
  - **P4 holds.** At 1440 px, Trigger Run → a `Pending` row "Run details, not started yet (row 1)" with Cancel at 0.57 s; `CronJobRuns` polled at +0.3, +3.3, +6.6, +9.9 s. At 390 px the same at 0.31 s, no horizontal overflow (scrollWidth 390). No row name contains `crr-`. A run canceled before it started reads "Run details, never started (row 1)" and Canceled (t009).
  - **Running cancel holds.** The `sleep 90` run, canceled with `DELETE` 13 s after it started, reads `canceled` 05:55:49–05:56:02.4.
  - **Adjacent classes hold.** Another service's run id (get and cancel) and an unknown id → `404 CRON_RUN_NOT_FOUND`; cancel of a canceled run → `409 CRON_RUN_TERMINAL`; unauthenticated `POST` → 401; trigger on a suspended cron → `409 CRON_SUSPENDED`. With the schedule patched to `*/5`, the 06:00:00 run succeeded and `lastSuccessfulRunAt` moved 05:57:24 → 06:00:08. Forbidden callers were not probed (no second account).
  - **Cleanup.** Both crons deleted (204, then 404); no `qa-20261116-m300` service left; this run's session revoked.
- **Fix for P2b** (t010, this run): the operator creates a manual run's Job no sooner than 2 s after `runAt` (`manualRunStartHold` in `ensureManualRun`, `lego/operator/internal/controller/app_controller.go`), then re-reads the App as before, so a cancel or replacement sent right after the trigger is seen before any Pod exists. Render reference: a run canceled right after its trigger never starts there either (render.com/docs/cronjobs § single-run guarantee). Tests: `TestAFreshTriggerIsHeldSoAnImmediateCancelNeverStartsIt` (fails with the hold at 0) and `TestManualRunHoldRemaining`. ADR038 § Manual runs records the hold.
- **Remaining gate (clears by itself):** a production deploy whose commit contains the `w4/m300 t010 start hold` fix (`git log --grep 'w4/m300 t010' origin/main`, then `git merge-base --is-ancestor <sha> <deployed sha>`). Then a worker replays P2b and nine rapid cancels (alternating `runs/{id}/cancel` and `DELETE`, 0.3 s after the `POST`) and P3, checks that none has a `cron_job_run_started` or a `cron-demo:` line, and closes t010 and t008. No user action.

## Unblock record (2026-11-16 evening, loopx w4)

- **Affected:** w4/m300 (t008, t010).
- **Source:** verified finding. The newest platform pin is `7aaa8700befa` (`546945798`, 16:22Z), and `git merge-base --is-ancestor eae6edf67 7aaa8700befa` holds, so it carries the `w4/m300 t010 start hold` fix. Production runs it: every `bex-system` platform Deployment (`bex-api`, `bex-controller-manager`, …) runs `bex-operator@sha256:49f79557…`, the digest that pin commit writes, and `bex-controller-manager` last rolled at 16:27:42Z (read-only `kubectl get deploy`).
- **Remaining work:** none gated. A worker replays P2b, nine rapid cancels (alternating `runs/{id}/cancel` and `DELETE`, 0.3 s after the `POST`) and P3 on production, checks that no canceled or replaced run has a `cron_job_run_started` or a `cron-demo:` line, that B runs once and that `startedAt ≤ finishedAt`, then closes t010 and t008.
- **Live replay (16:28–16:39Z, `muse.env`, `bex-canary`).** Fixture: Starter `busybox:1.36` cron `qa-20261116e-m300-cron` (`srv-dbtiue8ddohc73aiv260`, `sh -c 'echo cron-demo: run host=$(hostname); sleep 3'`, schedule `0 0 1 1 *`). All REST with the QA session cookie against `api.bex.co`; each cancel was sent 0.3 s after its `POST` answered.
  - **P1 holds.** `crr-3k6g…` read `pending` 0.5 s after the trigger and reached `successful` (16:29:18–16:29:27). Its Job started 2 s after `runAt`, the hold.
  - **P3 holds.** A `crr-gabd…` read `canceled` 0.4 s after the two `POST`s and never started; B `crr-f9em…` ran once (16:37:12–16:37:18). The list holds both.
  - **P2b and the rapid cancels fail in 3 of 19.** P2b (`runs/{id}/cancel`) plus 18 more, alternating with `DELETE`, every call 200/204 and every row `canceled`. 16 never started. `crr-ma7k62dl592uu21g52aa`, `crr-0omioe0hf8aiokv03e2j` and `crr-h0fibsje47d61dokuqlc` each have a `cron_job_run_started` and a `cron-demo:` line (16:31:35.52, 16:31:53.58, 16:35:15.44). `startedAt ≤ finishedAt` held on every row.
  - **Why: the operator is right, bex-api is slow.** In each of the three, the Job was created about 2 s after `runAt`, as the hold intends. The cancel reached the App 4–8 s after `runAt`: `0omi`'s `runAt` is 16:31:49.88 and its `cancelRun.requestedAt` is 16:31:57.62. bex-api stamps `runAt` before it answers the trigger, and stamps `requestedAt` only after the cancel's `AuthorizeApp` and actor record (`lego/backend/internal/apps/service.go` `TriggerCronRun`, `cancelCronRunFetched`). Those calls sometimes took seconds: one `POST` answered after 13.1 s, and one cancel after 2.9 s. Reads answered in about 0.27 s. bex-api logged repeated optimistic-lock conflicts on this App between its control-plane reconciler and the operator.
- **Fix (this run):** `manualRunStartHold` 2 s → 10 s (`lego/operator/internal/controller/app_controller.go`; ADR038 § Manual runs). That covers the 8 s tail observed, and a Render run takes longer than that to start anyway. `TestAFreshTriggerIsHeldSoAnImmediateCancelNeverStartsIt` and `TestManualRunHoldRemaining` follow the constant.
- **Cleanup.** The fixture was deleted (204, then 404), and only this run's session was revoked.
- **Remaining gate (clears by itself):** a production deploy whose commit contains the `w4/m300 t010 10 s hold` fix (`git log --grep 'w4/m300 t010 10 s hold' origin/main`, then `git merge-base --is-ancestor <sha> <deployed sha>`). Then replay P2b, at least 19 rapid cancels alternating `runs/{id}/cancel` and `DELETE` (0.3 s after the `POST` answers) and P3, check that none of them has a `cron_job_run_started` or a `cron-demo:` line, and close t010 and t008. No user action.
