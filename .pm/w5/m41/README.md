# w5 · m41 — Populated project overview parity

**Worker:** worker5 **Goal:** A populated bex Project has the same fast operating surface as Render: one selected Environment, contextual creation, truthful Runtime/Region/Updated facts, and bulk Move. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Pin the populated-project contract and exact bulk-Move semantics | 30m | — |
| t002 | Project list data: runtime, region, and updatedAt across all resource kinds | 45m | t001 |
| t003 | Selected-Environment project layout without stacked/duplicate resource tables | 45m | t001 |
| t004 | Contextual New Service with Project/Environment preselection | 40m | t003 |
| t005 | Render-shaped resource metadata columns with honest missing-value states | 45m | t002, t003 |
| t006 | Row selection and bulk Move for the selected Environment | 60m | t001, t003 |
| t007 | Render parity: live populated-project re-walk and ledger/evidence refresh | 30m | t004-t006 |
| t008 | Simplify: behavior-preserving pass over the project resource flow | 20m | t007 |
| t009 | Test coverage: selection, URL state, metadata, create context, and bulk Move | 45m | t007 |
| t010 | Closeout — move to `done/` when the Definition of Done holds | 15m | t008, t009 |

## Definition of done

Against a populated dev project, the dashboard shows one URL-owned Environment at a time; search and existing type filters operate on that Environment and show accurate counts; the table carries truthful Runtime, Region, and Updated facts for services, Postgres, and Key Value; New Service opens with the current Project/Environment selected; and selecting one or more rows exposes a working Move flow with an accurate selected count. Project/environment management, unassigned resources, empty states, per-row actions, and refresh/share behavior remain available. A live side-by-side re-walk against Render records the result.

## Source + Goal linkage

- **Source:** user-requested authenticated comparison on 2026-07-17 between a live bex Project and a live Render Project. Render's populated `Production` view showed `All (8) · Services (8) · Env Groups (0)`, eight rows, `Name · Status · Runtime · Region · Updated`, a contextual **New service**, row checkboxes, and `0 services selected: Move`. bex showed uncounted type filters on a `prod` Environment card plus a separate **All resources** table, `Name · Type · Status · Created`, no project-context create action, and no selection toolbar. The bex fixture happened to be empty, so the repository implementation was also inspected to distinguish data absence from missing controls.
- **Already shipped; do not duplicate:** inline project rename and contextual Overview/Manage/Settings navigation; Add/New Environment and environment CRUD/settings; Manage resources; per-row lifecycle/Options menus and Move-to-project; URL-owned project resource search plus All/Services/Databases/Key Values/Env Groups filters (`w5/m25`, `w5/m31`, `w5/m36`).
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) pillar 1, Render parity, and [ADR032](../../../docs/ADR032-environments.md), making Projects/Environments operable at real resource counts rather than only feature-complete in isolation.
- **Expected outcome:** a user can scan and reorganize a busy project from one table, see the operational facts Render puts in that table, and create a service without reselecting the project/environment in a second form.
- **Why now:** the earlier dashboard walk classified the Environment-card layout as an information-architecture choice and closed only search/filtering. This populated comparator exposed the controls and metadata that the sparse comparison did not: contextual create, authoritative update/placement/runtime facts, and selection-based Move. The backend now carries most of these facts (`updatedAt`, runtime, `BEX_REGION`), making the remaining gap actionable rather than speculative.
- **Render parity closing task included:** this milestone changes dashboard behavior and adds the one missing GraphQL Service metadata field needed by that UI; t007 verifies the complete contract and refreshes evidence/ledger claims.

## Scope guardrails

- Keep bex's additional Database and Key Value categories; Render's narrower visible filter set is not a reason to remove supported resource kinds.
- Never label `createdAt` as Updated. Use the authoritative resource `updatedAt`, and render an honest missing state when the server omits a fact.
- `BEX_REGION` is installation placement, not user-selectable per-resource placement. Display it when configured; do not add a region picker.
- Preserve the existing full environment-management capability. The selected-Environment overview is an operating view, not a removal of create/rename/delete/settings/Manage resources.
- T001 must verify what Render's Move dialog targets before implementation. Do not infer cross-project versus cross-Environment semantics from the collapsed toolbar label.
