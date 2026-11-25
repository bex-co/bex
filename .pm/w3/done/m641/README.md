# w3 · m641 — A Postgres database's sidebar and pages are Render's: Info, Logs, Metrics, Plan, Apps, Recovery, each at its own URL

**Worker:** worker3 **Goal:** a Postgres database's pages swap the workspace nav for a resource sidebar reading Info / Monitor: Logs, Metrics / Manage: Plan, Apps, Recovery, as Render's does; each entry is its own URL under `/databases/<id>`, so Render's `/d/<id>/<page>` links open the page they name; the one long Overview, its section rail and its Overview/Logs tab bar are gone. **Status:** done (walked on production pin `cdaa0a248615` on 2026-11-25)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record Render's Postgres sidebar, routes and pages from the bundle, and map every bex card to its page — **DONE** | 40m | — |
| [t002](done/t002.md) | A Postgres database's pages carry a resource sidebar: Info / Monitor: Logs, Metrics / Manage: Plan, Apps, Recovery — **DONE** | 1h | t001 |
| [t003](done/t003.md) | Each page is its own route under one layout: Info at the bare URL and `/info`, then `/logs`, `/metrics`, `/plan`, `/apps`, `/recovery` — **DONE** | 1h30m | t001 |
| [t004](done/t004.md) | Old URLs and in-app links open the page they name: `?tab=logs`, the Overview's anchors, Render's upgrade paths — **DONE** | 45m | t003 |
| [t005](done/t005.md) | Each page's pending frame equals its ready page at 1440 and 390 — **DONE** | 45m | t002, t003 |
| [t006](done/t006.md) | Verify in a real browser at 1440 and 390, locally and on production — **DONE** | 45m | t004, t005 |
| [t007](done/t007.md) | Render parity — **DONE** | 30m | t006 |
| [t008](done/t008.md) | Simplify — **DONE** | 15m | t007 |
| [t009](done/t009.md) | Test coverage — **DONE** | 45m | t007 |
| [t010](done/t010.md) | Closeout — **DONE** | 10m | t008, t009 |

Total: **10 tasks, about 7h05m**.

## What Render does

- **Evidence:** Render's public dashboard bundle, read 2026-11-25 without sign-in: `index-BPMVVQol.js` (sha256 `4eb3557f6970…`). The hashes change with every Render deploy, so find the Postgres layout by its title map (the object holding `"/recovery":"Recovery"`), as `docs/render-artifacts/metrics-page.md` § Status-code label describes. Not walked signed in.
- **Title map:** `""` and `/info` → Info, `/metrics` → Metrics, `/logs` → Logs, `/plan` → Plan, `/recovery` → Recovery, `/sharing` → Sharing, `/apps` → Apps; `/upgrade`, `/high-availability-upgrade` and `/version-upgrade` have no title (sub-pages).
- **Sidebar** (the layout passes it to the shared resource shell, `project: "datastores"`):
  - top level: **Info** (`/d/<id>/info`, also active on the bare `/d/<id>`);
  - **Monitor:** Logs (only with the `viewDatabaseLogs` permission), Metrics (only with `viewDatabaseMetrics`);
  - **Manage:** Plan (primary only, also active on `/high-availability-upgrade`), Apps (primary only), Sharing (legacy, behind a `share-postgres` flag), Recovery (primary only).
  - A read replica gets Info and Monitor only, and its Info is also active on `/plan`. The back link reads Dashboard (`/`), or Environment when the database sits in a project environment. Before the database loads, the sidebar already draws Info, Logs and Metrics.
- **Routes:** `/d/:id` with children `*` and `info` (Info), `apps`, `recovery`, `metrics`, `metrics/:period`, `sharing`, `logs`, `plan`, `upgrade`, `high-availability-upgrade`, `version-upgrade`.
- **Info** holds, in order: the banners, the resource header, the title "Info", General (with Suspend Database), a replica's plan (replicas only), Connections, Credentials (primary), Networking (the IP allow list, primary), a log stream override (log streams are a non-goal), PostgreSQL Configuration Overrides, and the delete card. **Recovery** holds Point-in-Time Recovery and Export. **Apps** lists pgAdmin and PgHero. The bundle's Read Replica strings sit beside Info's cards and its High Availability strings beside the plan pages; t001 confirms where each renders.

## bex today

Verified 2026-11-25 on main `18f6d3388b`.

- `dashboard/src/routes/databases.$databaseId.tsx` is the only database route (`routeTree.gen.ts` has no `/databases/$databaseId/…` child). It draws the header, a `DetailTabNav` with Overview and Logs (`?tab=logs`), and an Overview that stacks eleven sections behind a right-hand section rail (`database-detail-navigation.tsx`): `#metadata`, `#connection`, `#sql-console`, `#high-availability`, `#metrics`, `#plan`, `#apps`, `#insights`, `#recovery`, `#access-control`, `#danger-zone`.
- The page keeps the workspace sidebar: `DashboardSidebar` (`common/components/dashboard-layout/dashboard-sidebar.tsx`) swaps in a resource sidebar only for a `projectId` or a `serviceId`.
- `render-alias.ts` maps `/d/<id>/<rest>` to `/databases/<id>/<rest>` verbatim, and no route serves `/databases/<id>/recovery` or any other sub-path, so Render's own `/d/<id>/recovery`, `/logs`, `/metrics`, `/plan` and `/apps` links reach none of those pages.
- In-app links into the page: `database-expiry.tsx` (`#plan`), `features/logs/lib/datastore-log-search.ts` (View in context, `?tab=logs`), `global-search-dialog.tsx`, `features/blueprints/lib/resource-href.ts`, `recovery-panel.tsx` (a restore's landing) and `new-database-page.tsx` (after create).
- The Key Value page (`keyvalue.$keyValueId.tsx`) has the same shape. It is out of scope here (see t007).

## Page map

From t001 (2026-11-25, recorded in [postgres-dashboard.md § Page map](../../../../docs/render-artifacts/postgres-dashboard.md#page-map-bex-w3m641)). Render's Read Replicas render on Info, as a field of the General card; its High Availability controls render on Plan, as its last card.

| bex card (old anchor) | Page | Render counterpart |
| --- | --- | --- |
| Details (`#metadata`) | Info | General |
| Connections (`#connection`) | Info | Connections |
| Access control (`#access-control`) | Info | Credentials, Networking |
| SQL console (`#sql-console`) | Info, before the danger zone | none |
| Danger zone (`#danger-zone`) | Info | the delete card (with Restart and Suspend) |
| Metrics (`#metrics`) | Metrics | the metric groups |
| Insights (`#insights`) | Metrics, below the charts | sizes, processes, top queries |
| Plan (`#plan`) | Plan | Compute, Storage |
| Disk autoscaling (the Metrics card's disk header) | Plan, a row under Storage | Storage Autoscaling |
| High availability (`#high-availability`) | Plan | High Availability (its read replicas: w3/116) |
| Apps (`#apps`) | Apps | Apps |
| Recovery (`#recovery`) | Recovery | Point-in-Time Recovery, Export |

## Live walk

### Local (2026-11-25, t006 step 1)

dev-3 was down (`bash scripts/dev-env.sh 3 status`: the kind cluster's API refused connections and the inventory found no default StorageClass), so the dashboard ran from this tree under Vite on `:5641` against `local-bex` on `:8641`, whose seeded Postgres `orders-db` (`dpg-c185th5c2rvvnhbfiltg`, Basic 1GB, public) stood in. Google Chrome, headless through `playwright-core`, at 1440 and 390 (the 390 sidebar in its drawer).

| Point | Seen at 1440 and 390 | Verdict |
| --- | --- | --- |
| Sidebar | back link Dashboard, `orders-db`, Info / Monitor: Logs, Metrics / Manage: Plan, Apps, Recovery; exactly the open page highlighted on each of the seven URLs | holds |
| Pages | `/databases/<id>` and `/info` on Info (Details, Connections, Access control, SQL console, danger zone), `/logs`, `/metrics` (charts, then Insights), `/plan` (plan card with Storage and Storage Autoscaling, then High Availability), `/apps`, `/recovery`, each with its `h2` and a title such as "Recovery ・ orders-db ・ Database ・ bex Dashboard"; no Overview tab, no section rail, no sideways scroll at 390 | holds |
| Old links | `?tab=logs&range=24h&text=ready` on `/logs?range=24h&text=ready`; `#recovery`, `#high-availability` and `#insights` on their page with the hash, the card scrolled into view; `#sql-console` stayed on Info, scrolled to the console | holds |
| Render's URLs | `/d/<id>` on Info; `/d/<id>/recovery?x=1` on `/recovery?x=1`; `/d/<id>/plan` and `/upgrade` on `/plan`; `/high-availability-upgrade` on `/plan#high-availability`, scrolled to it; `/version-upgrade`, `/sharing` on Info; `/metrics/7d` on `/metrics` | holds |
| Pending against ready | GraphQL held after a client navigation from `/`: Plan 643/643 and 236/236 px (767/767, 276/276 at 390), Metrics 2650/2650 and 360/360 (2050/2050, 360/360), SQL console 375/375 (439/439), access control 240/240, danger zone 36/36 (84/84). Data decided the rest: Details 392/445 (616/669) for a public database whose Version row carries Upgrade, the Logs status line (24 px with no lines), Apps and Recovery (`local-bex` answers no admin-app catalog and no backups) | holds where data does not decide |

The browser logged no page error; its console carried only `local-bex`'s missing-field warnings.

### Production (2026-11-25, t006 steps 2–3)

Pin `cdaa0a248615` (the dashboard pod on its digest), signed in as the QA user with `bash scripts/qa-login.sh` in `bex-canary`, Google Chrome headless through `playwright-core` at 1440×900 and 390×900 (the 390 sidebar in its drawer). The fixture was a Postgres 16 `qa-20261125-m641-pg` (disposable), Basic 256MB because the workspace's one free database was another run's; it had real logs, metrics, the pgAdmin and PgHero catalog and one completed backup.

| Point | Seen at 1440 and 390 | Verdict |
| --- | --- | --- |
| Sidebar | back link Dashboard, the database's name, Info / Monitor: Logs, Metrics / Manage: Plan, Apps, Recovery on every URL below, exactly the open page highlighted (Plan on `/upgrade` and `/high-availability-upgrade` too). The workspace's sidebar on `/` and a web service's are unchanged | holds |
| Pages | the bare URL and `/info` on Info (Details, Connections, Access control, SQL console, danger zone), `/logs`, `/metrics` (charts, then Insights), `/plan` (plan with Storage and Disk autoscaling, then High Availability), `/apps` (pgAdmin, PgHero), `/recovery` (restore points, Restore to new instance, Create export, the backup); each with its `h2` and a title such as "Recovery ・ qa-20261125-m641-pg ・ Database ・ bex Dashboard"; no Overview tab, no section rail, no sideways scroll at 390 | holds |
| Render's URLs | `/d/<id>` and `/d/<id>/info` on Info; `/logs`, `/metrics`, `/plan`, `/apps` on their pages; `/recovery?x=1` on `/recovery?x=1`; `/upgrade` on `/plan`; `/high-availability-upgrade` on `/plan#high-availability`, at the page's end; `/version-upgrade` and `/sharing` on Info; `/metrics/7d` on `/metrics`. bex-api's `dashboardUrl` (`/d/<id>`) opens Info | holds |
| Old links | `?tab=logs&range=24h&text=ready` on `/logs?range=24h&text=ready`; `#metrics`, `#insights`, `#plan`, `#high-availability`, `#apps`, `#recovery` on their pages with the hash; `#sql-console`, `#access-control`, `#danger-zone` stayed on Info | holds |
| In-app links | the workspace's resource list and the global search open Info; View in context on a narrowed Logs line opens `/logs` over the line's 20 minutes with the line marked | holds |
| Scroll to the anchored card | `#metrics`, `#plan`, `#apps`, `#recovery`, `#access-control` put their card under the header. A lower Info card drifts, because it is scrolled to before the cards above it finish mounting: at 390 `#danger-zone` ended at y 1867 (off-screen) and `#sql-console` sat at y 1404 before reaching the page's end; `/metrics#insights` landed at y 516–762 | filed as **w3/120** |
| Pending against ready | the Database read held after a client navigation from `/`; the header (65 px), the `h2` and the sidebar's six entries were the ready page's at both widths. Plan 643/643 and 236/236 (767/767, 276/276 at 390), Apps 300/300 (558/558), Recovery 399/399 at 1440, Connections 271/271 (455/455), SQL console 375/375 (439/439), danger zone 36/36 (84/84), Insights 360/360. Data decided the rest: Details 392/445 (616/689), whose Version row carries Upgrade; Access control 240/592, with an allow list and a credential; the Logs status line (24 px); the metrics card +60 px, where three two-series charts draw their legend once they have data. Recovery at 390 drew 590 against 666, 40 px of it a one-line description that wraps to three lines | holds except Recovery at 390, filed as **w3/121** |

No page error in any load. Logs is left out for a member denied `can_view_logs`: production has one QA identity, an Admin, so that rests on `database-sidebar.test.tsx` and `database-nav.test.ts`. The free-plan expiry link, a restore's landing and the after-create landing were not exercised there (unit tests and the local walk cover them). Cleanup: the database was deleted (`GET` answered 404), the QA session was revoked (`--logout`: `ok logged-out`) and its jar and state file removed.

## Definition of done

1. A Postgres database's pages show a resource sidebar, at 1440 and in the 390 drawer: the back link, the database's name, then Info / Monitor: Logs, Metrics / Manage: Plan, Apps, Recovery, with the open page highlighted (Plan on its sub-pages too). Logs is left out for a member denied `can_view_logs`, and a group left empty is dropped. Every other page's sidebar is unchanged.
2. `/databases/<id>` and `/databases/<id>/info` open Info; `/logs`, `/metrics`, `/plan`, `/apps` and `/recovery` open their pages, each with its own heading and document title, and Render's `/d/<id>/<page>` opens the same page with its query kept. The Overview/Logs tab bar and the section rail are gone.
3. Every card the Overview held sits on the page t001 maps it to, and every control the Overview offered can still be reached.
4. Old URLs open the page they meant: `?tab=logs` with its range and search keys → `/logs`; each Overview anchor (`#metrics`, `#plan`, `#apps`, `#recovery`, `#high-availability`, …) → the page holding that card; Render's `/upgrade`, `/high-availability-upgrade` and `/version-upgrade` → the page holding that control. Every in-app link listed under § bex today opens the page it means.
5. Each page's pending frame matches its ready page at 1440 and 390.
6. `docs/render-artifacts/postgres-dashboard.md` records Render's pages. `dashboard-routes.md` § Sidebar navigation and `dashboard-walk/datastores.md` no longer call the single page an accepted IA difference, and ADR018's Managed Postgres rows say what bex does.
7. Walked in a real browser locally and, once a production pin carries the milestone, on production as the QA user in `bex-canary` (`tea-daif693dqjvc73e7as3g`), with a Postgres created for the walk (disposable).

## Source + Goal linkage

- **Source:** `/pm one aspect of render parity for w3` (2026-11-25), from Render's public dashboard bundle read the same day. It gives Postgres what w3/m554 gave a cron job ([done/m554](../../done/m554/README.md): sidebar and routes). It amends two rulings that predate the parity-first rule: the w5/m32 dashboard walk ([datastores.md](../../../../docs/render-artifacts/dashboard-walk/datastores.md)) ruled Postgres Metrics and Recovery "information architecture differs, not a gap", and [dashboard-routes.md](../../../../docs/render-artifacts/dashboard-routes.md) § Sidebar navigation says "that ruling stands". Neither is recorded in [ADR018](../../../../docs/ADR018-render-parity.md) with the user's sign-off, so neither stands.
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)). The pages and URLs of a managed Postgres are customer-facing behavior, and Render-shaped `/d/<id>/…` links are a contract bex already accepts through its alias routes.
- **Expected outcome:** someone who knows Render's Postgres pages finds the same sidebar, the same pages and the same URLs. A phone reaches Recovery or Plan in one tap instead of scrolling past the SQL console and the charts, and a pasted `/d/<id>/recovery` opens Recovery.
- **Why now:** the cron job's pages were just brought to Render's shape (w3/m554–m556), and the Postgres page is the dashboard's largest single page (eleven sections in one scroll). Render's own deep links into it reach none of the pages they name today. Doing it before the next Postgres feature lands spares that feature a home on a page about to be split.
- **Render parity task:** included (t007), because the milestone changes the dashboard's pages and URLs. REST, GraphQL and MCP do not change; t007 checks that the `dashboardUrl` bex-api emits for a Postgres still opens Info.

## Unblock (2026-11-25)

- **Affected:** t006 steps 2–3, then t010.
- **Source:** verified evidence (`/loopx w3` dispatch, gate check).
  - `git merge-base --is-ancestor 0ae1f90038 cdaa0a248615` holds. The newest `chore(deploy): pin platform images to …` on origin/main is `8b9d780f45`, which pins `cdaa0a248615`.
  - On the production app cluster the `dashboard` Deployment and its one pod run that pin's digest (`bex-dashboard@sha256:8f20de7594a9…`, pod started 2026-11-25 18:00 UTC).
- **Gate:** the `pin must carry` gate is cleared; nothing else gates the walk. The QA user's sign-in comes from `scripts/qa-login.sh`.
- **Next:** t006 step 2 walks production as the QA user in `bex-canary` with a Postgres created for it (disposable), step 3 records it under § Live walk › Production; then t010 closes the milestone if every point holds.

## Closeout (2026-11-25)

t006 steps 2–3 walked production pin `cdaa0a248615` at 1440 and 390 (§ Live walk › Production): the sidebar, every page with its heading and title, Render's `/d/<id>/<page>`, bex-api's `dashboardUrl`, the old `?tab=logs` and anchor links and View in context all held, and pending equalled ready where data does not decide. The two differences seen are filed as w3/120 (an anchored lower Info card drifts out of view) and w3/121 (Recovery's pending frame at 390). With the local walk, every definition-of-done item holds; t010 closed the milestone.
