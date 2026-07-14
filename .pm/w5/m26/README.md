# w5 · m26 — Workspace-level Env Groups page

**Worker:** worker5 **Goal:** A user can create and manage environment groups as first-class **workspace** resources — view all groups, create/rename/delete, edit their env vars & secret files, and link/unlink services — without opening any individual service's Environment tab. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                                                                             | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Data layer: promote `use-env-groups.ts` out of service scope into a `features/env-groups/` feature — hooks over `envGroups`/`envGroup` + `createEnvGroup`/`deleteEnvGroup` + `setEnvGroupVars`/`setEnvGroupSecretFile`/`deleteEnvGroupSecretFile` + `linkEnvGroup`/`unlinkEnvGroup` | 40m | —          |
| t002 | Env Groups list page + route (`/env-groups`) + sidebar nav entry (`dashboard-sidebar.tsx`): groups with var/file + linked-service counts, empty state, "New Env Group" action                    | 45m | t001       |
| t003 | Env Group detail page + route (`/env-groups/$groupId`): env-vars + secret-files editors (**reuse** the Environment tab's `env-groups-panel.tsx` editors), rename/delete, linked-services list     | 40m | t001       |
| t004 | Create-group dialog (mirror `new-project-dialog.tsx`) + link/unlink-service action from the detail page                                                                                          | 40m | t001       |
| t005 | Empty/loading/error states + `en`/`zh` locale strings                                                                                                                                            | 30m | t002, t003 |
| t006 | Live verification against the mock cluster: create → add vars + secret file → link to a service → confirm the service's Environment tab reflects it → unlink → delete                             | 30m | t004, t005 |
| t007 | Render parity — full-surface check (REST/GraphQL/MCP already ship env-group CRUD + link/unlink; confirm the new workspace-level UI matches their contract + Render's Env Groups page, flag drift) | 20m | t006       |
| t008 | Simplify — `/simplify` over the code this milestone changed                                                                                                                                      | 20m | t007       |
| t009 | Test coverage — meaningful tests for the behavior this milestone shipped                                                                                                                         | 30m | t007       |
| t010 | Closeout — DoD met → move milestone to `done/`                                                                                                                                                   | 10m | t009       |

## Definition of done

From a workspace-level Env Groups page, a user can list all env groups, create/rename/delete a group, edit its env vars and secret files, and link/unlink services — with no service Environment tab involved — verified against the mock cluster. A group created and left unlinked to any service is still fully manageable.

## Source + Goal linkage

- **Source:** `docs/ADR018-render-parity.md` "Environment groups" row (UI ✅ but _service-scoped only_: "dashboard Environment tab Env-Groups section"). Render exposes env groups as a dedicated workspace-level page (create group → add vars/files → link services). Proposed via `/pm-brainstorm more tasks for w5 to achieve feature parity` 2026-07-13.
- **Goal linkage:** closes the IA divergence in the Environment-groups parity cell — the `internal/envgroups` backend (full CRUD + link/unlink across REST/GraphQL/MCP) has no workspace-level home in the dashboard.
- **Expected outcome:** users manage env groups as workspace resources (Render's model), including groups not yet linked to any service.
- **Why now:** backend fully shipped and inert at the workspace level; one of only two genuine w5 parity gaps left after m25.
- **Render parity closing task: included** — the milestone adds a workspace-level dashboard UI over an existing REST/GraphQL/MCP contract (`internal/envgroups`).
