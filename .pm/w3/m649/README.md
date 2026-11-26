# w3 · m649 — A Key Value instance's sidebar and pages are Render's: Info, Logs, Metrics, each at its own URL

**Worker:** worker3 **Goal:** a Key Value instance's pages swap the workspace nav for a resource sidebar reading Info / Monitor: Logs, Metrics, as Render's does; each entry is its own URL under `/keyvalue/<id>`, so Render's `/r/<id>/<page>` links open the page they name; the Overview/Metrics/Logs tab bar and the Overview's section rail are gone. **Status:** todo (t001–t005 and t007–t009 done; t006 walked locally, its production walk waits for a pin carrying the milestone)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record Render's Key Value sidebar, routes and Info page from the bundle, and map every bex card to its page — **DONE** | 25m | — |
| [t002](done/t002.md) | A Key Value instance's pages carry a resource sidebar: Info / Monitor: Logs, Metrics — **DONE** | 45m | t001 |
| [t003](done/t003.md) | Each page is its own route under one layout: Info at the bare URL and `/info`, then `/logs` and `/metrics` — **DONE** | 1h | t001 |
| [t004](done/t004.md) | Old URLs and in-app links open the page they name: `?tab=logs`, `?tab=metrics`, the Overview's anchors, Render's `/plan` and `#compute` — **DONE** | 40m | t003 |
| [t005](done/t005.md) | Each page's pending frame equals its ready page at 1440 and 390 — **DONE** | 30m | t002, t003 |
| [t006](t006.md) | Verify in a real browser at 1440 and 390, locally and on production | 45m | t004, t005 |
| [t007](done/t007.md) | Render parity — **DONE** | 25m | t006 |
| [t008](done/t008.md) | Simplify — **DONE** | 15m | t007 |
| [t009](done/t009.md) | Test coverage — **DONE** | 40m | t007 |
| [t010](t010.md) | Closeout | 10m | t008, t009 |

Total: **10 tasks, about 5h35m**.

## What Render does

- **Evidence:** Render's public dashboard bundle, read 2026-11-25 without sign-in. The note's first read was `index-BPMVVQol.js` (sha256 `4eb3557f6970…`); this promotion re-read `index-D5V4ZgbE.js` (sha256 `8d15889ace2d…`) the same evening. The hashes change with every Render deploy, so find the Key Value layout by its frozen title map, `Object.freeze({"":"Info","/info":"Info","/logs":"Logs","/metrics":"Metrics","/plan":null})`, as `docs/render-artifacts/metrics-page.md` § Status-code label describes. Not walked signed in.
- **Sidebar** (the layout passes it to the shared resource shell, `project: "datastores"`): top level **Info** (`/r/<id>/info`, `hrefPattern` the bare `/r/<id>`, so also active there); then a **Monitor** group with Logs (only with `viewRedisLogs`) and Metrics (only with `viewRedisMetrics`). With both denied the group has no entries. Before the instance loads the sidebar draws Info alone (`initialSideNav: [{ items: [info] }]`). The back link reads Dashboard (`/`), or Environment when the instance sits in a project environment.
- **Routes:** `/r/:id` with children `*` and `info` (Info), `metrics`, `metrics/:period`, `logs`, and `plan`. `/plan` has no title: it is a full-page plan chooser whose Cancel and Change plan both return to the bare URL. The resource header's Upgrade instance link is `/r/<id>/info#compute`.
- **Info** holds, in order: the title "Info"; General (name, Created, Status, Maxmemory Policy, Persistence Mode, Region, Runtime); Compute (the plan picker, `#compute`); Connections (its private-store note links `#networking`); Networking (the IP allow list); a log stream override (log streams are a non-goal); and the delete card. **Logs** is the shared log explorer (`searchInUrl`, full-screen layout). **Metrics** is the metric groups, with a disk paywall on a free instance.
- **Resource header:** the layout passes `hideResourceHeader` from the same hook as the Postgres layout. Whether bex hides the header on Logs and Metrics follows w3/117's answer for Postgres; this milestone keeps the header as it is.

## bex today

Verified 2026-11-25 on main `4a75a22154`.

- `dashboard/src/routes/keyvalue.$keyValueId.tsx` is the only Key Value detail route (`routeTree.gen.ts` has no `/keyvalue/$keyValueId/…` child). It draws the header, a `DetailTabNav` with Overview, Metrics (`?tab=metrics`) and Logs (`?tab=logs`), and an Overview that stacks seven sections behind a right-hand section rail (`features/keyvalue/components/key-value-detail-navigation.tsx`): `#metadata`, `#connection`, `#networking`, `#plan`, `#maxmemory-policy`, `#persistence-mode`, `#danger-zone`.
- The page keeps the workspace sidebar: `DashboardSidebar` (`common/components/dashboard-layout/dashboard-sidebar.tsx`) swaps in a resource sidebar for a `projectId`, a `serviceId` or a `databaseId`, not a `keyValueId`.
- `common/lib/render-alias.ts` maps `/r/<id>/<rest>` to `/keyvalue/<id>/<rest>` verbatim, and no route serves `/keyvalue/<id>/logs` or any other sub-path, so Render's own `/r/<id>/logs`, `/metrics`, `/info` and `/plan` links reach none of those pages.
- In-app links into the page: `features/logs/lib/datastore-log-search.ts` (`datastoreLogContextHref`, View in context, still `?tab=logs`), `common/components/dashboard-layout/global-search-dialog.tsx`, `features/projects/components/resource-table.tsx`, `features/blueprints/lib/resource-href.ts` and `features/keyvalue/pages/new-key-value-page.tsx` (after create). `common/lib/route-skeleton-manifest.ts` lists `/keyvalue/$keyValueId` with the `keyvalue-detail` shape.
- The Postgres pages are the shape to copy (w3/m641): `features/databases/components/database-nav.tsx`, `common/components/dashboard-layout/database-sidebar.tsx`, the `databases.$databaseId*.tsx` layout and child routes, `features/databases/lib/database-pages.ts` (`movedAnchorPage`, `databaseSubpathLanding`) and `database-page-head.ts`.

## Page map

From t001 (2026-11-25, recorded in [key-value.md § Pages](../../../docs/render-artifacts/key-value.md#pages-sidebar-routes-and-info-w3m649-2026-11-25)). Every card stays on Info, in the order of its Render counterpart. Render's General holds the Maxmemory Policy and Persistence Mode fields; bex keeps them as cards of their own, with their confirmations, right after Details, because folding them into the Details card is not a small change (each card owns a read, an editor and a confirmation dialog).

| bex card (old anchor) | Page, place | Render counterpart |
| --- | --- | --- |
| Details (`#metadata`) | Info, 1st | General |
| Maxmemory policy (`#maxmemory-policy`) | Info, 2nd | General's Maxmemory Policy |
| Persistence mode (`#persistence-mode`) | Info, 3rd | General's Persistence Mode |
| Plan (`#plan`) | Info, 4th | Compute (`#compute`) |
| Connections (`#connection`) | Info, 5th | Connections |
| Networking (`#networking`) | Info, 6th | Networking |
| Danger zone (`#danger-zone`): delete, suspend | Info, 7th | the delete card |
| the Logs tab (`?tab=logs`) | Logs | Logs |
| the Metrics tab (`?tab=metrics`) | Metrics | Metrics |

The re-read of the bundle (`index-D5V4ZgbE.js`, sha256 `8d15889ace2d…`) matched § What Render does point for point, with three details added to the artifact: the Info title and General's "Creating since" while creating, Connections' "External traffic not allowed" note linking `#networking`, and the Metrics page's disk paywall linking `/plan`.

## Live walk

### Local (2026-11-25, t005 step 3 and t006 step 1)

The dashboard ran from this tree under Vite on `:5649` against `local-bex` on `:8649` (as w3/m641 did), whose seeded Key Value `sessions-cache` (Starter, public, region `fsn1`) stood in. Google Chrome, headless through `playwright-core`, at 1440×900 and 390×900 (the 390 sidebar in its drawer).

| Point | Seen at 1440 and 390 | Verdict |
| --- | --- | --- |
| Sidebar | back link Dashboard, `sessions-cache`, Info / Monitor: Logs, Metrics; exactly the open page highlighted on the bare URL, `/info`, `/logs` and `/metrics`. The workspace's sidebar on `/` and `/new/redis`, and a Postgres database's on its `/plan`, are unchanged | holds |
| Pages | the bare URL and `/info` on Info (Details, Maxmemory policy, Persistence mode, plan, Connections, Networking, danger zone), `/logs`, `/metrics`, each with its `h2` and a title such as "Logs ・ sessions-cache ・ Key Value ・ bex Dashboard"; no tab bar, no section rail, no sideways scroll at 390 | holds |
| Old links | `?tab=logs&range=24h&text=ready` on `/logs?range=24h&text=ready`; `?tab=metrics&range=7d` on `/metrics?range=7d`; each of the seven anchors stayed on Info with its hash, the upper four cards at the top of the scroll area (24 px under it) and the lower three at the page's end, the page being shorter | holds |
| Render's URLs | `/r/<id>` and `/r/<id>/info` on Info; `/r/<id>/logs?range=24h` and `/metrics?range=7d` on their pages, query kept; `/r/<id>/plan` on `/keyvalue/<id>#plan` and `/r/<id>/info#compute` on Info, both with the plan card at the top of the scroll area; `/metrics/7d` on `/metrics`; an unknown sub-path on Info | holds |
| View in context | on `/logs?instance=…`, a line's menu offers `/keyvalue/<id>/logs?range=custom&rangeStart=…&rangeEnd=…&contextLine=…` (the line's 20 minutes), which opens Logs over that window. `local-bex` regenerates its synthetic lines per read, so the marked line itself is not in the new read; production checks the mark | holds, mark on production |
| Pending against ready | the `KeyValue` read held after a client navigation from `/`: the header 61/61 px and the `h2` at the same place on every page at both widths; Metrics 1390/1390 (1090/1090 at 390); Connections 182/182 (250/250); the danger zone 36/36 (84/84); Maxmemory policy 188/186 and Persistence mode 208/206 at 1440 (288/286 for the latter at 390), within 2 px. Data decided the rest: Details 351/392 (534/575), a public store's External host row; Maxmemory policy at 390 268/246, the frame reserving two lines for the longest policy where `allkeys-lru` takes one; the plan card 665/743 (871/1287), where the ready list adds a reason line under a tier the instance cannot move to (Free) or the catalog marks unavailable; Networking 222/182 (414/342), a public store having no internal-only line and a one-line hint; the Logs toolbar block 596/572 (740/716), the status line the frame reserves for a read with no lines | holds where data does not decide |

The browser logged no page error.

## Definition of done

1. A Key Value instance's pages show a resource sidebar, at 1440 and in the 390 drawer: the back link, the instance's name, then Info / Monitor: Logs, Metrics, with the open page highlighted. Logs is left out for a member denied `can_view_logs` and Metrics for one denied `can_view_activity`, and a group left empty is dropped. Every other page's sidebar is unchanged.
2. `/keyvalue/<id>` and `/keyvalue/<id>/info` open Info; `/logs` and `/metrics` open their pages, each with its own heading and document title, and Render's `/r/<id>/<page>` opens the same page with its query kept. The tab bar and the section rail are gone.
3. Every card the Overview held sits on Info in Render's order as t001 maps it, and every control the Overview offered can still be reached.
4. Old URLs open the page they meant: `?tab=logs` and `?tab=metrics` with their range and search keys → `/logs` and `/metrics`; each Overview anchor → Info with the card scrolled into view; Render's `/plan` and `/info#compute` → Info at the plan card; `/metrics/<period>` → `/metrics`. Every in-app link listed under § bex today opens the page it means, View in context included.
5. Each page's pending frame matches its ready page at 1440 and 390.
6. `docs/render-artifacts/key-value.md` records Render's pages. `dashboard-routes.md` § Sidebar navigation no longer says "A Key Value instance still has the consolidated page", `keyvalue-logs.md` names the Logs page's URL, and ADR018's Key Value row (`docs/ADR018/other-datastores-storage.md`) says what bex does.
7. Walked in a real browser locally and, once a production pin carries the milestone, on production as the QA user in `bex-canary` (`tea-daif693dqjvc73e7as3g`), with a Key Value instance created for the walk (disposable).

## Source + Goal linkage

- **Source:** promoted from [w3/115](../done/115.md) on 2026-11-25 by `/loopx w3`; the note was filed by w3/m641 t007, which gave Postgres the same sidebar and pages ([done/m641](../done/m641/README.md)) and left Key Value out of scope. The note's ~3h estimate already spanned a nav model, a sidebar, a layout with child routes, redirects, pending frames and docs, more than one sitting.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)). A Key Value instance's pages and URLs are customer-facing behavior, and Render-shaped `/r/<id>/…` links are a contract bex already accepts through its alias routes and emits as `dashboardUrl`.
- **Expected outcome:** someone who knows Render's Key Value pages finds the same sidebar, the same pages and the same URLs; a pasted `/r/<id>/logs` opens Logs; the two datastores' pages share one shape.
- **Why now:** Postgres just moved to Render's pages (w3/m641, walked on production pin `cdaa0a248615`), so the two datastore detail pages now disagree with each other as well as with Render, and the Postgres code is fresh to copy. `dashboard-routes.md` names the Key Value page as the one consolidated page left.
- **Render parity task:** included (t007), because the milestone changes the dashboard's pages and URLs. REST, GraphQL and MCP do not change; t007 checks that the `dashboardUrl` bex-api emits for a Key Value instance (`/r/<id>`) still opens Info.
