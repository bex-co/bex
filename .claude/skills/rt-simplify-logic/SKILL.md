---
name: rt-simplify-logic
description: >-
  Sweeps existing code for convoluted business logic — deep nesting, boolean spaghetti, multi-responsibility functions — and simplifies it behavior-preservingly with test coverage as the safety net. Use when the user asks to run the rt-simplify-logic routine or untangle convoluted logic across the codebase; for reviewing just-changed code use /simplify instead.
---

# Routine: Logic Simplifier

Find convoluted business logic in existing code and simplify it without changing behavior. Unlike `/simplify` (which reviews the current diff), this routine sweeps code that has already landed.

## Routine ground rules

- A user request to run this routine authorizes the full cycle: plan, fix, verify, then invoke `$ship` (Codex) or `/ship` (Claude) directly in the same run. Honor explicit user limits such as audit-only or no-ship.
- Keep a concise execution plan in the conversation and update it as findings are confirmed. Continue into implementation without a planning approval pause. Do not create or file a `.pm` milestone/task as a prerequisite or substitute for fixing; use the board only if the user requests it.
- After the verification phase, follow the shipping phase below without asking the user to invoke ship separately. Preserve the ship skill's branch, conflict-resolution, and safety rules.
- Fix everything you find directly in the working tree. If a fix can't be finished safely in this run, revert only your partial work for that fix and list it under **Deferred** in the report — never leave the tree half-refactored.
- Scope: parse `$ARGUMENTS` per Phase 1 — an optional path/module scope or skill-specific target. No argument = the skill's stated default (usually the whole repo via parallel agents per module: lego/types, lego/operator, lego/backend, lego/cli, dashboard).
- False-positive discipline: if a finding is intentional, load-bearing, or ambiguous, skip it with a one-line reason — don't argue it into a change.
- Verify every change with targeted checks before reporting it done, and leave the full suites to CI. Run only what the change can break, one heavy command at a time: never two in parallel, never the same suite twice.
  - Go: `gofmt -l` on the touched files; `go build` and `go vet` of the touched packages; `go test` of those packages and of the packages that import them (find importers with `grep -rl '"<import path>"' --include='*.go' lego/`).
  - Dashboard: `yarn vitest related --run <changed files>`, `yarn eslint <changed files>`, and `yarn typecheck` when TypeScript changed.
  - Never the module-wide suites: `make lint`, `make test`, `go test ./...`, the whole `yarn test` or `yarn lint`. CI runs them on the pull request, against the real Postgres, OpenFGA and OpenBao a sandbox lacks. A failed check comes back to the session as its next turn (ADR047 D11). A run that ships straight to main gets them from the ship skill's gates and main's CI.
- Run `npx prettier@3.4.2 --write` on any markdown touched.
- Before shipping, summarize results in commentary using: **Fixed** (file:line + one-line rationale) / **Deferred** (item + why) / **Skipped** (finding + reason) / **Gates** (commands run + outcomes).

## Phase 1 — Scope

Parse `$ARGUMENTS` as an optional path/module scope. Empty = whole repo: launch parallel explore agents per module (lego/types, lego/operator, lego/backend, lego/cli, dashboard) to collect candidates.

## Phase 2 — Find candidates

Each agent flags functions with: more than 3 nesting levels; boolean expressions with 3+ operators; functions doing 3+ distinguishable things; repeated if/else ladders that want a table; conditions re-deriving state a caller already knows. Rank by how load-bearing the logic is — business rules over glue.

## Phase 3 — Check the safety net

A candidate is eligible only when its branch behavior is exercised by tests. Find its `_test.go` (or dashboard test) and confirm the branches are covered (`go test -cover -run TestXxx ./pkg/`, or read the tests). For an uncovered candidate, first write characterization tests that pin current behavior — including odd edge cases; pin what IS, not what ought to be — then simplify.

## Phase 4 — Simplify

One function at a time: early returns over nesting; extract named predicate functions; table-drive if/else ladders; split at responsibility seams. Don't change signatures used across packages without migrating callers in the same pass. Re-run the package's tests after each function; revert any refactor whose diff you can't argue is behavior-identical.

## Phase 5 — Verify and report

Run the targeted checks (ground rules); report in the standard format, noting which simplifications required new characterization tests.

## Final phase — Ship

After all required gates pass, read and execute [the ship skill](../ship/SKILL.md) for the routine's completed changes. Stage only intended changes from this run; preserve unrelated work. Do not end at a plan, findings report, milestone, or request for shipping approval. If there are no changes, report that outcome without creating an empty commit. If a required gate remains blocked or fails after reasonable repairs, report the exact blocker and do not ship unverified changes.

Once the push succeeds, report the shipped HEAD SHA and subject per the ship skill and stop; do not monitor CI or deployment.
