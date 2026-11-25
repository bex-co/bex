# w3 · m556 — A cron job's Runs page lists runs as Render's does: search, rows, empty states

**Worker:** worker3 **Goal:** a cron job's Runs page is Render's list: a "Search cron job runs" field, rows of status, "Cron job run", Trigger and Duration that open the run's Logs window, inline Cancel on active runs (Render's bundle has no row menu or Copy run ID on cron job runs; `hhe = false`), and Render's empty and error states. **Status:** done (shipped in `11266e81ef`; walked locally at 1440 and 390 against `local-bex` on 2026-11-22 and on production pin `9cc529d10432` on 2026-11-25)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record Render's cron Runs page from the walk and the bundle — **DONE** | 30m | — |
| [t002](done/t002.md) | Rows: Render's run row, linked to the run's Logs window, with inline Cancel (no row menu/Copy run ID on Render's cron runs) — **DONE** | 1h30m | t001 |
| [t003](done/t003.md) | Search: "Search cron job runs" over the loaded runs, with No matching results and Reset search — **DONE** | 45m | t002 |
| [t004](done/t004.md) | Empty and error states: never run, none in the last 90 days, and the load error, each with Render's text — **DONE** | 45m | t002, w3/m555/t002 |
| [t005](done/t005.md) | Skeleton: the Runs page's pending state matches the new list — **DONE** | 30m | t003, t004 |
| [t006](done/t006.md) | Verify in a real browser at 1440 and 390, on dev-3 and on production (step 1 locally against `local-bex`, step 2 on production pin `9cc529d10432`) — **DONE** | 45m | t005 |
| [t007](done/t007.md) | Render parity — **DONE** | 30m | t006 |
| [t008](done/t008.md) | Simplify — **DONE** | 15m | t007 |
| [t009](done/t009.md) | Test coverage — **DONE** | 45m | t007 |
| [t010](done/t010.md) | Closeout — **DONE** | 10m | t008, t009 |

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

### Production (2026-11-25, t006 step 2)

Pin `9cc529d10432` (carries `11266e81ef`), 15:19–15:37 UTC; the `dashboard` pod ran the pin's digest. As the QA user in `bex-canary` (`tea-daif693dqjvc73e7as3g`), over REST: the repo-backed cron job `qa-20261125-m556-repo` (disposable, `srv-dc3fp4lksp0s73b5oqbg`, `render-examples/express-hello-world` on `main`, Node), the image-backed `qa-20261125-m556-img` (disposable, `srv-dc3fp4p5k4kc73dtaje0`, `docker.io/library/busybox:1.36`, a 45 s `sleep`, switched once to `exit 3` and back for a failed run) and `qa-20261125-m556-fail` (disposable, `srv-dc3fustksp0s73b5oqp0`, busybox, `sleep 40; exit 3`, so it never succeeds), each `0 9 * * *` on Starter. Google Chrome headless through `playwright-core`, `en-US`, zone America/Los_Angeles, at 1440 and 390.

| Point | Seen at 1440 and 390 | Verdict |
| --- | --- | --- |
| Never run | the repo-backed cron job during its first build: "This cron job hasn't run yet. It will run once its first build finishes, or you can trigger a run manually." over a disabled Trigger Run, tooltip "No successful build detected. View Deploys."; once the build was live, the 90-day state with Trigger Run enabled | holds |
| 90 days and suspended | the image-backed cron job before a run: "This cron job hasn't run in the last 90 days. You can trigger a run manually, or wait for the next scheduled run." over an enabled Trigger Run; suspended over REST, the same text over a disabled Trigger Run, tooltip "Resume this cron job to trigger a run.", then resumed | holds |
| Load error | the `CronJobRuns` read answered 500 by a Playwright route on a client navigation from Deploys: the standalone alert "We're sorry. We weren't able to load runs for this cron job. Refresh and try again?" and no search box | holds |
| Triggered run, live duration | Trigger Run in the empty state (1440) and in the header (390): no dialog, one `RunCronJob`, the Logs on Live tail within 350 ms. Back on Runs the new row read "In progress", "Run just now by you", Manual, and its duration ticked 2s → 5s over 3 s; it turned "Last successful run" (49s) with no reload, and the success before it became "Run succeeded" | holds |
| Rows | RUN with the count (4, then 6), TRIGGER, DURATION at 1440; at 390 Trigger and Duration fold under the run ("Manual 4s"). Tooltips "Run failed", "Last successful run", "Canceled", "Run succeeded"; a canceled row adds "· Canceled by you" | holds |
| Inline Cancel | on an active run (1440: 87×32 at the row's end; 390: 87×44): no dialog, one `CancelCronJobRun`, the row turned "Canceled" in place | holds |
| Row Logs link | a click on the newest success's row (started 15:25:33Z, ended 15:26:23Z) opened `/services/<id>/logs?range=custom&rangeStart=2026-11-25T15:24:33.000Z&rangeEnd=2026-11-25T15:27:23.000Z`, the run's window ± 1 minute | holds |
| Search and Reset search | over 4 runs: "failed" 1 row, "manual" 4, "canceled" 1, "last successful" 1, the full id `crr-1c82un8mqlcvlvnqgo37` 1, "zzz" "No matching results" with Reset search, which restored all 4 and emptied the field; Clear search emptied it too | holds |
| Pending against ready | on a client navigation from Deploys, the route's chunk (`services._serviceId.runs-*.js`) held, then the `CronJobRuns` read held, then both released. 1440: search 313+36, header 365+37, rows 402/467/532/597/662/727 at 65 px in all three states. 390: search 391+44, header 447+37, rows 484/569/654/739/824/909 at 85 px in all three. The ready list's last row is 1 px shorter (no divider under it); the frame's 30 rows shrink to the ready count once, as t005 meant. `scrollWidth` 1440 / 390 | holds |
| Page errors | none on any page of a cron job without a successful run, with in-progress, failed and canceled rows on its Runs page (direct loads at both widths). Once a cron job had succeeded, React #418 on every load, on its Deploys and Settings pages as much as on Runs | **w3/m555's**, not this list's |

The React #418 is w3/m555's header defect ("Last successful run <date>" in a `<p>` over a `<div>` placeholder), fixed in `a43d36790f`, which this pin does not carry; w3/m555 is parked until production carries it and re-walks those pages. Not walked on production: Load more (it needs over 30 runs) and a Scheduled trigger (the schedule did not come round), both walked locally above.

**Cleanup:** the three cron jobs were deleted (`DELETE` 204, by-id `GET` 404 each); the app cluster kept no App, CronJob, Job, Pod or Deployment of any of them. The QA session was logged out (`qa-login.sh --logout`, after which it answered 401), and the session jar, the state file and the fetched kubeconfig were deleted.

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

## Unblock (2026-11-25)

- **Affected:** t006 step 2, then t010.
- **Source:** verified evidence (`/loopx w3` dispatch, gate check).
  - `git merge-base --is-ancestor 11266e81ef 9cc529d10432` holds. The newest `chore(deploy): pin platform images to …` on origin/main is `d8d0713b48`, which pins `9cc529d10432`.
  - On the production app cluster the `dashboard` Deployment and its one pod run that pin's digest (`bex-dashboard@sha256:5e6b912879ab…`, pod started 2026-11-25 11:12 UTC).
- **Gate:** the `pin must carry` gate is cleared; nothing else gates the walk. The QA user's sign-in comes from `scripts/qa-login.sh`.
- **Next:** t006 step 2 as § Live walk › Production describes, recorded there; then t010 closes the milestone if every point holds.

## Closeout (2026-11-25)

t006 step 2's production walk held every point of the definition of done at 1440 and 390 (§ Live walk › Production); t010 closed the milestone. Its one page error, React #418 on a cron job that has succeeded, belongs to w3/m555 and is fixed in `a43d36790f`, which w3/m555 waits on.
