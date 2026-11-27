# w1 · m683 — An environment's settings are Render's page at `/project/<id>/environment/<envId>/settings`, opened from the section's ••• menu

**Worker:** worker1 **Goal:** each environment has Render's settings page and ••• menu, and bex's settings dialog is retired. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | The route `/project/<id>/environment/<envId>/settings`, with Render's title and breadcrumbs | 1h | — |
| [t002](t002.md) | The Name card | 45m | t001 |
| [t003](t003.md) | The Permissions card: Default or Protected | 1h | t001 |
| [t004](t004.md) | The Connections card, its confirm modal and toast | 1h | t001 |
| [t005](t005.md) | The Environment Groups card and the Networking card | 1h | t001 |
| [t006](t006.md) | The Delete Environment card, with the last-environment refusal | 1h | t001 |
| [t007](t007.md) | The section's ••• menu: Block cross-environment connections and All settings; retire the settings dialog | 45m | t004, w1/m682 t005 |
| [t008](t008.md) | The settings page's pending frame, and a walk on dev-1 and production | 45m | t002, t003, t005, t006, t007 |
| [t009](t009.md) | Render parity | 20m | t008 |
| [t010](t010.md) | Simplify | 15m | t009 |
| [t011](t011.md) | Test coverage | 45m | t009 |
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
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)).
- **Expected outcome:** the path Render's docs describe works on bex, and a Render URL to an environment's settings opens the same page.
- **Why now:** w1/m682 adds the section this menu sits on; the dialog's controls (protection, isolation, IP rules) already exist and only change place and words, so the gap is the page and the menu.
- **Render parity task:** included (t009): dashboard UI, and possibly a coded last-environment refusal on REST/GraphQL/MCP.
