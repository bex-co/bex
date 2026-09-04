---
name: routine-logic-simplifier
description: >-
  Sweeps existing code for convoluted business logic — deep nesting, boolean spaghetti, multi-responsibility functions — and simplifies it behavior-preservingly with test coverage as the safety net. Use when the user asks to run the logic-simplifier routine or untangle convoluted logic across the codebase; for reviewing just-changed code use /simplify instead.
---

# Routine: Logic Simplifier

Find convoluted business logic in existing code and simplify it without changing behavior. Unlike `/simplify` (which reviews the current diff), this routine sweeps code that has already landed.

## Routine ground rules

- NEVER `git commit` or `git push` — only the user-invoked `/ship` commits. End by reporting what changed so the user can review and `/ship`.
- Fix everything you find directly in the working tree. If a fix can't be finished safely in this run, revert the partial work and list it under **Deferred** in the report — never leave the tree half-refactored.
- Scope: parse `$ARGUMENTS` per Phase 1 — an optional path/module scope or skill-specific target. No argument = the skill's stated default (usually the whole repo via parallel agents per module: lego/types, lego/operator, lego/backend, lego/cli, dashboard).
- False-positive discipline: if a finding is intentional, load-bearing, or ambiguous, skip it with a one-line reason — don't argue it into a change.
- Every change must pass the verification gates for the modules it touched before being reported as done: operator `make test` + `make lint` (from `lego/operator/`), `cd lego/backend && go test ./...`, `cd lego/cli && go test ./...`, dashboard `yarn test` + typecheck.
- Run `npx prettier@3.4.2 --write` on any markdown touched.
- Report format: **Fixed** (file:line + one-line rationale) / **Deferred** (item + why) / **Skipped** (finding + reason) / **Gates** (commands run + outcomes).

## Phase 1 — Scope

Parse `$ARGUMENTS` as an optional path/module scope. Empty = whole repo: launch parallel explore agents per module (lego/types, lego/operator, lego/backend, lego/cli, dashboard) to collect candidates.

## Phase 2 — Find candidates

Each agent flags functions with: more than 3 nesting levels; boolean expressions with 3+ operators; functions doing 3+ distinguishable things; repeated if/else ladders that want a table; conditions re-deriving state a caller already knows. Rank by how load-bearing the logic is — business rules over glue.

## Phase 3 — Check the safety net

A candidate is eligible only when its branch behavior is exercised by tests. Find its `_test.go` (or dashboard test) and confirm the branches are covered (`go test -cover -run TestXxx ./pkg/`, or read the tests). For an uncovered candidate, first write characterization tests that pin current behavior — including odd edge cases; pin what IS, not what ought to be — then simplify.

## Phase 4 — Simplify

One function at a time: early returns over nesting; extract named predicate functions; table-drive if/else ladders; split at responsibility seams. Don't change signatures used across packages without migrating callers in the same pass. Re-run the package's tests after each function; revert any refactor whose diff you can't argue is behavior-identical.

## Phase 5 — Verify and report

Run the gates for touched modules; report in the standard format, noting which simplifications required new characterization tests.
