# w3 · m555 — A cron job's header and actions are Render's: Trigger Run while it runs, the schedule in words, Manual Build on its Deploys page

**Worker:** worker3 **Goal:** every page of a running cron job carries Render's one header action, Trigger Run, which runs at once and opens the Logs on Live tail; the header says "Runs `<command>` <schedule in words>"; and a repo-backed cron job builds from its Deploys page's Manual Build, as on Render. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Record Render's cron job header and actions from the walk and the bundle | 30m | — |
| [t002](t002.md) | Header: Trigger Run while the cron job runs, no Connect or Manual Deploy menu, and Render's disabled reasons | 1h15m | t001 |
| [t003](t003.md) | Trigger Run's click: no confirm, then Logs on Live tail; the Runs card loses its own button | 45m | t002 |
| [t004](t004.md) | Header facts: "Runs `<command>` <schedule in words>" and "Last successful run <date>" | 45m | t001 |
| [t005](t005.md) | Deploys page: Manual Build for a repo-backed cron job, and Render's "Search deploys and commits" on every Deploys page | 1h | t001, w3/m554/t003 |
| [t006](t006.md) | Skeletons: the header's action slot and facts line, and the Deploys toolbar | 30m | t002, t004, t005 |
| [t007](t007.md) | Verify in a real browser at 1440 and 390, on dev-3 and on production | 45m | t003, t006 |
| [t008](t008.md) | Render parity | 30m | t007 |
| [t009](t009.md) | Simplify | 15m | t008 |
| [t010](t010.md) | Test coverage | 45m | t008 |
| [t011](t011.md) | Closeout | 10m | t009, t010 |

Total: **11 tasks, about 7h10m**.

## What Render does

- Evidence: a signed-in, read-only walk of four cron jobs in a Render workspace on 2026-11-21 (nothing that acts was clicked), plus Render's public dashboard bundle read the same day without sign-in (`CronJob-D3WeWa8V.js` sha256 `23d3ad7d7c97…`, `index-BR2q8yex.js` `f19b12ced9c1…`, `urls-B5ihPFNO.js` `3ab9b8053a93…`, `helpers-C_tFPHBn.js` `302444d539f3…`; the hashes change with every Render deploy, so find the chunks by name, as `metrics-page.md` § Status-code label describes).
- Walked: one header action, Trigger Run, while the cron job runs; none while suspended (a notice carries Resume service); the facts line "Runs `<command>` at 09:00 AM UTC" then "Last successful run <date>"; Manual Build beside "Search deploys and commits" on Deploys; a suspended cron job's disabled Trigger Run says "Resume this cron job to trigger a run."
- Bundle only: no confirm, `logs?r=live` after a trigger, the no-build and permission reasons, the two failure toasts, Manual Build's hidden and disabled states.

## bex today

- The cron header carries Connect, Resume while suspended, and the Manual Deploy menu (`service-detail-header.tsx`, `manual-deploy-button.tsx`).
- Trigger Run sits only in the Runs card's header, behind a confirm (w4/m114 t003), and stays on Runs.
- The header shows "Schedule: <expression>"; `describeCron` exists in `features/services/lib/cron.ts`.
- Deploys search reads "Search…".

## Definition of done

1. A running cron job's header shows Trigger Run on every cron page and nothing else; a suspended one's shows no action and Render's suspension notice with Resume service; other types' headers are unchanged.
2. Trigger Run is disabled with Render's reason when suspended, when a repo-backed cron job has no successful build, and when refused; a click runs at once and opens the Logs on Live tail; a failure toasts until dismissed. No other Trigger Run or confirm remains on the Runs page.
3. The header reads "Runs `<command>` <schedule in words>" and "Last successful run <date>".
4. A repo-backed cron job's Deploys page has Manual Build (hidden for an image-backed one), which builds and opens the new build; every Deploys page's search reads "Search deploys and commits".
5. Pending and ready match at 1440 and 390; walked on dev-3 and on production.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w3` (2026-11-21), filed by `/pmx the verified parity to w3`, item 2.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)).
- **Expected outcome:** a cron job is run and built from the places Render puts those actions, and its header says what it runs and when in words.
- **Why now:** it follows w3/m554's routes (Manual Build lives on the Deploys page m554 restores), and w3/m556's empty states reuse the Trigger Run this milestone builds.
- **Open decision, recorded as a recommendation:** dropping the confirm w4/m114 t003 added follows Render's bundle. That is the proposal's recommendation under ADR008's parity-first rule (parity work is not parked on an optional decision), not a user decision. Keeping the confirm would be a divergence awaiting the user's sign-off in ADR018. Checking the click on Render needs a disposable Render cron job (at least $1 a month), the user's call; until then t001 records the click as bundle-only.
- **Render parity task included:** the dashboard's cron header and Deploys page change. No REST, GraphQL or MCP surface changes.
