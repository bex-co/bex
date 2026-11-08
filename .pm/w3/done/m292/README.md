# w3 · m292 — Protected placement follow-ups: Blueprint auto-sync, preselection, ledger and cost

**Worker:** worker3 **Goal:** Blueprint auto-sync applies render.yaml changes in protected environments as on Render, a non-Admin is never left with a preselected protected target, ADR018 row 225 is current, and an Admin's Blueprint pass checks placement once **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Blueprint auto-sync applies render.yaml changes in protected environments, as on Render — **DONE** | 1h | — |
| t002 | Rewrite ADR018 row 225's stale protected-environment sentences — **DONE** | 15m | — |
| t003 | A preselected protected environment is cleared, with its reason, for a non-Admin — **DONE** | 30m | — |
| t004 | One environment listing and one manage check per Blueprint placement pass — **DONE** | 45m | — |
| t005 | Render parity — **DONE** | 20m | t001, t003 |
| t006 | Simplify — **DONE** | 20m | t005 |
| t007 | Test coverage — **DONE** | 30m | t005 |
| t008 | Closeout — **DONE** | 10m | t006, t007 |

## Definition of done

- An identity-less Blueprint auto-sync that creates in, or moves into or out of, a protected environment applies; a Member's manual sync of the same manifest still gets 403 `PROTECTED_ENVIRONMENT_ADMIN_REQUIRED`; ADR032 records the rule with Render's wording.
- A non-Admin opening a create URL that preselects a protected environment sees Unassigned and the reason.
- ADR018 row 225 matches ADR032 and render.com/docs/projects.
- A Blueprint apply runs one environment listing and one fresh manage check (counting-fake test); m281's refusal tests still pass.

## Source + Goal linkage

- **Source:** `w3/059` (follow-ups from w3/m281's closeout and `/simplify`), promoted 2026-11-08 by `/loopx w3` because it spans four tasks and more than an hour.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)); protected environments ([ADR032](../../../docs/ADR032-environments.md)).
- **Expected outcome:** teams whose production environment is protected keep push-to-deploy Blueprints working as on Render, and non-Admins meet the protection before they fill a form.
- **Why now:** m281 (2026-11-08) made identity-less auto-syncs into protected environments fail; Render documents that Blueprint updates still apply there, so this is a parity regression to close right away.
- **Render parity:** included — the change touches Blueprint sync over REST/GraphQL/MCP and the dashboard create pickers.

## Closed — 2026-11-08

Blueprint auto-sync applies render.yaml changes in protected environments as on Render while member syncs stay gated; a non-Admin's preselected protected environment is cleared; ADR018 row 225 is current; an Admin's Blueprint apply runs one fresh manage check and one workspace environment listing for placement.
