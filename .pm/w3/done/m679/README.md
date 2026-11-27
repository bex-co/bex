# w3 · m679 — A Blueprint's Settings page is Render's: Blueprint Name, Auto Sync, Branch, Blueprint Path, then Disconnect Blueprint

**Worker:** worker3 **Goal:** a Blueprint's Settings page holds Render's one Settings card in Render's order and words, its Branch can be changed there as on Render, and Disconnect Blueprint asks with Render's modal. **Status:** done (2026-11-27: every task done; t005 walked locally against `local-bex` and on production pin `546d61ac7343` as the QA user in `bex-canary` with Blueprints created for the walk (disposable), at 1440 and 390)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record Render's Settings card and Disconnect modal in `docs/render-artifacts/blueprint-pages.md` — **DONE** | 15m | — |
| [t002](done/t002.md) | Blueprint Name, Auto Sync and Blueprint Path are Render's rows, in Render's order and words — **DONE** | 1h | t001, w3/m677 t003 |
| [t003](done/t003.md) | Branch can be changed with the repository's branch picker — **DONE** | 1h | t002 |
| [t004](done/t004.md) | Disconnect Blueprint asks with Render's modal — **DONE** | 30m | t001, w3/m677 t004 |
| [t005](done/t005.md) | Verify in a real browser at 1440 and 390, locally and on production — **DONE** | 30m | t003, t004 |
| [t006](done/t006.md) | Render parity — **DONE** | 20m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 35m | t006 |
| [t009](done/t009.md) | Closeout — **DONE** | 10m | t007, t008 |

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

## Production walk (2026-11-27, t005 step 2)

Pin `546d61ac7343` (`d465649ee4`, carries `122bf94989`): the `dashboard` pod ran `bex-dashboard@sha256:28dafd897168…` and the `bex-api` pods `bex-operator@sha256:91cf7828761c…`, the pin's digests, read through the admin kubeconfig just before the walk. Walked 17:55–18:09Z as the QA user (`scripts/qa-login.sh`) in `bex-canary` (`tea-daif693dqjvc73e7as3g`) with a run-minted API key. Two Blueprints were created for the walk through REST `POST /v1/blueprints` from `https://github.com/bex-co/bex`, branch `main`, `examples/static-site/render.yaml`, auto-sync off, `create_new`, and their first syncs approved, so both were paused (bex-api checks a taken name only against approved Blueprints): `qa-20261127-m679` (disposable, `exs-dc4s7a0jnsts73b031p0`, suffix `-tdkq`) and `qa-20261127-m679-b` (disposable, `exs-dc4s7aivp92c739idfd0`, suffix `-1azo`). The second branch is `fix-ci-m37-probe`, which holds `examples/static-site/render.yaml` and `examples/hello-node/render.yaml` but not `examples/blueprint-refs/render.yaml` or `examples/retained-env/render.yaml`. Google Chrome headless through `playwright-core`, `en-US`, at 1440×900 and 390×900. The shared Playwright browser stayed free for other sessions.

| Point | Seen at 1440 and at 390 | Verdict |
| --- | --- | --- |
| The card (DoD 1) | One card titled Settings heads the page, its only heading under the header: Blueprint Name, Auto Sync, Branch and Blueprint Path in that order, each with Render's help and an Edit pencil. Details, the render.yaml manifest and Validate follow, and Disconnect Blueprint, a destructive button, is last. The title reads `Settings ・ qa-20261127-m679 ・ Blueprint ・ bex Dashboard` | holds |
| Blueprint Name (DoD 2) | An empty name read **Required** with Save changes disabled. `qa-20261127-m679-b` read **Name already in use** after Save (bex-api `BLUEPRINT_NAME_TAKEN`): the row stayed open with Save disabled, and Cancel restored the name. A free name saved, and the header and the title followed (`qa-20261127-m679-1440` at 1440, then back at 390) | holds |
| Auto Sync (DoD 3) | The paused Blueprint read **No** beside the Paused badge, and the choice offers Yes and No. Yes was refused under the row with bex-api's reason ("AutoSync needs a connected Git provider: bex doesn't receive this repo's pushes…", as `bex-canary` has no GitHub connection) and saved nothing | holds |
| Branch (DoD 4) | Edit read the branches (`RepoBranches` on Edit, not on load). It answered none, because `bex-canary` has no GitHub connection, so the row took the branch as typed (w5/m54's fallback; the list itself was walked locally). With the path on `examples/blueprint-refs/render.yaml`, `fix-ci-m37-probe` read `Blueprint file "examples/blueprint-refs/render.yaml" not found on "fix-ci-m37-probe" branch. Please check that the file exists on this branch.` and saved nothing, as `examples/retained-env/render.yaml` did at 390. `qa-no-such-branch-m679` read **Branch not found in this repository. Choose another branch.** With the path on `examples/static-site/render.yaml`, `fix-ci-m37-probe` saved at both widths, and REST `GET /v1/blueprints/<id>` read it | holds |
| The next plan (DoD 4) | On `fix-ci-m37-probe` with the path on `examples/hello-node/render.yaml`, Manual sync opened the plan `Sync: 0380167`, that branch's head commit and subject, with Create Service `hello-node-tdkq` and Stop managing Service `static-site-tdkq`. At 390, on `examples/static-site/render.yaml`, it read Resources already up to date. Auto-sync matches a push on the stored branch (`ListAutoSyncBlueprints(ctx, branch, …)`, `lego/backend/internal/apps/webhook.go`); no push was made to see it live | holds |
| Blueprint Path (DoD 5) | On `fix-ci-m37-probe`, `examples/blueprint-refs/render.yaml` (1440) and `examples/retained-env/render.yaml` (390) read Render's sentence for that branch and saved nothing; `../x.yaml` read the path rule with Save disabled. "Failed to update Blueprint. Please try again." answers a refusal without a reason, which production did not give; t008's `settings-refusal.test.ts` pins it | holds |
| Disconnect Blueprint (DoD 6) | The button opened Render's modal: its title, the three sentences with the name in bold, Cancel and Disconnect Blueprint (340 px wide at 390). Cancel kept the page. Confirm disconnected `qa-20261127-m679-b` at 1440 and `qa-20261127-m679` at 390 (the mutation answered, then REST read 404) and opened `/blueprints`. That list still showed the disconnected Blueprint until a reload: filed as [w3/153](../../153.md) | holds |
| Pending frame | From the Blueprints list with the `Blueprint` read held, the frame's Settings card (512 px at 1440, 572 at 390), Details (126, 178) and Validate (82) equalled the ready cards. The manifest frame is 274 px where this Blueprint's 15-line manifest draws 411, following the file. At 390 the ready header wraps Manual sync under `qa-20261127-m679 -tdkq Paused`, 40 px taller than the frame's one-row header (the layout's `BlueprintHeaderSkeleton`, sized for a typical name by w3/m680), which moves every card down by 40 px | holds for Settings; the header is w3/m680's |
| Layout | No sideways scroll at either width; a refusal wraps under its row; no page or console error | holds |

At 390, at 17:58 and 18:00Z, a branch change to `main` and a path change on `main` read "Bex's GitHub app is not authorized to read this repository…" (`access_denied`) for the public repository, where the same branch change through REST went through seconds later, and the UI's at 18:08Z. bex-api reads a repository with no GitHub connection anonymously, within GitHub's shared budget of 60 an hour, and reads GitHub's refusal as an App without access: [w6/185](../../../w6/185.md).

Both Blueprints were disconnected by the walk (REST `GET` 404, and `GET /v1/blueprints` empty), and their services `static-site-tdkq` and `static-site-1azo` (disposable) deleted through REST (204, then 404). `bex-canary` is back to the services, Blueprints and environment groups it had before the walk. The run key was revoked and the QA session logged out (`qa-login.sh --logout`).

## Unblock record

- **2026-11-27, `/loopx w3` (verified evidence, no user decision):** the gate, pin must carry `122bf94989`, holds. The newest pin on `origin/main` is `d465649ee4` `chore(deploy): pin platform images to 546d61ac7343`, `git merge-base --is-ancestor 122bf94989 546d61ac7343` exits 0, and production's `dashboard` and `bex-api` run that pin's digests. Affects t005 (step 2) and t009. No gate remains.
- **2026-11-27, `/loopx w3`, closed:** t005 step 2 held every point of the definition of done on production pin `546d61ac7343` (§ Production walk), with the Blueprints and their services deleted, the run key revoked and the QA session logged out. t009 moved the milestone to `done/`.
