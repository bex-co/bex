# w3 · m641 — A Postgres database's sidebar and pages are Render's: Info, Logs, Metrics, Plan, Apps, Recovery, each at its own URL

**Worker:** worker3 **Goal:** a Postgres database's pages swap the workspace nav for a resource sidebar reading Info / Monitor: Logs, Metrics / Manage: Plan, Apps, Recovery, as Render's does; each entry is its own URL under `/databases/<id>`, so Render's `/d/<id>/<page>` links open the page they name; the one long Overview, its section rail and its Overview/Logs tab bar are gone. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Record Render's Postgres sidebar, routes and pages from the bundle, and map every bex card to its page | 40m | — |
| [t002](t002.md) | A Postgres database's pages carry a resource sidebar: Info / Monitor: Logs, Metrics / Manage: Plan, Apps, Recovery | 1h | t001 |
| [t003](t003.md) | Each page is its own route under one layout: Info at the bare URL and `/info`, then `/logs`, `/metrics`, `/plan`, `/apps`, `/recovery` | 1h30m | t001 |
| [t004](t004.md) | Old URLs and in-app links open the page they name: `?tab=logs`, the Overview's anchors, Render's upgrade paths | 45m | t003 |
| [t005](t005.md) | Each page's pending frame equals its ready page at 1440 and 390 | 45m | t002, t003 |
| [t006](t006.md) | Verify in a real browser at 1440 and 390, locally and on production | 45m | t004, t005 |
| [t007](t007.md) | Render parity | 30m | t006 |
| [t008](t008.md) | Simplify | 15m | t007 |
| [t009](t009.md) | Test coverage | 45m | t007 |
| [t010](t010.md) | Closeout | 10m | t008, t009 |

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

## Definition of done

1. A Postgres database's pages show a resource sidebar, at 1440 and in the 390 drawer: the back link, the database's name, then Info / Monitor: Logs, Metrics / Manage: Plan, Apps, Recovery, with the open page highlighted (Plan on its sub-pages too). Logs is left out for a member denied `can_view_logs`, and a group left empty is dropped. Every other page's sidebar is unchanged.
2. `/databases/<id>` and `/databases/<id>/info` open Info; `/logs`, `/metrics`, `/plan`, `/apps` and `/recovery` open their pages, each with its own heading and document title, and Render's `/d/<id>/<page>` opens the same page with its query kept. The Overview/Logs tab bar and the section rail are gone.
3. Every card the Overview held sits on the page t001 maps it to, and every control the Overview offered can still be reached.
4. Old URLs open the page they meant: `?tab=logs` with its range and search keys → `/logs`; each Overview anchor (`#metrics`, `#plan`, `#apps`, `#recovery`, `#high-availability`, …) → the page holding that card; Render's `/upgrade`, `/high-availability-upgrade` and `/version-upgrade` → the page holding that control. Every in-app link listed under § bex today opens the page it means.
5. Each page's pending frame matches its ready page at 1440 and 390.
6. `docs/render-artifacts/postgres-dashboard.md` records Render's pages. `dashboard-routes.md` § Sidebar navigation and `dashboard-walk/datastores.md` no longer call the single page an accepted IA difference, and ADR018's Managed Postgres rows say what bex does.
7. Walked in a real browser locally and, once a production pin carries the milestone, on production as the QA user in `bex-canary` (`tea-daif693dqjvc73e7as3g`), with a Postgres created for the walk (disposable).

## Source + Goal linkage

- **Source:** `/pm one aspect of render parity for w3` (2026-11-25), from Render's public dashboard bundle read the same day. It gives Postgres what w3/m554 gave a cron job ([done/m554](../done/m554/README.md): sidebar and routes). It amends two rulings that predate the parity-first rule: the w5/m32 dashboard walk ([datastores.md](../../../docs/render-artifacts/dashboard-walk/datastores.md)) ruled Postgres Metrics and Recovery "information architecture differs, not a gap", and [dashboard-routes.md](../../../docs/render-artifacts/dashboard-routes.md) § Sidebar navigation says "that ruling stands". Neither is recorded in [ADR018](../../../docs/ADR018-render-parity.md) with the user's sign-off, so neither stands.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)). The pages and URLs of a managed Postgres are customer-facing behavior, and Render-shaped `/d/<id>/…` links are a contract bex already accepts through its alias routes.
- **Expected outcome:** someone who knows Render's Postgres pages finds the same sidebar, the same pages and the same URLs. A phone reaches Recovery or Plan in one tap instead of scrolling past the SQL console and the charts, and a pasted `/d/<id>/recovery` opens Recovery.
- **Why now:** the cron job's pages were just brought to Render's shape (w3/m554–m556), and the Postgres page is the dashboard's largest single page (eleven sections in one scroll). Render's own deep links into it reach none of the pages they name today. Doing it before the next Postgres feature lands spares that feature a home on a page about to be split.
- **Render parity task:** included (t007), because the milestone changes the dashboard's pages and URLs. REST, GraphQL and MCP do not change; t007 checks that the `dashboardUrl` bex-api emits for a Postgres still opens Info.
