# w5 · m107 — Preserve saved settings during rollback

**Worker:** worker5 **Goal:** Run the selected historical configuration during rollback while retaining the saved configuration for the next standard deploy. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Research Render rollback configuration and reconcile m152 | 30m | — |
| t002 | Apply historical configuration only to the rollback release | 45m | t001 |
| t003 | Keep saved settings authoritative for the next standard deploy | 45m | t002 |
| t004 | Verify rollback A and next-deploy B on isolated dev-5 | 45m | t003 |
| t005 | Render parity and surface disposition | 25m | t004 |
| t006 | Simplify | 20m | t005 |
| t007 | Test coverage | 30m | t005, t006 |
| t008 | Closeout | 10m | t007 |

## Definition of done

- A disposable dev-5 service demonstrates deploy A → save/deploy B → rollback A: the ready runtime uses A while saved Settings, environment and secret-file reads retain B. The next standard deploy runs B.
- Release-specific configuration remains correct through reconciliation, pod replacement, explicit service Restart and operator restart, with no transient mutation of shared environment groups or saved configuration.
- The researched target/current configuration matrix is applied to supported fields, with unsupported or unverified behavior explicitly dispositioned. Existing release snapshots and cancel-deploy protections remain covered; current plan/disks/domains retain the researched current-setting semantics.
- REST, GraphQL, MCP and dashboard agree on saved values and rollback outcomes; preserve the documented dashboard-versus-API auto-deploy distinction. Target records missing or evicted have an explicit compatible disposition. Same-image/config-only targets, already-running-target guards and saved-versus-running indicators remain truthful.
- Relevant backend/operator/type tests and lint pass, dashboard checks pass if affected, and local runtime evidence plus fixture cleanup are recorded before closeout.

## Source + Goal linkage

- **Source:** User-approved follow-up to the w5 brainstorm and Render parity review, 2026-10-01; reproduced saved-state drift in [w5/m105](../blocked/m105/README.md). Corrects the saved-state interpretation in [w1/m152](../../w1/done/m152/README.md) while retaining its release snapshots and cancellation protections.
- **Goal linkage:** ADR008 dependable hosting and ADR004/ADR018 Render-compatible deploy lifecycle.
- **Expected outcome:** After deploying A, saving/deploying B and rolling back to A, the service runs A while Settings still contains B; the next standard deploy runs B.
- **Why now:** The m105 local replay observed both the start command and API-owned environment reverting in saved state, and the next normal deploy kept the old command. Research and correct this narrow semantic gap before extending rollback behavior.
- **Render parity included:** This changes user-visible deploy or cron behavior. Check REST, GraphQL, MCP and dashboard semantics against the t001 research, distinguishing direct Render requirements from bex correctness and undocumented details.

## Execution notes

Start with t001 before implementation. Work only in isolated dev-5 for live checks. Queue order is m107 → m108 → m109; m108 and m109 share cron code, so land m108 before changing it for m109. Research and all implementation tasks remain todo at filing.
