# rt-simplify-logic ledger

Earlier runs' **Deferred** and **Skipped** findings, read at the start of every run and updated by its change (see the ground rules in [SKILL.md](SKILL.md)). One line per finding, newest first, at most 40 entries.

## Deferred

None yet.

## Skipped

- 2026-11-17 lego/backend/internal/apps/blueprint_plan.go probeBlueprintServiceSpec — long but already table-driven (appliers + applies/omits); restructuring risks the plan/apply parity it guards.
- 2026-11-17 lego/backend/internal/agenttranscript/coalesce.go toolChunk — big switch is a chunk-type dispatch with linear cases, not convoluted logic.
- 2026-11-17 lego/backend/internal/apps/deploy.go currentStateCopyProblems — deep-looking nesting is loop guards with continue; already in early-exit style.
