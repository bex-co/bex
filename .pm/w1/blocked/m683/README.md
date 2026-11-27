# w1 · m683 — An environment's settings are Render's page at `/project/<id>/environment/<envId>/settings`, opened from the section's ••• menu

**Worker:** worker1 **Goal:** each environment has Render's settings page and ••• menu, and bex's settings dialog is retired. **Status:** blocked (pin must carry `cf73fe4fab`: t008's production walk as the QA user, then t012; t001–t007 and t009–t011 done, and t008's local walk passed)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | The route `/project/<id>/environment/<envId>/settings`, with Render's title and breadcrumbs — **DONE** | 1h | — |
| [t002](done/t002.md) | The Name card — **DONE** | 45m | t001 |
| [t003](done/t003.md) | The Permissions card: Default or Protected — **DONE** | 1h | t001 |
| [t004](done/t004.md) | The Connections card, its confirm modal and toast — **DONE** | 1h | t001 |
| [t005](done/t005.md) | The Environment Groups card and the Networking card — **DONE** | 1h | t001 |
| [t006](done/t006.md) | The Delete Environment card, with the last-environment refusal — **DONE** | 1h | t001 |
| [t007](done/t007.md) | The section's ••• menu: Block cross-environment connections and All settings; retire the settings dialog — **DONE** | 45m | t004, w1/m682 t005 |
| [t008](t008.md) | The settings page's pending frame, and a walk on dev-1 and production | 45m | t002, t003, t005, t006, t007 |
| [t009](done/t009.md) | Render parity — **DONE** | 20m | t008 |
| [t010](done/t010.md) | Simplify — **DONE** | 15m | t009 |
| [t011](done/t011.md) | Test coverage — **DONE** | 45m | t009 |
| [t012](t012.md) | Closeout | 10m | t010, t011 |

Total: **12 tasks, about 8h45m**.

## Definition of done

1. Each environment section's ••• menu offers Block cross-environment connections (confirm modal, Render's toast, plan gate) and All settings, which opens `/project/<id>/environment/<envId>/settings`.
2. The settings page has Name, Permissions, Connections, Environment Groups, Networking and Delete Environment with Render's words, validation and gates; other `…/environment/<envId>/<page>` URLs land on the project.
3. Deleting needs `sudo delete environment <name>`; a project's only environment shows Render's refusal; the REST/GraphQL/MCP outcome of the last-environment probe is recorded.
4. `environment-settings-dialog.tsx` and the card's Rename / Settings / Delete are gone.
5. The pending frame matches the ready page at 1440 and 390 px; walked on production.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w3` (2026-11-26), filed to w1 by `/pm` the same day after re-reading Render's dashboard bundle. Render's public dashboard bundle, read 2026-11-26 without sign-in (entry `index-D5V4ZgbE.js`; chunk hashes change with every Render deploy, so find the code by the strings quoted here): `index` holds the settings page's cards ("Environment name is required", "Name is already used by another environment in this project", "For staging and development", "Best for production", "Restrict connections", "Yes, remove restrictions", `sudo delete environment`, "projects require at least one environment"); the `Project` chunk holds the ••• menu ("Block cross-environment connections", "All settings"). render.com/docs/projects documents "••• → All settings".
- **bex today (checked 2026-11-26):** settings are a dialog (`environment-settings-dialog.tsx`); `/project/<id>/environment/<envId>/settings` answers NotFound; nothing refuses deleting the last environment.
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)).
- **Expected outcome:** the path Render's docs describe works on bex, and a Render URL to an environment's settings opens the same page.
- **Why now:** w1/m682 adds the section this menu sits on; the dialog's controls (protection, isolation, IP rules) already exist and only change place and words, so the gap is the page and the menu.
- **Render parity task:** included (t009): dashboard UI, and possibly a coded last-environment refusal on REST/GraphQL/MCP.

## Walks (t008)

- **Local, 2026-11-27** — `yarn local-bex` (its seeded `storefront` project: a protected, isolated Production holding one env group, and Staging; the stub now seeds every environment with the allow-all pair, as the real store does, and `LOCAL_BEX_PLAN` sets its workspace's plan) with the dashboard's dev server, in Chromium, instead of `dev-1` (a dashboard-only change):
  - DoD 1: Staging's ••• menu on Hobby showed `Block cross-environment connections` disabled with its Pro badge and `Upgrade to unlock this feature`, then `All settings`, which opened `/project/prj-local0001/environment/env-localstaging/settings` titled `storefront / Staging / Settings`, with the trail storefront › Staging and the sidebar's back link `Environment`. Production's toggle (isolated, Hobby) asked `Remove restrictions`, and `Yes, remove restrictions` toasted `Cross-environment connections allowed for Production.`
  - DoD 2: the Name card refused an empty name (`Environment name is required`) and `Production` (`Name is already used by another environment in this project`), and renamed Staging to `QA` (trail and title followed). Permissions saved Protected without asking and asked Render's `Remove restrictions` before Default. Connections showed Render's Pro gate on Hobby; Environment Groups listed Production's `production-shared` and Staging's `No environment groups found`; Networking showed the Scale gate's reset on Hobby and the editor on Scale. `…/environment/env-localproduction/anything` and `…/environment/evm-nope/settings` landed on the project; `…/environment/env-localproduction` stayed the project page.
  - DoD 3: deleting QA needed `sudo delete environment QA`, toasted `Deleted environment QA successfully.` and landed on Project Settings; Production, then the only environment, showed Render's refusal with its button disabled. The API outcome: Render's API was not probed (no Render API key in the run), so the API is unchanged and the probe is w1/181.
  - DoD 4: `environment-settings-dialog.tsx` and the section's Rename / Settings / Delete are gone; `rg EnvironmentSettingsDialog dashboard/src` finds nothing.
  - DoD 5: with the Environments read held 5 s (`LOCAL_BEX_HOLD_OPS=Environments`), the pending frame against the ready page, navigating straight and after a preload: the heading, the section navigation and every card's height and inner rows matched in all eight languages at 1440 and 390 px, on Hobby (the Connections gate and the Networking reset) and on Scale (the editor), 64 comparisons; the window itself never scrolled. The Environment Groups table previews one row, so an environment with none (`No environment groups found`) moves the cards under it by 21 px.
  - The Permissions, Connections and Networking cards and the menu's toggle by component tests for a non-Admin (Render shows them only to a member who may change them).
- **Production:** not yet — needs a pin that carries this milestone.
