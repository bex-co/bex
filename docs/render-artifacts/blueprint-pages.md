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

- **Resources** has no title of its own: its tables of services and datastores, Build Sources and Environment Groups, or `No resources managed by this Blueprint` ([§ Resources](#resources-w3m680)).
- **Syncs** opens with the title "Syncs", then the sync cards, newest first, each linking to its page (w3/m678), or `No syncs yet.`.
- **Settings** is one card titled "Settings": Name, Auto Sync, Sync Hook (when the Blueprint has a sync key), Branch, Blueprint Path, Git Credentials (w3/m679). Below the card, **Disconnect Blueprint** (a critical button, behind the same tooltip without `updateBlueprint`).

## A sync's page, Syncs and the sync card (w3/m678)

Read 2026-11-26 for w3/m678 from the same bundle, without sign-in: `index-D5V4ZgbE.js` (sha256 `8d15889ace2d…`, unchanged), `IACSyncCard-CwII5eI4.js` (`1f4ec599d75e…`, the card), `iac-D9RnC10j.js` (`be64bc010544…`, the GraphQL documents and the sync states) and `urls-ubVUii_6.js` (`830179d15f15…`). Find the page by `Couldn't approve this sync. Try again.`, the card by `View details`, the list by `No syncs yet.`. No signed-in Render session was available, so this too is bundle-only.

### Sync states

`created`, `pending`, `running`, `success`, `error` (`iac-*.js`). `success` and `error` are terminal; every other state is still moving, and `created` is a plan waiting for **Approve**.

### The sync card

One `EventCard` per sync (`IACSyncCard-*.js`):

| State | Icon | Tone | Animated | Badge |
| --- | --- | --- | --- | --- |
| `created` | workflow | neutral | no | none |
| `pending`, `running` | workflow | neutral | yes | the state's name, small, animated |
| `success` | a check | success | no | none |
| `error` | a cross | critical | no | none |

- The first line is the commit, monospace: its short id, linked to the commit on the Git provider in a new tab, a colon, then the commit message (one line, the full message as its title). A sync without a commit has no first line.
- **View details** links to the sync's page.
- On the right, the relative creation time, refreshed every 30 seconds.

### Syncs

- The title "Syncs", then the cards, newest first, read 20 at a time with infinite scroll (`executions(before, limit)`, `hasMore`), or "No syncs yet.".
- A subscription prepends each new sync as it starts, so a push or a Manual sync appears at the top without a reload.
- The list includes a sync still `created` (the card has an icon for it): a plan waiting for approval is on Syncs.

### A sync's page

`/blueprint/<id>/sync/<syncId>` reads the sync (`iacExecution(id)`: `plan`, `state`, `actionStates { state message }`, `commit { id shortId message }`, `errors`) and draws:

1. The title `Sync: <short id>` (`Sync` alone when the sync has no commit).
2. The sync's card, without View details.
3. A card listing the plan's actions, one row per action, an update split into one row per changed sub-resource. Each row starts with its state's icon (`created` an outlined next arrow, `pending` a warning clock, `running` a spinner, `success` a green check, `error` a red cross, anything else an unknown mark), then the action's sentence by kind ("Create web service **api**", "Update … to …", "Associate … with this Blueprint"), the plan in parentheses where one applies, and an errored action's reason ("deploy failed" links the failed deploy). There is no delete: a resource dropped from the file is not removed.
   - A plan with no actions reads **Resources already up to date**, and a failed sync with no actions and no errors reads the critical callout **We weren’t able to fully sync these resources.** ("If you continue to see this error, " and a link that opens support chat).
   - The card shows only when the plan has actions or the sync has no errors, so a failed plan shows its errors alone.
4. While the sync is `created`, **Approve** under the actions (`approveIACSync(sourceId, executionId)`). It stays while the mutation runs or after it failed; a failure reads the server's message, or "Couldn't approve this sync. Try again.". The page sends no env var values: the create flow is where Render asks for `sync: false` values.
5. When the sync has errors, a card: "There was a problem syncing with your Blueprint:" ("There were a few problems" for several), then each error's path and message, and the Blueprint file at the commit with the erroring lines marked.

A sync that is not terminal subscribes to its state and action states, so the page follows it to the end, Approve's run included. The sidebar's **Syncs** is the open entry.

`/blueprint/<id>/sync` with no id draws the sync handed over in the navigation's state (a plan Manual sync just made that saved no sync, so a plan with no changes reads Resources already up to date) and redirects to `/blueprint/<id>/syncs` when there is none.

### Manual sync

The header's **Manual sync** runs `planIACSync` with the Blueprint's repository, branch and path, then navigates to `/blueprint/<id>/sync/<executionId>` (or `/sync` when the plan saved none) with the planned sync in the navigation's state. A failed plan shows a dismissible **Sync Error** banner with the server's message above the page. Settings' branch change opens the sync it returns the same way.

### What Approve must check (bex, t001 step 2)

Render's Approve carries no confirmation phrase, and neither does bex's `approveBlueprint`. bex refuses, at planning and again when the approved plan applies, what the reviewed-commit sync dialog asked about:

- **A protected environment.** A non-Admin's plan that would create a resource in a protected environment, or move one in or out, fails with a located notice (`blueprintProtectedPlacementProblems`, w3/m281), and the apply runs the same check before its first write ([protected-environments.md](protected-environments.md): Admins only). bex-api issues no confirmation phrase for a protected environment since w8/187, so the dialog's protected branch no longer fires.
- **Another Blueprint's resource.** A plan naming a resource another Blueprint manages fails with a notice naming the owner and Render's two ways forward, a `create_new` Blueprint or releasing it from the other Blueprint's file (`plannedOwnershipNotice`, w8/251), and an approved plan's apply refuses it without a phrase (`withPlannedApply`).

So Approve needs no guard of its own, and the sync dialog can retire. What retires with it is the dashboard's typed takeover of a resource another Blueprint manages: Render has no such takeover, and `syncBlueprint` with `confirm` stays on REST, GraphQL and MCP for the clients that use it.

## Settings and Disconnect Blueprint (w3/m679)

Read 2026-11-27 for w3/m679 from the same bundle, without sign-in: `index-D5V4ZgbE.js` (sha256 `8d15889ace2d…`, unchanged since 2026-11-25) holds the page, its rows and the modal. Find the page by `A unique name for your Blueprint.` and `Disconnect Blueprint`, the error wording by `Failed to update Blueprint. Please try again.`, the Branch row by `Branch not found in this repository. Choose another branch.`. No signed-in Render session was available, so this too is bundle-only, and no Blueprint's Settings was opened.

### The Settings card

One card titled **Settings**, its rows in this order. Each is a `RowInput`: the value read-only with an Edit action, then Save and Cancel while editing. Without `updateBlueprint` every row is read-only behind the no-permission tooltip.

| Row | Help | Control and refusals |
| --- | --- | --- |
| **Blueprint Name** | "A unique name for your Blueprint." | A text input. Empty reads **Required**. A name another Blueprint of the workspace has reads **Name already in use**, checked as typed (`blueprintNameExists(name, ownerId)`) and again when a save is refused as a duplicate. |
| **Auto Sync** | `Automatically sync changes to your Blueprint file? Select "No" to handle syncs manually.` | A select: **Yes** / **No**. It reads No when the Blueprint's status is paused, Yes otherwise (`setIACAutoSync(sourceId, autoSync)`). |
| **Sync Hook** | "Your private URL to trigger a sync for this Blueprint. Remember to keep this a secret." | Only when the Blueprint has a `syncKey`: the read-only URL `<api>/sync/<id>?key=<syncKey>`. Render's docs do not mention it. |
| **Branch** | "The Git branch to build and deploy." (the shared Branch row services use) | The repository's branch picker (`updateIacSourceBranch(id, branch)`). Empty reads **Required**. A branch the repository lacks reads **Branch not found in this repository. Choose another branch.**, and a failed lookup "Couldn't check that branch. Try again.". |
| **Blueprint Path** | "The path to the Blueprint file in your repo (e.g., `infra/render.yaml`). Defaults to `render.yaml` at the root of your repository." | A file picker over the repository at the branch, placeholder `render.yaml` (`updateIacSourcePath(id, path)`). |
| **Git Credentials** | — | Whose Git access syncs use, with Use My Credentials. A signed-off divergence ([`DO_NOT_DO.md` #GIT-WORKSPACE-OWNED](../../.pm/DO_NOT_DO.md)). |

The Branch and Blueprint Path rows word a refusal with one function (`sL`): a server message containing `not found: file:` reads `Blueprint file "<path>" not found on "<branch>" branch. Please check that the file exists on this branch.` (with the path the save reads, `render.yaml` when none, and the branch it reads from; `Blueprint file "<path>" not found. Please check that the file exists in your repository.` when there is no branch), any other refusal reads the server's own message, and a failure with no message reads **Failed to update Blueprint. Please try again.**

After a save, the Auto Sync, Branch and Blueprint Path mutations each select an `execution { id }`, and when one comes back the page navigates to that sync (`/blueprint/<id>/sync/<executionId>`). Whether Render plans a sync on those saves was not seen: its REST `PATCH /blueprints/{blueprintId}` says of `branch` "The branch must contain a Blueprint file at the configured path. Updating this field does not plan a sync.", and answers 404 for "the Blueprint file could not be found at the requested path when the branch is unchanged" (pinned `render-public-api-1.json`).

### Disconnect Blueprint

Below the card, a critical **Disconnect Blueprint** button (disabled behind the no-permission tooltip without `updateBlueprint`) opens a modal titled **Disconnect Blueprint**:

1. "Resources will no longer sync automatically from your Blueprint file."
2. "This will not delete the file itself. This will not delete the managed resources. You can always connect your file later from the Blueprint page."
3. "Are you sure you want to disconnect your Blueprint spec **&lt;name&gt;**?" (the name in semibold)

**Cancel** (secondary) and a critical **Disconnect Blueprint** (`deleteIACExecutionSource(id)`, loading while it runs). A refusal shows its message in an error callout inside the modal, which stays open; success removes the Blueprint from the list's cache and opens `/blueprints`.

## Resources (w3/m680)

Read 2026-11-27 for w3/m680 from the same bundle, without sign-in: `index-D5V4ZgbE.js` (sha256 `8d15889ace2d…`, unchanged since 2026-11-25) holds the page and its three tables, `iac-D9RnC10j.js` (`be64bc010544…`) its query, and `artifact-sources-table-utils-B6M4NPij.js` (`d5c90c8f06f4…`) the Build Sources table's grouping. Find the page by `No resources managed by this Blueprint`, the Build Sources table by `Build Source Name`. No signed-in Render session was available, so this too is bundle-only.

### What Render reads

The page runs `executionSourceResources(id)` network-only: `iacExecutionSource.resources`, each a full object (`Server`, `CronJob`, `Database`, `Redis`, `EnvGroup`, `WorkflowService`, `ArtifactSource`). It splits them three ways: the environment groups, the build sources (`ArtifactSource`), and everything else.

- **The file has problems.** When the query fails with a validation error, the page draws one card instead of the tables: "There was a problem syncing with your Blueprint file:" ("There were a few problems" for several), each error's path and message in the critical color, then the Blueprint file at the branch with the erroring lines marked. A payment error opens the payment modals instead.
- **The repository is out of reach.** When the query fails with `not found`, the page draws "Render cannot access the following repo:", the repository's URL, and "Please make sure the repository exists, or if this is a private repository, connect your Git provider to Render."
- **Nothing to list.** A card reading **No resources managed by this Blueprint**.

### The three tables

Each table is drawn only when it has rows, in this order, 32 px apart, with no heading of its own:

| Table | Columns | Rows |
| --- | --- | --- |
| The resource table (`ServicesTable`, the Overview's and a project environment's) | a type icon (its type as a tooltip), **Service Name** (linked, with the Blueprint badge), **Status**, **Runtime** (a pill), **Region**, **Updated** (relative), the row menu | the services, Postgres, Key Value and workflows; sortable; selectable into the action bar |
| Build Sources | **Build Source Name** (an icon, the name linked to the build source, **Unlinked** while no service builds from it), **Latest build** (its status icon and id, which opens the build's drawer), **Source** (the repository and branch, or the image), **Updated** (the latest build's time) | grouped by project: a row per group, the ungrouped build sources first as **Ungrouped**, then each project by name, each group open and collapsible |
| Env Groups | **Env Group Name**, **Environment**, **Env Vars**, **Secret Files** (a count column is left out when no group has any), **Updated** | sortable |

- **The row menu** is the Overview's with Move left out (`disableMove`): Metrics, Settings, Generate Blueprint (w3/m681), then Suspend or Resume.
- **The action bar** floats at the bottom of the screen while rows are selected: "N services selected:", then Generate Blueprint, Resume and Suspend, and no Move (the page's provider is `disableMove`). The selection clears when the route changes and when the workspace does.

### How bex reads the page (t001)

- **The rows come from the workspace's own lists**, filtered to the Blueprint's `resources` ids: services, Postgres and Key Value as the Overview reads them, build sources as `/build-sources` does, environment groups as `/env-groups` does. Each row is the Overview's own, with its live status, its menu and its lifecycle actions, and the page makes no read per row. GraphQL `Blueprint.resources` stays `{id, name, type}`, where Render's query resolves full objects.
- **The file's problems come from the newest sync.** bex-api resolves a Blueprint's resources from its last applied file, not the file at the branch, so the page reads the newest sync: when it failed with errors located in the file (at a line or a field), they replace the tables under Render's sentence, as Render's page does while its file does not validate. A sync that failed otherwise (a deploy, a busy Blueprint, an unreachable repository) leaves the tables.
- **bex's three empty messages give way to Render's one.** "No resources yet — sync the Blueprint", "approve the planned sync" and "the last sync failed", and their Go to Syncs link, are gone: Render has none of them, and Syncs, where the waiting plan and the failed sync are, is one click away in the sidebar.

## Create Blueprint (w3/m681)

Read 2026-11-27 for w3/m681 from the same bundle, without sign-in: `index-D5V4ZgbE.js` (sha256 `8d15889ace2d…`, unchanged since 2026-11-25) holds the selection bar and the row menu, `Project-C36VVmJD.js` (`772315df3935…`) a project environment's New menu, `ExportBlueprint-iFmH1Srl.js` (`5e3b63e6fce9…`) the page, `IACSourcesForUser-CCjS67Hn.js` (`dbb9b0508d48…`) the Blueprints list and `CodeSnippet-OVIRl2YL.js` the snippets. Find them by the navigation `source` values `action bar`, `table row menu` and `project environment menu`, and by `Create a new Blueprint from your services` and `Connect a new Blueprint`. No signed-in Render session was available, so this too is bundle-only.

### Where Render offers Generate Blueprint

Each entry point navigates to `/blueprint/generate` with the resources in the navigation's state (`resources`, `ownerTier`, `source`), and each is disabled without `createBlueprint` (`hasPermission === false`), its icon swapped for the protected one:

| Entry point | Shown | Resources | `source` |
| --- | --- | --- | --- |
| The floating action bar of a resource table (the Overview, a project environment, a Blueprint's Resources page), after Move | while a resource is selected (`selectedResources`, which leaves sandbox groups out) | the selected services, Postgres, Key Value and workflows | `action bar` |
| A row's menu: Metrics, Settings, Move, **Generate Blueprint**, then Suspend or Resume | on every service, Postgres, Key Value and workflow row | that resource | `table row menu` |
| A project environment's New menu ("Add services to environment"): Create new, Env Group, Move existing services, **Generate Blueprint** | while the environment has a service, Postgres, Key Value or workflow | all of them, with `projectName` and `environmentName` | `project environment menu` |

The topbar names the page **Create Blueprint**. Render's Blueprints list (`IACSourcesForUser-*.js`) has no Generate Blueprint: its only action is New Blueprint Instance.

### The page

- **Title**, also the document title: "Create a new Blueprint from your services", or "Create a new Blueprint from &lt;project&gt; / &lt;environment&gt;" when `source` is the environment menu and both names came with it.
- **No resources** (a direct load, a pasted link): the page navigates to `/`.
- **The export** is `blueprintExport(serviceIds, databaseIds, redisIds, workflowIds)`, network-only. A resource of any other kind is not exported.
- **Three numbered steps** in a vertical stepper (the wide column, 4 of 6 grid columns from `lg`):
  1. **Download generated render.yaml.** While the export loads, a 140 px box reading "Generating render.yaml...". Then "Includes N service:" or "services:" (every exported resource counts, the services first, then Postgres, Key Value and workflows), the names in bold as "A", "A and B" or "A, B, and C"; the YAML in a code snippet with its own Copy; then **Download render.yaml** (a `render.yaml` file, `application/yaml`) and **Copy**, which reads **Copied!** with a check for 2 seconds. Both are disabled, and faded, while the export loads.
  2. **Add render.yaml to Git.** "Commit render.yaml to the root directory of your repository and push to your Git provider." and the snippet `git add render.yaml` / `git commit -m "Add render.yaml"` / `git push`, with its Copy.
  3. **Connect your new Blueprint.** "Create a new Blueprint instance and connect your repository. See the docs." (Render's infrastructure-as-code docs) and **Connect a new Blueprint**, the create page in a new tab, disabled while the export loads.
- **Helpful Resources**, a card beside the steps (below them on a narrow page): Learn about Blueprints, Learn about Preview Environments, each a docs link in a new tab.
- A failed export shows nothing of its own: the sentence without the YAML.

### How bex builds it (t001)

- **The export is `generateBlueprint`** (GraphQL; REST `POST /v1/blueprints/generate`, MCP `generate_blueprint`, a labeled bex extension, unchanged). It takes services, Postgres, Key Value and environment groups; bex has no workflows.
- **A selection's environment groups go too.** Render's tables select no environment group; bex's environment section selects them for Move, so the selection bar exports a selected group as `envVarGroups` (a selected service's link to it becomes `fromGroup`), named in the sentence after the datastores. The environment menu exports what Render's does: the services and datastores, not the groups.
- **The environment's Generate Blueprint is in its ••• menu** until w1/m684 brings Render's New menu, whose t004 moves it there.
- **Helpful Resources has one link.** Learn about Blueprints opens bex's Blueprint docs (`bex.co/docs/platform/blueprints`), as See the docs does. Learn about Preview Environments is left out: PR previews are a non-goal (`DO_NOT_DO.md`), and bex has no page to send it to.
- **A refused export reads its reason** in an error callout where the YAML would be, with Download and Copy disabled, where Render's page shows nothing.

## Page map (bex, w3/m677)

Every card and control of bex's former single page (`routes/blueprints.$blueprintId.tsx`), moved unchanged to the page Render puts its counterpart on. What has no Render counterpart goes with the page it belongs to.

| bex card or control | Page, place | Render counterpart |
| --- | --- | --- |
| Name, `BlueprintResourceSuffix`, `BlueprintStatusBadge` | the layout's header | the resource header (Render's has no suffix or status badge: bex extras) |
| **Sync** and its dialog (preview, reviewed commit, `BlueprintConfirmationDialog`) | the header, as **Manual sync** | Manual sync |
| Managed Resources (Name, Type) | Resources: Render's three tables since w3/m680 | the resource tables |
| `PlannedSyncPanel` (a waiting plan, Approve, Plan again) | Syncs, 1st | a waiting sync's page (w3/m678) |
| Sync History (Commit, State, Started, Completed, Details) | Syncs, 2nd | the sync cards (w3/m678) |
| The metadata card: Name, Manifest path, Branch, Auto-sync | Settings, 1st: Render's Settings card since w3/m679 | the Settings card's Blueprint Name, Auto Sync, Branch, Blueprint Path |
| The metadata card's Repository, Created, Updated | Settings, 2nd: the Details card since w3/m679 | none: bex extras |
| render.yaml manifest | Settings, 2nd | none: a bex extra |
| Validate (`ValidatePanel`) | Settings, 3rd | none: a bex extra |
| **Disconnect** and its dialog | Settings, last | Disconnect Blueprint, with Render's words and modal since w3/m679 |

## What bex does (w3/m677, w3/m678, w3/m679, w3/m680, w3/m681)

- A Blueprint's pages swap the workspace nav for this sidebar: the **Blueprints** back link, the name, then Resources, Syncs and Settings, with the open page highlighted (Resources on the bare URL, Syncs on `/sync` and `/sync/<id>`).
- Each page is its own URL under `/blueprints/<id>`: the bare URL and `/resources`, then `/syncs`, a sync's `/sync/<syncId>`, and `/settings`. Any other sub-path lands on Resources, and a path past a sync's page on Syncs.
- Render's `/blueprint/<id>` and its sub-paths redirect there with the query and hash kept. `/blueprint/new` stays the create page, and `/blueprint/generate` is the Create Blueprint page (w3/m681).
- The header names the Blueprint with its suffix and status and carries **Manual sync**, which plans a sync and opens its page, or `/sync` with the plan handed over when it changed nothing; a refused plan shows the dismissible **Sync Error** banner (w3/m678).
- Syncs lists the sync cards, newest first, with **Load more** for older ones; bex's GraphQL lists a plan waiting for approval among them, as Render's dashboard does, where REST and MCP leave it out as Render's API does (w3/m678).
- A sync's page draws `Sync: <short sha>`, its card, its actions with their states, **Resources already up to date**, Render's critical line, the errors and **Approve** as Render's does, with a field per `sync: false` value the plan needs (bex-api's approve requires them; Render's page asks for none). It reads the sync until it settles. A sync recorded before syncs kept their actions (migration 0353) says so; an id that is not one of this Blueprint's syncs (a plan deleted after 24 hours) says it does not exist and links Syncs (w3/m678).
- bex-api's Blueprint links use Render's shape: a planned and an approved sync's `dashboardUrl` is the sync's page, `<dashboard>/blueprint/<exs-id>/sync/<exe-id>` (w3/m678), and the sync-failure email links `<dashboard>/blueprint/<exs-id>`.
- The breadcrumb is **Blueprints** → the name.
- Resources draws Render's tables (w3/m680), each only when it has rows: the Blueprint's services, Postgres and Key Value in the Overview's resource table (Name over the type, Status, Runtime, Region, Updated, the row's menu), whose selection toolbar offers Resume and Suspend and whose row menus offer no Move; its build sources under **Build Source Name**, **Unlinked** on one no service builds from, Source and Updated, a row per project above that project's; its environment groups in the Env Groups table. A Blueprint managing nothing reads **No resources managed by this Blueprint**; one whose newest sync failed on its file lists the located errors under "There was a problem syncing with your Blueprint file:" ("were a few problems" for several) instead of the tables. The pending frame draws the tables the Blueprint already read has, each with its rows.
- Settings is Render's card (w3/m679): **Blueprint Name**, **Auto Sync** (Yes / No, No while paused), **Branch** (the repository's branches, typed or picked) and **Blueprint Path**, each with Render's help, saving only its own field through GraphQL `updateBlueprint`. **Required**, **Name already in use**, Render's branch and file sentences, the server's reason, or **Failed to update Blueprint. Please try again.** read under the row, which stays open and saves nothing. A role without `can_create` sees the rows read-only with the reason. Below it come bex's extras: the Details card (Repository, Created, Last synced), the render.yaml manifest and Validate. **Disconnect Blueprint** is last, a critical button opening Render's modal, its three sentences with the name in bold; a refusal stays in the modal, and a disconnect opens `/blueprints`.
- bex-api refuses what Render's `update-blueprint` refuses, on REST, GraphQL and MCP alike (w8/418, w3/m679): a path whose file is not on the unchanged branch (404), and a new branch without the file at the path (400). Neither is saved, and a change plans nothing.
- **Generate Blueprint** is where Render's is (w3/m681): on the selection bar of every resource table (the Overview, a project environment, a Blueprint's Resources page), in each service, Postgres and Key Value row's menu, and in a project environment's ••• menu. Each is disabled with the reason for a role without `can_create`, and opens `/blueprint/generate` with its resources in the navigation's state: Render's title (the environment's form from its menu, in the document title too, which a reload keeps), the three steps with Render's words, `generateBlueprint`'s YAML with Download render.yaml and Copy (Copied! for 2 seconds), the commit snippet, and Connect a new Blueprint opening the create page in a new tab. A direct load goes to `/`. The pending frame is the page with its first step generating. The Blueprints list's own Generate Blueprint button and dialog are gone, as Render's list has none.

### Still apart

- **Each page's title names the page first** (`Syncs ・ <name> ・ Blueprint ・ bex Dashboard`; a sync's page `Sync ・ <name> ・ Blueprint`), as bex's Postgres and Key Value pages do, where Render's is `<name> ・ Blueprint` on every page.
- **Every page opens with a heading.** Render's Resources has none; bex's Resources opens with "Resources", as each bex resource page names its page, so a phone with the sidebar in its drawer still says which page is open.
- **The breadcrumb's name has no menu of the other Blueprints.** bex's datastore breadcrumbs have none either ([w3/135](../../.pm/w3/135.md)).
- **A failed sync's actions show no state of their own,** and a sync's page polls (every 3 seconds while it applies, every 25 while it waits) where Render subscribes. bex records a sync's state and error, not each action's result.
- **An action names its resource's kind** ("Create Service **api**") where Render names the service's type ("Create web service **api**") and links a resource that exists; bex adds **Stop managing** for a resource the file dropped, which Render's plan has no row for.
- **The errors are a list with their line and column.** Render also draws the Blueprint file at the sync's commit with the erroring lines marked.
- **Syncs loads older syncs with Load more**, as bex's deploy history does, where Render's list scrolls on.
- **The sync-failure email links the Blueprint**, not the failed sync: Render's email was not seen.
- **Settings checks a name when it is saved**, not as it is typed: bex-api has no query for whether a name is taken, so **Name already in use** reads after Save ([w3/140](../../.pm/w3/140.md)).
- **A Settings save stays on Settings.** Render's Auto Sync, Branch and Path mutations open the sync one returns; bex's `updateBlueprint` returns none, as Render's REST plans none on a branch change. Whether Render's dashboard plans on these saves was not seen ([w3/141](../../.pm/w3/141.md)).
- **Settings has no Sync Hook row or Git Credentials row.** The Sync Hook shows only for a Blueprint with a sync key, which Render's docs do not mention and bex has none of; Git Credentials is #GIT-WORKSPACE-OWNED.
- **The Branch row is a combobox over the repository's branches** (bex's shared Branch row) and the Path row a text field, where Render's Path row browses the repository's files. A branch the repository lacks reads Render's sentence from bex-api's refusal at Save, not from a lookup as it is typed.
- **Below the Settings card come bex's extras** (Details, the manifest, Validate) before Disconnect Blueprint, which Render's page has right under the card.
- **Resources' resource table is the Overview's.** Its name column reads Name, with the type as a badge under the name, where Render's reads Service Name beside a type icon and a Blueprint badge; the rows are in the file's order, not sortable; each row's menu is the Overview's (Suspend, Restart, Delete), without Render's Metrics and Settings ([w3/144](../../.pm/w3/144.md)); and the selection toolbar sits above the table, as on the Overview, where Render's floats at the bottom of the screen.
- **The Build Sources table has no Latest build column,** its Updated is the build source's own change, not its latest build's, and its project rows do not collapse ([w3/142](../../.pm/w3/142.md)).
- **A repository bex cannot reach leaves the tables.** Render's page says "Render cannot access the following repo:" ([w3/143](../../.pm/w3/143.md)).
- **The file's problems are a list with their line and column,** as on a sync's page: Render also draws the file at the branch with the erroring lines marked ([w3/137](../../.pm/w3/137.md) for both pages).
- **The Create Blueprint page's snippets have no line numbers or expand button,** and their copy control toasts where Render's shows a "Copied!" tooltip ([w3/145](../../.pm/w3/145.md)). The page's own Copy reads Copied! as Render's does.
- **Helpful Resources has no Preview Environments link,** and a refused export reads its reason where Render's page shows nothing (§ How bex builds it).
- **The environment's Generate Blueprint is in its ••• menu,** not Render's New menu, until w1/m684.
