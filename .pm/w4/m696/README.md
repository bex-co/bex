# w4 · m696 — Saving the Autoscaling section behaves as Render's: enabling confirms the billing, and both toggles off turns autoscaling off

**Worker:** worker4 **Goal:** on a service's Scaling page, turning autoscaling on asks for confirmation with Render's billing note, and saving with both the CPU and memory toggles off turns autoscaling off, as Render documents, instead of refusing the save. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Record Render's enable dialog and its both-toggles-off flow from Render's public dashboard bundle | 30m | — |
| [t002](t002.md) | The Autoscaling section confirms an enable, and a save with both toggles off turns autoscaling off | 1h | t001 |
| [t003](t003.md) | All 8 languages, tests, and a walk of both paths on a fixture service | 45m | t002 |
| [t004](t004.md) | Render parity | 25m | t003 |
| [t005](t005.md) | Simplify | 15m | t004 |
| [t006](t006.md) | Test coverage | 30m | t004, t005 |
| [t007](t007.md) | Closeout | 10m | t006 |

Total: **7 tasks, about 3h35m**.

## Definition of done

- Turning autoscaling on from the Scaling page opens a confirmation with Render's wording (as t001 records it) before anything is saved; Cancel saves nothing.
- Saving with both utilization toggles off on an autoscaled service opens the existing disable confirmation and, once confirmed, turns autoscaling off; the "At least one utilization target" refusal is gone from the page.
- All 8 languages carry the new strings, and both paths work on a fixture service in production.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect other than postgresql to w4`, 2026-11-26 (the aspect: scaling and autoscaling), item 3. Re-checked by hand at `origin/main` `20dfa991d4` before filing; no open, blocked or done item owns it.
- **Render:** [render.com/docs/scaling](https://render.com/docs/scaling): "If you enable _neither_ toggle, autoscaling is _disabled_ for the service." The live capture of Render's Scaling page (`docs/render-artifacts/manual-scaling.md:34`, 2026-07-16) records "confirm modals on both enable ('each running instance … is billed accordingly') and disable".
- **bex today:** `dashboard/src/features/services/components/autoscaling-section.tsx` refuses a save with both toggles off (`validate`, `:53-58`, `services.scalingValidationTargets`) and saves an enable with no dialog (`handleSave`, `:299-315`); only the disable confirmation exists (w7/m43, `:227-294`).
- **Scope:** REST `PUT /v1/services/{id}/autoscaling` with both criteria disabled keeps answering 400 (`lego/backend/internal/apps/render.go:355-359`): what Render's API does there is not published. The dashboard reaches the off state through the existing disable call.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first).
- **Expected outcome:** a user sees what autoscaling will cost before it starts, and can switch it off the way Render's docs describe.
- **Why now:** enabling autoscaling starts paid instances with no confirmation today, while Render asks first.
- **Render parity:** included: the dashboard changes, and REST, GraphQL and MCP must stay consistent with it.
