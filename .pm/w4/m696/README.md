# w4 · m696 — Saving the Autoscaling section behaves as Render's: enabling confirms the billing, and both toggles off turns autoscaling off

**Worker:** worker4 **Goal:** on a service's Scaling page, turning autoscaling on asks for confirmation with Render's billing note, and saving with both the CPU and memory toggles off turns autoscaling off, as Render documents, instead of refusing the save. **Status:** todo (t001, t002 and t004–t006 done; t003's strings and tests done, its production walk next, then t007)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record Render's enable dialog and its both-toggles-off flow from Render's public dashboard bundle — **DONE** | 30m | — |
| [t002](done/t002.md) | The Autoscaling section confirms an enable, and a save with both toggles off turns autoscaling off — **DONE** | 1h | t001 |
| [t003](t003.md) | All 8 languages, tests, and a walk of both paths on a fixture service | 45m | t002 |
| [t004](done/t004.md) | Render parity — **DONE** | 25m | t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 30m | t004, t005 |
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

## Progress

**2026-11-26 (t001, t002, t004–t006; t003's strings and tests).**

- **What Render does (t001):** its public dashboard bundle (`docs/render-artifacts/manual-scaling.md` § Autoscaling's confirmations) shows that the switch saves nothing by itself. Turning autoscaling on opens "Enable Autoscaling?" with "When your service scales up, each running instance uses the same compute plan and is billed accordingly." (Cancel, Enable), and Enable saves the stored settings or the first-enable defaults (the instance count up to twice it, CPU and memory at 60%). The settings form shows only while autoscaling is on. Both toggles off never reaches Save: turning off the last target opens "Disable Autoscaling?" at once, with "Disabling CPU-driven autoscaling will disable autoscaling entirely for this service. Are you sure?" (or memory-driven), Cancel turning the toggle back on.
- **Deviation from the DoD's wording, following its intent:** the second DoD line imagined a save with both toggles off. Render's page has no such save, so bex copies Render: the last target's toggle opens the disable confirmation itself, and Disable turns autoscaling off through the existing `disableAutoscaling`. The "At least one utilization target" refusal and its string are gone from all 8 locales. Following the same bundle, the form now shows only while autoscaling is on, Save Changes waits for a change and Cancel shows only then, both dialogs' buttons are primary rather than destructive, and the disable title reads "Disable Autoscaling?".
- **Code (t002, t003 step 1):** `dashboard/src/features/services/components/autoscaling-section.tsx` (the Compute page passes the instance count), 5 new strings in each of the 8 `dashboard/src/features/services/locales/*.ts`. REST, GraphQL and MCP are unchanged.
- **Tests (t006):** 9 new cases in `components/__tests__/autoscaling-section.test.tsx` and 1 in `routes/__tests__/services.$serviceId.compute.test.tsx`. Reverting each guarded behavior in turn (enable saves without asking, last target off as a plain edit, first-enable defaults, the 100 cap, Save waiting for a change, Cancel only when dirty, the target-specific body, the Compute page's instance count) fails at least one of them. `yarn lint` passes; `yarn test` passed apart from 3 cases in two untouched files (`keyvalue.$keyValueId`, `dashboard-topbar`) that failed under full-suite load and pass alone.
- **Parity (t004):** ADR018 § Services & lifecycle, Autoscaling config row, amended. One drift found and filed: Render's page on a disk-backed service shows only "Scaling is not supported for servers with disks.", while bex shows both scaling cards and its API refuses the save (w4/753).
- **Simplify (t005):** `/simplify` ran over the diff; the save mapping is shared by `formValues`, and the old `switchedOn` and `validate()` collapsed. It added one comment and found nothing else.
- **Next (t003 step 2):** once production carries the change, sign in as the QA user, enable autoscaling on a fixture service through the dialog (Cancel first, then Enable), turn off the last target and confirm Disable, restore the service's original scaling, and record the walk here.
