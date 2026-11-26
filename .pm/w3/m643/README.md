# w3 · m643 — A cron job's header, its suspension notice and Manual Build say and do what Render's do: the last run's zone, every schedule in words, Resume opens the new deploy

**Worker:** worker3 **Goal:** the five differences from Render's cron header and actions that w3/m555 left are gone: the last run's date carries its zone and Render's UTC tooltip, every valid schedule reads in Render's cronstrue words, a Hobby workspace's notice says "Suspended by you", Resume service opens the deploy the resume starts, and Manual Build toasts only a failure, which stays until dismissed. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | The last successful run reads Render's date with its zone, and "UTC: …" in its tooltip off UTC | 30m | — |
| [t002](t002.md) | Every valid schedule in Render's cronstrue words | 1h | — |
| [t003](t003.md) | "Suspended by you" on a Hobby workspace's suspension notice | 30m | — |
| [t004](t004.md) | Resume opens a deploy with Render's `service_resumed` trigger, and GraphQL `resumeService` answers it | 1h30m | — |
| [t005](t005.md) | Resume service opens the new deploy, with Render's resume toasts | 45m | t004 |
| [t006](t006.md) | Manual Build: no toast on success, and a failure that stays until dismissed | 20m | — |
| [t007](t007.md) | Verify in a real browser at 1440 and 390, locally and on production | 45m | t001, t002, t003, t005, t006 |
| [t008](t008.md) | Render parity | 30m | t007 |
| [t009](t009.md) | Simplify | 15m | t008 |
| [t010](t010.md) | Test coverage | 45m | t008 |
| [t011](t011.md) | Closeout | 10m | t009, t010 |

Total: **11 tasks, about 7h**.

## What Render does

Read on 2026-11-25 from Render's public dashboard bundle without sign-in (`https://dashboard.render.com/` and its `./*.js` imports, 459 chunks; the hashes change with every Render deploy, so find the chunks by name, as [metrics-page.md § Status-code label](../../../docs/render-artifacts/metrics-page.md) describes): `page-templates-TTlbtDSy.js` (sha256 `9e445c27098e…`), `AuthContext-P0VAIYE0.js` (`d9a49c1b5d3c…`), `suspension-CuMMXxIN.js` (`e2929a353e47…`), `CronJob-DioyV2Vf.js` (`24992a6b22d2…`), `git-sDZPd43G.js` (`6df549965d16…`), `service-DenIj-uU.js` (`9b4afb0269a0…`), `billing-confirm-plan-utils-DuDK6s98.js` (`2d6258290622…`) and `index-D5V4ZgbE.js`; plus Render's public API spec as pinned in `lego/backend/internal/api/openapi/render-public-api-1.json`.

- **The last run** (`page-templates-*.js` `CronJobRunMetadata`, `AuthContext-*.js` `H`, `G`, `q`): "Last successful run " then the date formatted `LL [at] LT z` in the viewer's zone ("November 21, 2026 at 9:00 AM PST"). When the viewer's zone (`moment().local().format("z")`) is not `UTC`, the date is a hover-underlined term whose tooltip is a `<time>` reading "UTC: " and the instant formatted `MMM D hh:mm:ss A` in UTC ("UTC: Nov 21 05:00:00 PM"); in UTC there is no tooltip. "No successful runs yet." before the first.
- **The schedule** (`AuthContext-*.js` `z`, `R`, `V`): split on spaces, five fields, `cron-validator`'s `isValidCron(schedule.replace(/\?/g, "*"), { alias: true, allowNthWeekdayOfMonth: true })`, and not a day-of-month no listed month has (`R`); then `cronstrue.toString(schedule)` with default options, its first letter lowercased, and " UTC" after each AM or PM. Anything else reads nothing. So `*/40 * * * *` reads "every 40 minutes" on Render.
- **The notice's title** (`suspension-*.js` `g`, `CronJob-*.js` `Fr`): `tier === TIER1 ? "Suspended by you" : "Manually suspended"`, from the current workspace's tier. `TIER1` is the Hobby workspace plan (`billing-confirm-plan-utils-*.js`: `TIER1` → `hobby`, one member; `TIER2` → `professional`, `TIER3` → `organization`). The same title heads the removed-card suspension's notice.
- **Resume service** (`CronJob-*.js` `Pr`, `service-*.js` `resumeService`): the mutation selects `... on CronJob { build { ...buildFields } }` and `... on Server { deploy { ...deployFields } }`. On success it toasts "**`<name>`** has been resumed." (auto-dismissed) and opens the cron job's build or the server's deploy when the answer carries one. A failure toasts "An error occurred while resuming **`<name>`**. Please try again." (auto-dismissed). Render's REST `deploy.trigger` enum carries `service_resumed`.
- **Manual Build** (`index-*.js`, the Deploys page's `buildCronJob`): `onCompleted` opens the new build with no toast; `onError` toasts the server's message with `autoDismiss: false`.

## bex today (main `9ae100c599`)

- `CronRunFacts` (`dashboard/src/features/services/components/service-detail-header.tsx`) draws the date with `LocalDateTime` and no zone, inside `InstantTooltip`, which lists local, UTC and Unix time (w4/716).
- `describeCronRun` (`dashboard/src/features/services/lib/cron.ts`) words only the shapes `cronShape` names and the header shows any other schedule as its expression; it declines `*/40` on purpose, as `describeCron` does. The dashboard has no `cronstrue` dependency.
- `suspendedNoticeTitleKey` (`dashboard/src/features/services/lib/status.ts`) gives "Manually suspended" for a `user` suspender on every plan. The workspace's plan is on the workspace context (`dashboard/src/features/workspaces/types.ts` `plan`).
- `setSuspended` (`lego/backend/internal/apps/service.go`) flips `Spec.Suspended` and opens no deploy; GraphQL `resumeService` answers `id suspended suspenders phase`; `useServiceLifecycle` toasts "Resuming {name}…" or "Could not update {name}. Please try again." and stays on the page.
- `ManualBuildButton` uses `useTriggerDeploy` (`dashboard/src/features/services/hooks/use-trigger-deploy.ts`), which toasts "Deploy triggered." and a failure that closes itself.

## Definition of done

1. A cron job's "Last successful run" reads Render's date with the zone's abbreviation in the viewer's zone, with a "UTC: …" tooltip only off UTC, and no hydration error.
2. The header words every schedule Render words, in Render's words in English (cronstrue's other locales in the other dashboard languages), and shows nothing where Render shows nothing.
3. A suspended service's notice in a Hobby workspace is titled "Suspended by you"; on other plans "Manually suspended".
4. A resume opens a deploy with trigger `service_resumed` on REST, GraphQL and MCP reads, and Resume service opens it, with Render's two resume toasts.
5. Manual Build shows no toast on success and keeps a failure until dismissed.
6. Each is walked at 1440 and 390 locally and on production; ADR018's Cron job and Suspend / Resume rows say what bex does, with any difference left filed or listed as awaiting sign-off.

## Source + Goal linkage

- **Source:** promoted from [w3/109](../done/109.md) (filed 2026-11-22 by w3/m555 t008) by `/loopx w3` on 2026-11-25. Weighing the note took it past one sitting: Render's resume opens a deploy, which bex-api does not record, so one item is a backend change; and wording every schedule needs cronstrue across eight dashboard languages.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)).
- **Expected outcome:** a cron job's header, its suspension notice and its Manual Build say and do what Render's do, and a resume leaves a deploy in the history as Render's does.
- **Why now:** w3/m555 shipped the header and actions and left exactly these five differences, each unsigned in ADR018; w3/125 changes the same notice next.
- **Render parity task included:** the dashboard's cron header, the suspension notice on every type, Manual Build, and the resume verb's deploy on REST, GraphQL and MCP.
