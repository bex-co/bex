# w5 · m18 — Cron-job Settings write path (Schedule + Command editable)

**Worker:** worker5 **Goal:** Retire the w5/m11 deferral: a cron job's Schedule + Command become editable from the dashboard Settings tab, Render-consistent. REST `PATCH` already threads `schedule`/`command`, but GraphQL has no update mutation (verified 2026-07-12: none in `lego/backend/internal/apps/graphql.go`) — this milestone adds that small backend half here rather than a gated w2 note (the w5/m9 precedent), plus the MCP decision and the UI. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                             | est | depends_on   |
| ---- | ---------------------------------------------------------------------------------------------------------------------------------- | --- | ------------ |
| t001 | GraphQL mutation to update a cron job's `schedule`/`command`, delegating to the same core verb REST `PATCH` uses                     | 30m | —            |
| t002 | MCP: make `update_cron_job` functional (Render ships a non-functional stub — the static-routes precedent) or document the omission   | 25m | t001         |
| t003 | Dashboard: `cron-deploy-section` editable — edit-in-place Schedule + Command, cron validation, save + convergence messaging          | 35m | t001         |
| t004 | Render parity — REST/GraphQL/MCP/UI consistency vs Render's cron Deploy section + OpenAPI `PATCH` shapes; ledger cron caveat retired | 25m | t002, t003   |
| t005 | Simplify — `/simplify` over the code this milestone changed                                                                          | 15m | t004         |
| t006 | Test coverage — mutation validation, non-cron 4xx, UI save/error states                                                              | 30m | t004         |
| t007 | Closeout — DoD met → move milestone to `done/`                                                                                       | 10m | t006         |

## Definition of done

Editing a cron job's schedule or command from the Settings tab converges the underlying k8s CronJob (new schedule visible on the CronJob object and reflected in the next run); the same edit works over GraphQL and MCP (or the MCP omission is documented as deliberately mirroring Render's stub); an invalid cron expression is rejected with the same actionable error on every surface; editing these fields on a non-cron service fails with Render's 4xx semantics; the parity ledger's cron-job row drops the "write path is a follow-on" caveat.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w5` 2026-07-12 (Proposal 2); the explicit deferral in `dashboard/src/features/services/components/cron-deploy-section.tsx` ("Schedule + Command, read-only for now (the write path is a follow-on)"); parity-ledger cron-job row.
- **Goal linkage:** pillar 1 (Render's cron Deploy section is editable) + pillar 3 (an agent retunes a schedule without recreating the service).
- **Expected outcome:** all four surfaces can update a cron job's schedule/command; REST stops being the odd surface out's inverse (today it's the *only* writer).
- **Why now:** REST `PATCH` already writes these fields — the surfaces have drifted; w5/m11 left an explicit IOU in the shipped component.
- **Render parity closing task: included** — REST/GraphQL/MCP/UI surface change; per user instruction at materialization ("ensure they are render.com consistent", 2026-07-12), t004 compares against Render's live cron Deploy section and OpenAPI `PATCH` field shapes, and MCP behavior is decided against Render's actual `update_cron_job` stub semantics.
- **Scope note:** contains a small backend GraphQL/MCP half inside w5 (flagged in the brainstorm, accepted at materialization) — self-contained beats a ~30m gated w2 note.
