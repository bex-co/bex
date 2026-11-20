# rt-simplify-logic ledger

Earlier runs' **Deferred** and **Skipped** findings, read at the start of every run and updated by its change (see the ground rules in [SKILL.md](SKILL.md)). One line per finding, newest first, at most 40 entries.

## Deferred

None yet.

## Skipped

- 2026-11-20 lego/backend/internal/apps/blueprint_state_plan.go PlanBlueprintResource — long service branch is sequential per-field gates with early returns, other arms are a kind dispatch; splitting makes once-called helpers.
- 2026-11-20 lego/backend/internal/apps/deploy.go parseCompiledStack — 170 lines but linear build/env/database/service passes with early returns; no shared condition to table.
- 2026-11-20 lego/backend/internal/usage/service.go resolveServiceNames — long but three independent kind blocks; the sandbox Deleted override is intentional specialization, covered by comments.
- 2026-11-20 lego/backend/internal/apps/service.go setSuspended — else-if ladder reads as sequential resume guards; unifying the verbs adds indirection without removing a hazard.
- 2026-11-20 lego/backend/internal/agentsession/model.go RegisteredModelEndpoint — 4-operator condition is the documented release/route/BYO precedence; tests pin each arm.
- 2026-11-20 lego/backend/internal/sandbox/agent_dispatch.go CleanupAgentDispatches — negated match condition is the documented abandoned-generation rule; naming it is cosmetic.
- 2026-11-17 lego/backend/internal/apps/blueprint_plan.go probeBlueprintServiceSpec — long but already table-driven (appliers + applies/omits); restructuring risks the plan/apply parity it guards.
- 2026-11-17 lego/backend/internal/agenttranscript/coalesce.go toolChunk — big switch is a chunk-type dispatch with linear cases, not convoluted logic.
- 2026-11-17 lego/backend/internal/apps/deploy.go currentStateCopyProblems — deep-looking nesting is loop guards with continue; already in early-exit style.
