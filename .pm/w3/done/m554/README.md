# w3 · m554 — A cron job's sidebar and routes are Render's: Runs, Deploys, Settings, and its bare URL opens Runs

**Worker:** worker3 **Goal:** a cron job's sidebar reads Runs, Deploys, Settings / Monitor: Events, Logs, Metrics / Manage: Environment, Shell, as Render's does; its bare URL opens Runs, `…/builds` lands on `…/deploys`, and its compute plan is Settings' `#compute` section, not a Compute tab. **Status:** done (2026-11-23: walked on dev-3 and on production pin `19fdb46f1758`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record Render's cron job navigation from the walk and the bundle, and close cron-dashboard.md's Unverified list — **DONE** | 40m | — |
| [t002](done/t002.md) | Sidebar: a cron job's top level is Runs, Deploys, Settings, and its Manage group has no Compute — **DONE** | 45m | t001 |
| [t003](done/t003.md) | Routes: a cron job's bare URL opens Runs, `/builds` lands on `/deploys`, and `/compute` lands on Settings' compute section — **DONE** | 1h | t001 |
| [t004](done/t004.md) | Settings' compute plan section answers `#compute`, and the changed routes' skeletons match their ready pages — **DONE** | 45m | t002, t003 |
| [t005](done/t005.md) | Verify in a real browser at 1440 and 390, on dev-3 and on production — **DONE** | 45m | t004 |
| [t006](done/t006.md) | Render parity — **DONE** | 30m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 45m | t006 |
| [t009](done/t009.md) | Closeout — **DONE** | 10m | t007, t008 |

Total: **9 tasks, about 5h35m**.

## What Render does

- Evidence: a signed-in, read-only walk of four cron jobs in a Render workspace on 2026-11-21 (nothing that acts was clicked), plus Render's public dashboard bundle read the same day without sign-in (`CronJob-D3WeWa8V.js` sha256 `23d3ad7d7c97…`, `index-BR2q8yex.js` `f19b12ced9c1…`, `urls-B5ihPFNO.js` `3ab9b8053a93…`, `helpers-C_tFPHBn.js` `302444d539f3…`; the hashes change with every Render deploy, so find the chunks by name, as `metrics-page.md` § Status-code label describes).
- Sidebar: Runs, Deploys, Settings · Monitor: Events, Logs, Metrics · Manage: Environment, Shell, One-Off Jobs (One-Off Jobs stays out: `DO_NOT_DO.md`). No Compute: the bundle's cron title map has no `/compute`, where the Server map has `/compute` and `/scaling`.
- `/cron/<id>` shows Runs; `/cron/<id>/builds` lands on `/cron/<id>/deploys`; a build's detail stays under `/cron/<id>/builds/<id>`.
- Settings has a `#compute` section right after General, and the header's plan badge links to it.
- Render's docs and its 2026-08 changelog screenshot still say Builds; the signed-in dashboard says Deploys.

## bex today

- `serviceNavGroups` (`dashboard/src/features/services/components/service-nav.tsx`): cron top level `[BUILDS, RUNS, SETTINGS]`, Manage `[ENVIRONMENT, SHELL, COMPUTE]`.
- `service-root-redirect.ts`: the cron root and a cron `/deploys` land on `/builds`.
- `services.$serviceId.compute.tsx` serves every runtime type.

## Live walk

### dev-3 (2026-11-22, t005 step 1)

`bash scripts/dev-env.sh 3 up` rebuilt bex-api from the tree. A fresh account (`m554-cron@dev3.test`, Kratos admin, password, verified) in its Hobby workspace `tea-dc134fpjg4r0hraeqfvg` created the cron job `nightly-report` (`srv-dc134m1jg4r0hraeqg0g`, `docker.io/library/busybox:1.36`, `*/2 * * * *`, Starter; deploy `dep-dc134m1jg4r0hraeqg10` live, runs succeeding). The dashboard ran from this tree on `:50030`, in Chromium through Playwright.

| Point | Seen | Verdict |
| --- | --- | --- |
| Sidebar | Runs, Deploys, Settings / Events, Logs, Metrics / Environment, Shell at 1440 and in the 390 drawer; Runs active on `/runs`, Deploys on `/deploys` and on `/deploys/<dep>`, Settings on `/settings` | holds |
| Bare URLs | `/services/<id>` and `/cron/<id>` landed on `/services/<id>/runs` ("Runs ・ nightly-report ・ Cron Job") | holds |
| Builds | `/services/<id>/builds?status=live` → `/deploys?status=live`; `/cron/<id>/builds` → `/deploys`; `/cron/<id>/builds/<dep>` and `/cron/<id>/deploys/<dep>` → `/services/<id>/deploys/<dep>`, back link Deploys | holds |
| Compute | `/services/<id>/compute` and `/plan` → `/settings#compute`; on a cold load and from the header's Starter badge (`href …/settings#compute`) the Compute card sat at y 72 at 1440 (y 121 under the section bar at 390); General held Service Name and Region only; the rail and the bar listed General, Compute, Source, …; Edit opened the plan cards with Cancel and Save, and Cancel closed them; no sideways scroll at 390 | holds |
| Settings pending against ready | the settings chunk held, then released: General 298/298 px at 1440 and 414/414 at 390 after t004 set the measured heights (the first pass drew 366 and 498, the plan row's), Compute 144/144 and 188/189 | holds |
| Runs and Deploys pending against ready | Runs: the card's title and action slot, a four-column table, three rows against five plus Load more; Deploys: count, search, status filter, the Deploy/Trigger/Duration header and rows. Same frames at 1440 and 390, rows data-dependent. At 390 the Runs card's action slot is 32 px against Trigger Run's 44 (w2/m360's skeleton, which w3/m556 t005 redraws and w3/m555 t003 changes) | holds, apart from that slot |
| A cron job's bare URL from Projects | with GraphQL held, the parent frame drew the `cron-runs` skeleton, then landed on `/runs` | holds |

Not this milestone's, found on the way: the other Settings cards' pending heights have drifted from their ready cards (most at 390), filed as **w3/104**. Render's standalone `/cron/<id>/plan` page is filed as **w3/105**.

### Production (2026-11-23 04:00 UTC, t005 step 2)

Pin `19fdb46f1758` (`5330be9f54`), which carries `30deb75e06`; the `dashboard` pod ran its pinned digest (`sha256:fb08770a61cb…`). As the QA user in `bex-canary` (`tea-daif693dqjvc73e7as3g`), a cron job `qa-20261122-m554-cron` (disposable, `srv-dc1rjil3vdpc73b0i7m0`, `docker.io/library/busybox:1.36`, `*/2 * * * *`, Starter) was created over REST; its deploy `dep-dc1rjil3vdpc73b0i7mg` went live and its runs succeeded. Google Chrome 156, headless through `playwright-core`, `en-US`, at 1440 and 390. The existing web service `hello-go` served as the other type.

| Point | Seen at 1440 and 390 | Verdict |
| --- | --- | --- |
| Sidebar | Runs, Deploys, Settings / Monitor: Events, Logs, Metrics / Manage: Environment, Shell, at 1440 and in the 390 drawer; Runs active on `/runs`, Deploys on `/deploys` and on `/deploys/<dep>` (back link Deploys), Settings on `/settings`. `hello-go` kept Deploys, Settings / Events, Logs, Metrics / Environment, Disk, Shell, Compute | holds |
| Bare URLs | `/services/<id>` and `/cron/<id>` landed on `/services/<id>/runs` ("Runs ・ qa-20261122-m554-cron ・ Cron Job") | holds |
| Builds | `/services/<id>/builds?status=live` → `/deploys?status=live`; `/cron/<id>/builds` → `/deploys`; `/cron/<id>/builds/<dep>` and `/cron/<id>/deploys/<dep>` → `/services/<id>/deploys/<dep>`. `hello-go`'s `/runs` and `/builds` landed on its `/deploys`, and its `/compute` stayed its Compute page | holds |
| Compute | `/services/<id>/compute`, `/services/<id>/plan` and `/cron/<id>/compute` → `/settings#compute` with the Compute card at y 72 at 1440 and y 121 under the section bar at 390, and the same from the header's Starter badge (`href …/settings#compute`) clicked on Runs; General held Service Name and Region; the rail and the bar listed General, Compute, Source, Deploy, Image registry credential, Notifications, Deploy Hook, Suspend Service, Danger Zone; Edit opened six plan cards with Cancel and Save, and Cancel left Edit; scroll width 390 at 390 | holds |
| Settings pending against ready | the settings chunk held, then released: General 298/298 px at 1440 and 414/414 at 390, Compute 144/144 and 188/189 | holds |
| Runs and Deploys pending against ready | each route's chunk held, then released. The frames dev-3 saw: Runs' title and action slot over a four-column table (three skeleton rows against one and two runs); Deploys' count, search, status filter and Deploy/Trigger/Duration header (rows against one deploy). At 390 the Runs action slot is still 32 px against Trigger Run's 44 (w3/m556 t005, w3/m555 t003) | holds, apart from that slot |
| A cron job's bare URL from the workspace's service list | with GraphQL held, the parent frame drew `cron-runs`, then landed on `/runs` | holds |

The browser logged no page errors. **Cleanup:** the cron job was deleted (`DELETE` 204), and by-id `GET` answered 404 for it and its deploy; the app cluster kept no App, CronJob, Job or Pod of it. The QA session was logged out (`qa-login.sh --logout`, after which its cookie answered 401), and the session jar, the state file, the fetched kubeconfig and the screenshots were deleted.

## Definition of done

1. A cron job's sidebar is Runs, Deploys, Settings / Events, Logs, Metrics / Environment, Shell, with Deploys highlighted on the list and on a deploy's detail; no Compute entry for a cron job; every other type's sidebar is unchanged.
2. `/services/<cron>` and `/cron/<id>` open Runs; `…/builds` lands on `…/deploys` with its search; a cron `…/compute` lands on Settings with the plan section (`#compute`) in view; a non-cron `/runs` or `/builds` still lands on its primary tab.
3. Each changed route's pending state matches its ready state at 1440 and 390.
4. `cron-dashboard.md` has no Unverified list, `compute-page.md` says a cron job has no Compute page, and ADR018's Cron job row says what bex does.
5. Walked in a real browser on dev-3 and, once the pin carries it, on production.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w3` (2026-11-21), filed by `/pmx the verified parity to w3`, item 1. Amends [w2/m360](../../../w2/done/m360/README.md), which mapped Render's docs ("Builds page") because no Render dashboard was available; the bundle alone disagreed with the docs on the tab's name, and the signed-in walk settled it.
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)).
- **Expected outcome:** a Render user finds a cron job's runs, deploys and plan where Render puts them, and Render-shaped links such as `/cron/<id>/builds` land.
- **Why now:** w3/m555 and w3/m556 build on these routes, and every week `/builds` stays a cron job's canonical URL adds links and bookmarks to it.
- **Render parity task included:** the dashboard's cron sidebar and routes change. No REST, GraphQL or MCP surface changes.
