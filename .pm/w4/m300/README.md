# w4 · m300 — A triggered cron run exists from the moment it is acknowledged: readable, cancellable, and canceled (not lost) when a second trigger replaces it, as on Render

**Worker:** worker4 **Goal:** the run object that `POST /cron-jobs/{id}/runs` returns is the run. Every read finds it as `pending` at once, Render's cancel call cancels it, and a second trigger records it as canceled instead of dropping it. **Status:** in progress — t001–t007 done (shipped); t008 runs the P1–P4 production replay once the release pins this build

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
| [t008](t008.md) | Closeout | 15m | w4/m300/t006, w4/m300/t007 |

Eight tasks, 300m (5h). One finding, major. This milestone schedules the work only.

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
- **P4 replay.** At 1440 and 390 px, the dashboard's Trigger Run shows a pending row for the returned ID within 1 s, with Cancel offered and the 3 s poll running. The row's accessible name never contains the `crr-` ID.
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
  - [ADR038](../../../docs/ADR038-cron-jobs.md) (cron jobs).
  - [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first) and [ADR018](../../../docs/ADR018-render-parity.md).
  - [ADR006](../../../docs/ADR006-bex-api.md): REST, GraphQL and MCP move together.
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
