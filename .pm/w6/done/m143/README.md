# w6 · m143 — Make dashboard service and deploy actions permission-aware

**Worker:** worker6 **Goal:** service and deploy controls predict the backend's permission and resource preconditions before users submit an action **Status:** done

**Size:** 3h15m implementation + 1h45m standing closing work = ~5h (8 tasks).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Query and normalize workspace-scoped service and deploy action decisions — **DONE** | 45m | `w6/m136/t009` |
| t002 | Gate service lifecycle and deploy entry points by their actual backend verb — **DONE** | 45m | `w6/m143/t001` |
| t003 | Gate Cancel and Rollback while preserving selected-deploy eligibility — **DONE** | 60m | `w6/m143/t001` |
| t004 | Bind and recheck action confirmations before dispatch — **DONE** | 45m | `w6/m143/t002`, `w6/m143/t003` |
| t005 | Render parity — **DONE** | 30m | `w6/m143/t004` |
| t006 | Simplify — **DONE** | 20m | `w6/m143/t005` |
| t007 | Test coverage — **DONE** | 45m | `w6/m143/t005` |
| t008 | Closeout — **DONE** | 10m | `w6/m143/t006`, `w6/m143/t007` |
## Definition of done

- [x] Service row/header/settings and existing deploy entry points consult the decision for the backend verb they actually invoke; a UI Restart routed through triggerDeploy uses deploy semantics.
- [x] Viewers cannot open or dispatch forbidden lifecycle/deploy actions through affected controls; contributors retain allowed operations and cannot roll back. Denied, unavailable, and temporary resource preconditions have distinct English/Chinese explanations.
- [x] The Events list and deploy detail share consistent Cancel/Rollback behavior. Eligibility and confirmation bind the exact selected deploy; an older eligible history row is not disabled solely because it falls outside the service projection's latest-20 scan.
- [x] Confirmation rechecks the current workspace, target, action, and permission before dispatch; stale context clears the pending confirmation. Protected confirmation and billing recovery compose with the action decision, and cancellation remains possible when its own verb allows it.
- [x] Mounted interaction tests assert permitted dispatch and zero forbidden dispatch. Pending layouts preserve ready geometry in unit-covered surfaces.
- [ ] Live authenticated `dev-6` walkthrough of authorized/denied/unavailable states at desktop and narrow-mobile widths — deferred: local kind API server was unavailable this session (see Verification evidence).
- [x] `yarn typecheck`, `yarn lint`, and `yarn test` pass in dashboard; affected backend checks pass if projection code changes. Evidence records scope and any remaining limitation before closeout.

## Source + Goal linkage

- **Source:** Approved proposal 1 from `$pm-brainstorm for w6`, materialized by the user's `$pm for them all for w6` on 2026-09-10. Source review: `dashboard/src/features/deploys/components/deploy-actions.tsx` chooses Cancel/Rollback from status alone; `dashboard/src/features/services/components/service-row-actions.tsx` offers lifecycle actions without capability checks.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) API-first operation and [ADR024](../../../docs/ADR024-members.md) workspace roles: the dashboard must consume the same authority its mutations enforce.
- **Expected outcome:** Viewers see disabled, explained operations; contributors keep supported lifecycle/deploy controls while rollback requires the server's create permission. Authorized callers can act from both list and detail pages.
- **Why now / gap analysis:** [m136](../m136/README.md) already supplies action decisions, and [m141](../m141/README.md) demonstrates native consumption. The dashboard has no serverActions/deployActions query. This is a concrete uncovered control family, distinct from the completed editor/datastore sweeps in w9/m84, w9/m87, w2/m74, and w4/m85.
- **Render parity:** Included: tenant-facing UI changes touch ADR018's Suspend / Resume, Restart, Trigger a deploy, Cancel, and Rollback capability rows. Preserve existing backend authorization and documented Render divergences; capability projections are bex extensions.

## Scope and dependencies

- Keep server authorization, relation names, and resource preconditions authoritative. Do not infer permission from role names or alter backend permissions to accommodate a UI control.
- Use existing projections and shared permission UI. A per-service recent-history summary does not prove a selected deploy's eligibility; retain exact-target checks and add a narrowly scoped shared-predicate projection correction only if needed to satisfy the DoD.
- m143 owns action consumers and confirm-time rechecks. m144 owns persistent capability freshness and membership recovery; m143 must stand on its own without depending on m144.
- Datastore projection adoption, new operational verbs, one-off jobs, mobile work, and production deployment are outside this milestone. Dashboard authentication remains Kratos cookie-session based.
- Use the isolated dev-6 stack and clean its verification fixtures; do not use another worker's stack.

## Verification evidence

- Dashboard `yarn typecheck`, `yarn lint`, and `yarn test` green (3114 tests).
- Mounted tests: `service-row-actions`, `deploy-actions`, `resource-actions` policy — permitted dispatch and zero forbidden dispatch (viewer deny, restart→deploy verb, cancel under billing, selected-target rollback).
- Backend projection contract unchanged; no backend permission matrix edits.
- Limitation: live authenticated two-role walkthrough on `dev-6` was not completed this session (local kind API server unavailable); unit/mounted coverage stands in for permission gating. Re-run browser evidence when `dev-6` is up.
