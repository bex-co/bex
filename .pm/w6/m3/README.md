# w6 · m3 — Dashboard workspace UX: `/new/workspace` flow · switcher · settings

**Worker:** worker6 **Goal:** The human half of the lifecycle, Render-consistent: a workspace switcher dropdown at the top of the left pane (current workspace, list, **+ New Workspace**), a `/new/workspace` creation flow (name + plan cards for Hobby/Pro/Scale/Enterprise), and a workspace settings page with rename and a guarded delete — all over m1's GraphQL mutations, authenticated by the existing Kratos session. **Status:** todo (gated on w6/m1)

## Tasks (in order)

| id   | title                                                                                      | est | depends_on   |
| ---- | ------------------------------------------------------------------------------------------ | --- | ------------ |
| t001 | Workspace switcher dropdown + selected-workspace context scoping all queries               | 40m | w6/m1        |
| t002 | `/new/workspace` route: name + plan picker (Render plan cards), create → switch            | 35m | t001         |
| t003 | Workspace settings page: rename, plan display, workspace id/metadata                       | 30m | t001         |
| t004 | Delete workspace: danger zone with type-to-confirm guard matching captured Render UX       | 25m | t003         |
| t005 | Acceptance pass: create → switch → rename → delete e2e in a real browser                   | 25m | t002, t004   |
| t006 | Render parity — side-by-side vs live Render dashboard; flag drift                          | 20m | t005         |
| t007 | Simplify — `/simplify` over the code this milestone changed                                | 20m | t006         |
| t008 | Test coverage — meaningful tests for the behavior this milestone shipped                   | 30m | t006         |
| t009 | Closeout — move milestone to done when DoD holds                                           | 10m | t008         |

## Definition of done

In a real browser against a live cluster: a logged-in user opens the switcher, clicks **+ New Workspace**, names it, picks Hobby, lands in the new (empty) workspace; the switcher flips between workspaces and every page (services, databases, env vars, metrics) shows only the selected workspace's resources; rename in settings propagates to the switcher; delete requires typing the workspace name, then removes the workspace and its resources and lands the user in their remaining workspace; a 6th Hobby workspace attempt shows the limit error inline. No OAuth tokens in browser storage — Kratos session only.

## Source + Goal linkage

- **Source:** deep-research report [`w6/RESEARCH-workspaces.md`](../RESEARCH-workspaces.md) (findings 2–5) + `docs/render-artifacts/workspace-lifecycle.md` (w6/m1/t001 live capture); user request 2026-07-08 naming dashboard.render.com/new/workspace explicitly.
- **Goal linkage:** docs/vision.md pillar 1 (Render parity — dashboard); GOAL.md #5 (multi-tenant). Respects DO_NOT_DO: dashboard stays on Kratos sessions (no OAuth2-izing).
- **Expected outcome:** multi-workspace tenancy is operable by humans end-to-end without kubectl or API calls — the last missing piece between "tenants exist in Postgres" and "a user can run two isolated projects."
- **Why now:** m1's mutations otherwise ship with no consumer; w4/m12's Team page and w5's inbox items assume a workspace settings surface exists to hang off.
- **Render parity task included:** yes — UI feature work; compared side-by-side against the live Render dashboard as w3/m4.5 and w5/m7 did.
