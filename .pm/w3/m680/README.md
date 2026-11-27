# w3 · m680 — A Blueprint's Resources page lists what it manages in Render's tables: services and datastores, Build Sources, Environment Groups

**Worker:** worker3 **Goal:** a Blueprint's Resources page draws its services, Postgres and Key Value in the resource table the Overview uses (status, runtime, region, updated, row menus, selection), its build sources and environment groups in their own tables, and Render's empty and file-problem states. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Record Render's Resources page in `docs/render-artifacts/blueprint-pages.md`, and choose how the page reads each resource's row | 25m | — |
| [t002](t002.md) | Services, Postgres and Key Value in the shared resource table, with row menus and the selection bar without Move | 1h15m | t001, w3/m677 t003 |
| [t003](t003.md) | Build Sources and Environment Groups tables | 45m | t001, w3/m677 t003 |
| [t004](t004.md) | Render's empty state, and the Blueprint file's problems with located errors | 30m | t002 |
| [t005](t005.md) | The pending frame equals the ready page at 1440 and 390 | 30m | t002, t003, t004 |
| [t006](t006.md) | Verify in a real browser at 1440 and 390, locally and on production | 45m | t005 |
| [t007](t007.md) | Render parity | 20m | t006 |
| [t008](t008.md) | Simplify | 15m | t007 |
| [t009](t009.md) | Test coverage | 40m | t007 |
| [t010](t010.md) | Closeout | 10m | t008, t009 |

Total: **10 tasks, about 5h35m**.

## What Render does

- **Evidence:** Render's public dashboard bundle, read 2026-11-25 without sign-in: `index-D5V4ZgbE.js` (sha256 `8d15889ace2d…`) and the chunks it loads, `urls-ubVUii_6.js`, `IACSyncCard-CwII5eI4.js` (`1f4ec599d75e…`), `ExportBlueprint-iFmH1Srl.js` (`5e3b63e6fce9…`) and `IACSourcesForUser-CCjS67Hn.js` (`dbb9b0508d48…`). The hashes change with every Render deploy, so find the code by the strings quoted below. A signed-in walk was not possible when this was filed (the local Playwright browser was held by another session): t001 confirms the bundle read-only when a signed-in Render session is available, and otherwise the bundle stands as the evidence, as it did for w3/m641. Find the page by "No resources managed by this Blueprint".
- **Up to three tables**, each drawn only when it has rows, in this order:
  - The resource table Render's Overview and project environments use: a type icon, **Service Name** (linked, with the Blueprint badge), **Status**, **Runtime** (a pill), **Region**, **Updated** and a row menu, sortable; rows can be selected into the floating action bar, with Move turned off on this page (`disableMove`). The row menu is the Overview's: Metrics, Settings, Generate Blueprint (w3/m681), Suspend or Resume ([service-list-bulk-actions.md](../../../docs/render-artifacts/service-list-bulk-actions.md)).
  - The Build Sources table, its name column titled **Build Source Name**, grouped by project, opening a build source's drawer.
  - The Environment Groups table.
- **Empty:** "No resources managed by this Blueprint".
- **The file has problems:** "There was a problem syncing with your Blueprint file:" ("were a few problems" for several) and the located errors, linked into the repository at the branch. A repository Render cannot reach shows its repository-access notice.

## bex today

Verified 2026-11-25 on main `c196617a8b`.

- After w3/m677, Resources holds today's card from `routes/blueprints.$blueprintId.tsx`: a two-column table (Name, linked through `blueprintResourceHref`; Type, the raw kind with underscores as spaces) and three bex messages (`blueprints.resourcesEmpty`, `resourcesEmptyAwaitingApproval`, `resourcesEmptyAfterError`).
- GraphQL `Blueprint.resources` is `[BlueprintResource {id, name, type}]`; its `type` covers services, Postgres, Key Value, environment groups and build sources (`type: build_source`, w3/m270).
- The Overview's and environments' table is `features/projects/components/resource-table.tsx` with `ServiceRowActions`, `DatabaseRowActionsWithCapabilities` and `KeyValueRowActionsWithCapabilities`; selection is `useResourceSelection` and the bar `BulkResourceToolbar` (w4/m550). `BlueprintErrorList` draws located errors.

## Definition of done

1. Resources draws the Blueprint's services, Postgres and Key Value in the table the Overview uses, with live status, runtime, region, updated, each row's menu and the selection bar (Resume and Suspend, no Move); its build sources in a table whose name column reads Build Source Name; its environment groups in an Environment Groups table; each table only when it has rows.
2. A Blueprint managing nothing reads "No resources managed by this Blueprint"; one whose last sync failed on its file lists the file's located errors under Render's sentence. bex's own empty messages give way to Render's, unless t001 records a reason to keep one as a bex extra.
3. The pending frame matches the ready page at 1440 and 390.
4. `blueprint-pages.md` and ADR018's Blueprint row say so; walked locally and, once a production pin carries it, on production as the QA user in `bex-canary`, with a Blueprint created for the walk (disposable) that manages a web service, a Postgres database, a Key Value instance and an environment group.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w3` (2026-11-25), item 4, verified the same day against Render's bundle (the Resources page and the resource table's columns).
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)). A Blueprint's Resources page is where Render shows what a Blueprint runs and their health.
- **Expected outcome:** a Blueprint's page shows whether its services are up and lets someone act on them, as the Overview does, instead of a list of names and kinds.
- **Why now:** w3/m677 gives Resources its own page, and the Overview's table and selection bar (w4/m550) are ready to reuse; w3/m681 adds Generate Blueprint to the same row menus and bar, so this page gets it too.
- **Render parity task:** included (t007): the dashboard changes; t001 decides whether GraphQL needs a field for the rows.
