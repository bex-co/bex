# w3 · m681 — Generate Blueprint starts from selected resources and opens Render's Create Blueprint page

**Worker:** worker3 **Goal:** Generate Blueprint is offered where Render offers it (the selection bar, each resource row's menu, a project environment's menu) and opens Render's three-step Create Blueprint page at `/blueprint/generate`; the Blueprints list's own Generate Blueprint button and dialog, which Render's list does not have, are gone. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Record Render's Generate Blueprint entry points and Create Blueprint page in `docs/render-artifacts/blueprint-pages.md` | 25m | — |
| [t002](t002.md) | The Create Blueprint page at `/blueprint/generate`, built on `generateBlueprint` | 1h15m | t001 |
| [t003](t003.md) | Generate Blueprint on the selection bar, each resource row's menu and a project environment's menu | 45m | t002 |
| [t004](t004.md) | The Blueprints list drops its Generate Blueprint button and dialog, as Render's list has none | 20m | t003 |
| [t005](t005.md) | Verify in a real browser at 1440 and 390, locally and on production | 40m | t004 |
| [t006](t006.md) | Render parity | 20m | t005 |
| [t007](t007.md) | Simplify | 15m | t006 |
| [t008](t008.md) | Test coverage | 35m | t006 |
| [t009](t009.md) | Closeout | 10m | t007, t008 |

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
- `BulkResourceToolbar` (`features/projects/components/bulk-resource-toolbar.tsx`) offers Move, Resume and Suspend; the row menus (`features/services/components/service-row-actions.tsx`, `features/databases/components/database-row-actions*.tsx`, `features/keyvalue/components/key-value-row-actions*.tsx`) and the environment card's menu (`features/environments/components/environment-card.tsx`) have no Generate Blueprint. [service-list-bulk-actions.md](../../../docs/render-artifacts/service-list-bulk-actions.md) records Render's action-bar entry; w4/m550 left it out.
- `/blueprint/generate` is not a page; after w3/m677 it goes to `/`, as Render's does with nothing selected.

## Definition of done

1. Generate Blueprint appears on the selection bar of every resource table (the Overview, project environments, and a Blueprint's Resources page once w3/m680 draws it), in each service, Postgres and Key Value row's menu, and in a project environment's menu; each is disabled with the reason for a role that cannot create a Blueprint.
2. Each opens `/blueprint/generate` with its resources: Render's title (the environment form from an environment's menu), the three steps with Render's words, the generated YAML, Download render.yaml and Copy, the commit snippet, and Connect a new Blueprint opening bex's create page in a new tab; a direct load with nothing selected goes to `/`.
3. The Blueprints list has no Generate Blueprint button or dialog.
4. The page's pending frame matches its ready page at 1440 and 390; strings in all eight languages.
5. `blueprint-pages.md`, `service-list-bulk-actions.md` and ADR018's Blueprint row (the Generate Blueprint text) say so; walked locally and, once a production pin carries it, on production as the QA user in `bex-canary` with resources created for the walk (disposable).

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w3` (2026-11-25), item 5, verified the same day against Render's bundle and its `ExportBlueprint` and `IACSourcesForUser` chunks. The verification corrected the brainstorm: Render's list has no Generate button, so bex's list dialog is itself a difference.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)). Render documents generating a Blueprint from existing services ([render.com/docs/infrastructure-as-code](https://render.com/docs/infrastructure-as-code), "Generating a Blueprint from existing services").
- **Expected outcome:** someone who selects services the way Render teaches finds Generate Blueprint there, and lands on the page Render shows; the list page matches Render's.
- **Why now:** the selection bar (w4/m550) and the API (w8/m22) both exist, so this is entry points and a page; w3/m680 puts the same bar on a Blueprint's Resources page.
- **Render parity task:** included (t006): the dashboard changes; REST, GraphQL and MCP `generateBlueprint` are reused, not changed.
