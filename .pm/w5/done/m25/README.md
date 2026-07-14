# w5 · m25 — Environments dashboard UX

**Worker:** worker5 **Goal:** A dashboard user can see a Project's services grouped into named Environments (e.g. staging/production), and create/rename/delete an Environment and assign/remove services from it — all without touching the API/MCP directly. **Status:** done (2026-07-13)

## Tasks (in order)

| id   | title                                                                                           | est | depends_on | |
| ---- | ------------------------------------------------------------------------------------------------ | --- | ---------- |---|
| t001 | Data layer: TanStack Query hooks over `environments`/`environment`/`create`/`rename`/`delete`/`setEnvironmentServices` (mirror `use-projects.ts`) | 40m | —          | — **DONE** |
| t002 | Environment grouping/switcher inside a Project's unified page (reuse `resource-table.tsx`)       | 45m | t001       | — **DONE** |
| t003 | Create/rename/delete Environment dialogs + "assign service" action (mirror `new-project-dialog.tsx`) | 40m | t001       | — **DONE** |
| t004 | Empty/loading/error states + `en`/`zh` locale strings                                            | 30m | t002, t003 | — **DONE** |
| t005 | Live verification: create → assign → rename → delete against the mock cluster, confirm project auto-join | 30m | t004       | — **DONE** |
| t006 | Render parity — full-surface check (REST/GraphQL/MCP already ship this; confirm the new UI matches their contract, no drift) | 20m | t005       | — **DONE** |
| t007 | Simplify — `/simplify` over the code this milestone changed                                      | 20m | t006       | — **DONE** |
| t008 | Test coverage — meaningful tests for the behavior this milestone shipped                         | 30m | t006       | — **DONE** |
| t009 | Closeout — DoD met → move milestone to `done/`                                                   | 10m | t008       | — **DONE** |

## Definition of done

A user can view services grouped into named Environments within a Project, create/rename/delete an Environment, and assign/remove services from it — all from the dashboard — with assignment auto-joining the parent Project per ADR032; verified live (or against the mock cluster).

## Source + Goal linkage

- **Source:** `docs/ADR018-render-parity.md` "Projects & environments" row (UI ◐: "Environments has no dashboard UX yet"); backend fully shipped `w1/m32`. Proposed via `/pm-brainstorm more milestones to work on` 2026-07-13.
- **Goal linkage:** closes the last ◐ cell in the parity ledger's Projects & Environments row — the environments backend (`w1/m32`) is otherwise inert without a UI.
- **Expected outcome:** users can actually use environments (grouping, create/rename/delete, service assignment) without going through the API/MCP tools directly.
- **Why now:** the only remaining Render-relevant ✖/◐ item in the entire parity ledger without an owning milestone; everything else is either done or an explicit non-goal.
- **Render parity closing task: included** — the milestone adds a dashboard UI over an existing REST/GraphQL/MCP contract (`w1/m32`).

## Closeout notes (2026-07-13)

- **Shipped:** `dashboard/src/features/environments/` — `useEnvironments` + create/rename/delete/set-services hooks, and an `EnvironmentsPanel` mounted on `routes/project.$projectId.index.tsx` (below the project header, above a now-labelled "All resources" table). Each environment is a card of its assigned services (reusing `features/projects/components/resource-table.tsx`) with rename/delete and a "Manage services" checkbox dialog whose full-replace assignment auto-joins the parent project. GraphQL documents hand-spliced into `graphql/definitions.ts` (Projects-style; `definitions.ts` is hand-maintained). `en`/`zh` locales added and registered in `i18n/index.ts`.
- **Simplify (t007):** the four mutation hooks use Apollo `refetchQueries` (`["Environments"]`, plus `["Projects"]` for assignment) instead of a refetch callback prop-drill — matching `features/services/hooks/use-trigger-deploy.ts`; the shared `toServiceRow` helper (now exported from `use-grouped-resources.ts`) is reused for the card rows.
- **Verification (t005/t006):** the five hand-written GraphQL documents were validated against the **real backend schema** dumped from `lego/backend` (`buildClientSchema` + `validate`, 0 errors) — proving no contract drift; the full create → assign → rename → delete lifecycle **and the auto-join into the parent project** were exercised via curl against the local-bex stub (extended this milestone to speak the environments verbs); 14 unit tests cover the hook mapping, the assign/remove full-replace, the panel states, and the card rename/delete. The in-browser click-through was not run because the local `yarn dev:local` port (:8099) was occupied by a real `bex-api` instance and the dashboard `.env` pins that port — not worth killing the user's process; `yarn dev:local` will drive it once the port is free.
