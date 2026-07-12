# w6 · m2 — Render `owners` read API + MCP workspace tools

**Worker:** worker6 **Goal:** Agents and API users see workspaces exactly as Render exposes them: `GET /v1/owners` (list, name/email filters), `GET /v1/owners/{ownerId}` (with user-ID→default-workspace behavior), `GET /v1/owners/{ownerId}/members` — read-only, `tea-`/`own-` ID semantics, `ownerId` scoping on every resource — plus the official MCP trio `list_workspaces` / `select_workspace` / `get_selected_workspace` with stateful per-session selection. **Status:** done

## Tasks (in order)

| id   | title                                                                                           | est | depends_on        |
| ---- | ------------------------------------------------------------------------------------------------ | --- | ------------------ |
| t001 | Pin the owner object schema against Render's OpenAPI spec (fields, `own-` prefix)                | 20m | —                  | — **DONE** |
| t002 | REST `GET /v1/owners` + `GET /v1/owners/{ownerId}` (filters, user-ID→default-workspace)          | 30m | t001, w6/m1        | — **DONE** |
| t003 | REST `GET /v1/owners/{ownerId}/members` (Render-shaped, roles mapped from FGA)                   | 25m | t002               | — **DONE** |
| t004 | `ownerId` scoping parity: field on every resource object + `ownerId` filter on list endpoints    | 25m | t002               | — **DONE** |
| t005 | MCP tools: `list_workspaces` / `select_workspace` / `get_selected_workspace` (session-scoped)    | 30m | t002               | — **DONE** |
| t006 | Render parity — field-for-field diff of owners/members/MCP responses vs Render's docs            | 20m | t003, t004, t005   | — **DONE** |
| t007 | Simplify — `/simplify` over the code this milestone changed                                      | 20m | t006               | — **DONE** |
| t008 | Test coverage — meaningful tests for the behavior this milestone shipped                         | 30m | t006               | — **DONE** |
| t009 | Closeout — move milestone to done when DoD holds                                                 | 10m | t008               | — **DONE** |

## Definition of done

With two workspaces and an API key: `GET /v1/owners` returns both, Render-shaped (cursor list of `{owner}` objects, `tea-` ids), filterable by name/email; `GET /v1/owners/{tea-…}` returns one; passing the caller's user id (`own-…`) returns their default workspace instead of erroring; `/members` lists members with roles; services/databases list responses carry `ownerId` and accept an `ownerId` filter; over MCP, an agent calls `list_workspaces`, `select_workspace(ownerID)`, and subsequent tool calls (list services) are scoped to the selection, `get_selected_workspace` echoing it. No POST/PATCH/DELETE exists under `/v1/owners`.

## Source + Goal linkage

- **Source:** deep-research report [`w6/RESEARCH-workspaces.md`](../RESEARCH-workspaces.md) (findings 7–10 + MCP tool shapes; open questions 2–3); supersedes inbox note `w2/002.md` (workspace MCP tools, which was blocked on real workspaces).
- **Goal linkage:** docs/ADR008-vision.md pillar 1 (Render-compatible API) and pillar 3 (MCP/agent-native); GOAL.md #5 (multi-tenant).
- **Expected outcome:** Render-targeting clients and agents (including Render's own MCP client conventions) can enumerate and scope to bex workspaces without translation; `ownerId` appears wherever Render puts it.
- **Why now:** m1's verbs make multiple workspaces exist; without list/select surfaces, agents and API users are trapped in their default workspace — and w2's deploy-from-chat milestones consume `select_workspace`.
- **Render parity task included:** yes — this milestone is precisely an API-compatibility surface.
