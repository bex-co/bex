# w5 · m25 — Environments dashboard UX

**Worker:** worker5 **Goal:** A dashboard user can see a Project's services grouped into named Environments (e.g. staging/production), and create/rename/delete an Environment and assign/remove services from it — all without touching the API/MCP directly. **Status:** todo

## Tasks (in order)

| id   | title                                                                                           | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Data layer: TanStack Query hooks over `environments`/`environment`/`create`/`rename`/`delete`/`setEnvironmentServices` (mirror `use-projects.ts`) | 40m | —          |
| t002 | Environment grouping/switcher inside a Project's unified page (reuse `resource-table.tsx`)       | 45m | t001       |
| t003 | Create/rename/delete Environment dialogs + "assign service" action (mirror `new-project-dialog.tsx`) | 40m | t001       |
| t004 | Empty/loading/error states + `en`/`zh` locale strings                                            | 30m | t002, t003 |
| t005 | Live verification: create → assign → rename → delete against the mock cluster, confirm project auto-join | 30m | t004       |
| t006 | Render parity — full-surface check (REST/GraphQL/MCP already ship this; confirm the new UI matches their contract, no drift) | 20m | t005       |
| t007 | Simplify — `/simplify` over the code this milestone changed                                      | 20m | t006       |
| t008 | Test coverage — meaningful tests for the behavior this milestone shipped                         | 30m | t006       |
| t009 | Closeout — DoD met → move milestone to `done/`                                                   | 10m | t008       |

## Definition of done

A user can view services grouped into named Environments within a Project, create/rename/delete an Environment, and assign/remove services from it — all from the dashboard — with assignment auto-joining the parent Project per ADR032; verified live (or against the mock cluster).

## Source + Goal linkage

- **Source:** `docs/ADR018-render-parity.md` "Projects & environments" row (UI ◐: "Environments has no dashboard UX yet"); backend fully shipped `w1/m32`. Proposed via `/pm-brainstorm more milestones to work on` 2026-07-13.
- **Goal linkage:** closes the last ◐ cell in the parity ledger's Projects & Environments row — the environments backend (`w1/m32`) is otherwise inert without a UI.
- **Expected outcome:** users can actually use environments (grouping, create/rename/delete, service assignment) without going through the API/MCP tools directly.
- **Why now:** the only remaining Render-relevant ✖/◐ item in the entire parity ledger without an owning milestone; everything else is either done or an explicit non-goal.
- **Render parity closing task: included** — the milestone adds a dashboard UI over an existing REST/GraphQL/MCP contract (`w1/m32`).
