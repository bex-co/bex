# w1 · m31 — Projects: group services within a workspace

**Worker:** worker1 **Goal:** A workspace can create a named project, assign existing services to it, and see them grouped together on the services list across REST/GraphQL/MCP/dashboard — matching Render's project-grouping behavior. **Status:** DONE (all tasks complete, live verified 2026-07-13)

## Tasks (in order)

| id   | title                                                                                                                                | est | depends_on | status |
| ---- | ---------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- | ------ |
| t001 | Data model: `projects` table (workspace-scoped: id, name, created-by/at) + a nullable `project_id` column on the apps/services row; mint the id through `lego/backend/internal/id` | 45m | —          | — **DONE** |
| t002 | REST: CRUD `/v1/projects` + assign/unassign a service to a project                                                                   | 40m | t001       | — **DONE** |
| t003 | GraphQL: mirror t002 (`projects`/`project` queries, `createProject`/`deleteProject`/`assignServiceToProject` mutations)              | 30m | t002       | — **DONE** |
| t004 | MCP: `list_/create_/delete_project` + assign tool                                                                                    | 25m | t002       | — **DONE** |
| t005 | Dashboard: services list groups by project (Render's dashboard-organization shape — ungrouped services still show flat), a lightweight project-create/rename/delete flow | 1h  | t003, t004 | — **DONE** |
| t006 | Live verification: create a project, assign two real services to it, confirm both REST and dashboard reflect the grouping consistently | 30m | t005       | — **DONE** |

## Definition of done

A workspace can create a named project, assign existing services to it, and see them grouped together on the services list across REST/GraphQL/MCP/dashboard — matching Render's project-grouping behavior, verified live.

## Source + Goal linkage

- **Source:** verified live via search 2026-07-13 (render.com/docs/projects, render.com/blog/projects) — a real Render capability, not previously in the parity ledger's tracked-gap table (only noted as "belongs to the tenancy line... low").
- **Goal linkage:** Render-parity core surface — organizational/dashboard-IA parity.
- **Expected outcome:** parity ledger gains a tracked "Projects" row, flipped to `✅` (or `◐` if a real divergence surfaces).
- **Why now:** clean, bounded scope (grouping only); no DO_NOT_DO conflict; **Environments (multi-environment promotion/deploy-target semantics) is deliberately out of scope** — a much larger architectural change that would need its own milestone and design discussion before sizing, not bundled in here. Render also gates this to Team-plan-or-higher; bex has no billing/plan-gating system, so it ships ungated (same divergence pattern as `w8/m7`'s pricing work).
