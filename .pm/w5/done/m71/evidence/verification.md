# m71 verification — 2026-08-17

## Outcome

All milestone DoD items are implemented locally. No push or deployment was performed.

## Evidence

- `go test ./... && go build ./...` from `lego/backend`: passed after the final production-code changes.
- `make lint-backend` from `lego/operator`: passed with `0 issues`.
- Real Postgres 17 run with `BEX_TEST_DB_URI`, targeting `TestPGStore` and `TestAgentSessionTurnPersistenceMigrationBackfillsRecoverableIntent`: passed. This exercised migration 0081 up/down, legacy creating-session prompt backfill, turn-local part-index uniqueness, cascade behavior, aggregate prompt quota rollback, and the concurrent-Steer row-lock CAS (one accepted, one conflict).
- `npm test && npm run typecheck && npm run build` from `lego/agent-image/driver`: passed; 42 tests.
- `yarn typecheck && yarn lint && yarn test && yarn build` from `dashboard`: passed; 320 test files / 2204 tests. The build emitted only the repository's existing TanStack code-split, module-directive, and chunk-size warnings.
- Final agent-session dashboard regression run: 3 files / 27 tests passed after the conversation remount/optimistic-echo adjustment.
- `npx prettier@3.4.2` on touched ADRs and `--ignore-path /dev/null` on `.pm` Markdown: clean.
- `git diff --check`: clean.
- Every temporary `bex-m71-postgres-*` container was stopped by an EXIT trap; final `docker ps` filter returned none.

## Covered regressions

- Atomic initial and follow-up prompt persistence, including creating-row legacy backfill.
- One accepted concurrent Steer and one coded conflict.
- Replacement-turn acceptance clears the old sandbox binding while preserving its teardown target out of row state.
- Turn-local part index zero across fresh sandboxes; global cursor allocation remains monotonic.
- Partial live prefix plus completion suffix merge.
- 16 MiB log harvest with explicit transport/log/part/quota/store completeness state.
- Promptless hibernated Resume and exactly-one-turn hibernated Steer, including failed provisioning.
- Public non-durable live POST refusal.
- Ordered role-correct prompt/assistant replay and refreshed dashboard rendering.
- REST/GraphQL/MCP transcript turn, part-index, and completeness fields.

## Honest limitation

Previously lost historical follow-up prompts cannot be reconstructed. Migration 0081 recovers only the initial task still present in `agent_config.task`. Production verification awaits a separately authorized deployment; this goal did not authorize push or deploy.
