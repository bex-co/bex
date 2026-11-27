# w3 · m679 — A Blueprint's Settings page is Render's: Blueprint Name, Auto Sync, Branch, Blueprint Path, then Disconnect Blueprint

**Worker:** worker3 **Goal:** a Blueprint's Settings page holds Render's one Settings card in Render's order and words, its Branch can be changed there as on Render, and Disconnect Blueprint asks with Render's modal. **Status:** blocked (t001–t004 and t006–t008 done; t005 walked locally, its production walk waits for a pin carrying `122bf94989`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record Render's Settings card and Disconnect modal in `docs/render-artifacts/blueprint-pages.md` — **DONE** | 15m | — |
| [t002](done/t002.md) | Blueprint Name, Auto Sync and Blueprint Path are Render's rows, in Render's order and words — **DONE** | 1h | t001, w3/m677 t003 |
| [t003](done/t003.md) | Branch can be changed with the repository's branch picker — **DONE** | 1h | t002 |
| [t004](done/t004.md) | Disconnect Blueprint asks with Render's modal — **DONE** | 30m | t001, w3/m677 t004 |
| [t005](t005.md) | Verify in a real browser at 1440 and 390, locally and on production | 30m | t003, t004 |
| [t006](done/t006.md) | Render parity — **DONE** | 20m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 35m | t006 |
| [t009](t009.md) | Closeout | 10m | t007, t008 |

Total: **9 tasks, about 4h35m**.

## What Render does

- **Evidence:** Render's public dashboard bundle, read 2026-11-25 without sign-in: `index-D5V4ZgbE.js` (sha256 `8d15889ace2d…`) and the chunks it loads, `urls-ubVUii_6.js`, `IACSyncCard-CwII5eI4.js` (`1f4ec599d75e…`), `ExportBlueprint-iFmH1Srl.js` (`5e3b63e6fce9…`) and `IACSourcesForUser-CCjS67Hn.js` (`dbb9b0508d48…`). The hashes change with every Render deploy, so find the code by the strings quoted below. A signed-in walk was not possible when this was filed (the local Playwright browser was held by another session): t001 confirms the bundle read-only when a signed-in Render session is available, and otherwise the bundle stands as the evidence, as it did for w3/m641. Find the page by "Disconnect Blueprint" and "A unique name for your Blueprint.".
- **One card titled Settings**, its rows in this order, each a `RowInput` (read view with Edit, then Save and Cancel), read-only behind the no-permission tooltip without `updateBlueprint`:
  1. **Blueprint Name**, help "A unique name for your Blueprint."; empty reads "Required"; a name another Blueprint of the workspace has reads "Name already in use" (checked as typed through `blueprintNameExists`, and again when a save is refused as a duplicate).
  2. **Auto Sync**, a Yes / No choice, help `Automatically sync changes to your Blueprint file? Select "No" to handle syncs manually.`; it reads No when the Blueprint is paused.
  3. **Sync Hook**, only when the Blueprint has a `syncKey`: a private URL and "Your private URL to trigger a sync for this Blueprint. Remember to keep this a secret." Render's docs do not mention it; out of scope.
  4. **Branch**, the repository's branch picker. A branch without the file reads `Blueprint file "<path>" not found on "<branch>" branch. Please check that the file exists on this branch.`
  5. **Blueprint Path**, a file picker over the repository at the branch, placeholder `render.yaml`, help "The path to the Blueprint file in your repo (e.g., `infra/render.yaml`). Defaults to `render.yaml` at the root of your repository."; the same not-found sentence; any other failure "Failed to update Blueprint. Please try again."
  6. **Git Credentials** (whose Git access syncs use, Use My Credentials): a signed-off divergence ([`DO_NOT_DO.md` #GIT-WORKSPACE-OWNED](../../../DO_NOT_DO.md)); not built.
  - A save of Auto Sync, Branch or Path whose mutation returns a sync goes to that sync's page.
- **Disconnect Blueprint**, a critical button under the card, opens a modal titled "Disconnect Blueprint": "Resources will no longer sync automatically from your Blueprint file." / "This will not delete the file itself. This will not delete the managed resources. You can always connect your file later from the Blueprint page." / "Are you sure you want to disconnect your Blueprint spec **<name>**?", with Cancel and a critical Disconnect Blueprint; an error shows in the modal; success goes to `/blueprints`.
- **REST:** `PATCH /blueprints/{blueprintId}` accepts `branch`: "The branch must contain a Blueprint file at the configured path. Updating this field does not plan a sync." (pinned `lego/backend/internal/api/openapi/render-public-api-1.json`).
- **Docs** ([render.com/docs/infrastructure-as-code](https://render.com/docs/infrastructure-as-code)): "set Auto Sync to No on your Blueprint's Settings page".

## bex today

Verified 2026-11-25 on main `c196617a8b`.

- After w3/m677, Settings holds today's metadata card from `routes/blueprints.$blueprintId.tsx`: `InlineEdit` name and path (path checked by `validBlueprintPath`), repository, a read-only branch (`blueprints.metaBranch`, read-only since w8/m21 as "the identity key"), an auto-sync `Switch`, created and updated, and Disconnect with bex's own dialog (`blueprints.disconnect*` strings).
- bex-api already changes a branch: REST `PATCH /v1/blueprints/{id}` and GraphQL `updateBlueprint(branch:)` check the file exists on the new branch (`lego/backend/internal/apps/blueprint.go` `UpdateBlueprint`, `discoverBlueprintFile`) and refuse a taken name (`errBlueprintNameTaken`, w8/250). No plan is opened, as Render's REST says.
- A Blueprint is its id since w8/m153 ([ADR049 § A Blueprint is its id](../../../../docs/ADR049-render-yaml-parity.md#a-blueprint-is-its-id-several-may-track-one-file-w8m153-2026-11-11)), so the branch is no longer an identity key.
- The branch picker exists: `features/services/hooks/use-repo-branches.ts` (GraphQL `repoBranches`), used by `features/blueprints/pages/new-blueprint-page.tsx` and `features/services/components/service-source-card.tsx`. `EditableFieldRow` (`features/services/components/editable-field-row.tsx`) is bex's `RowInput`.

## Definition of done

1. Settings shows one card titled Settings with Blueprint Name, Auto Sync, Branch and Blueprint Path in that order, each with Render's label and help in all eight languages, as rows with Edit, Save and Cancel, read-only (with the reason) for a role that cannot change the Blueprint. bex's read-only facts (repository, created, updated) and the bex extras w3/m677 placed sit below it, recorded as bex extras.
2. An empty name reads "Required" and a taken one "Name already in use" at the field, without saving.
3. Auto Sync is Yes / No with Render's help, and reads No for a paused Blueprint.
4. Branch opens the repository's branches and saves through the API's `branch`; a branch without the file reads Render's sentence at the field and saves nothing; afterwards the page, its next plan and its auto-sync use the new branch.
5. A path without the file reads Render's sentence; any other failure reads "Failed to update Blueprint. Please try again.".
6. Disconnect Blueprint is a critical button under the card and asks with Render's modal; Cancel keeps the Blueprint, and Disconnect Blueprint disconnects it and opens `/blueprints`.
7. `blueprint-pages.md` and ADR018's Blueprint row say so; walked locally and, once a production pin carries it, on production as the QA user in `bex-canary`, with a Blueprint created for the walk (disposable) whose repository has a second branch carrying the file.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w3` (2026-11-25), item 3, verified the same day against Render's bundle and its pinned OpenAPI.
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)). Render documents Settings as where Auto Sync is turned off, and lets a Blueprint follow another branch there.
- **Expected outcome:** someone who knows Render finds the same Settings card, can move a Blueprint to another branch without the API, and reads Render's words when something is wrong.
- **Why now:** the API has changed branches since w8/m153 made a Blueprint its id, but the dashboard still withholds it for a reason that no longer holds; w3/m677 creates the Settings page this fills. It runs after w3/m678 so a save that opens a sync can go to its page.
- **Render parity task:** included (t006): the dashboard changes; REST, GraphQL and MCP already accept `branch` and are checked, not changed.

## Local walk (2026-11-27)

Walked in headless Chrome against `yarn local-bex` behind a throwaway proxy that answers the Blueprint reads and Settings' `updateBlueprint`, `repoBranches` and `disconnectBlueprint` statefully (three fixture Blueprints, one paused; a repository whose `main` holds `render.yaml` and `infra/render.yaml`, `staging` only `render.yaml`, and `no-file` neither; one Blueprint's disconnect refused with `BLUEPRINT_SYNC_BUSY`), the dashboard on `vite dev`, at 1440 and 390. The shared Playwright browser was held by another session; the proxy and the walk script stayed in `/tmp` and are not committed.

- The page opens with Render's Settings card, its one heading: Blueprint Name, Auto Sync, Branch and Blueprint Path with Render's help, then Details, the manifest, Validate and **Disconnect Blueprint**. The pending frame and the ready page have the same boxes at both widths (Settings 512 px tall at 1440, 572 at 390; every card's top, height and width equal).
- An empty name read **Required**; the other fixture's name read **Name already in use** with Save held; a free name saved, and the header and title followed.
- Branch listed `main`, `staging` and `no-file` once Edit was pressed; `no-file` read `Blueprint file "render.yaml" not found on "no-file" branch. Please check that the file exists on this branch.`, a typed `gone` read **Branch not found in this repository. Choose another branch.**, and `staging` saved.
- On `staging`, Blueprint Path `infra/render.yaml` read Render's file sentence for `staging`; `../x.yaml` read the path rule before any save.
- Auto Sync No saved and the header's badge read Paused; the paused fixture read No.
- Disconnect Blueprint opened Render's modal with the name in bold; Cancel kept the page; the busy fixture's refusal stayed in the modal; a disconnect opened `/blueprints` without that Blueprint.
- At 390 a refusal wraps under its row, and an edited path field narrows beside Cancel and Save changes, as every bex edit-in-place row does.

## Parked (2026-11-27)

Shipped in `122bf94989`. Parked until production's pin carries it: then t005 step 2 walks Settings as the QA user in `bex-canary` with a Blueprint created for the walk (disposable) from a public repository whose second branch holds the file, at 1440 and 390 (each row's save and refusal, a branch change and a Manual sync afterwards, a path without the file, Disconnect's Cancel and confirm), records the pin and the result here, and t009 closes the milestone.
