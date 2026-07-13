# w6 · m16 — Projects & environments: basic grouping (model + verbs + API)

**Worker:** worker6 **Goal:** A workspace can group its services into a `Project` and assign them to named `Environment`s (e.g. staging/production), readable and filterable over REST, GraphQL, and MCP — the MVP grouping half of Render's Projects & Environments feature, scoped to model + verbs + API only (no protected-environment ACLs, no dashboard UX, mirroring how `w6` itself phased workspaces: model+verbs → API → dashboard). **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                    | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Design: `Project` (groups N services within a workspace) + `Environment` (a named subset of a project's services) — control-plane Postgres tables, mirroring the workspace-lifecycle shape (`w6/m1`) | 1h  | —          |
| t002 | Store + migration: `projects`, `environments`, `service_environment` join — tenant-scoped like every other control-plane table            | 45m | t001       |
| t003 | Core verbs: create/rename/delete a project; create/rename/delete an environment; assign/unassign a service to an environment              | 1h  | t002       |
| t004 | REST/GraphQL: CRUD surface mirroring Render's `/projects`/`/environments` shape (api-docs.render.com)                                      | 1h  | t003       |
| t005 | MCP: read/list tools (mirroring the workspace tools' read-only-on-MCP precedent, `w6/m2`)                                                  | 30m | t003       |
| t006 | Live verification: group three existing services into a project with two environments, list/filter by project+environment across all three surfaces | 30m | t004, t005 |
| t007 | Docs: `docs/ADR032-projects-environments.md` recording the MVP scope and what's deferred (protected environments, dashboard UX)             | 20m | t006       |

## Definition of done

A workspace can group services into a project and assign them to named environments over REST/GraphQL/MCP; a service's project/environment membership is readable and filterable on all three surfaces, verified live.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones to work on` 2026-07-13; `docs/ADR018-render-parity.md`'s "Projects & environments (grouping)" row (currently ✖ on all four surfaces, previously noted "Belongs to the tenancy line → nearest w1/m9; low").
- **Goal linkage:** pillar 1 (Render parity) — closes the grouping row; assigned to `w6` (not `w1`, its original proposal target) because grouping resources *within a workspace* is exactly `w6`'s founding charter ("w6 makes workspaces a real product surface") and `w6` composes directly with the existing workspace/control-plane store this needs, and per user direction was routed to the workstream with the fewest open milestones.
- **Expected outcome:** `docs/ADR018-render-parity.md`'s Projects & environments row moves off ✖ for REST/GraphQL/MCP (UI stays a deliberate follow-on, not this milestone's scope).
- **Why now:** every prerequisite (workspaces `w6/m1`, multi-service creation, control-plane store) already exists; previously filed "low" only for lack of urgency, not lack of buildability — user direction 2026-07-13 to keep closing remaining parity gaps makes this next in line.
- **Render parity closing task: included.** **Dashboard UX and protected-environment ACL gating are explicitly out of scope** — file as a `w5` follow-on once this backend lands, keeping this milestone testable and bounded rather than open-ended.
