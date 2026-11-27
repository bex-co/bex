# Render Blueprint pages capture (w3/m677)

A Blueprint's pages on Render, from its public dashboard bundle, and where each part of bex's former single Blueprint page went. w3/m678 (the sync page), w3/m679 (Settings), w3/m680 (Resources) and w3/m681 (Create Blueprint) extend this capture with their page's detail.

## Evidence (2026-11-26)

Read 2026-11-26 for w3/m677 from Render's public dashboard bundle, without sign-in. Not walked signed in: no signed-in Render session was available to this run, so the bundle is the evidence, as it was for the Postgres and Key Value pages ([postgres-dashboard.md](postgres-dashboard.md), [key-value.md § Pages](key-value.md#pages-sidebar-routes-and-info-w3m649-2026-11-25)).

`https://dashboard.render.com/` loads `assets/index-D5V4ZgbE.js` (sha256 `8d15889ace2d…`, the same file the milestone was filed against on 2026-11-25), which holds the layout, its route table, its header and its three pages. `assets/urls-ubVUii_6.js` builds the URLs. The hashes change with every Render deploy, so find the code by these strings: the layout's route id `iac-source-app`, the sidebar's `hrefPattern` `/blueprint/:id/syncs`, `Manual sync`, `Disconnect Blueprint`, `No resources managed by this Blueprint` and `No syncs yet.`.

## Layout and sidebar

`/blueprint/:id` renders the layout (`id: "iac-source-app"`) in the shared resource shell. Its sidebar is one group with no label:

| Entry | `href` | Also active on |
| --- | --- | --- |
| **Resources** | `/blueprint/<id>/resources` | the bare `/blueprint/<id>` |
| **Syncs** | `/blueprint/<id>/syncs` | a sync's page, `/blueprint/<id>/sync/:syncId` |
| **Settings** | `/blueprint/<id>/settings` | — |

- No entry depends on a permission. The shell draws all three before the Blueprint loads (`initialSideNav` already holds them).
- The back link reads **Blueprints** (`backHref: "/blueprints"`, `backTitle: "Blueprints"`).
- The breadcrumb is **Blueprints** (`/blueprints`) → the Blueprint's name, or its repository's name when it has none. The name crumb carries a menu of the workspace's other Blueprints.

## Routes

Under the layout, one `*` child (the header shell) holds the pages:

| Path | Page |
| --- | --- |
| `*`, `resources` | Resources (the bare URL shows Resources with the URL unchanged, and so does any unknown sub-path) |
| `syncs` | Syncs |
| `sync/:syncId` | one sync's page (w3/m678) |
| `sync` | the sync handed over by navigation state, else a redirect to `/blueprint/<id>/syncs` |
| `settings` | Settings |

`urls-*.js` builds `/blueprints`, `/blueprint/new`, `/blueprints/new`, `/blueprint/${id}`, `/blueprint/${id}/resources`, `/blueprint/${id}/sync[/${syncId}]` and `/blueprint/generate`.

## Header and title

- The resource header names the Blueprint and carries one action, **Manual sync**, with a refresh icon after its label. Without `updateBlueprint` it is disabled behind the no-permission tooltip.
- Manual sync plans a sync (`planIACSync`) and opens that sync's page with it (w3/m678). A failed plan shows a dismissible **Sync Error** banner above the page.
- The document title is `<name> ・ Blueprint` on every page (the repository's name when the Blueprint has none).
- An id that does not resolve draws the not-found page.

## Pages

- **Resources** has no title of its own: its tables of services and datastores, Build Sources and Environment Groups (w3/m680), or `No resources managed by this Blueprint`.
- **Syncs** opens with the title "Syncs", then the sync cards, newest first, each linking to its page (w3/m678), or `No syncs yet.`.
- **Settings** is one card titled "Settings": Name, Auto Sync, Sync Hook (when the Blueprint has a sync key), Branch, Blueprint Path, Git Credentials (w3/m679). Below the card, **Disconnect Blueprint** (a critical button, behind the same tooltip without `updateBlueprint`).

## Page map (bex, w3/m677)

Every card and control of bex's former single page (`routes/blueprints.$blueprintId.tsx`), moved unchanged to the page Render puts its counterpart on. What has no Render counterpart goes with the page it belongs to.

| bex card or control | Page, place | Render counterpart |
| --- | --- | --- |
| Name, `BlueprintResourceSuffix`, `BlueprintStatusBadge` | the layout's header | the resource header (Render's has no suffix or status badge: bex extras) |
| **Sync** and its dialog (preview, reviewed commit, `BlueprintConfirmationDialog`) | the header, as **Manual sync** | Manual sync |
| Managed Resources (Name, Type) | Resources | the resource tables (w3/m680) |
| `PlannedSyncPanel` (a waiting plan, Approve, Plan again) | Syncs, 1st | a waiting sync's page (w3/m678) |
| Sync History (Commit, State, Started, Completed, Details) | Syncs, 2nd | the sync cards (w3/m678) |
| The metadata card: Name, Manifest path, Branch, Auto-sync | Settings, 1st | the Settings card's Name, Blueprint Path, Branch, Auto Sync (w3/m679) |
| The metadata card's Repository, Created, Updated | Settings, in the same card | none: bex extras |
| render.yaml manifest | Settings, 2nd | none: a bex extra |
| Validate (`ValidatePanel`) | Settings, 3rd | none: a bex extra |
| **Disconnect** and its dialog | Settings, last | Disconnect Blueprint (w3/m679 gives it Render's words and modal) |

## What bex does (w3/m677)

- A Blueprint's pages swap the workspace nav for this sidebar: the **Blueprints** back link, the name, then Resources, Syncs and Settings, with the open page highlighted (Resources on the bare URL, Syncs on `/sync` and `/sync/<id>`).
- Each page is its own URL under `/blueprints/<id>`: the bare URL and `/resources`, then `/syncs` and `/settings`. Any other sub-path lands on Resources, and `/sync` and `/sync/<id>` land on Syncs until w3/m678 gives a sync its page.
- Render's `/blueprint/<id>` and its sub-paths redirect there with the query and hash kept. `/blueprint/new` stays the create page, and `/blueprint/generate` goes to `/`, as Render's does with nothing selected, until w3/m681 builds the page.
- The header names the Blueprint with its suffix and status and carries **Manual sync**, which still opens today's reviewed-commit dialog until w3/m678.
- bex-api's Blueprint links use Render's shape, `<dashboard>/blueprint/<exs-id>`: a planned and an approved sync's `dashboardUrl`, and the sync-failure email.
- The breadcrumb is **Blueprints** → the name.

### Still apart

- **Each page's title names the page first** (`Syncs ・ <name> ・ Blueprint ・ bex Dashboard`), as bex's Postgres and Key Value pages do, where Render's is `<name> ・ Blueprint` on every page.
- **Every page opens with a heading.** Render's Resources has none; bex's Resources opens with "Resources", as each bex resource page names its page, so a phone with the sidebar in its drawer still says which page is open.
- **Manual sync is disabled while a Blueprint's first plan waits for approval,** with the reason, because the reviewed-commit sync refuses a `created` Blueprint (w2/m256 t012). w3/m678 makes it plan a sync, as Render's does, which removes that rule.
- **The breadcrumb's name has no menu of the other Blueprints.** bex's datastore breadcrumbs have none either.
- The waiting plan's panel and the sync history table stand in for Render's sync page and sync cards until w3/m678, the metadata card for Render's Settings card until w3/m679, and the two-column table for Render's resource tables until w3/m680.
