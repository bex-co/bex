# w3 · m643 — A cron job's header, its suspension notice and Manual Build say and do what Render's do: the last run's zone, every schedule in words, Resume opens the new deploy

**Worker:** worker3 **Goal:** the five differences from Render's cron header and actions that w3/m555 left are gone: the last run's date carries its zone and Render's UTC tooltip, every valid schedule reads in Render's cronstrue words, a Hobby workspace's notice says "Suspended by you", Resume service opens the deploy the resume starts, and Manual Build toasts only a failure, which stays until dismissed. **Status:** blocked (pin must carry `c28239c97b`: t007's production re-check outside English, then t011; t001–t006 and t008–t010 done, t007 walked locally and on production pin `9b91284a4a2e`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | The last successful run reads Render's date with its zone, and "UTC: …" in its tooltip off UTC — **DONE** | 30m | — |
| [t002](done/t002.md) | Every valid schedule in Render's cronstrue words — **DONE** | 1h | — |
| [t003](done/t003.md) | "Suspended by you" on a Hobby workspace's suspension notice — **DONE** | 30m | — |
| [t004](done/t004.md) | Resume opens a deploy with Render's `service_resumed` trigger, and GraphQL `resumeService` answers it — **DONE** | 1h30m | — |
| [t005](done/t005.md) | Resume service opens the new deploy, with Render's resume toasts — **DONE** | 45m | t004 |
| [t006](done/t006.md) | Manual Build: no toast on success, and a failure that stays until dismissed — **DONE** | 20m | — |
| [t007](t007.md) | Verify in a real browser at 1440 and 390, locally and on production | 45m | t001, t002, t003, t005, t006 |
| [t008](done/t008.md) | Render parity — **DONE** | 30m | t007 |
| [t009](done/t009.md) | Simplify — **DONE** | 15m | t008 |
| [t010](done/t010.md) | Test coverage — **DONE** | 45m | t008 |
| [t011](t011.md) | Closeout | 10m | t009, t010 |

Total: **11 tasks, about 7h**.

## What Render does

Read on 2026-11-25 from Render's public dashboard bundle without sign-in (`https://dashboard.render.com/` and its `./*.js` imports, 459 chunks; the hashes change with every Render deploy, so find the chunks by name, as [metrics-page.md § Status-code label](../../../../docs/render-artifacts/metrics-page.md) describes): `page-templates-TTlbtDSy.js` (sha256 `9e445c27098e…`), `AuthContext-P0VAIYE0.js` (`d9a49c1b5d3c…`), `suspension-CuMMXxIN.js` (`e2929a353e47…`), `CronJob-DioyV2Vf.js` (`24992a6b22d2…`), `git-sDZPd43G.js` (`6df549965d16…`), `service-DenIj-uU.js` (`9b4afb0269a0…`), `billing-confirm-plan-utils-DuDK6s98.js` (`2d6258290622…`) and `index-D5V4ZgbE.js`; plus Render's public API spec as pinned in `lego/backend/internal/api/openapi/render-public-api-1.json`.

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

## Live walk

**Locally, 2026-11-25** (t007's local half): dev-3 (`bash scripts/dev-env.sh 3 up`, after `bash scripts/mock-cluster.sh` refreshed the shared cluster's stale kubeconfig) with bex-api and the dashboard built from the tree on top of `407fcb0ff0`, in Google Chrome (headless, Playwright) at 1440 and 390, as a fresh sign-up in its own Hobby workspace (disposable) with an image-backed web service, an image-backed cron job and a repo-backed cron job (all disposable, local only).

- **The last run** (cron job, after its scheduled runs): America/Los_Angeles reads "Last successful run November 25, 2026 at 6:32 PM PST" and its hover tooltip "UTC: Nov 26 02:32:03 AM"; UTC reads "… November 26, 2026 at 2:32 AM UTC" with no tooltip; zh in Asia/Shanghai reads "上次成功运行 2026年11月26日 上午10:40 GMT+8" with "UTC：11月26日 上午02:40:04". At 390 the line wraps to two lines and the header's pending frame (a client navigation with GraphQL held) has the same box as the ready line at both widths (1440: y 180, 20px; 390: y 252, 40px). No page error.
- **The schedule:** `*/2 * * * *` "every 2 minutes", `0 9 * * 1-5` "at 09:00 AM UTC, Monday through Friday", `*/40 * * * *` "every 40 minutes", `0 0 1 * *` "at 12:00 AM UTC, on day 1 of the month", at both widths; zh "每隔 2 分钟" and "在 UTC 上午 09:00, 星期一至星期五".
- **The notice:** in the Hobby workspace a suspended web service (390) and cron job (1440) read "Suspended by you"; with the workspace on Pro (`changeWorkspacePlan`) both read "Manually suspended" (1440 and 390).
- **Resume service:** from the header notice (Hobby, 1440 cron job and 390 web service) and the Settings card (Pro, 1440 and 390) it toasts "**`<name>`** has been resumed." (auto-dismissed within 6 s) and lands on `/services/<id>/deploys/<dep>`, whose trigger reads **Service Resumed**. With the mutation's answer replaced by an error in the browser (Playwright `route`), it toasts "An error occurred while resuming **m643-web**. Please try again." and stays. On the cluster the web service's resume deploy ran the serving image's digest, went live, deactivated the previous deploy and the service returned to Running; the cron job's went live too.
- **Manual Build** (repo-backed cron job): success opens the new deploy with no toast (1440, 390); a failure (the trigger's answer replaced by an error in the browser) toasts the server's message, still shown after 8 s, gone on its close button (1440, 390).

### Production (2026-11-26, t007 step 2, pin `9b91284a4a2e`)

Pin `9b91284a4a2e` (`c767c1bbd9`, carries `18d2615235`): both `bex-api` pods ran `bex-operator@sha256:3838967ad9ff…` and the `dashboard` pod `bex-dashboard@sha256:b867065d6b55…`, started 07:05–07:06 UTC. Walked 08:13–08:45 UTC as the QA user (`scripts/qa-login.sh`) in Google Chrome headless through `playwright-core` 1.65.1, `en-US`, America/Los_Angeles unless named, at 1440 and at 390 as a touch device. In `bex-canary` (Scale), created over REST: the repo-backed cron job `qa-20261126-m643-cron` (disposable, `srv-dc3uke6sfruc73ft1sug`, `render-examples/express-hello-world` on `main`, Node, build `yarn`, command `node -e "console.log('m643 run ok')"`, Starter) and the image-backed web service `qa-20261126-m643-web` (disposable, `srv-dc3ukeesfruc73ft1t0g`, `docker.io/traefik/whoami:v1.10`, Starter). `bex-canary` is not a Hobby workspace, so the Hobby notice was read in `tian-personal` (Hobby) on the web service `qa-20261126-m643-hobby` (disposable, `srv-dc3um9esfruc73ft1tjg`, the same image, Free).

| Point | Seen at 1440 and 390 | Verdict |
| --- | --- | --- |
| The last run | after a triggered run that succeeded at 08:18:06 UTC: "Last successful run November 26, 2026 at 12:18 AM PST", its hover tooltip "UTC: Nov 26 08:18:06 AM" (underlined on hover at 1440; the touch device draws no hover underline, and the tooltip opens); in UTC "… November 26, 2026 at 8:18 AM UTC" with no tooltip trigger. One line at 1440 (y 180, 20 px), two at 390 (y 252, 40 px) | holds |
| The schedule | `*/40 * * * *` "every 40 minutes", then over REST `0 9 * * 1-5` "at 09:00 AM UTC, Monday through Friday" and `0 0 1 * *` "at 12:00 AM UTC, on day 1 of the month", at both widths | holds in English; see Page errors |
| The notice | suspended over REST. Hobby: "Suspended by you · You are not billed for suspended services. · Resume service" (1136×46 at 1440, 358×102 at 390). Scale: the web service and the cron job read "Manually suspended" at both widths | holds |
| Resume service | from the notice: the Hobby web service at 390, the Scale web service at 1440, the cron job at 1440 and again at 390. Each toasted "**`<name>`** has been resumed." (the name bold) within 1 s, gone by 5.3 s, and landed on `/services/<id>/deploys/<dep>` reading Service Resumed. Each deploy went live with trigger `service_resumed` on REST (`/v1/services/<id>/deploys`), GraphQL `deploy.trigger` and MCP `get_deploy` (read for the Scale web service), and deactivated the one before; every App returned to Running and the web service answered 200. The cron job's resume deploy reused its built image. With the `ResumeService` answer replaced by a 502 in the browser (Hobby at 1440, Scale web at 390): "An error occurred while resuming **`<name>`**. Please try again.", gone by 4.7 s, the page and the notice stayed | holds |
| Manual Build | success at 1440 and 390: one `TriggerDeploy`, the new deploy's page in 1.1 s, no toast within 4 s; both builds went live (trigger `manual`). With the trigger's answer replaced by a GraphQL error in the browser: a toast of the server's message ("M643 injected failure: the build could not be started"), still shown after 8 s, gone on its close button; the page stayed. The button 111×32 at 1440, 111×44 at 390 | holds |
| Pending against ready | a click from the workspace's service list with the `Server` read held: band 189/189 px and the last-run line at y 180, 20 px in both states at 1440; band 259/259 and the line at y 252, 40 px at 390 | holds |
| Page errors | English: none on the cron job's bare URL, Runs, Deploys, Events, Logs and Settings, or the web service's pages, at either width. zh-CN in Asia/Shanghai: React #418 on every cron job page (Runs, Deploys), none on the web service or the service list. The server's HTML worded the schedule in English ("at UTC 12:00 AM, on day 1 of the month"), the browser in Chinese ("在 UTC 上午 12:00, 限每月 1 号"). The rest of the zh header held: "上次成功运行 2026年11月26日 下午4:18 GMT+8", tooltip "UTC：11月26日 上午08:18:06" | **fails outside English** |

**The defect, fixed in `c28239c97b`:** Nitro's server build treats every module as free of side effects, so it dropped the side-effect imports of `cronstrue/locales/<lang>.js` that registered each language; the server's cronstrue held English only (reproduced on a local `yarn build`: `Locale 'zh_CN' could not be found; falling back to 'en'`), so every cron job page outside English hydrated into #418. The words now come from `cronstrue/i18n`, cronstrue's own build with every locale inside it, and `yarn test:built-server` checks the built server's copy in every language `CRONSTRUE_LOCALE` names (it fails on the previous import). The local walk ran `vite dev`, which does not tree-shake, so it never saw the server drop them.

**Cleanup:** the three services were deleted (`DELETE` 204, by-id `GET` 404); the app cluster kept no App, CronJob, Job or Pod of them. The QA session was logged out (`qa-login.sh --logout`, after which its cookie answered 401), and the session jar, the state file and the fetched kubeconfig were deleted.

**Left for the next pin** (t007 step 2's re-check, once production carries `c28239c97b`): as the QA user in `bex-canary`, a cron job created for it (disposable) that has run once, opened in zh-CN (Asia/Shanghai) and de-DE at 1440 and 390: no page error on its Runs, Deploys and Settings, and the server's HTML (`data-header-item="cron-runs"`) carries the page language's schedule words.

## Definition of done

1. A cron job's "Last successful run" reads Render's date with the zone's abbreviation in the viewer's zone, with a "UTC: …" tooltip only off UTC, and no hydration error.
2. The header words every schedule Render words, in Render's words in English (cronstrue's other locales in the other dashboard languages), and shows nothing where Render shows nothing.
3. A suspended service's notice in a Hobby workspace is titled "Suspended by you"; on other plans "Manually suspended".
4. A resume opens a deploy with trigger `service_resumed` on REST, GraphQL and MCP reads, and Resume service opens it, with Render's two resume toasts.
5. Manual Build shows no toast on success and keeps a failure until dismissed.
6. Each is walked at 1440 and 390 locally and on production; ADR018's Cron job and Suspend / Resume rows say what bex does, with any difference left filed or listed as awaiting sign-off.

## Source + Goal linkage

- **Source:** promoted from [w3/109](../../done/109.md) (filed 2026-11-22 by w3/m555 t008) by `/loopx w3` on 2026-11-25. Weighing the note took it past one sitting: Render's resume opens a deploy, which bex-api does not record, so one item is a backend change; and wording every schedule needs cronstrue across eight dashboard languages.
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)).
- **Expected outcome:** a cron job's header, its suspension notice and its Manual Build say and do what Render's do, and a resume leaves a deploy in the history as Render's does.
- **Why now:** w3/m555 shipped the header and actions and left exactly these five differences, each unsigned in ADR018; w3/125 changes the same notice next.
- **Render parity task included:** the dashboard's cron header, the suspension notice on every type, Manual Build, and the resume verb's deploy on REST, GraphQL and MCP.

## Unblock record

- **2026-11-26, `/loopx w3` (verified evidence, no user decision):** the gate, pin must carry `18d2615235`, holds. The newest pin on `origin/main` is `c767c1bbd9` `chore(deploy): pin platform images to 9b91284a4a2e`, and `git merge-base --is-ancestor 18d2615235 9b91284a4a2e` exits 0. Affects t007 (production half) and t011. Next: t007 walks every point of its step 2 on production as the QA user in `bex-canary` at 1440 and 390 and records it under `## Live walk`; t011 closes the milestone once definition of done 6 holds. No gate remains.
- **2026-11-26, `/loopx w3`, parked again (verified evidence, no user decision):** t007's production walk held every point in English at 1440 and 390 (§ Live walk › Production) and found React #418 on every cron job page outside English, fixed in `c28239c97b`. Gate: pin must carry `c28239c97b`; nothing is needed from the user. Next: t007 step 2's re-check as § Live walk › Production lists it under "Left for the next pin", recorded there; then t011 closes the milestone if it holds.
