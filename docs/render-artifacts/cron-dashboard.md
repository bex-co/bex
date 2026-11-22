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

## How bex maps it

| Render | bex (w3/m554) |
| --- | --- |
| Sidebar: Runs, Deploys, Settings / Events, Logs, Metrics / Environment, Shell, One-Off Jobs | `service-nav.tsx`: a cron job's top level is Runs, Deploys, Settings; Monitor is Events, Logs, Metrics; Manage is Environment, Shell. One-Off Jobs stays out (`.pm/DO_NOT_DO.md`), and a cron job has no Disk and no Compute entry |
| `/cron/<id>` shows Runs | `/services/<id>` (and the alias `/cron/<id>`) redirects to `/services/<id>/runs`; bex lands every bare service URL on its type's primary tab (w5/m57), so the sidebar highlights Runs either way |
| Runs page | `/services/<id>/runs`: the run list, Trigger Run and cancel (w2/m360; its rows are w3/m556's) |
| Deploys page | `/services/<id>/deploys`: the deploy history every compute type has, titled Deploys; rows open `/services/<id>/deploys/<dep>`, with Deploys highlighted |
| `/cron/<id>/builds` lands on `…/deploys` | `/services/<id>/builds` lands on `/services/<id>/deploys` with its query (the filters w4/284 put in the URL), and `/services/<id>/builds/<dep>` on that deploy's detail; the w5/m39 alias maps `/cron/<id>/<rest>` to `/services/<id>/<rest>`, so Render's URLs land the same way. Another type's `/builds` lands on its primary tab |
| Settings' Compute section, `settings#compute` | A cron job's Settings has a Compute section after General, `id="compute"`, with its plan and Edit (the Compute page's section). The header's plan badge links `settings#compute`, and a cron job's `/services/<id>/compute` (and the older `/plan`) lands there. Render's standalone `/plan` picker is w3/105 |
| Runs and Deploys under Render's names in the docs ("Builds page") | bex follows the dashboard. Render's docs still say Builds |
