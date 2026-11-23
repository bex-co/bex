# Render cron job dashboard: Runs, Deploys and Settings

Captured 2026-11-13 for w2/m360 from Render's docs alone, then walked signed in and read from Render's dashboard bundle on 2026-11-21 for w3/m554. The walk settled the points the docs could not; where the two disagree, the dashboard wins.

## What Render documents

- **Changelog, 2026-09-03** ("See your service's deploys on a new page"): "Services now have a new **Deploys** page … Cron jobs also have new **Builds** and **Runs** pages that separate build and run history." Its screenshot (taken in 2026-08) shows Runs above Builds.
- **[render.com/docs/cronjobs](https://render.com/docs/cronjobs)** (read 2026-11-13, re-read 2026-11-21 unchanged):
  - "After creating your cron job, you can track build progress and history on the **Builds** page."
  - "Your cron job's **Runs** page shows run history and logs for each run."
  - "To trigger a run manually, go to its **Runs** page in the Render Dashboard and click **Trigger Run**." A manual trigger during an active run "first _cancels_ the active run".

The docs and the changelog still say Builds. The signed-in dashboard says **Deploys**: the order held, the name changed after the changelog.

## The dashboard, walked and read (2026-11-21, w3/m554)

**Evidence.** A signed-in, read-only walk of four cron jobs in one Render workspace (nothing that acts was clicked), and Render's public dashboard bundle read the same day without sign-in: `index-BR2q8yex.js` (sha256 `f19b12ced9c1…`), `CronJob-D3WeWa8V.js` (`23d3ad7d7c97…`), `urls-B5ihPFNO.js` (`3ab9b8053a93…`), `helpers-C_tFPHBn.js` (`302444d539f3…`) and `AsyncComponents-DgBnhTX5.js`. The hashes change with every Render deploy, so find the chunks by name: `https://dashboard.render.com/` loads `assets/index-*.js`, which imports the others ([metrics-page.md § Status-code label](metrics-page.md#status-code-label-w3080-2026-11-20) describes the method). Commands, names and ids from the walked workspace are not recorded here.

**Sidebar** (walk and `CronJob-*.js`):

| Group | Entries, in order |
| --- | --- |
| top level | **Runs**, **Deploys**, **Settings** |
| Monitor | **Events**, **Logs**, **Metrics** |
| Manage | **Environment**, **Shell**, **One-Off Jobs** (and Sharing, badged Legacy, for some workspaces) |

- The cron title map is `"" Events, /runs Runs, /events Events, /logs Logs, /deploys Deploys, /metrics Metrics, /env Environment, /shell Shell, /jobs One-Off Jobs, /sharing Sharing, /settings Settings, /plan null`. It has no `/builds` and no `/compute`; the Server map has `/compute` and `/scaling`. A cron job has no Compute entry and no Compute page.
- Runs links `/cron/<id>/runs` and is also highlighted on `/cron/<id>` (`hrefPattern: /cron/:id`).
- Deploys links `/cron/<id>/deploys` and is highlighted on `/cron/:id/deploys/*` and `/cron/:id/builds/*`, so a deploy's or a build's detail keeps Deploys selected.

**Routes** (the cron route table in `index-*.js`, and the walk):

- `/cron/<id>` renders the Runs page with the URL unchanged (the `*` route renders Runs on the bare path and sends any unknown sub-path back to it; without permission to view events it sends you to Settings).
- `/cron/<id>/builds` replaces itself with `/cron/<id>/deploys` (a `Navigate … replace`), and the walk landed there.
- A build's detail stays at `/cron/<id>/builds/<buildId>` and a deploy's at `/cron/<id>/deploys/<deployId>` (`urls-*.js` builds both).
- `/cron/<id>/plan` is still a route (`CronJobUpdatePlan-*.js`): the plan cards with Cancel (back to Settings) and a change-plan action, no sidebar title, and no link to it from the header or the upgrade dialog. A suspended cron job's redirects to Settings.

**Settings** (walk and `index-*.js`): General (name, then the read-only region), then **Compute** (`id="compute"`: the plan cards, read-only until Edit, then "Review your compute selection" and Update compute), then Build or Image, Deploy, Log Stream, Notifications, and Delete or suspend. The header's plan badge links `settings#compute` (`upgradeInstancePath`), and the upgrade dialog's "Review upgrade options" goes to `settings#compute` for a cron job and `/compute` for every other type (one helper picks by type).

**The points w2/m360 left unverified, answered:**

| Point | Answer | Source |
| --- | --- | --- |
| Order of Builds and Runs | Runs, then Deploys, then Settings | walk, bundle |
| Does a cron job keep Events | Yes, first under Monitor, beside Logs and Metrics | walk, bundle |
| Runs page columns | RUN (with the count), TRIGGER, DURATION; rows read "Cron job run" with the run's age; w3/m556 records the page in full | walk |
| Run logs in place or on Logs | On Logs: each row links to the Logs page from a minute before the run to a minute after it | walk |
| Builds page vs a Deploys page | There is no Builds page: the cron job's Deploys page is the deploy list, with "Search deploys and commits" and, for a repo-backed job, Manual Build (w3/m555 records the header and actions) | walk, bundle |

## The header and its actions (2026-11-22, w3/m555)

**Evidence.** The 2026-11-21 walk above (four cron jobs, nothing that acts clicked), and the same public bundle re-read on 2026-11-22 without sign-in: `CronJob-*.js` (`ka` Connect, `Aa` the failure text, `ja` and `Ma` Trigger Run, `Fr`, `Lr`, `Mr` and `Pr` the suspension notice, `resourceHeaderProps` the header), `page-templates-*.js` (`Vn` the command, `Fn` the last run, `CronJobRunMetadata`), `AuthContext-*.js` (`z` the schedule in words, `H` the date), `cronjob-*.js` (the `cronJobFields` fragment and the `runCronJob` and `buildCronJob` mutations), `suspension-*.js` (the notice's title), `envs-*.js` (`docker` and `image`) and `index-*.js` (`She` and `Ohe`, the Deploys page and Manual Build). Each source below is **walk** (seen signed in) or **bundle** (read in code only).

**The header's actions.**

| Fact | Source |
| --- | --- |
| A running cron job's header has one action, **Trigger Run**, and no Manual Deploy menu: `actions: state === "Running" && [Connect, Trigger Run]` | walk, bundle |
| Connect (`ka`) renders only when the cron job has an internal address, SSH or outbound IPs (`items.length > 0 ? … : null`); none of the four walked cron jobs showed it | walk, bundle |
| A suspended cron job's header has no action. Under the facts a notice (`Fr`, informative) reads **Manually suspended** (**Suspended by you** for one account tier, `TIER1`), "You are not billed for suspended services.", and an inline **Resume service** link. The link resumes at once, toasts "`<name>` has been resumed.", and opens the new build when the answer carries one; a failure toasts "An error occurred while resuming `<name>`. Please try again." | walk (the notice and its link), bundle (the rest) |
| Other suspensions (billing, a removed card, crash loops, Render's own) get their own banners from `Lr`; the fallback reads **Suspended** with the same not-billed line | bundle |
| Render hides the whole header on Logs, Metrics and Shell (`fluidWidth`) and on a deploy's or a build's detail (`hideResourceHeader`); every other cron page has it | bundle |

**Trigger Run** (`ja`, `Ma`).

| Fact | Source |
| --- | --- |
| A primary button labelled Trigger Run | walk |
| A click calls `runCronJob` at once, with no confirm, then opens `<cron>/logs?r=live` (Logs on Live tail), or `<cron>/events` when the mutation answers no run | bundle |
| Disabled with a tooltip, in this order: without permission (the permission's own message); while suspended, "Resume this cron job to trigger a run." (seen on the walk, on a suspended cron job's Runs page); for a cron job that is not image-backed whose build has not succeeded, "No successful build detected. View Deploys." with Deploys linked. Until the build succeeds the page polls it every 2 s | walk (the suspended reason), bundle |
| A failure raises an error toast that stays until dismissed (`autoDismiss: false`): "You don't have permission to run this cron job." for a permission error, else the server's message, else "Couldn't trigger a run. Try again in a moment." | bundle |
| Render's docs: a manual trigger during an active run first cancels it; they show no confirm ([render.com/docs/cronjobs](https://render.com/docs/cronjobs)) | docs |

**The facts under the source.**

| Fact | Source |
| --- | --- |
| First line: "Runs `<command>` `<schedule in words>`", the command as inline code | walk |
| The command is `dockerCommand` or `command`. A Docker or image cron job with neither reads "Runs `CMD` in `Dockerfile`" (the Dockerfile path, or `Dockerfile`): the image-backed case says Dockerfile too | bundle |
| The schedule in words is `cronstrue.toString(schedule)` with its first letter lowercased and " UTC" after AM or PM: "at 09:00 AM UTC", "every minute", "every 2 minutes", "every hour", "at 5 minutes past the hour", "at 0 minutes past the hour, every 2 hours", "at 09:00 AM UTC, only on Monday", "at 09:00 AM UTC, on day 1 of the month". A schedule `cron-validator` refuses, or one that never fires, reads nothing | walk ("at 09:00 AM UTC", "every minute"), bundle (the rest; the phrases from cronstrue 2.50.0) |
| Second line: "Last successful run `<date>`", the date local as "November 21, 2026 at 9:00 AM" plus the zone's name, with "UTC: `<date>`" in a tooltip when the viewer's zone is not UTC; "No successful runs yet." before the first | walk (the line), bundle (the format and the empty text) |
| No next-run line | walk, bundle |

**The Deploys page** (`She`, `Ohe`).

| Fact | Source |
| --- | --- |
| A "Search deploys and commits" field, and for a cron job that is not image-backed a secondary **Manual Build** button beside it; an image-backed cron job has none | walk, bundle |
| Manual Build calls `buildCronJob` at once, with no confirm or menu, and opens the new build. A failure toasts the server's message until dismissed | bundle |
| Disabled without permission (the run permission's message, in a tooltip), and while the workspace's pipeline minutes are spent, with "You've used up the maximum number of pipeline minutes specified by your spend limit. Learn more." | bundle |
| Every other type's Deploys page has the same "Search deploys and commits" placeholder | bundle |

## How bex maps it

| Render | bex (w3/m554) |
| --- | --- |
| Sidebar: Runs, Deploys, Settings / Events, Logs, Metrics / Environment, Shell, One-Off Jobs | `service-nav.tsx`: a cron job's top level is Runs, Deploys, Settings; Monitor is Events, Logs, Metrics; Manage is Environment, Shell. One-Off Jobs stays out (`.pm/DO_NOT_DO.md`), and a cron job has no Disk and no Compute entry |
| `/cron/<id>` shows Runs | `/services/<id>` (and the alias `/cron/<id>`) redirects to `/services/<id>/runs`; bex lands every bare service URL on its type's primary tab (w5/m57), so the sidebar highlights Runs either way |
| Runs page | `/services/<id>/runs`: the run list and cancel (w2/m360; its rows are w3/m556's); Trigger Run is the header's since w3/m555 |
| Deploys page | `/services/<id>/deploys`: the deploy history every compute type has, titled Deploys; rows open `/services/<id>/deploys/<dep>`, with Deploys highlighted |
| `/cron/<id>/builds` lands on `…/deploys` | `/services/<id>/builds` lands on `/services/<id>/deploys` with its query (the filters w4/284 put in the URL), and `/services/<id>/builds/<dep>` on that deploy's detail; the w5/m39 alias maps `/cron/<id>/<rest>` to `/services/<id>/<rest>`, so Render's URLs land the same way. Another type's `/builds` lands on its primary tab |
| Settings' Compute section, `settings#compute` | A cron job's Settings has a Compute section after General, `id="compute"`, with its plan and Edit (the Compute page's section). The header's plan badge links `settings#compute`, and a cron job's `/services/<id>/compute` (and the older `/plan`) lands there. Render's standalone `/plan` picker is w3/105 |
| Runs and Deploys under Render's names in the docs ("Builds page") | bex follows the dashboard. Render's docs still say Builds |

| Render (header and actions) | bex (w3/m555) |
| --- | --- |
| A running cron job's one header action, Trigger Run; no Connect, no Manual Deploy | `service-detail-header.tsx`: a cron job's action slot holds `CronTriggerRunButton` (`cron-header-actions.tsx`) while it runs, on every cron page. A cron job has no SSH (cron shells are a `.pm/DO_NOT_DO.md` non-goal) and no internal address, so Render's rule for Connect (render it only with something to show) leaves none. Every other type's header is unchanged |
| Trigger Run: no confirm, then `logs?r=live`, or Events without a run | `useTriggerCronRun`: `runCronJob` at once, then `/services/<id>/logs?range=live` (bex's spelling of Render's `r=live`, w3/m537), or `/services/<id>/events` when no run comes back. The server cancels an active run first, as Render's docs say. The confirm w4/m114 t003 added and the Runs card's own Trigger Run are gone. **Bundle only: Render's click was not tried on Render** (the walked cron jobs do real work; a disposable Render cron job costs at least $1 a month) |
| Disabled: permission, suspended, no successful build (Deploys linked) | The server's `cron_run_now` decision (permission, suspended, billing) with its reason; then, for a repo-backed cron job whose build never went live (`revision` empty), "No successful build detected. View Deploys." with Deploys linked. An image-backed one is never held for a build |
| A failure toasts until dismissed: the permission text, the server's message, or "Couldn't trigger a run. Try again in a moment." | The same, kept until dismissed (`duration: Infinity`, with a close button); a suspended refusal reads as its localized reason (w4/203) |
| Suspended: no header action; "Manually suspended · You are not billed for suspended services. · Resume service" | `CronSuspendedNotice` under the facts, titled "Manually suspended" for Render's `user` suspender and "Suspended" otherwise; Resume service is the gated `resume` verb |
| "Runs `<command>` `<schedule in words>`" | `CronRunFacts`: the command override, else a native runtime's start command, else "`CMD` in `<Dockerfile path>`" (Render's wording, the image-backed case included); the schedule in Render's cronstrue words for the shapes `describeCronRun` names (every minute, every N minutes, every hour, at M minutes past the hour, every N hours, daily, weekly and monthly times, "at 09:00 AM UTC"). Other languages use their own words and a 24-hour clock |
| "Last successful run `<local date>`", "No successful runs yet." | "Last successful run November 22, 2026 at 1:00 AM" in the viewer's zone, with local, UTC and Unix time in its tooltip (`InstantTooltip`); "No successful runs yet." before the first. The header's next run is gone (`nextRunAt` stays a REST, GraphQL and MCP extension) |
| Deploys: "Search deploys and commits"; Manual Build for a cron job that is not image-backed | Every Deploys page's search reads "Search deploys and commits". A repo-backed cron job's has `ManualBuildButton` beside the search and filter once the list is wide, on the count's line on a phone: the deploy trigger at once, then the new deploy's page; disabled with the `deploy` decision's reason. An image-backed cron job has none |
