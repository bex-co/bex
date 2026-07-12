# w5 · m16 — Manual-scaling section in service Settings

**Worker:** worker5 **Goal:** Add Render's manual-scaling stepper (instance count) to the service Settings tab, closing the dashboard's last gap on an otherwise fully-shipped verb (REST/GraphQL/MCP done via `w2/m12`). **Status:** done

## Tasks (in order)

| id   | title                                                                                                                                   | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Capture Render's live manual-scaling UI (instance-count stepper, save/confirm affordance) in `docs/render-artifacts/` — same pattern as m7's instance-type picker | 20m | —          | — **DONE** |
| t002 | Build the stepper component: current `replicas` from the existing `services` query, +/- controls, client-side bounds validation mirroring the backend's | 30m | t001       | — **DONE** |
| t003 | Wire to the `scaleService` GraphQL mutation; loading state; error surfacing on rejection                                                  | 25m | t002       | — **DONE** |
| t004 | Place in `dashboard/src/routes/services.$serviceId.settings.tsx`, respecting `w5/m11`'s type-aware settings (hidden for cron jobs)         | 20m | t003       | — **DONE** |
| t005 | Tests: `services.$serviceId.settings.test.tsx` — stepper renders current count, submits the correct mutation, handles the suspend-wins interaction | 30m | t004       | — **DONE** |
| t006 | Render parity — compare the shipped dashboard stepper against the t001 capture and the REST/GraphQL/MCP surfaces (`w2/m12`); flag any drift as follow-up | 20m | t005       | — **DONE** |
| t007 | Simplify — `/simplify` over the code this milestone changed                                                                                | 20m | t006       | — **DONE** |
| t008 | Test coverage — add meaningful tests for the behavior this milestone shipped, beyond t005's baseline                                      | 20m | t006       | — **DONE** |
| t009 | Closeout — verify DoD holds, then move the milestone to `done/`                                                                            | 10m | t007,t008  | — **DONE** |

## Definition of done

Settings tab shows a working instance-count stepper for web/worker services (hidden for cron jobs per m11's type-aware pattern); scaling 1→3→1 in the dashboard matches the already-verified backend behavior (`w2/m12`); tests green.

## Source + Goal linkage

- **Source:** promotes inbox note `w5/004` (unblocked as of `w2/m12`'s completion 2026-07-08; confirmed via `docs/ADR018-render-parity.md` row 25 — REST/GraphQL/MCP ✅✅✅, dashboard ✖).
- **Goal linkage:** Render parity — closes the dashboard's last gap on an otherwise fully-shipped verb.
- **Expected outcome:** manual scaling reachable from the dashboard, not just REST/GraphQL/MCP.
- **Why now:** the backend has sat done and unused from the dashboard since 2026-07-08; this is the cheapest kind of parity gap to close (API-first work already paid for).
