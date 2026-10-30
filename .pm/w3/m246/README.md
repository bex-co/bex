# w3 · m246 — Workspaces get Render's freeform display name ("<Name>'s Workspace")

**Worker:** worker3 **Goal:** every workspace has a freeform display name, as on Render, shown wherever a workspace is named; a new account's workspace is called "<Name>'s Workspace" instead of its raw id, while the DNS-label slug keeps its current jobs **Status:** in progress — t001, t002 done; t003 surfaces next

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Record Render's workspace naming contract (owner object, dashboard, CLI) and decide the slug/display split — **DONE** | 30m | — |
| t002 | Store: `tenants.display_name` (freeform, not unique) + backfill; signup sets "<Name>'s Workspace" — **DONE** | 1h | t001 |
| t003 | REST/GraphQL/MCP: Render's owner `name` is the display name; the slug stays reachable as a bex extension | 1h | t002 |
| t004 | Dashboard: switcher, settings (rename) and pickers show the display name; create flow asks for a name | 1h | t003 |
| t005 | Render parity | 20m | t004 |
| t006 | Simplify | 15m | t004 |
| t007 | Test coverage | 30m | t004 |
| t008 | Closeout | 10m | t005, t006, t007 |

## Definition of done

- A new signup's workspace reads "<Name>'s Workspace" (email local part when the identity has no name) on REST (`GET /v1/owners`), GraphQL, MCP and the dashboard switcher; existing workspaces read their current name (no silent rename of a user-chosen slug).
- The display name is freeform and editable (Render's rules); the slug keeps its uniqueness and DNS-label rule wherever it is still used (create-time identifier, App/resource naming).
- Real-DB backend suite covers the migration/backfill and signup naming; dashboard tests cover rendering and rename; ADR018's recorded freeform-name drift (w6/m1/t007) is closed.

## Evidence (2026-10-30)

- `CreateTenantWithMember` (`lego/backend/internal/store/store.go`) inserts `VALUES ($1, $1, …)`: the signup workspace's name **is its id** (dev-3 first-run walk, `w3/done/m176/done/t005.md`: switcher shows `tea-dbi22f1jg4r76k3u33gg`; production uses the same path).
- `tenants.name` is `NOT NULL UNIQUE` (`migrations/0001_core.up.sql`) and `validWorkspaceName` holds it to a DNS label ("Unlike Render's freeform workspace names … recorded as parity drift (w6/m1/t007)"), so "Tian Pan's Workspace" cannot go into `name` — a separate display field is required.
- Render: the operator's own account shows "Tian Pan's Workspace" (2026-10-29).

## Source + Goal linkage

- **Source:** promoted from `w3/048` (2026-10-30), found by the `w3/m176` first-run walk.
- **Goal linkage:** Render parity first (ADR008): the first screen every new user sees.
- **Expected outcome:** new users see a human workspace name; Render clients reading `owner.name` get a Render-shaped value.
- **Why now:** every signup since open signup shipped sees its raw id as the workspace name.
- **Render parity included:** REST/GraphQL/MCP/UI change.
