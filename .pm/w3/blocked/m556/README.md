# w3 · m556 — A cron job's Runs page lists runs as Render's does: search, rows, empty states

**Worker:** worker3 **Goal:** a cron job's Runs page is Render's list: a "Search cron job runs" field, rows of status, "Cron job run", Trigger and Duration that open the run's Logs window, inline Cancel on active runs (Render's bundle has no row menu or Copy run ID on cron job runs; `hhe = false`), and Render's empty and error states. **Status:** blocked (t001–t005, t007–t009 done in `11266e81ef`; t006 step 1 walked locally at 1440 and 390 against `local-bex`; t006 step 2 waits for a production pin carrying `11266e81ef`, then t010 closes it)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record Render's cron Runs page from the walk and the bundle — **DONE** | 30m | — |
| [t002](done/t002.md) | Rows: Render's run row, linked to the run's Logs window, with inline Cancel (no row menu/Copy run ID on Render's cron runs) — **DONE** | 1h30m | t001 |
| [t003](done/t003.md) | Search: "Search cron job runs" over the loaded runs, with No matching results and Reset search — **DONE** | 45m | t002 |
| [t004](done/t004.md) | Empty and error states: never run, none in the last 90 days, and the load error, each with Render's text — **DONE** | 45m | t002, w3/m555/t002 |
| [t005](done/t005.md) | Skeleton: the Runs page's pending state matches the new list — **DONE** | 30m | t003, t004 |
| [t006](t006.md) | Verify in a real browser at 1440 and 390, on dev-3 and on production (step 1 done locally; step 2 waits for pin carrying `11266e81ef`) | 45m | t005 |
| [t007](done/t007.md) | Render parity — **DONE** | 30m | t006 |
| [t008](done/t008.md) | Simplify — **DONE** | 15m | t007 |
| [t009](done/t009.md) | Test coverage — **DONE** | 45m | t007 |
| [t010](t010.md) | Closeout | 10m | t008, t009 |

Total: **10 tasks, about 6h25m**.

## What Render does

- Evidence: a signed-in, read-only walk of four cron jobs in a Render workspace on 2026-11-21 (nothing that acts was clicked), plus Render's public dashboard bundle read on 2026-11-21 and re-read on 2026-11-22 without sign-in (`CronJob-D3WeWa8V.js`, `index-BR2q8yex.js`, `urls-B5ihPFNO.js`, `helpers-C_tFPHBn.js`, `Duration-DvaXAync.js`, `CancelReasonText-Bio2zVo0.js`).
- Walked: the search field; RUN (count), TRIGGER, DURATION; rows "Cron job run" / "Run 21h ago" / Scheduled / 14.5s, each linking to the Logs from a minute before to a minute after the run; the 90-day empty state over a disabled Trigger Run.
- Bundle correction (2026-11-22, t001): `Copy run ID` and `Cancel run` in `index-*.js` belong to Render Workflows' `taskRun` table (`resource_type: "taskRun"`), not cron job runs. On cron job runs (`ghe`), `hhe = false` (`data-has-menu: false`) and active runs have an inline destructive-ghost `Cancel` button that cancels at once with no confirm dialog.
- A repo-backed cron job's rows show "Cron job run", not a commit (`ahe` passes `build: void 0`), so no run-to-build field is needed.

## Live walk

### Local, against `yarn local-bex` (2026-11-22, t006 step 1)

**dev-3 could not run** on this host (`fs.inotify.max_user_instances` = 128 < 512 required by `scripts/mock-cluster.sh`). Walked the tree's dashboard (`vite dev`) against `local-bex` with Playwright Chromium (`en-US`, `America/Los_Angeles`) at **1440** and **390** across all six scenarios (`list`, `recent`, `never`, `suspended`, `error`, `more`):

| Point | Seen at 1440 and 390 | Verdict |
| --- | --- | --- |
| Pending vs ready geometry | Route skeleton, loading frame and ready list identical to the pixel: at 1440 `search 313+36`, `header 365+37`, rows `402+65 / 467+65 / 532+65`; at 390 `search 325+44`, `header 381+37`, rows `418+85 / 503+85 / 588+85`; `scrollW` 1440 / 390 | holds |
| Icons & tooltips | `In progress`, `Last successful run`, `Run failed`, `Canceled` | holds |
| Search & Reset | `"failed"` → 1 row, `"manual"` → 2 rows, `"zzz"` → `No matching results` + `Reset search` restoring all rows; `Load more` stays visible while searching | holds |
| Inline Cancel | Clicking `Cancel` on active run: 0 dialogs, 1 `CancelCronJobRun` call | holds |
| Row Logs link | Clicking row 2 opened `/services/nightly-report/logs?range=custom&rangeStart=…&rangeEnd=…` (exact ±1m window) | holds |
| Empty & error states | `recent` (90-day text + enabled Trigger Run), `never` (never-run text + disabled Trigger Run, tooltip "No successful build detected. View Deploys."), `suspended` (90-day text + disabled Trigger Run, tooltip "Resume this cron job to trigger a run."), `error` (standalone warning alert, no search box) | holds |
| Paging (`more`) | First read `limit: 30`, `Load more` appended 4 older runs to 34 | holds |

No page errors at either width.

### Production

Waits for a production pin carrying `11266e81ef` (t006 step 2). Then walk on production as the QA user in `bex-canary` with a disposable cron job at 1440 and 390 (empty states, triggered run, live duration, row Logs link, Cancel, search + Reset search, pending vs ready), then delete the fixture and run t010 closeout.

## bex today

- Replaced in `11266e81ef`: `cron-runs-section.tsx` now renders Render's list via `cron-runs-frame.tsx` and `cron-run-list.ts`, and `use-cron-run.ts` + `CronJobRun` query are removed.

## Definition of done

1. The Runs page lists runs as Render's does: RUN with the count, TRIGGER, DURATION; each row's status icon and tooltip, "Cron job run", relative time, trigger and live duration; the whole row opens the Logs over the run's window ± 1 minute.
2. Active runs carry Render's inline Cancel button (no confirm dialog; Render's cron run rows have no row menu or Copy run ID).
3. "Search cron job runs" filters the loaded runs; No matching results offers Reset search; Load more still works.
4. The never-run, 90-day and load-error states show Render's text, the two empty ones with Trigger Run (disabled with its reason when suspended).
5. Pending and ready match at 1440 and 390; walked locally and on production.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w3` (2026-11-21), filed by `/pmx the verified parity to w3`, item 3.
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)).
- **Expected outcome:** a Render user reads, searches and opens a cron job's runs the way Render shows them.
- **Why now:** it closes the last part of w2/m360's Runs page that was mapped without Render's dashboard, and reuses w3/m555's Trigger Run.
- **Render parity task included:** the dashboard's cron Runs page changes. No REST, GraphQL or MCP surface changes.
