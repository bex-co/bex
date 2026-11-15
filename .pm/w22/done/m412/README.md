# w22 · m412 — A sandbox group can be placed in a project environment, as in Render's dashboard

**Worker:** worker22 **Goal:** the "+ New > Sandbox Group" dialog and the create API let a group be placed in a project environment, as render.com's dashboard does, and the group reads back its `environmentId`. **Status:** done (2026-11-15)

## Tasks (in order)

| id | title | est | depends_on |
| ---- | ------------------------------------------------------------------------ | ----- | ------------------------------ |
| t001 | Store: sandbox_groups.environment_id — **DONE** | 40m | — |
| t002 | Service and surfaces: environmentId on create, echoed everywhere — **DONE** | 1h30m | t001 |
| t003 | Dashboard: an optional Environment select in the New Sandbox Group dialog — **DONE** | 1h15m | t002 |
| t004 | Render parity — **DONE** | 30m | t003 |
| t005 | Simplify — **DONE** | 20m | t004 |
| t006 | Test coverage — **DONE** | 30m | t004 |
| t007 | Closeout — **DONE** | 10m | t005, t006 |

## Definition of done

- Creating a group with an `environmentId` of the workspace returns it on REST, GraphQL and MCP, on create, get and list; another workspace's environment id answers the not-found of an id that does not exist; a protected environment needs an Admin; deleting the environment keeps the group and clears the field
- The New Sandbox Group dialog has an optional Environment select that appears when the workspace has environments, with a loading state that matches its ready state at desktop and narrow widths
- An official Render client's create (no `environmentId`) behaves as before; ADR018 and ADR104 no longer call the picker a drift

## Source + Goal linkage

- **Source:** inbox note w22/003 (promoted 2026-11-14 by `/loopx w22`), whose 2026-11-14 record decided the semantics from Render's documentation: [render.com/docs/sandboxes-creating](https://render.com/docs/sandboxes-creating) says only "In the Dashboard, you can optionally place a group in a project environment", so a group belongs to an environment and nothing more. Found by w22/m311 t012 as the one Render dashboard drift.
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)); pillar 5 sandboxes (re-opened 2026-07-27, `.pm/DO_NOT_DO.md` #18).
- **Expected outcome:** a customer who files sandbox groups under projects on Render can do the same on bex, and a group shows its environment wherever it is read.
- **Why now:** w22/m311 closed on 2026-11-14 with this as the only dashboard gap, and the production dialog (walked the same day) has just the Name field.
- **Render parity:** included (REST, GraphQL, MCP and the dashboard change). Stricter variants (access control by environment) are separate items.

## Outcome (2026-11-15)

- **Shipped:** `sandbox_groups.environment_id` (migration 0270, `ON DELETE SET NULL`, a partial index), the optional `environmentId` on the create of REST (a bex extension of the Render body, accepted by the gate on `create-sandbox-group` only), GraphQL and MCP, placement through the resolver every other resource kind uses (another workspace's environment answers the same 404 as an unknown id; a protected environment needs an Admin), `environmentId` echoed on create, get, list and update, and the optional Environment select in "+ New > Sandbox Group" (held until the environments are read, with a loading state that matches its ready state at desktop and narrow width; measured at 60 px in both).
- **Checks:** gofmt, vet and golangci-lint clean; the real-dependency tests of the sandbox, store, api and cmd packages pass; dashboard `yarn lint` and `yarn test` (8444 tests) pass; 13 surface-parity scenarios across REST, GraphQL and MCP; mutation checks on every key behavior (the resolver call, the foreign-workspace arm, the echoed id, the gate extension, the foreign key's `SET NULL`, and nine dashboard mutations), each caught and reverted.
- **Review (three lenses, one reviewer):** two redundant steps in `placeGroup`, three narrating comments and a dead dashboard condition were removed; the rest was recorded as skipped (cosmetics, a migration-test lock window that matches the other migration tests).
- **Recorded as unknown, not enforced:** deleting or editing a group in a protected environment (Render documents none of it); a stricter rule is a separate item. ADR018, ADR042 § Sandbox groups, ADR104 D7 and `render-artifacts/sandbox-groups.md` carry it.
- **Not done:** a production walk of the new select (the dialog was walked on production on 2026-11-14 before this change), and `testdata/mcp_tools.golden.json`, which t002 named but which exists only for agent sessions.

