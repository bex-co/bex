# w22 · m412 — A sandbox group can be placed in a project environment, as in Render's dashboard

**Worker:** worker22 **Goal:** the "+ New > Sandbox Group" dialog and the create API let a group be placed in a project environment, as render.com's dashboard does, and the group reads back its `environmentId`. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| ---- | ------------------------------------------------------------------------ | ----- | ------------------------------ |
| t001 | Store: sandbox_groups.environment_id | 40m | — |
| t002 | Service and surfaces: environmentId on create, echoed everywhere | 1h30m | t001 |
| t003 | Dashboard: an optional Environment select in the New Sandbox Group dialog | 1h15m | t002 |
| t004 | Render parity | 30m | t003 |
| t005 | Simplify | 20m | t004 |
| t006 | Test coverage | 30m | t004 |
| t007 | Closeout | 10m | t005, t006 |

## Definition of done

- Creating a group with an `environmentId` of the workspace returns it on REST, GraphQL and MCP, on create, get and list; another workspace's environment id answers the not-found of an id that does not exist; a protected environment needs an Admin; deleting the environment keeps the group and clears the field
- The New Sandbox Group dialog has an optional Environment select that appears when the workspace has environments, with a loading state that matches its ready state at desktop and narrow widths
- An official Render client's create (no `environmentId`) behaves as before; ADR018 and ADR104 no longer call the picker a drift

## Source + Goal linkage

- **Source:** inbox note w22/003 (promoted 2026-11-14 by `/loopx w22`), whose 2026-11-14 record decided the semantics from Render's documentation: [render.com/docs/sandboxes-creating](https://render.com/docs/sandboxes-creating) says only "In the Dashboard, you can optionally place a group in a project environment", so a group belongs to an environment and nothing more. Found by w22/m311 t012 as the one Render dashboard drift.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)); pillar 5 sandboxes (re-opened 2026-07-27, `.pm/DO_NOT_DO.md` #18).
- **Expected outcome:** a customer who files sandbox groups under projects on Render can do the same on bex, and a group shows its environment wherever it is read.
- **Why now:** w22/m311 closed on 2026-11-14 with this as the only dashboard gap, and the production dialog (walked the same day) has just the Name field.
- **Render parity:** included (REST, GraphQL, MCP and the dashboard change). Stricter variants (access control by environment) are separate items.
