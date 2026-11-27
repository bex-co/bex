# w3 · m678 — Every Blueprint sync has its own page, and Manual sync opens it for review and Approve, as Render's does

**Worker:** worker3 **Goal:** each sync opens at `/blueprints/<id>/sync/<syncId>` with its commit, its state, the actions it planned and their states, and **Approve** while it waits; Syncs lists Render's sync cards linking there; Manual sync plans a sync and opens its page; every sync `dashboardUrl` bex-api emits opens that sync's page. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Record Render's sync page, sync card, Syncs page and Manual sync in `docs/render-artifacts/blueprint-pages.md` | 30m | — |
| [t002](t002.md) | bex-api records each sync's actions and commit message on every path, not only planned syncs | 1h15m | t001 |
| [t003](t003.md) | GraphQL reads one sync by id with its actions, their states, its errors and its required manual env vars, and lists syncs with their commit messages | 45m | t002 |
| [t004](t004.md) | The sync page at `/blueprints/<id>/sync/<syncId>` | 1h30m | t003, w3/m677 t003 |
| [t005](t005.md) | Syncs lists Render's sync cards, each linking to its page | 40m | t003, w3/m677 t003 |
| [t006](t006.md) | Manual sync plans a sync and opens its page; the sync dialog and the plan panel retire | 1h | t004 |
| [t007](t007.md) | Every sync `dashboardUrl` bex-api emits opens that sync's page | 20m | t004 |
| [t008](t008.md) | Verify in a real browser at 1440 and 390, locally and on production | 45m | t005, t006, t007 |
| [t009](t009.md) | Render parity | 25m | t008 |
| [t010](t010.md) | Simplify | 15m | t009 |
| [t011](t011.md) | Test coverage | 45m | t009 |
| [t012](t012.md) | Closeout | 10m | t010, t011 |

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
- [ADR049 § Approving from the dashboard](../../../docs/ADR049-render-yaml-parity.md) says "a CLI-created Blueprint on render.com's dashboard was not walked live"; the bundle now shows the page Render links.

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
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)). The sync `dashboardUrl` is part of Render's Blueprint API that bex serves on REST and MCP, and the review page is where Render's plan-then-approve flow (w5/m283) sends a person.
- **Expected outcome:** a link printed by the CLI, MCP or an agent opens the sync it names; a person reviews a plan and approves it on the page Render uses; the sync history shows what each sync did.
- **Why now:** plan-then-approve is how Render's MCP server and CLI create and change Blueprints, so every agent-driven plan prints a link that today lands on the Blueprint, not the plan. It builds on w3/m677's routes and comes before Settings (w3/m679), whose saves may open a sync.
- **Render parity task:** included (t009): GraphQL gains a read, the dashboard gains pages, and the `dashboardUrl` REST, GraphQL and MCP emit changes.
