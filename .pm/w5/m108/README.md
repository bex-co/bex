# w5 · m108 — Prevent canceled cron runs returning after history eviction

**Worker:** worker5 **Goal:** Keep an old manual trigger settled after its canceled run leaves the bounded display history. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Research Render cancellation and reproduce history eviction | 30m | — |
| t002 | Persist manual-run settlement independently of history | 60m | t001 |
| t003 | Verify existing Apps, restarts and fresh triggers | 45m | t002 |
| t004 | Render parity and surface disposition | 20m | t003 |
| t005 | Simplify | 20m | t004 |
| t006 | Test coverage | 30m | t004, t005 |
| t007 | Closeout | 10m | t006 |

## Definition of done

- A focused reproduction demonstrates or disproves resurrection after terminal-history eviction and replacement of spec.cancelRun; if disproven, close with concrete evidence instead of a speculative patch.
- When confirmed and fixed, a canceled manual Job remains absent after more than ten later runs, a later cancellation, controller restart and service suspend/resume. A fresh manual trigger executes exactly once through the existing mechanism.
- Run settlement survives bounded display-history retention without unbounded status growth, suppressing valid fresh triggers, or resurrecting historical triggers during migration.
- REST, GraphQL, MCP and dashboard retain truthful cancellation/run-history semantics. This is bex correctness supporting Render cancellation parity; no Render history-retention implementation is claimed.
- Operator/type and affected backend tests plus lint pass; local lifecycle evidence and fixture cleanup are recorded.

## Source + Goal linkage

- **Source:** User-approved w5 brainstorm follow-up, 2026-10-01; residual documented in app_controller.go manualRunSettled. Builds on [w4/m114 t001](../../w4/blocked/m114/done/t001.md) and [w5/m106](../done/m106/README.md), which cover retained cancellation history and scheduling during preemption, respectively.
- **Goal linkage:** ADR008 dependable hosting and ADR038/ADR018 cron execution semantics.
- **Expected outcome:** A canceled manual run never reappears merely because more than ten newer runs replace its history or the cancellation slot is reused; a fresh trigger still executes.
- **Why now:** The current settled check uses one cancellation slot or a matching terminal entry in a ten-run history while RunAt persists. Reproduce the documented residual now that preemption work has shipped.
- **Render parity included:** This changes user-visible deploy or cron behavior. Check REST, GraphQL, MCP and dashboard semantics against the t001 research, distinguishing direct Render requirements from bex correctness and undocumented details.

## Execution notes

Start with t001 before implementation. Work only in isolated dev-5 for live checks. Queue order is m107 → m108 → m109; m108 and m109 share cron code, so land m108 before changing it for m109. Research and all implementation tasks remain todo at filing.
