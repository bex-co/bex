# w4 · m169 — Blueprint by-id verbs resolve the Blueprint's own workspace

**Worker:** worker4 **Goal:** a Blueprint id works like a service or env-group id: every by-id verb finds it in whichever of the caller's workspaces owns it, with no `ownerId` needed **Status:** todo

## Tasks (in order)

| id   | title                                                          | est | depends_on                 |
| ---- | -------------------------------------------------------------- | --- | -------------------------- |
| t001 | Resolve the owning workspace from the Blueprint id             | 1h  | —                          |
| t002 | Dashboard Blueprint deep links across workspaces               | 30m | w4/m169/t001               |
| t003 | Render parity across Blueprint by-id surfaces                  | 30m | w4/m169/t001, w4/m169/t002 |
| t004 | Simplify changed code                                          | 20m | w4/m169/t003               |
| t005 | Test coverage for shipped behavior                             | 45m | w4/m169/t003               |
| t006 | Closeout                                                       | 15m | w4/m169/t005               |

## Definition of done

Probes run from the dashboard origin (page `fetch`, `credentials:'include'`). The signed-in QA user belongs to `bex` (default workspace), `tian-personal` and `bex-canary`; Blueprint `blp-db136288mmqc73d4hpug` belongs to `bex-canary`:

- REST `GET /v1/blueprints/blp-db136288mmqc73d4hpug` with **no** `ownerId` returns 200 and the Blueprint (today: 404 `blueprint not found`; with `?ownerId=tea-daif693dqjvc73e7as3g` it is 200). The same holds for `GET /v1/blueprints/<id>/syncs` (today 404 vs 200 `[]`), and for `PATCH`, `DELETE` and `POST /sync` on an owned `qa-` fixture Blueprint.
- GraphQL `blueprint(id:)` without `ownerId` returns the Blueprint (today: "blueprint not found"), and the MCP blueprint read tool agrees.
- Control, kept: `GET /v1/services/srv-daif6dsmg29s73d1umvg` and `GET /v1/env-groups/<id>` without `ownerId` already resolve across the caller's workspaces, and still do.
- Adjacent classes: a Blueprint id in a workspace the caller is **not** a member of returns the same 404 as a nonexistent id, not a 403 (no existence oracle). A viewer-role member still gets the manifest gated by `can_view_sensitive` (round-7 #11 unchanged).
- Dashboard: with `tian-personal` selected, opening `/blueprints/blp-db136288mmqc73d4hpug` shows that Blueprint, or an explicit "belongs to bex-canary — switch?" state (target behavior chosen in t002). Today it silently redirects to `/`; a bex-canary service URL opens normally.

## Source + Goal linkage

- **Source:** infinite `/qa-find-bugs` loop7 2026-10-04 UTC (muse.env, `bex-canary`), journey 13. The workspace was switched to `tian-personal` for the deep-link check, then restored. Probes as listed in the DoD. Root cause:
  - `lego/backend/internal/apps/blueprint.go:1052-1073` `GetBlueprintByID` scopes the store read to `resolveTenantID(ctx)` (`blueprint.go:1642-1655`), which is the request's `ownerId` or else the caller's **default** workspace. It never looks up which workspace owns the id.
  - The REST routes forward an optional `?ownerId` (`internal/apps/rest.go:1278-1300`, `1392`).
  - Env groups do it right: `envgroups.fetchGroup` (`internal/envgroups/service.go:1667-1679`) loads the group by id, then `AuthorizeLabeled` on the group's own workspace.
- **Blast radius:** the five shared verbs `GetBlueprintByID`, `ListBlueprintSyncs`, `UpdateBlueprint`, `DisconnectBlueprint` and `SyncBlueprint` (enumerated by `w6/m96` t004), each served on REST, GraphQL and MCP.
- **Goal linkage:** ADR006/ADR018 Render-compatible API. Render's `GET /blueprints/{blueprintId}` takes no owner parameter, and the Render CLI/API keys span every workspace a user belongs to.
- **Expected outcome:** an API, CLI or MCP caller with several workspaces can read and operate their Blueprints by id, and shared or bookmarked Blueprint links work like service links.
- **Why now:** today a multi-workspace user's own Blueprint is "not found" on every by-id call unless they already know its workspace id. That's a silent 404 the dashboard turns into an unexplained redirect. Render parity task INCLUDED (t003): REST/GraphQL/MCP/UI surfaces change.
- **Unverified:** whether the pinned Render CLI calls these endpoints without an owner (not probed); MCP tools (same service functions).
- **Severity:** major.
