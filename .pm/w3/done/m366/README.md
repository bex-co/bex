# w3 · m366 — A cron job's run history and `lastSuccessfulRunAt` outlive the ten-run status window, as on Render

**Worker:** worker3 **Goal:** a cron job's runs list, run reads and `lastSuccessfulRunAt` come from the durable run facts bex already records, merged with the live status. A run is no longer forgotten once ten newer runs have happened, and the last success stays known through any number of failures. **Status:** done (2026-11-17)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Store: list a cron job's runs from the `cron_job_run_started`/`_ended` facts, newest first with a cursor, plus the newest succeeded run — **DONE** | 60m | — |
| [t002](done/t002.md) | apps: `ListCronRuns`, `GetCronRun` and `lastSuccessfulRunAt` read the facts merged with `status.runs` and the unmaterialized runs `cronRunsOf` adds; no run regresses or appears twice — **DONE** | 75m | t001 |
| [t003](done/t003.md) | REST, GraphQL and MCP: runs older than the status window list and read with 200; trigger and cancel unchanged — **DONE** | 45m | t002 |
| [t004](done/t004.md) | Edges and production check: runs before the facts existed, runs whose logs aged out, and a long-running production cron — **DONE** | 30m | t003 |
| [t005](done/t005.md) | Docs: ADR038 run history, `docs/render-artifacts/cron-runs.md`, the ADR018 cron row — **DONE** | 20m | t004 |
| [t006](done/t006.md) | Render parity — **DONE** | 20m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 30m | t007 |
| [t009](done/t009.md) | Closeout — **DONE** | 10m | t008 |

## Definition of done

- On dev-3, a cron job that has run more than ten times lists every run through REST `GET /v1/cron-jobs/{id}/runs` with cursor paging, and `GET …/runs/{runId}` of the eleventh-newest run answers 200 with its status, times and failure reason.
- After one success followed by ten failures, `serviceDetails.lastSuccessfulRunAt` (REST), the GraphQL service read and MCP `get_service` all still carry that success's finish time.
- A run in progress, and a run just triggered but not yet in status, each appear exactly once. A run canceled before it started stays canceled after it leaves the status window (w5/m108's rule).
- On production, after the deploy, a QA cron in `bex-canary` that has run at least twelve times lists all of them, and the dashboard Runs page (w2/m360) pages past the tenth.
- Trigger, cancel and the cancel-then-replace rule behave as before (w4/m300's tests stay green).

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w3` 2026-11-12, item 2, filed by `/pmx them all to w3`. Re-checked 2026-11-12 against `origin/main` `24bea6c5c`.
- **bex today:**
  - The operator keeps at most ten runs in `App.status.runs` (`lego/operator/internal/controller/app_controller.go:5024`, `maxCronRuns = 10`; ADR038 and `docs/render-artifacts/cron-runs.md` say "capped at ten entries").
  - `ListCronRuns` and `GetCronRun` (`lego/backend/internal/apps/service.go:4322-4346`) read only `cronRunsOf(a, now)`, i.e. that status slice plus unmaterialized runs (`service.go:1360`). REST (`apps/rest.go:1277-1280`), GraphQL (`apps/graphql.go:1305-1315`) and MCP (`apps/mcp.go:740-758`) all go through them.
  - `lastSuccessfulCronRunAt` (`service.go:1444`) scans the same slice, so ten failures in a row drop the field.
  - A `*/5` cron therefore shows 50 minutes of history. A run named in the Events feed can answer 404 by id.
  - The durable data already exists: the reconciler records `cron_job_run_started` and `cron_job_run_ended` facts with the run's `crr-` id, status, reason code and exit code (`lego/backend/internal/store/reconciler.go:578-620`) into `service_event_facts` (migration 0043; cron facts since 0075, 2026-08-16). They are not purged.
- **Render:** `cronJobDetails.lastSuccessfulRunAt` (pinned `render-public-api-1.json`; optional, date-time) reports the last success regardless of how many runs followed. [render.com/docs/cronjobs](https://render.com/docs/cronjobs): "Your cron job's **Runs** page shows run history and logs for each run." Render's public API has only trigger and cancel for runs; bex's list and read routes are documented extensions (w2/m36) that back the Runs page.
- **Coordination:**
  - w2/m360 (done) shipped the Runs page; it pages whatever `ListCronRuns` returns, so it gains the history with no dashboard change.
  - w4/m300 (blocked; t009 and t010 open) owns acknowledged-trigger and canceled-run semantics in the same functions. Keep its tests green and do not change those semantics here.
  - w5/m108 (done): a canceled manual run must not come back after eviction. The facts path must respect `CancelRunRecorded`.
- **Goal linkage:** ADR008 Render parity first, on the cron job surface (API field and dashboard Runs page).
- **Expected outcome:** a cron job's history and its last success are as durable as its Events feed. Frequent cron jobs keep useful history, and a run linked from an event can always be opened.
- **Why now:** the Runs page just shipped (w2/m360) and makes the ten-run ceiling visible to every user. The facts it needs have been recorded for three months.
- **Render parity included:** REST, GraphQL and MCP reads, and the dashboard Runs page through w2/m360.

## Unblock — 2026-11-17 (`/loopx w3`)

- **Affected:** t004, then t009. **Source:** verified evidence, no user decision needed (the gate was a deploy).
- **Finding:** the latest production pin on `origin/main` is `db48cde2bae9` (`fc4fbf28a`, 12:08Z), and `git merge-base --is-ancestor 3db02dcd9 db48cde2bae9` holds. bex-api (2/2), the controller manager and the other platform Deployments in `bex-system` run that pin's `bex-operator@sha256:ced89914…`, and the live `apps.app.bex.co` CRD carries `status.lastSuccessfulRunAt` (kubectl against the CAPH `bex` cluster through its Hetzner `bex-kube-apiserver` load balancer, TLS verified against the cluster CA).
- **Next step:** run t004's production check in `bex-canary` as the QA user. Its results follow.

## Closeout — 2026-11-17 (`/loopx w3`, t004 + t009)

- **Production, pin `db48cde2bae9`:** QA cron `srv-dbu5tjeggh5s73emef30` ran `* * * * *` in `bex-canary`, with one success at 14:09Z among 20 finished runs.
  - REST cursor paging by 5 listed all 20. GraphQL, MCP and the dashboard Runs page (Load more to 21 rows) listed the same runs.
  - The eleventh-newest run, which had left the status window, read 200 on all three APIs. Its failure reason is on its `cron_job_run_ended` event; Render's `cronJobRun` has no reason field.
  - After ten failures, `lastSuccessfulRunAt` read 14:09:19Z on REST, GraphQL and MCP.
  - The canceled-before-start run stays `canceled` after eviction.
  - An evicted run's cancel still answers 409.
  - A log window past the plan's 30-day retention shows "No logs in this time range", not an error.
  - The fixture was deleted. Details, ids and times are in [t004](done/t004.md).
- **Observed, not filed:** a manual run canceled before it starts, while a scheduled run is active, moves one place down the list once it leaves the window. The window orders by trigger time; the facts order by end time. Paging has no gap or repeat.
