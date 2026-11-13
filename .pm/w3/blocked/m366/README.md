# w3 · m366 — A cron job's run history and `lastSuccessfulRunAt` outlive the ten-run status window, as on Render

**Worker:** worker3 **Goal:** a cron job's runs list, run reads and `lastSuccessfulRunAt` come from the durable run facts bex already records, merged with the live status. A run is no longer forgotten once ten newer runs have happened, and the last success stays known through any number of failures. **Status:** blocked (t004: waits on the batched production deploy carrying `3db02dcd9`; the production pin is batched (`deploy.yml` pins only when the current pin is ≥ 6 h old; the pin was `4c3c68e6b` at 05:33Z, so the next is ~11:33Z 2026-11-13 or later). Then a worker, as the QA user in `bex-canary`, creates a `* * * * *` QA cron, waits for at least twelve runs, checks REST paging, the eleventh-newest run's read and the Runs page past the tenth, and deletes the fixture — no user decision needed; then t009 closes)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Store: list a cron job's runs from the `cron_job_run_started`/`_ended` facts, newest first with a cursor, plus the newest succeeded run — **DONE** | 60m | — |
| [t002](done/t002.md) | apps: `ListCronRuns`, `GetCronRun` and `lastSuccessfulRunAt` read the facts merged with `status.runs` and the unmaterialized runs `cronRunsOf` adds; no run regresses or appears twice — **DONE** | 75m | t001 |
| [t003](done/t003.md) | REST, GraphQL and MCP: runs older than the status window list and read with 200; trigger and cancel unchanged — **DONE** | 45m | t002 |
| [t004](t004.md) | Edges and production check: runs before the facts existed, runs whose logs aged out, and a long-running production cron | 30m | t003 |
| [t005](done/t005.md) | Docs: ADR038 run history, `docs/render-artifacts/cron-runs.md`, the ADR018 cron row — **DONE** | 20m | t004 |
| [t006](done/t006.md) | Render parity — **DONE** | 20m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 30m | t007 |
| [t009](t009.md) | Closeout | 10m | t008 |

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
