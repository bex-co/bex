# w4 · m696 — Saving the Autoscaling section behaves as Render's: enabling confirms the billing, and both toggles off turns autoscaling off

**Worker:** worker4 **Goal:** on a service's Scaling page, turning autoscaling on asks for confirmation with Render's billing note, and saving with both the CPU and memory toggles off turns autoscaling off, as Render documents, instead of refusing the save. **Status:** done 2026-11-27 (both paths walked on production pin `486e3f4afbf6` as the QA user in `bex-canary`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record Render's enable dialog and its both-toggles-off flow from Render's public dashboard bundle — **DONE** | 30m | — |
| [t002](done/t002.md) | The Autoscaling section confirms an enable, and a save with both toggles off turns autoscaling off — **DONE** | 1h | t001 |
| [t003](done/t003.md) | All 8 languages, tests, and a walk of both paths on a fixture service — **DONE** | 45m | t002 |
| [t004](done/t004.md) | Render parity — **DONE** | 25m | t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 30m | t004, t005 |
| [t007](done/t007.md) | Closeout — **DONE** | 10m | t006 |

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

**2026-11-26, parked.** The change shipped as `68dd65d9e3` (and the drift filing as `d9e817067d`). The DoD's last line needs both paths walked on production, which waits for a deploy carrying `68dd65d9e3`.

**2026-11-27, unblocked (verified finding).** The gate held: the newest `chore(deploy): pin platform images to 486e3f4afbf6` on `origin/main` (`a6f9307d74`) carries `68dd65d9e3` (`git merge-base --is-ancestor 68dd65d9e3 486e3f4afbf6` exits 0). No decision was involved and no scope changed. Next: t003 step 2 on production as the QA user, on a paid fixture service in a Pro-or-higher first-party workspace (Cancel on "Enable Autoscaling?" saves nothing, Enable turns autoscaling on, turning off the last target then confirming "Disable Autoscaling?" turns it off), restore the service's original scaling, record the walk here, then t007.

## Production walk (t003 step 2, 2026-11-27, pin `486e3f4afbf6`): PASS

Signed in as the QA user (`scripts/qa-login.sh`) in `bex-canary` (`tea-daif693dqjvc73e7as3g`, Scale) and read the Compute page with a headless Chromium at 1440 px, recording every GraphQL mutation the page sent and reading `autoscalingConfig` back after each step. **Fixture:** `qa-20261127-m696-sc` (disposable), a Starter image web service (`traefik/whoami:v1.10.1`) created for the walk, Running with 1 instance and no stored autoscaling (`enabled: false`, range 0–0, no targets).

| Step | Page | Mutation sent | Server afterwards |
| --- | --- | --- | --- |
| Switch on | "Enable Autoscaling?", "When your service scales up, each running instance uses the same compute plan and is billed accordingly.", Cancel and Enable | — | — |
| Cancel | switch off, no settings form | none | unchanged (`enabled: false`, 0–0) |
| Switch on, Enable | "Autoscaling on", the form with 1–2 instances and both targets on at 60% | `SetAutoscaling` (min 1, max 2, CPU 60, memory 60): Render's first-enable defaults from the instance count | `enabled: true`, 1–2, CPU 60, memory 60; 1 instance |
| Memory off (CPU still on) | a plain edit, no dialog, Cancel and Save Changes shown | none | unchanged |
| CPU off (the last target) | "Disable Autoscaling?", "Disabling CPU-driven autoscaling will disable autoscaling entirely for this service. Are you sure?", Cancel and Disable | — | — |
| Cancel | the CPU toggle back on | none | unchanged (`enabled: true`) |
| CPU off again, Disable | "Autoscaling off", the form gone, Manual Scaling back | `DisableAutoscaling` | `enabled: false`, the range and targets kept (w4/597); 1 instance |
| Reload | "Autoscaling off" | none | the same |

"At least one utilization target" appeared at no step. At 390 px the enable dialog fits the viewport (340 px wide, no horizontal scroll), Enable above Cancel, and its Cancel sent nothing. **Restore and cleanup:** the walk's own Disable returned the fixture to autoscaling off on 1 instance, its original scaling, and the fixture was then deleted (`deleteService` true; `bex-canary` lists only the services it had before). This run's Kratos session was logged out (`qa-login.sh --logout`) and its session files removed. The walk's screenshots stayed in a scratch directory, out of the repository.

