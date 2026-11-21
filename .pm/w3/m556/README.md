# w3 · m556 — A cron job's Runs page lists runs as Render's does: search, rows, empty states

**Worker:** worker3 **Goal:** a cron job's Runs page is Render's list: a "Search cron job runs" field, rows of status, "Cron job run", Trigger and Duration that open the run's Logs window, a row menu with Cancel run and Copy run ID, and Render's empty and error states. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Record Render's cron Runs page from the walk and the bundle | 30m | — |
| [t002](t002.md) | Rows: Render's run row, linked to the run's Logs window, with Cancel run and Copy run ID | 1h30m | t001 |
| [t003](t003.md) | Search: "Search cron job runs" over the loaded runs, with No matching results and Reset search | 45m | t002 |
| [t004](t004.md) | Empty and error states: never run, none in the last 90 days, and the load error, each with Render's text | 45m | t002, w3/m555/t002 |
| [t005](t005.md) | Skeleton: the Runs page's pending state matches the new list | 30m | t003, t004 |
| [t006](t006.md) | Verify in a real browser at 1440 and 390, on dev-3 and on production | 45m | t005 |
| [t007](t007.md) | Render parity | 30m | t006 |
| [t008](t008.md) | Simplify | 15m | t007 |
| [t009](t009.md) | Test coverage | 45m | t007 |
| [t010](t010.md) | Closeout | 10m | t008, t009 |

Total: **10 tasks, about 6h25m**.

## What Render does

- Evidence: a signed-in, read-only walk of four cron jobs in a Render workspace on 2026-11-21 (nothing that acts was clicked), plus Render's public dashboard bundle read the same day without sign-in (`CronJob-D3WeWa8V.js` sha256 `23d3ad7d7c97…`, `index-BR2q8yex.js` `f19b12ced9c1…`, `urls-B5ihPFNO.js` `3ab9b8053a93…`, `helpers-C_tFPHBn.js` `302444d539f3…`; the hashes change with every Render deploy, so find the chunks by name, as `metrics-page.md` § Status-code label describes).
- Walked: the search field; RUN (count), TRIGGER, DURATION; rows "Cron job run" / "Run 21h ago" / Scheduled / 14.5s, each linking to the Logs from a minute before to a minute after the run; the 90-day empty state over a disabled Trigger Run.
- Bundle only: the tooltips, the row menu, No matching results, the never-run state, the load error, the search's fields.
- A repo-backed cron job's rows show "Cron job run", not a commit, so no run-to-build field is needed (the brainstorm's earlier item for one was dropped after the walk).

## bex today

- `cron-runs-section.tsx`: a Started / Duration / Status table with an expandable detail (View logs), a confirmed Cancel, cursor paging, and one empty line.
- Runs come from `cronJobRuns` with `triggeredBy`/`canceledBy` (w4/631) over w3/m366's fact-backed history.

## Definition of done

1. The Runs page lists runs as Render's does: RUN with the count, TRIGGER, DURATION; each row's status icon and tooltip, "Cron job run", relative time, trigger and live duration; the whole row opens the Logs over the run's window ± 1 minute.
2. A row menu offers Cancel run (for a pending or running run) and Copy run ID.
3. "Search cron job runs" filters the loaded runs; No matching results offers Reset search; Load more still works.
4. The never-run, 90-day and load-error states show Render's text, the two empty ones with Trigger Run (disabled with its reason when suspended).
5. Pending and ready match at 1440 and 390; walked on dev-3 and on production.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w3` (2026-11-21), filed by `/pmx the verified parity to w3`, item 3.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)).
- **Expected outcome:** a Render user reads, searches and opens a cron job's runs the way Render shows them.
- **Why now:** it closes the last part of w2/m360's Runs page that was mapped without Render's dashboard, and reuses w3/m555's Trigger Run.
- **Render parity task included:** the dashboard's cron Runs page changes. No REST, GraphQL or MCP surface changes.
