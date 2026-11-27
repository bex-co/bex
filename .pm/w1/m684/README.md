# w1 · m684 — Each environment's New menu, and Render's environment-scoped create URLs

**Worker:** worker1 **Goal:** each environment section has Render's New menu, Render's environment create URLs open bex's forms preselected, and New Environment Group is Render's page. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Each environment section's New menu: "Add services to environment" and the table's trailing New row, with Render's items and gates | 1h30m | w1/m682 t005 |
| [t002](t002.md) | Render's environment-scoped create URLs open the create forms with the project and environment chosen | 1h15m | — |
| [t003](t003.md) | New Environment Group is Render's page at `/new/env-group` and `/project/<id>/environment/<envId>/new/env-group` | 2h | t002 |
| [t004](t004.md) | One Generate Blueprint item in the New menu, shared with w3/m681 t003 | 15m | t001 |
| [t005](t005.md) | Walk the New menu and the create URLs on dev-1 and production | 45m | t003, t004 |
| [t006](t006.md) | Render parity | 20m | t005 |
| [t007](t007.md) | Simplify | 15m | t006 |
| [t008](t008.md) | Test coverage | 30m | t006 |
| [t009](t009.md) | Closeout | 10m | t007, t008 |

Total: **9 tasks, about 7h**.

## Definition of done

1. Each environment section has "Add services to environment" and a trailing New row opening Render's menu: Create new (every resource type bex hosts), Env Group, Move existing services and Generate Blueprint, with Render's protected and plan gates.
2. `/project/<id>/environment/<envId>/new`, `…/new/<type>` and `…/<type>/new` open the create forms with the project and environment chosen; `?projectId&environmentId` still works.
3. `/new/env-group` and `…/environment/<envId>/new/env-group` are Render's New Environment Group page; the list's New button opens it.
4. Walked on production at 1440 and 390 px.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w3` (2026-11-26), filed to w1 by `/pm` the same day after re-reading Render's dashboard bundle. Render's public dashboard bundle, read 2026-11-26 without sign-in (entry `index-D5V4ZgbE.js`; chunk hashes change with every Render deploy, so find the code by the strings quoted here): the `Project` chunk's New menu ("Add services to environment", "Create new", "Env Group", "Move existing services", "Generate Blueprint", `source: "project environment menu"`), `index`'s five `/project/:projId/environment/:envId/…` create routes and `/new/env-group`, and the `NewEnvGroup` chunk ("New Environment Group", "Create Environment Group").
- **bex today (checked 2026-11-26):** one New Service link per environment; Render's environment create URLs and `/new/env-group` answer NotFound; env groups are created in a dialog.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)).
- **Expected outcome:** every resource type, and env groups, start from the environment they will live in, at Render's URLs.
- **Why now:** w1/m682's section has the slot; w3/m681 t003 is about to add Generate Blueprint to this same menu, so building the menu now keeps one menu instead of two.
- **Render parity task:** included (t006): dashboard UI.
