# w1 · m686 — A new workspace's first resource lands in "My project" › "Production", as on Render

**Worker:** worker1 **Goal:** a workspace that has never created a resource puts its first one in a new "My project" › "Production", as Render's create forms do, over Render's `firstServiceCreationCompleted` query. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | GraphQL `firstServiceCreationCompleted(ownerId)` over a durable workspace fact | 1h30m | — |
| [t002](t002.md) | The create forms put a new workspace's first resource in "My project" › "Production" and hide the Project field, as Render's do | 1h30m | t001 |
| [t003](t003.md) | Re-disposition the ledger's `firstServiceCreationCompleted` row from n/a to matched | 30m | t002 |
| [t004](t004.md) | Walk a fresh workspace's first and second creates on dev-1 and production | 45m | t003 |
| [t005](t005.md) | Render parity | 20m | t004 |
| [t006](t006.md) | Simplify | 15m | t005 |
| [t007](t007.md) | Test coverage | 30m | t005 |
| [t008](t008.md) | Closeout | 10m | t006, t007 |

Total: **8 tasks, about 5h30m**.

## Definition of done

1. GraphQL `firstServiceCreationCompleted(ownerId)` answers false for a workspace that never created a service, Postgres or Key Value, and true afterwards, whatever surface created it and even after deletes; existing workspaces answer true.
2. In such a workspace with no projects, the first resource created from any of the seven create forms lands in project "My project", environment "Production", and the form hides its Project field; the second create creates no project.
3. The ledger row says matched with evidence, and ADR018 records the behavior; walked on production in a disposable workspace.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w3` (2026-11-26), filed to w1 by `/pm` the same day after re-reading Render's dashboard bundle. Render's public dashboard bundle, read 2026-11-26 without sign-in (entry `index-D5V4ZgbE.js`; chunk hashes change with every Render deploy, so find the code by the strings quoted here): `index` defines "My project" and "Production" and the hook that creates them when `firstServiceCreationCompleted === false` and the workspace has no projects; `NewServiceSourceCode`, `NewDatabase` and `NewRedis` call it, and the Key Value form hides its Project/Environment fields while it applies; `owner` holds the query. No flag gates it. render.com's docs do not describe it.
- **Reverses a disposition:** [w1/m645](../done/m645/README.md)'s ledger called this query a growth experiment (n/a). The bundle shows it is neither flagged nor limited to one form, and where a new customer's first resource lives is visible behavior, so parity applies; the user may drop this milestone to keep the n/a.
- **bex today (checked 2026-11-26):** no first-service fact and no default project; a new workspace's first resource has no project.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)).
- **Expected outcome:** a new workspace's first resource is grouped as on Render, so the project page (w1/m682) is where a newcomer finds it.
- **Why now:** independent of w1/m682–m685, and small; the ledger row is wrong today.
- **Render parity task:** included (t005): a GraphQL query and dashboard forms.
