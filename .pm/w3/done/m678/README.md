# w3 · m678 — Every Blueprint sync has its own page, and Manual sync opens it for review and Approve, as Render's does

**Worker:** worker3 **Goal:** each sync opens at `/blueprints/<id>/sync/<syncId>` with its commit, its state, the actions it planned and their states, and **Approve** while it waits; Syncs lists Render's sync cards linking there; Manual sync plans a sync and opens its page; every sync `dashboardUrl` bex-api emits opens that sync's page. **Status:** done (2026-11-27: every task done; t008 walked locally against `local-bex` and on production pin `546d61ac7343` as the QA user in `bex-canary` with a Blueprint created for the walk (disposable), at 1440 and 390)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record Render's sync page, sync card, Syncs page and Manual sync in `docs/render-artifacts/blueprint-pages.md` — **DONE** | 30m | — |
| [t002](done/t002.md) | bex-api records each sync's actions and commit message on every path, not only planned syncs — **DONE** | 1h15m | t001 |
| [t003](done/t003.md) | GraphQL reads one sync by id with its actions, their states, its errors and its required manual env vars, and lists syncs with their commit messages — **DONE** | 45m | t002 |
| [t004](done/t004.md) | The sync page at `/blueprints/<id>/sync/<syncId>` — **DONE** | 1h30m | t003, w3/m677 t003 |
| [t005](done/t005.md) | Syncs lists Render's sync cards, each linking to its page — **DONE** | 40m | t003, w3/m677 t003 |
| [t006](done/t006.md) | Manual sync plans a sync and opens its page; the sync dialog and the plan panel retire — **DONE** | 1h | t004 |
| [t007](done/t007.md) | Every sync `dashboardUrl` bex-api emits opens that sync's page — **DONE** | 20m | t004 |
| [t008](done/t008.md) | Verify in a real browser at 1440 and 390, locally and on production — **DONE** | 45m | t005, t006, t007 |
| [t009](done/t009.md) | Render parity — **DONE** | 25m | t008 |
| [t010](done/t010.md) | Simplify — **DONE** | 15m | t009 |
| [t011](done/t011.md) | Test coverage — **DONE** | 45m | t009 |
| [t012](done/t012.md) | Closeout — **DONE** | 10m | t010, t011 |

Total: **12 tasks, about 8h20m**.

## What Render does

- **Evidence:** Render's public dashboard bundle, read 2026-11-25 without sign-in: `index-D5V4ZgbE.js` (sha256 `8d15889ace2d…`) and the chunks it loads, `urls-ubVUii_6.js`, `IACSyncCard-CwII5eI4.js` (`1f4ec599d75e…`), `ExportBlueprint-iFmH1Srl.js` (`5e3b63e6fce9…`) and `IACSourcesForUser-CCjS67Hn.js` (`dbb9b0508d48…`). The hashes change with every Render deploy, so find the code by the strings quoted below. A signed-in walk was not possible when this was filed (the local Playwright browser was held by another session): t001 confirms the bundle read-only when a signed-in Render session is available, and otherwise the bundle stands as the evidence, as it did for w3/m641. The API side is the pinned Render CLI (`github.com/render-oss/cli` `v1.1.3-0.20261120013850-09b7802aa379`, `lego/cli/go.mod`).
- **Sync page** (find it by "Couldn't approve this sync. Try again."): the heading `Sync: <commit short id>`, the sync's card, then a block listing one row per planned action (an update is split into one row per changed sub-resource), each drawn by its kind with its state from `actionStates`; "Resources already up to date" when the plan has no actions; a critical "We weren't able to fully sync these resources." on a failed sync with none. While the sync is `CREATED` an **Approve** button applies it (`approveIACSync`; its failure reads "Couldn't approve this sync. Try again."). A sync with errors adds "There was a problem syncing with your Blueprint:" ("were a few problems" for several) and the located errors. A sync that has not finished subscribes to its state and action states, so the page follows it to the end.
- **`/blueprint/<id>/sync`** with no id shows the sync handed over by navigation (the one Manual sync just planned), else redirects to `/syncs`.
- **Syncs page:** the heading "Syncs", "No syncs yet.", a list of sync cards newest first with load-more paging, and new syncs prepended as they start.
- **Sync card** (`IACSyncCard-*.js`): a status icon per state (created, pending and running neutral, pending and running animated; error critical; success a check); the commit's short id linked to the commit on the Git provider, a colon and the commit message; a **View details** link to the sync's page; a state badge only while pending or running; the relative creation time.
- **Manual sync** (the header's action, w3/m677) plans a sync (`planIACSync`) and navigates to `/blueprint/<id>/sync/<syncId>` with it. Settings saves that return a sync (auto-sync, branch, path) go to its page too.
- **API:** the CLI's `pkg/client/blueprints/blueprints_gen.go` documents `SyncWithPlan.dashboardUrl` as "Link to this Sync in the Render Dashboard" and `ApprovedSync.dashboardUrl` as "Link to the sync in the Render Dashboard"; `pkg/text/blueprint.go` prints them as "Review in Dashboard (optional):" after a plan and "Track in Dashboard:" after an approve.

## bex today

Verified 2026-11-25 on main `c196617a8b`.

- No sync has a page. The sync history is a table (Commit, State, Started, Completed, Error) on the Blueprint page (on Syncs after w3/m677). A waiting plan is `PlannedSyncPanel` (`features/blueprints/components/planned-sync-panel.tsx`, `hooks/use-planned-sync.ts`) with Approve and Plan again. Sync opens a dialog that previews the file (`useBlueprintPreview`) and applies through GraphQL `syncBlueprint`, which takes `confirm` for a protected environment or a takeover (`BlueprintConfirmationDialog`, w5/m31, w8/m41).
- Only a planned sync stores its actions: `blueprint_syncs.plan_actions` (migration 0187) is written by `lego/backend/internal/store/blueprint_plans.go`. A manual, automatic or first sync records commit, state, times, error and note only, and no sync records its commit message (deploys do, migrations 0026 and 0040).
- GraphQL `BlueprintSync` is `commitId, completedAt, errorMessage, id, note, startedAt, state`; `blueprintPlannedSync(id)` reads only the waiting plan; `planBlueprintSync(id)` plans; `approveBlueprint(id, manualEnvVars, ownerId, syncId)` applies a plan and takes no `confirm`.
- bex-api's sync links (`apps/blueprint_onboarding.go`, the planned and the approved sync) point at the Blueprint, at `/blueprint/<id>` once w3/m677 lands.
- [ADR049 § Approving from the dashboard](../../../../docs/ADR049-render-yaml-parity.md) says "a CLI-created Blueprint on render.com's dashboard was not walked live"; the bundle now shows the page Render links.

## Definition of done

1. `/blueprints/<id>/sync/<syncId>`, and Render's `/blueprint/<id>/sync/<syncId>`, open the sync: `Sync: <short sha>`, its card, one row per planned action with its state, "Resources already up to date" for a sync with none, the located errors of a failed one, and **Approve** while it waits, with a field per required manual env var. The page follows a running sync to its end without a reload; Syncs is highlighted in the sidebar.
2. Syncs lists sync cards in Render's shape (state icon, linked short sha and commit message, View details, relative time, a badge while pending or running), newest first, with "No syncs yet." and paging.
3. Manual sync plans a sync and opens its page, and approving it there applies it; a plan with no changes lands on a page saying "Resources already up to date". The sync dialog and the Blueprint page's plan panel are gone, and nothing they guarded is lost: a sync touching a protected environment or another Blueprint's resource still needs what it needs today before it applies.
4. Every sync bex records from now on (manual, automatic, first, approved) carries its actions and commit message; an older sync without them still opens, with its card and a line saying its actions were not recorded.
5. The planned and the approved sync's `dashboardUrl` on REST, GraphQL and MCP (which the pinned CLI prints as Review in Dashboard and Track in Dashboard) open that sync's page.
6. `docs/render-artifacts/blueprint-pages.md` records Render's sync pages; ADR049 § Approving from the dashboard and ADR018's Blueprint row describe bex's.
7. Walked locally and, once a production pin carries the milestone, on production as the QA user in `bex-canary`: a Manual sync of a Blueprint created for the walk (disposable) from plan to Approve, an automatic sync or a re-plan, and a `bex ea blueprints plan` whose Review in Dashboard link opens the page.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w3` (2026-11-25), item 2, verified the same day against Render's bundle, its `IACSyncCard` chunk and the pinned Render CLI.
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)). The sync `dashboardUrl` is part of Render's Blueprint API that bex serves on REST and MCP, and the review page is where Render's plan-then-approve flow (w5/m283) sends a person.
- **Expected outcome:** a link printed by the CLI, MCP or an agent opens the sync it names; a person reviews a plan and approves it on the page Render uses; the sync history shows what each sync did.
- **Why now:** plan-then-approve is how Render's MCP server and CLI create and change Blueprints, so every agent-driven plan prints a link that today lands on the Blueprint, not the plan. It builds on w3/m677's routes and comes before Settings (w3/m679), whose saves may open a sync.
- **Render parity task:** included (t009): GraphQL gains a read, the dashboard gains pages, and the `dashboardUrl` REST, GraphQL and MCP emit changes.

## Local walk (2026-11-27)

Walked in headless Chromium against `yarn local-bex` (port 8499) behind a throwaway proxy that answers the Blueprint reads, `planBlueprintSync` and `approveBlueprint` statefully: an in-sync Blueprint with 25 syncs (one running, one failed, one recorded before syncs kept their actions, one with a detachment note), a `created` one whose plan waits with a `sync: false` value and whose Manual sync the proxy refuses, and one whose plan changes nothing; the dashboard on `vite dev --port 5477`, at 1440 and 390. The shared Playwright browser stayed free for other sessions; the proxy and the scripts stayed in `/tmp` and are not committed.

- Syncs lists 20 cards (state icon, linked short sha and message, View details, a Running badge on the running one, the relative time); Load more appends the older ones and leaves. View details opens the sync's page (at 390 too, once the commit link's enlarged touch target stopped covering it), and Render's `/blueprint/<id>/sync/<id>` lands there; Syncs is the open entry in the sidebar and in the 390 drawer, and the title reads `Sync ・ platform ・ Blueprint ・ bex Dashboard`.
- A sync's page: `Sync: <short sha>`, its card, the actions with their states; the failed sync's error under "There was a problem syncing with your Blueprint:"; the failed plan's two located notices under "There were a few problems …" with no actions card; the older sync's "recorded before syncs kept their actions" line; the detachment note; an unknown id says it does not exist and links Syncs.
- Manual sync on the in-sync Blueprint opened the new plan's page with Approve; Approve turned it Running and the page followed it to Success without a reload. The waiting plan's Approve stayed disabled until its `API_TOKEN` field had a value. The refused plan showed the Sync Error banner with the server's reason, and its close button removed it. The no-change plan opened `/blueprints/<id>/sync` reading Resources already up to date, and a reload kept it (the browser keeps the navigation's state, as on Render).
- Pending against ready, with the sync and the syncs reads held on a client navigation: the heading (28 px), the sync's card (48 px at 1440, 76 px at 390) and Syncs' three cards (70 px, 100 px) matched to the pixel; the actions card follows its plan's length (94 px framed for two plain rows, 112 px for the walked plan whose second row lists changed fields). No horizontal overflow at 390, and no page error once the handed-over state waited for hydration (fixed in the walk).

## Parked (2026-11-27)

Shipped in `30d75225ff`. Everything but the production walk is done: once the newest `chore(deploy): pin platform images to <sha>` carries `30d75225ff`, t008 step 2 walks production as the QA user in `bex-canary` with a Blueprint created for the walk from a public repository (disposable), at 1440 and 390: Manual sync to Approve, a plan with no changes, a failed file, Syncs and its paging, and `bex ea blueprints plan`, whose Review in Dashboard link opens the sync's page. Then it deletes the Blueprint, and t012 closes the milestone if every point of the definition of done holds. Nothing is needed from the user.

## Production walk (2026-11-27, t008 step 2)

Pin `546d61ac7343` (`d465649ee4`, carries `30d75225ff`): the `dashboard` pod ran `bex-dashboard@sha256:28dafd897168…` and the `bex-api` pods `bex-operator@sha256:91cf7828761c…`, the pin's digests, read through the admin kubeconfig just before the walk. Walked 17:32–17:44Z as the QA user (`scripts/qa-login.sh`) in `bex-canary` (`tea-daif693dqjvc73e7as3g`) with a run-minted API key and the checkout's `bex` CLI, on the Blueprint `qa-20261127-m678` (disposable, `exs-dc4rteojnsts73b030mg`), created with `bex ea blueprints create` from `https://github.com/bex-co/bex`, branch `main`, `examples/blueprint-refs/render.yaml` (whose `MANUAL_SECRET` is `sync: false`), auto-sync off, `create_new` (suffix `-cx4p`). Google Chrome headless through `playwright-core`, `en-US`, at 1440×900 and 390×900. The shared Playwright browser stayed free for other sessions.

| Point | Seen at 1440 and at 390 | Verdict |
| --- | --- | --- |
| Links (DoD 5, 7) | `bex ea blueprints create` and `bex ea blueprints plan` printed `Review in Dashboard (optional): https://dashboard.bex.co/blueprint/<exs-id>/sync/<exe-id>` for their plans, MCP `plan_blueprint` and GraphQL `planBlueprintSync` returned the same `dashboardUrl` for theirs, and `bex ea blueprints approve` printed `Track in Dashboard:` with the approved sync's page. Each opened `/blueprints/<id>/sync/<exe-id>` on that sync | holds |
| The waiting plan (DoD 1) | `Sync: 587d3aa`, its card (the waiting icon, `587d3aa` linked to the commit on GitHub, the commit's subject, the relative time), Create rows for the environment group and the three services, "These values are not in the Blueprint file (sync: false). Enter them to approve." and a password field `blueprint-refs-app-cx4p: MANUAL_SECRET`. Approve stayed disabled until the field had a value. Syncs is the open sidebar entry, in the 390 drawer too, and the title reads `Sync ・ qa-20261127-m678 ・ Blueprint ・ bex Dashboard`. An earlier plan that later plans replaced still opens, and its Approve was refused with "Someone planned this Blueprint again. Review the newest plan, then approve it." | holds |
| Approve (DoD 1, 3) | on the newest plan's page, opened from Render's `/blueprint/<id>/sync/<id>`, Approve with the value applied it without a navigation: Approve went away and the four rows turned Success in place (the run took 3 s, 17:34:27–30Z), and the header went from Awaiting approval to Paused | holds |
| No changes (DoD 3) | Manual sync then opened `/blueprints/<id>/sync` reading `Sync` and Resources already up to date, and a reload kept it | holds |
| A failed file (DoD 1) | `bex ea blueprints update --path examples/whoami-app.yaml` (a Kubernetes manifest) planned a failed sync, whose Review in Dashboard link opened its page. Manual sync on Syncs opened the next failed sync: the error card and "There were a few problems syncing with your Blueprint:" over four located errors (`line 4, column 1 · apiVersion` …), with no actions card | holds |
| Syncs and paging (DoD 2) | after 19 more failed plans, Syncs listed 20 cards, newest first (the error icon, the linked short sha and subject, View details, the relative time, no badge on a settled sync). Load more appended the last two, the oldest being the approved sync with its check, and went away: 22 distinct syncs | holds |
| A re-plan (DoD 7) | an update to `examples/retained-env/render.yaml` planned Create Service `retained-env-cx4p` and Stop managing for the three services, with no field and Approve enabled. The CLI approved it, and its Track in Dashboard page showed those rows and the detachment note. The plan says nothing about the environment group the file no longer declares, and the Blueprint then stops listing it: filed as [w3/152](../../152.md) | holds |
| Pending frame | with `BlueprintSyncs` held on a client navigation from Resources, Syncs' heading (28 px) and its three frame rows (70 px at 1440, 100 at 390) equalled the ready cards. With `BlueprintSync` held on View details, the heading (28 px) and the card (48 px at 1440, 76 at 390) equalled the ready page's. The actions frame (94 px, two rows) follows the data, as in the local walk: this failed sync's four errors draw a 166 px card at 1440 and 266 px at 390 | holds |
| Layout | no sideways scroll on any page at either width, no page or console error | holds |

The Blueprint was deleted through REST (`DELETE` 204, then 404), and so were its services `retained-env-cx4p`, `blueprint-refs-app-cx4p`, `blueprint-refs-peer-cx4p` and `blueprint-refs-site-cx4p` (disposable) and its environment group `blueprint-refs-group-cx4p` (disposable): 204 each, then 404. `bex-canary` is back to the services, Blueprints and environment groups it had before the walk. The run key was revoked and the QA session logged out (`qa-login.sh --logout`).

## Unblock record

- **2026-11-27, `/loopx w3` (verified evidence, no user decision):** the gate, pin must carry `30d75225ff`, holds. The newest pin on `origin/main` is `d465649ee4` `chore(deploy): pin platform images to 546d61ac7343`, `git merge-base --is-ancestor 30d75225ff 546d61ac7343` exits 0, and production's `dashboard` and `bex-api` run that pin's digests. Affects t008 (step 2) and t012. No gate remains.
- **2026-11-27, `/loopx w3`, closed:** t008 step 2 held every point of the definition of done on production pin `546d61ac7343` (§ Production walk), with the Blueprint and its resources deleted, the run key revoked and the QA session logged out. t012 moved the milestone to `done/`.
