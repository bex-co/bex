# w3 · m680 — A Blueprint's Resources page lists what it manages in Render's tables: services and datastores, Build Sources, Environment Groups

**Worker:** worker3 **Goal:** a Blueprint's Resources page draws its services, Postgres and Key Value in the resource table the Overview uses (status, runtime, region, updated, row menus, selection), its build sources and environment groups in their own tables, and Render's empty and file-problem states. **Status:** done (t001–t010; walked locally and on production pins `546d61ac7343` and `7d1dce861173`, where the header's frame at 390 held after `3b8bebd3cd`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record Render's Resources page in `docs/render-artifacts/blueprint-pages.md`, and choose how the page reads each resource's row — **DONE** | 25m | — |
| [t002](done/t002.md) | Services, Postgres and Key Value in the shared resource table, with row menus and the selection bar without Move — **DONE** | 1h15m | t001, w3/m677 t003 |
| [t003](done/t003.md) | Build Sources and Environment Groups tables — **DONE** | 45m | t001, w3/m677 t003 |
| [t004](done/t004.md) | Render's empty state, and the Blueprint file's problems with located errors — **DONE** | 30m | t002 |
| [t005](done/t005.md) | The pending frame equals the ready page at 1440 and 390 — **DONE** | 30m | t002, t003, t004 |
| [t006](done/t006.md) | Verify in a real browser at 1440 and 390, locally and on production — **DONE** | 45m | t005 |
| [t007](done/t007.md) | Render parity — **DONE** | 20m | t006 |
| [t008](done/t008.md) | Simplify — **DONE** | 15m | t007 |
| [t009](done/t009.md) | Test coverage — **DONE** | 40m | t007 |
| [t010](done/t010.md) | Closeout — **DONE** | 10m | t008, t009 |

Total: **10 tasks, about 5h35m**.

## What Render does

- **Evidence:** Render's public dashboard bundle, read 2026-11-25 without sign-in: `index-D5V4ZgbE.js` (sha256 `8d15889ace2d…`) and the chunks it loads, `urls-ubVUii_6.js`, `IACSyncCard-CwII5eI4.js` (`1f4ec599d75e…`), `ExportBlueprint-iFmH1Srl.js` (`5e3b63e6fce9…`) and `IACSourcesForUser-CCjS67Hn.js` (`dbb9b0508d48…`). The hashes change with every Render deploy, so find the code by the strings quoted below. A signed-in walk was not possible when this was filed (the local Playwright browser was held by another session): t001 confirms the bundle read-only when a signed-in Render session is available, and otherwise the bundle stands as the evidence, as it did for w3/m641. Find the page by "No resources managed by this Blueprint".
- **Up to three tables**, each drawn only when it has rows, in this order:
  - The resource table Render's Overview and project environments use: a type icon, **Service Name** (linked, with the Blueprint badge), **Status**, **Runtime** (a pill), **Region**, **Updated** and a row menu, sortable; rows can be selected into the floating action bar, with Move turned off on this page (`disableMove`). The row menu is the Overview's: Metrics, Settings, Generate Blueprint (w3/m681), Suspend or Resume ([service-list-bulk-actions.md](../../../../docs/render-artifacts/service-list-bulk-actions.md)).
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
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)). A Blueprint's Resources page is where Render shows what a Blueprint runs and their health.
- **Expected outcome:** a Blueprint's page shows whether its services are up and lets someone act on them, as the Overview does, instead of a list of names and kinds.
- **Why now:** w3/m677 gives Resources its own page, and the Overview's table and selection bar (w4/m550) are ready to reuse; w3/m681 adds Generate Blueprint to the same row menus and bar, so this page gets it too.
- **Render parity task:** included (t007): the dashboard changes; t001 decides whether GraphQL needs a field for the rows.

## Local walk (2026-11-27)

Walked in headless Chrome against `yarn local-bex` behind a throwaway proxy that answers the Blueprint reads, `BuildSources` and Suspend and Resume statefully and can hold named reads (six fixture Blueprints: one with two services, a Postgres, a Key Value, two build sources in one project and an environment group; one with build sources in a project and in none; one managing nothing; one whose newest sync failed on its file; one whose newest sync failed at deploy; one with three resources), the dashboard on `vite dev`, at 1440 and 390. The shared Playwright browser was held by another session; the proxy and the walk scripts stayed in `/tmp` and are not committed.

- Resources drew the resource table (Name over the type badge, Status, Runtime, Region, Updated, the menu; Runtime, Region and Updated drop out on a phone as on the Overview), then Build Source Name with the project's row, **Unlinked** on the unused one, then the Env Groups table, each only for a Blueprint that has its rows.
- Each row's menu was the Overview's without Move to project (Suspend and Restart for a service, Delete for a datastore), where the Overview's still offers it. Selecting the web service read Selected: 1 with Suspend and no Move picker; Suspend asked for `sudo suspend 1 service`, the row read Suspended and the selection cleared; Resume brought it back to Running. Leaving for Syncs and coming back left nothing selected.
- Every name opened its page: the service, the Postgres, the build source and the environment group.
- The empty Blueprint read **No resources managed by this Blueprint**; the broken one "There were a few problems syncing with your Blueprint file:" with `line 4, column 11 · services[0].plan` and `line 9, column 5 · databases[0]` and no tables; the one whose deploy failed kept its table.
- Pending against ready, measured: with the lists held, the page's frame and the ready page have the same toolbar (32 px), the same tables at the same tops and heights (the resource table 288 px, Build Sources 151, Env Groups 89 at 1440) at both widths; a first visit with the Blueprint read held drew one table of three rows, equal to a three-resource Blueprint's ready page at both widths. Two fixes came from it: at 390 the header's name bar pushed Manual sync to a second row (40 px) where the ready header has one, and an Env Groups row's frame was 2 px taller and its Environment cell wrapped at 390.

## Parked (2026-11-27)

Shipped in `f5341407a5`. Parked until production's pin carries it: then t006 step 2 walks Resources as the QA user in `bex-canary` with a Blueprint created for the walk (disposable) that manages a web service, a Postgres database, a Key Value instance and an environment group, at 1440 and 390 (each table, a row menu, a selection suspended and resumed, the empty state, and a file problem from a broken push), records the pin and the result here, and t010 closes the milestone.

## Production walk (2026-11-27, t006 step 2)

Pin `546d61ac7343` (`d465649ee4`, carries `f5341407a5`): the `dashboard` pod ran `bex-dashboard@sha256:28dafd897168…` and both `bex-api` pods `bex-operator@sha256:91cf7828761c…`, the pin's digests, read through the admin kubeconfig before the walk. Walked 18:21–18:35Z as the QA user (`scripts/qa-login.sh`) in `bex-canary` (`tea-daif693dqjvc73e7as3g`) with a run-minted API key, in a headless Google Chrome of its own through `playwright-core`, `en-US`, at 1440×900 and 390×900.

No file in the repository declared all four kinds of resource, so `04ee4a1fab` added `examples/blueprint-resources/`: `render.yaml` (a prebuilt-image web service, a Basic 256MB Postgres, a Starter Key Value and an environment group, the datastores paid because `bex-canary` already held its free Postgres) and `broken.yaml` (two located errors), both checked against production's `POST /v1/blueprints/validate`. Two Blueprints were created through REST `POST /v1/blueprints` from `https://github.com/bex-co/bex`, branch `main`, auto-sync off, `create_new`: `qa-20261127-m680` (disposable, `exs-dc4skb2vp92c739idha0`, suffix `-gkun`) on `render.yaml`, and `qa-20261127-m680-b` (disposable, `exs-dc4sp9avp92c739idhp0`, suffix `-ok1m`) on `broken.yaml`. Creates were refused "bex's GitHub app is not authorized to read this repository" for minutes at a time before each went through, which is [w6/185](../../../w6/185.md)'s anonymous GitHub budget.

| Point | Seen at 1440 and at 390 | Verdict |
| --- | --- | --- |
| Empty (DoD 2) | `qa-20261127-m680`, before its first sync was approved, read **No resources managed by this Blueprint** under Resources and nothing else | holds |
| The tables (DoD 1) | After REST approve: the resource table (Name over the type badge, Status, Runtime, Region, Updated, the menu) with `resources-db-gkun` (Database, PostgreSQL 18), `resources-kv-gkun` (Key Value, Valkey 8) and `resources-web-gkun` (Web Service, Image), all in Frankfurt, their status live (Creating, then Available; Deploying, then Running). Then the Env Groups table with `resources-env-gkun`, Workspace (no Environment), 1 env var. No Build Sources table, as the Blueprint has none (the table was walked locally). At 390 Runtime, Region and Updated drop out as on the Overview, and the Env Groups table scrolls inside its card | holds |
| Row menus (DoD 1) | The Postgres and the Key Value: Delete, Generate Blueprint. The web service: Suspend, Restart, Generate Blueprint. No Move to project | holds |
| Selection (DoD 1) | Selecting the web service read Selected: 1 with Generate Blueprint and Suspend, and no Move picker. Suspend asked for `sudo suspend 1 service`, the row read Suspended and the selection cleared. Selected again, it offered Resume, which brought it back to Deploying. Both at each width | holds |
| File problems (DoD 2) | `qa-20261127-m680-b`, whose first plan failed on `broken.yaml`, read "There were a few problems syncing with your Blueprint file:" over `line 13, column 5 · databases[0]` and `line 10, column 11 · services[0].plan` with bex-api's messages, and no table | holds |
| Pending frame (DoD 3) | Moving from Syncs to Resources with the workspace lists and the syncs held, the frame's resource table (225 px) and Env Groups table (89 px) stood at the ready tables' tops (233 and 490 at 1440, 265 and 522 at 390). A cold navigation from the Blueprints list with the `Blueprint` read held drew one three-row table (the frame cannot know the Blueprint also has an environment group) under a one-row header, 65 px. At 390 the ready header wraps Manual sync under `qa-20261127-m680 -gkun Paused`, 105 px, so the page moved 40 px | holds at 1440 and with the lists held; the 390 header did not |
| Layout | No sideways scroll at either width; no page error, and no console error but Google Analytics' collect call refused in the headless browser | holds |

**The header at 390.** The wrapping row put Manual sync on a second line for any name past about ten characters with the suffix and status, which is most names, while t005 had sized the frame for one row. w3/m679's walk saw the same on Settings. Fixed in `3b8bebd3cd`: the ready header and its frame share one row class, with Manual sync under the name below 640 px and beside it from 640, where a long name truncates. On production's stylesheet with these classes, a 3-, a 16- and a 69-character name each gave 105 px below 640 and 65 px from 640, at 390, 639, 640, 768, 1024 and 1440.

**Cleanup.** Both Blueprints were deleted through REST (204, then 404), and their resources `resources-web-gkun`, `resources-db-gkun`, `resources-kv-gkun` and `resources-env-gkun` (disposable) deleted (204, then 404). `bex-canary` holds none of the walk's resources. The run key was revoked and the QA session logged out (`qa-login.sh --logout`).

## Production re-check (2026-11-28, t006 step 2)

Pin `7d1dce861173` (`207a050b22`, carries `3b8bebd3cd`): the `dashboard` pod ran `bex-dashboard@sha256:8ad155ac3cb1…` and both `bex-api` pods `bex-operator@sha256:d2f83bec3be6…`, the pin's digests, read through the admin kubeconfig before the walk. Walked 00:49–00:53Z as the QA user (`scripts/qa-login.sh`) in `bex-canary` (`tea-daif693dqjvc73e7as3g`) with a run-minted API key, in a headless Chromium of its own through `playwright-core`, `en-US`, at 1440×900 and 390×900.

`qa-20261128-m680-recheck` (disposable, `exs-dc52abr7laqs73bafep0`, suffix `-fuk6`), 24 characters, was created through REST `POST /v1/blueprints` from `https://github.com/bex-co/bex`, branch `main`, `examples/blueprint-resources/render.yaml`, auto-sync off, `create_new`; it went through on the first try, and its first sync was approved through REST (`success`). From the Blueprints list, its row was clicked with every GraphQL `Blueprint` read held, the frame measured, the read released and the ready page measured:

| Box (x, y, width × height, px) | Frame at 1440 | Ready at 1440 | Frame at 390 | Ready at 390 |
| --- | --- | --- | --- | --- |
| Header row | 256, 48, 1184 × 65 | 256, 48, 1184 × 65 | 0, 48, 390 × 105 | 0, 48, 390 × 105 |
| Manual sync | 1292, 64, 124 × 32 | 1292, 64, 124 × 32 | 16, 104, 124 × 32 | 16, 104, 124 × 32 |
| Resources heading | 336, 137 | 336, 137 | 16, 169 | 16, 169 |
| Selection bar and first table | 336, 189, 1024 × 269 | 336, 189, 1024 × 269 | 16, 221, 358 × 269 | 16, 221, 358 × 269 |
| First table | 233, 225 tall, 3 rows | 233, 225 tall, 3 rows | 265, 225 tall, 3 rows | 265, 225 tall, 3 rows |

The frame and the ready page share the header class (`flex-col` below `sm`, a row from it): at 390 Manual sync sits under `qa-20261128-m680-rec… -fuk6 Paused`, whose name truncates, and at 1440 beside the full name. The ready page then adds the Env Groups table under the first (490 at 1440, 522 at 390), which the frame cannot know before the Blueprint is read, as § Production walk recorded. Nothing moved: the header's frame at 390 holds. No sideways scroll at either width, and no page or console error.

**Cleanup.** The Blueprint was deleted through REST (204, then 404), and its resources `resources-web-fuk6`, `resources-db-fuk6`, `resources-kv-fuk6` and `resources-env-fuk6` (disposable) deleted (204 each, then 404); `bex-canary` lists no Blueprint. The run key was revoked (it then answered 401) and the QA session logged out (`qa-login.sh --logout`).

## Unblock record

- **2026-11-27, `/loopx w3` (verified evidence, no user decision):** the gate, pin must carry `f5341407a5`, holds. The newest pin on `origin/main` is `d465649ee4` `chore(deploy): pin platform images to 546d61ac7343`, `git merge-base --is-ancestor f5341407a5 546d61ac7343` exits 0, and production's `dashboard` and `bex-api` run that pin's digests. Affects t006 step 2.
- **2026-11-27, `/loopx w3`, parked again:** t006 step 2 held every point of the definition of done but the header's frame at 390 (§ Production walk), which `3b8bebd3cd` fixes. Remaining gate: pin must carry `3b8bebd3cd`. Then, as the QA user in `bex-canary`, a Blueprint created for the walk (disposable) from `examples/blueprint-resources/render.yaml`, with a name past ten characters and its first sync approved, is opened from the Blueprints list with its `Blueprint` read held, at 1440 and 390; its frame's header (105 px at 390, 65 at 1440) and its first table stand where the ready page's do. Then delete it and its resources, and t010 closes the milestone.
- **2026-11-28, `/loopx w3` (verified evidence, no user decision):** the gate, pin must carry `3b8bebd3cd`, holds. The newest pin on `origin/main` is `207a050b22` `chore(deploy): pin platform images to 7d1dce861173`, `git merge-base --is-ancestor 3b8bebd3cd 7d1dce861173` exits 0, and production's `dashboard` and `bex-api` run that pin's digests. Affects t006 step 2, which then held at 1440 and 390 (§ Production re-check), so t010 closed the milestone.
