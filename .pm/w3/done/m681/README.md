# w3 · m681 — Generate Blueprint starts from selected resources and opens Render's Create Blueprint page

**Worker:** worker3 **Goal:** Generate Blueprint is offered where Render offers it (the selection bar, each resource row's menu, a project environment's menu) and opens Render's three-step Create Blueprint page at `/blueprint/generate`; the Blueprints list's own Generate Blueprint button and dialog, which Render's list does not have, are gone. **Status:** done (walked locally and on production pin `546d61ac7343`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record Render's Generate Blueprint entry points and Create Blueprint page in `docs/render-artifacts/blueprint-pages.md` — **DONE** | 25m | — |
| [t002](done/t002.md) | The Create Blueprint page at `/blueprint/generate`, built on `generateBlueprint` — **DONE** | 1h15m | t001 |
| [t003](done/t003.md) | Generate Blueprint on the selection bar, each resource row's menu and a project environment's menu — **DONE** | 45m | t002 |
| [t004](done/t004.md) | The Blueprints list drops its Generate Blueprint button and dialog, as Render's list has none — **DONE** | 20m | t003 |
| [t005](done/t005.md) | Verify in a real browser at 1440 and 390, locally and on production — **DONE** | 40m | t004 |
| [t006](done/t006.md) | Render parity — **DONE** | 20m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 35m | t006 |
| [t009](done/t009.md) | Closeout — **DONE** | 10m | t007, t008 |

Total: **9 tasks, about 4h45m**.

## What Render does

- **Evidence:** Render's public dashboard bundle, read 2026-11-25 without sign-in: `index-D5V4ZgbE.js` (sha256 `8d15889ace2d…`) and the chunks it loads, `urls-ubVUii_6.js`, `IACSyncCard-CwII5eI4.js` (`1f4ec599d75e…`), `ExportBlueprint-iFmH1Srl.js` (`5e3b63e6fce9…`) and `IACSourcesForUser-CCjS67Hn.js` (`dbb9b0508d48…`). The hashes change with every Render deploy, so find the code by the strings quoted below. A signed-in walk was not possible when this was filed (the local Playwright browser was held by another session): t001 confirms the bundle read-only when a signed-in Render session is available, and otherwise the bundle stands as the evidence, as it did for w3/m641.
- **Entry points:** the floating action bar's **Generate Blueprint** while resources are selected, in every resource table (the Overview, project environments, a Blueprint's Resources page); **Generate Blueprint** in each resource row's menu, for that resource; and a project environment's menu, for all its resources (`source: "project environment menu"`). Each is disabled without `createBlueprint`. All navigate to `/blueprint/generate` with the resources in navigation state; the topbar names the page **Create Blueprint**. Render's Blueprints list (`IACSourcesForUser-*.js`) has no Generate Blueprint button: its only action is New Blueprint Instance.
- **The page** (`ExportBlueprint-*.js`): the title, also the document title, is "Create a new Blueprint from your services", or "Create a new Blueprint from <project> / <environment>" from an environment's menu. Three numbered steps:
  1. **Download generated render.yaml**: "Includes N service(s): A, B and C" (the names in bold, Oxford-comma list), "Generating render.yaml..." while it loads, the YAML, then **Download render.yaml** (a `render.yaml` file, `application/yaml`) and **Copy** (Copied! for 2 seconds).
  2. **Add render.yaml to Git**: "Commit render.yaml to the root directory of your repository and push to your Git provider." and the snippet `git add render.yaml` / `git commit -m "Add render.yaml"` / `git push`.
  3. **Connect your new Blueprint**: "Create a new Blueprint instance and connect your repository. See the docs." and **Connect a new Blueprint** (the create page, in a new tab).
  - A **Helpful Resources** aside: Learn about Blueprints, Learn about Preview Environments.
  - With nothing selected (a direct load) the page goes to `/`.
- Render exports services, Postgres, Key Value and workflows (`blueprintExport(serviceIds, databaseIds, redisIds, workflowIds)`).

## bex today

Verified 2026-11-25 on main `c196617a8b`.

- Generate Blueprint is a dialog on `/blueprints` (`features/blueprints/components/generate-blueprint-dialog.tsx`, w8/m22): resources are picked inside the dialog, then a YAML preview, copy and download. It is built on GraphQL `generateBlueprint` (REST `POST /v1/blueprints/generate`, MCP `generate_blueprint`, a labeled bex extension).
- `BulkResourceToolbar` (`features/projects/components/bulk-resource-toolbar.tsx`) offers Move, Resume and Suspend; the row menus (`features/services/components/service-row-actions.tsx`, `features/databases/components/database-row-actions*.tsx`, `features/keyvalue/components/key-value-row-actions*.tsx`) and the environment card's menu (`features/environments/components/environment-card.tsx`) have no Generate Blueprint. [service-list-bulk-actions.md](../../../../docs/render-artifacts/service-list-bulk-actions.md) records Render's action-bar entry; w4/m550 left it out.
- `/blueprint/generate` is not a page; after w3/m677 it goes to `/`, as Render's does with nothing selected.

## Local walk (2026-11-27)

Walked in headless Chrome (a private Playwright install; the shared browser stays the other session's) against `yarn local-bex`, which now answers `GenerateBlueprint` with a stand-in file naming the selection, and the dashboard on `vite dev`, at 1440 and 390. The walk scripts stayed in `/tmp` and are not committed.

- **The selection bar:** on the Overview, selecting `docs-site` showed Generate Blueprint after Move, before Suspend. It opened `/blueprint/generate` titled "Create a new Blueprint from your services" (the document title too), "Includes 1 service: **docs-site**", the YAML, Download render.yaml (a `render.yaml` holding exactly the YAML) and Copy (the clipboard held the YAML; the button read Copied! and Copy again after 2 seconds), the commit snippet, and Connect a new Blueprint (`/blueprint/new`, `target=_blank`). The breadcrumb read Blueprints › Create Blueprint, with Blueprints highlighted in the sidebar.
- **A row's menu:** Generate Blueprint, last after Move to project, opened the page for that one resource.
- **A project environment's menu:** the Production section's ••• menu offered Generate Blueprint between the isolation toggle and All settings; the page read "Create a new Blueprint from storefront / Production" and "Includes 3 services: **eden-cms-v2**, **orders-db**, and **sessions-cache**" (the service first, then the database and the Key Value; the section's environment group left out, as Render's menu leaves groups out).
- **A reload** kept the page and its selection. It first kept the server's "from your services" document title under the environment's heading; the page now re-runs its title loader in the browser when the two differ, and a reload reads the environment in both.
- **A direct load** in a fresh tab went to `/`. The Blueprints list's header reads New Blueprint Instance only.
- **Pending against ready:** with `GenerateBlueprint` held, the page's generating state and its server-rendered pending frame (scripts blocked on a reload) had equal boxes for every step, step title and the Helpful Resources card at 1440 and at 390; only the title's width differed at 1440, because the server cannot see the environment the page came from (the generic title at 390 wraps to the same two lines).

## Production walk (2026-11-27, t005 step 2)

Pin `546d61ac7343` (`d465649ee4`, carries `2902ee1877`): the `dashboard` pod ran `bex-dashboard@sha256:28dafd897168…` and both `bex-api` pods `bex-operator@sha256:91cf7828761c…`, the pin's digests, read through the admin kubeconfig before the walk. Walked 18:49–19:04Z as the QA user (`scripts/qa-login.sh`) in `bex-canary` (`tea-daif693dqjvc73e7as3g`) with a run-minted API key, in a headless Chrome for Testing of its own through `playwright-core`, `en-US`, at 1440×900 and 390×900. The walk scripts stayed in `/tmp` and are not committed.

Created through REST for the walk: the project `qa-20261127-m681` (disposable, `prj-dc4t1d8jnsts73b033g0`) with one environment, Production, holding the web service `qa-20261127-m681-web` (`traefik/whoami`, free), the Postgres `qa-20261127-m681-db` (Basic 256MB, as `bex-canary`'s free Postgres slot is taken) and the Key Value `qa-20261127-m681-kv` (Starter); and the ungrouped web service `qa-20261127-m681-solo` (disposable), because the Overview's table and its selection bar hold ungrouped resources only. No Blueprint and no GitHub read were needed.

Every point held at both widths, and each width downloaded the same bytes:

| Point (DoD) | Production | Verdict |
| --- | --- | --- |
| The Overview's selection bar (1, 2) | Selecting `qa-20261127-m681-solo` read Selected: 1 with the environment picker, Move, Generate Blueprint and Suspend. Generate Blueprint opened `/blueprint/generate`: "Create a new Blueprint from your services" as heading and document title, "Includes 1 service: **qa-20261127-m681-solo**", the YAML, the three steps with Render's words, the commit snippet and Helpful Resources (Learn about Blueprints, bex's Blueprint docs); the breadcrumb Blueprints › Create Blueprint with Blueprints highlighted at 1440, Create Blueprint in the topbar at 390 | holds |
| Download and Copy (2) | Download render.yaml saved `render.yaml`; Copy put the same bytes on the clipboard, read Copied!, then Copy again after 2 seconds. Connect a new Blueprint links `/blueprint/new` with `target=_blank` | holds |
| A reload (2) | Kept the page, its title and its selection, from the selection bar and from the environment's menu | holds |
| Row menus (1) | The Overview's web service: Suspend, Restart, Move to project, Generate Blueprint. The environment's Postgres and Key Value: Delete, Move to project, Generate Blueprint. Each opened the page for that one resource ("Includes 1 service: **qa-20261127-m681-db**", and the same for the Key Value) | holds |
| The environment's ••• menu (1, 2) | Generate Blueprint, then All settings. The page read "Create a new Blueprint from qa-20261127-m681 / Production" as heading and document title, and "Includes 3 services: **qa-20261127-m681-web**, **qa-20261127-m681-db**, and **qa-20261127-m681-kv**" | holds |
| The environment's selection bar (1) | Selecting all three gave the same file as the environment's menu, under the "from your services" title | holds |
| `POST /v1/blueprints/validate` (5) | All six downloads (the solo service from the bar and from its row, the environment from its menu and from its bar, the Postgres and the Key Value from their rows) at each width answered `valid: true`, each resource a `noop` against the resource it came from (the env var `WHOAMI_PORT_NUMBER` exported as `sync: false`) | holds |
| A direct load (2) | `/blueprint/generate` in a fresh tab went to `/` | holds |
| The Blueprints list (3) | Its header reads Blueprints and New Blueprint Instance; no Generate Blueprint button | holds |
| Pending against ready (4) | With `GenerateBlueprint` held, the generating state and the server-rendered pending frame (scripts blocked on a reload) had equal boxes for every step, step title and Helpful Resources at 1440 and at 390; only the title's width differed at 1440 (363 against 532 px), as locally, because the server cannot see the environment the page came from | holds |

A Blueprint's Resources page (DoD 1) was walked on the same pin by w3/m680: its selection bar and every row menu offer Generate Blueprint. The disabled state for a role that cannot create a Blueprint was not walked, as the QA user holds no lesser role in a workspace; the dashboard tests pin it (t008). The YAML wraps its long lines at 390, as Render's `wrapLines` snippet does; its line numbers and expand view are w3/145.

**Cleanup.** The four resources and the project were deleted through REST (204, then 404); `bex-canary` holds none of the walk's resources. The run key was revoked and the QA session logged out (`qa-login.sh --logout`).

## Unblock record

- **2026-11-27, `/loopx w3` (verified evidence, no user decision):** the gate, pin must carry `2902ee1877`, holds. The newest pin on `origin/main` is `d465649ee4` `chore(deploy): pin platform images to 546d61ac7343`, `git merge-base --is-ancestor 2902ee1877 546d61ac7343` exits 0, and production's `dashboard` and `bex-api` run that pin's digests. t005 step 2 walked it (§ Production walk) and t009 closed the milestone.

## Decisions (2026-11-27)

- **The selection bar exports a selected environment group too** (`envVarGroups`, with `fromGroup` for a selected service linked to it), named after the datastores. Render's tables select no group; bex's environment section selects them for Move, and `generateBlueprint` exports them, so leaving them out would silently drop what the user picked. The environment menu exports what Render's does: services and datastores only.
- **The environment's Generate Blueprint is in its ••• menu** until w1/m684 brings Render's New menu (its t004 moves the item there).
- **Helpful Resources has only Learn about Blueprints**, linking bex's Blueprint docs; Preview Environments is a non-goal (`DO_NOT_DO.md`).
- **A refused export reads its reason** in an error callout where the YAML would be, with Download and Copy disabled; Render's page shows nothing.

## Definition of done

1. Generate Blueprint appears on the selection bar of every resource table (the Overview, project environments, and a Blueprint's Resources page once w3/m680 draws it), in each service, Postgres and Key Value row's menu, and in a project environment's menu; each is disabled with the reason for a role that cannot create a Blueprint.
2. Each opens `/blueprint/generate` with its resources: Render's title (the environment form from an environment's menu), the three steps with Render's words, the generated YAML, Download render.yaml and Copy, the commit snippet, and Connect a new Blueprint opening bex's create page in a new tab; a direct load with nothing selected goes to `/`.
3. The Blueprints list has no Generate Blueprint button or dialog.
4. The page's pending frame matches its ready page at 1440 and 390; strings in all eight languages.
5. `blueprint-pages.md`, `service-list-bulk-actions.md` and ADR018's Blueprint row (the Generate Blueprint text) say so; walked locally and, once a production pin carries it, on production as the QA user in `bex-canary` with resources created for the walk (disposable).

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w3` (2026-11-25), item 5, verified the same day against Render's bundle and its `ExportBlueprint` and `IACSourcesForUser` chunks. The verification corrected the brainstorm: Render's list has no Generate button, so bex's list dialog is itself a difference.
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)). Render documents generating a Blueprint from existing services ([render.com/docs/infrastructure-as-code](https://render.com/docs/infrastructure-as-code), "Generating a Blueprint from existing services").
- **Expected outcome:** someone who selects services the way Render teaches finds Generate Blueprint there, and lands on the page Render shows; the list page matches Render's.
- **Why now:** the selection bar (w4/m550) and the API (w8/m22) both exist, so this is entry points and a page; w3/m680 puts the same bar on a Blueprint's Resources page.
- **Render parity task:** included (t006): the dashboard changes; REST, GraphQL and MCP `generateBlueprint` are reused, not changed.
