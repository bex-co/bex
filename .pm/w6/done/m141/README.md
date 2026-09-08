# w6 · m141 — Consume server resource-action decisions in mobile

**Worker:** worker6 **Goal:** make mobile operational controls reflect the same per-resource permission and precondition decisions used by backend execution **Status:** done — 2026-09-08. Mobile service/deploy/cron/datastore controls consume the m136 projections with fail-closed presentation and confirm-time gate binding; all mobile gates green.

**Size:** 2h15m implementation + 2h closing work; 7 tasks.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Generate mobile resource-action queries and normalize decisions — **DONE** | 45m | `w6/m136/t009` |
| t002 | Use server decisions for service deploy and cron controls — **DONE** | 45m | `w6/m141/t001` |
| t003 | Apply datastore preconditions and shared confirmation reasons — **DONE** | 45m | `w6/m141/t001`, `w6/m141/t002`, `w6/m139/t004` |
| t004 | Render parity — **DONE** | 30m | `w6/m141/t003` |
| t005 | Simplify — **DONE** | 20m | `w6/m141/t004` |
| t006 | Test coverage — **DONE** | 60m | `w6/m141/t004` |
| t007 | Closeout — **DONE** | 10m | `w6/m141/t005`, `w6/m141/t006` |

## Definition of done

- [x] The native client consumes all four generated action projections and fails closed on unknown action IDs, outcomes, or blocking codes with generic translated copy.
- [x] Service, deploy, cron, and datastore controls reflect server permission and eligibility. Client predicates superseded by the projections are removed; independent display/status helpers remain only where still needed.
- [x] Protected-environment confirmation, billing blocks, suspension/transitional state, active-run eligibility, and rollback targets are handled from the projection and rechecked by the existing server mutation.
- [x] Confirmation binds the exact target/action/current access generation through m139. Double taps, changed preconditions, and ambiguous results retain existing safe-action semantics; no automatic mutation replay is introduced.
- [x] The projection and execution paths agree in meaningful permitted/denied/blocked scenarios. Bilingual accessible reasons, codegen/scope inventory, required mobile checks, and affected backend checks pass.

## Source + Goal linkage

- **Source:** Promotion of [w6/070](../done/070.md), accepted as proposal 4 in the 2026-09-07 mobile review handoff to $pm for w6. Builds on [w6/m136](../done/m136/README.md) resource projections and follows [ADR087](../../../docs/ADR087-mobile-role-views.md) Required contract work item 3.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) one authoritative API and [ADR048](../../../docs/ADR048-mobile.md) short, informed operational actions.
- **Expected outcome:** Users see only permitted actions, with temporarily blocked actions explaining the current resource precondition before submission instead of encountering predictable tap-time failures.
- **Why now:** serverActions, deployActions, databaseActions, and keyValueActions already exist. Mobile still combines workspace grants with parallel lifecycle/status predicates that cannot express protected-environment and billing prerequisites or authoritative rollback eligibility. The work spans four projections and several action consumers, so the existing note exceeds the sub-hour sizing rule.
- **Render parity:** Included because this changes a tenant-facing native/API surface. ADR018's Suspend / Resume, Restart, deploy trigger/cancel/rollback, and managed-datastore lifecycle rows are complete. Verify native consumption against those existing contracts and document intentional native presentation differences.

## Review evidence and scope

- `mobile/src/features/services/service-actions-card.tsx` uses serviceLifecycleCapabilities, isCancelableDeployStatus, isRollbackableDeployStatus, and suspended checks for action presentation.
- `lego/backend/internal/apps/actioncaps.go`, `deploys/actioncaps.go`, `postgres/actioncaps.go`, and `keyvalue/actioncaps.go` project execution-side predicates.
- The original w6/070 note is archived as promoted, not as implemented. This milestone is its sole active successor.
- m139 supplies the shared current-access/connectivity/generation dispatch gate; this milestone adds target/action preconditions without implementing another role engine.

Promote and consume existing contracts only. Do not broaden mobile actions, request secrets/sensitive scopes, add billing administration, or change Render compatibility and server authorization rules to match a client heuristic.

## Completion evidence

Completed 2026-09-08 (worker6 + workflow-implemented t001/t002-controller tracks, reconciled in one tree). [Verification record](../../../../mobile/e2e/m141-resource-actions.md) maps the DoD to the normalizer/documents/consumer suites, parity trace, and all required checks: `format:check`, `typecheck`, `lint` (ESLint + knip), `test:unit` (361 passed plus the access-recovery node suite), `expo:check`, `bundle:ios`, `bundle:android` — rerun after the final production change. No backend, dashboard, REST, GraphQL-schema, or MCP change, so no backend suite delta and no Render matrix cell affected. Physical-device qualification remains in w11. No commit or push was requested.
