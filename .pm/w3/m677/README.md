# w3 · m677 — A Blueprint's pages are Render's: Resources, Syncs and Settings in a resource sidebar, each at its own URL

**Worker:** worker3 **Goal:** a Blueprint's pages swap the workspace nav for a resource sidebar reading Resources / Syncs / Settings, as Render's do; each entry is its own URL under `/blueprints/<id>`, so Render's `/blueprint/<id>/…` links open the page they name; the single stacked page is gone. **Status:** todo (t001–t006 and t008–t010 done; t007 walked locally, its production walk waits for a pin carrying the milestone)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record Render's Blueprint layout, routes and header in `docs/render-artifacts/blueprint-pages.md`, and map every bex card to its page — **DONE** | 25m | — |
| [t002](done/t002.md) | A Blueprint's pages carry a resource sidebar: Blueprints back link, the name, Resources / Syncs / Settings — **DONE** | 45m | t001 |
| [t003](done/t003.md) | Each page is its own route under one layout: Resources at the bare URL and `/resources`, then `/syncs` and `/settings` — **DONE** | 1h15m | t001 |
| [t004](done/t004.md) | The header carries Render's one action, Manual sync, and Disconnect moves to Settings — **DONE** | 30m | t003 |
| [t005](done/t005.md) | Render's `/blueprint/<id>/…` URLs, old bookmarks, in-app links and bex-api's Blueprint links open the page they name — **DONE** | 45m | t003 |
| [t006](done/t006.md) | Each page's pending frame equals its ready page at 1440 and 390 — **DONE** | 30m | t002, t003 |
| [t007](t007.md) | Verify in a real browser at 1440 and 390, locally and on production | 45m | t004, t005, t006 |
| [t008](done/t008.md) | Render parity — **DONE** | 25m | t007 |
| [t009](done/t009.md) | Simplify — **DONE** | 15m | t008 |
| [t010](done/t010.md) | Test coverage — **DONE** | 40m | t008 |
| [t011](t011.md) | Closeout | 10m | t009, t010 |

Total: **11 tasks, about 6h25m**.

## What Render does

- **Evidence:** Render's public dashboard bundle, read 2026-11-25 without sign-in: `index-D5V4ZgbE.js` (sha256 `8d15889ace2d…`) and the chunks it loads, `urls-ubVUii_6.js`, `IACSyncCard-CwII5eI4.js` (`1f4ec599d75e…`), `ExportBlueprint-iFmH1Srl.js` (`5e3b63e6fce9…`) and `IACSourcesForUser-CCjS67Hn.js` (`dbb9b0508d48…`). The hashes change with every Render deploy, so find the code by the strings quoted below. A signed-in walk was not possible when this was filed (the local Playwright browser was held by another session): t001 confirms the bundle read-only when a signed-in Render session is available, and otherwise the bundle stands as the evidence, as it did for w3/m641.
- **Layout** (`id: "iac-source-app"`; find it by the hrefPattern `/blueprint/:id/syncs`): the shared resource shell with a sidebar of three entries, no group label: **Resources** (`/blueprint/<id>/resources`, hrefPatterns also the bare `/blueprint/<id>`), **Syncs** (`/blueprint/<id>/syncs`, also active on `/blueprint/<id>/sync/:syncId`) and **Settings** (`/blueprint/<id>/settings`). The entries never depend on permissions. The back link reads **Blueprints** (`/blueprints`); the breadcrumb is Blueprints → the Blueprint's name (its repository's name when it has none).
- **Routes:** children `*` and `resources` (Resources), `syncs`, `sync` and `sync/:syncId` (a sync's own page, w3/m678; `sync` with no id shows the sync handed over by navigation, else redirects to `/syncs`), and `settings`. `urls-*.js` builds `/blueprints`, `/blueprint/new`, `/blueprint/${id}`, `/blueprint/${id}/sync/${syncId}` and `/blueprint/generate`.
- **Header:** the resource header with one action, **Manual sync** (refresh icon; disabled behind the no-permission tooltip without `updateBlueprint`). It plans a sync (`planIACSync`) and opens that sync's page (w3/m678); a failed plan shows a dismissible **Sync Error** banner. Disconnect Blueprint is on Settings (w3/m679).
- **Document title** `<name> ・ Blueprint` on every page; an unknown id draws the not-found page.
- **Docs** ([render.com/docs/infrastructure-as-code](https://render.com/docs/infrastructure-as-code), read 2026-11-25): "set Auto Sync to No on your Blueprint's Settings page … You can then manually trigger a sync by clicking Manual sync on your Blueprint's page."

## bex today

Verified 2026-11-25 on main `c196617a8b`.

- `dashboard/src/routes/blueprints.$blueprintId.tsx` (767 lines) is the only Blueprint detail route. Its header has the name, `BlueprintResourceSuffix`, `BlueprintStatusBadge`, **Sync** (hidden while the first plan awaits approval) and **Disconnect**. Below it: `PlannedSyncPanel` while a plan waits, a metadata card (inline-edited name and path, repository, read-only branch, the auto-sync switch, created, updated), a Resources card (Name, Type), a Sync history table (Commit, State, Started, Completed, Error), a Manifest card and `ValidatePanel`. Sync opens a dialog that previews the file (`useBlueprintPreview`) and retries a refused sync through `BlueprintConfirmationDialog`.
- `DashboardSidebar` (`common/components/dashboard-layout/dashboard-sidebar.tsx`) swaps in a resource sidebar for a project, a service or a database (and a Key Value instance once w3/m649 lands), not a Blueprint.
- `/blueprint/<id>` falls through to `routes/$.tsx` (`NotFoundPage`): `common/lib/render-alias.ts` maps only `web`, `worker`, `pserv`, `static`, `cron`, `d` and `r`, and `routes/blueprint.new.tsx` serves only `/blueprint/new`.
- Links into the page: `routes/blueprints.tsx` (the list), `common/components/dashboard-layout/global-search-dialog.tsx` and `features/blueprints/pages/new-blueprint-page.tsx` (after create). `common/lib/route-skeleton-manifest.ts` lists `/blueprints/$blueprintId` with the `blueprint-detail` shape; `canonicalBlueprintLinkId` keeps `blp-` bookmarks landing (w2/m269).
- bex-api links a Blueprint as `/blueprints/<exs-id>` through `resourcemeta.Config.DashboardURL("blueprints", …)`: `lego/backend/internal/apps/blueprint_onboarding.go` (the planned and the approved sync's `dashboardUrl`) and `lego/backend/internal/notifications/alert_email_worker.go` (the sync-failure email). `resourcemeta/metadata.go` already names `PostgresDashboardRoute = "d"` and `KeyValueDashboardRoute = "r"`. Render's public Blueprint object carries no `dashboardUrl`; its syncs do (w3/m678).
- The Postgres pages (w3/m641) and the Key Value pages (w3/m649) are the shape to copy: `features/databases/components/database-nav.tsx`, `common/components/dashboard-layout/database-sidebar.tsx`, the `databases.$databaseId*.tsx` layout and child routes, and the resource-page nav helpers w3/m649 shares.

## Definition of done

1. A Blueprint's pages show a resource sidebar at 1440 and in the 390 drawer: the Blueprints back link, the Blueprint's name, then Resources, Syncs and Settings, with the open page highlighted (Resources on the bare URL too). Every other page's sidebar is unchanged; `/blueprint/new` and `/blueprints/new` keep the workspace sidebar.
2. `/blueprints/<id>` and `/blueprints/<id>/resources` open Resources; `/syncs` and `/settings` open theirs, each with its own heading and document title. Render's `/blueprint/<id>`, `/blueprint/<id>/resources`, `/syncs` and `/settings` open the same page with the query and hash kept, `/blueprint/<id>/sync[/<syncId>]` opens Syncs until w3/m678 adds the sync page, and `/blueprint/new` stays the create page. The stacked single page is gone.
3. Every card and control the page held is on a page, in Render's order as t001 maps it: the managed resources on Resources; the sync history and the waiting plan's panel on Syncs (until w3/m678 replaces them); name, path, branch, auto-sync and the bex-only facts on Settings, with the Manifest card and `ValidatePanel` kept where t001 places them and recorded as bex extras.
4. The header names the Blueprint (with its suffix and status) and carries one action, **Manual sync**, which runs today's sync flow; Disconnect is on Settings.
5. Every in-app link to a Blueprint (the list, the search palette, the post-create landing) opens Resources, an old `blp-` bookmark still lands, and bex-api's Blueprint links (the planned and approved sync `dashboardUrl`, the sync-failure email) use Render's `/blueprint/<id>` shape.
6. Each page's pending frame matches its ready page at 1440 and 390.
7. `docs/render-artifacts/blueprint-pages.md` records Render's pages. `dashboard-routes.md`'s Blueprint row names `/blueprint/{id}` as verified and its § Sidebar navigation describes the Blueprint sidebar; ADR018's Blueprint row (`docs/ADR018/deployment-sources-iac.md`) says what the dashboard does.
8. Walked in a real browser locally and, once a production pin carries the milestone, on production as the QA user in `bex-canary` with a Blueprint created for the walk (disposable).

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w3` (2026-11-25), item 1, verified the same day against Render's bundle and its chunks. ADR018's Blueprint row scores the UI ◐ without recording this difference, and `dashboard-routes.md` called the detail path "docs-fallback … unverified"; Render's is `/blueprint/<id>`.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)). A Blueprint's pages and URLs are customer-facing, and Render-shaped links are a contract bex accepts through its alias routes and emits as `dashboardUrl`.
- **Expected outcome:** someone who knows Render's Blueprint pages finds the same sidebar, pages and URLs; a pasted `/blueprint/<id>` link opens the Blueprint instead of a 404; w3/m678–m680 have the pages they fill.
- **Why now:** after w3/m641 (Postgres) and w3/m649 (Key Value), a Blueprint is the last resource with a Render resource sidebar that bex draws as one stacked page, and their nav code is fresh to reuse. Run it after w3/m649 lands, which shares the resource-page nav helpers. It comes first of the five Blueprint milestones because the sync page (w3/m678), Settings (w3/m679) and Resources (w3/m680) build on its routes.
- **Render parity task:** included (t008): the milestone changes the dashboard's pages and URLs and the Blueprint links bex-api emits on REST, GraphQL, MCP and email.

## Local walk (2026-11-26)

Walked in headless Chromium against `yarn local-bex` (port 8399) behind a throwaway proxy that answers the Blueprint reads with two fixture Blueprints (one in sync with two resources and three syncs, one `created` with a waiting plan and a resource suffix), the dashboard on `vite dev --port 5377`, at 1440 and 390. The shared Playwright browser was held by another session; the proxy and the walk script stayed in `/tmp` and are not committed.

- Render's `/blueprint/<id>?from=cli#top`, `/resources`, `/syncs`, `/settings` and `/sync/<id>?from=email` land on `/blueprints/<id>[/<page>]` with the query and hash kept (`/sync/<id>` on Syncs); `/blueprints/<id>/sync` lands on Syncs and `/no-such-page` on Resources; a `blp-` bookmark stays on its spelling and opens; `/blueprint/new` and `/blueprints/new` are the create page with the workspace sidebar; `/blueprint/generate` goes to `/`.
- Each page's title names it (`Syncs ・ platform ・ Blueprint ・ bex Dashboard`) and its heading reads Resources, Syncs or Settings. The sidebar reads Blueprints, the name, Resources / Syncs / Settings with the open page highlighted, at 1440 and in the 390 drawer; the breadcrumb reads Blueprints › name.
- The header shows the name, the suffix, the status and Manual sync, disabled for the `created` Blueprint; its Syncs page shows the waiting plan with Approve and Plan again, and its empty Resources links Syncs.
- Pending against ready, with the Blueprint read held on a client navigation from `/blueprints`: at 1440 the header (65 px), the heading, the Resources card (196 px), Sync History (239 px) and the Settings cards (182, 82 px) equal the ready page; the manifest card follows the file's length. At 390 a header whose name and badge share one row with Manual sync is 40 px shorter than the frame, which reserves the two-row header a repository-length name takes; the frame matches that case exactly. A sync's error text that wraps makes the ready table taller. The sidebar did not move. No page error was logged.
