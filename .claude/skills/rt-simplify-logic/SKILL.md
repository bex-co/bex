---
name: rt-simplify-logic
description: >-
  Sweeps existing code for convoluted business logic — deep nesting, boolean spaghetti, multi-responsibility functions — and simplifies it behavior-preservingly with test coverage as the safety net. Use when the user asks to run the rt-simplify-logic routine or untangle convoluted logic across the codebase; for reviewing just-changed code use /simplify instead.
---

# Routine: Logic Simplifier

Find convoluted business logic in existing code and simplify it without changing behavior. Unlike `/simplify` (which reviews the current diff), this routine sweeps code that has already landed.

## Routine ground rules

Follow the shared contract in [`../../rt-shared-contract.md`](../../rt-shared-contract.md) (execution, tidy-routine value bar, batch cleanup & ≤15-line minimum threshold, **never opening ledger-only PRs**, board isolation, freshness check, `scripts/rt-pre-pr-check.sh` pre-PR fast gate, and report format).

- **Scope:** parse `$ARGUMENTS` per Phase 1. Empty = whole repo via parallel explore agents per module (`lego/types`, `lego/operator`, `lego/backend`, `lego/cli`, `dashboard`).

## Phase 1 — Scope

Parse `$ARGUMENTS` as an optional path/module scope. Empty = whole repo: launch parallel explore agents per module (lego/types, lego/operator, lego/backend, lego/cli, dashboard) to collect candidates.

## Phase 2 — Find candidates

Each agent flags functions with: more than 3 nesting levels; boolean expressions with 3+ operators; functions doing 3+ distinguishable things; repeated if/else ladders that want a table; conditions re-deriving state a caller already knows. Rank by how load-bearing the logic is — business rules over glue — and by churn: a file that keeps appearing in `fix` commits (`git log --oneline --grep='^fix' -- <file>`) is where convolution has already cost something.

While reading a candidate, look for the bug its shape hides; that is worth more than the tidy-up. The shapes that hide one: state derived twice that can disagree, a `default` arm that silently takes cases nobody listed, a caller's condition re-derived slightly differently, lists that must agree but are written out more than once, a value copied after the step that changes it. Prove a suspect with a failing test. A proven bug is fixed in this run with that test, as its own `fix` commit (ground rules); it is not left for rt-fix-logic-bugs.

## Phase 3 — Check the safety net

A candidate is eligible only when its branch behavior is exercised by tests. Find its `_test.go` (or dashboard test) and confirm the branches are covered (`go test -cover -run TestXxx ./pkg/`, or read the tests). For an uncovered candidate, first write characterization tests that pin current behavior — including odd edge cases; pin what IS, not what ought to be — then simplify.

Sensitive logic needs more than branch coverage: authorization and scope checks, custom-domain and host claims, billing and pricing, network isolation, and credential or token handling. When such a function checks several conditions in order, a test must pin which error wins when more than one fails at once, because a reordering changes that answer without failing any branch test. If no test pins the order, write one first or skip the candidate with that reason.

Stay inside one function's logic. Merging copies across functions or packages is rt-unify-duplicates' job, and a simplification that does it is reviewed as both.

## Phase 4 — Simplify

One function at a time: early returns over nesting; extract named predicate functions; table-drive if/else ladders; split at responsibility seams. Don't change signatures used across packages without migrating callers in the same pass. Re-run the package's tests after each function; revert any refactor whose diff you can't argue is behavior-identical.

## Phase 5 — Verify and report

Run the targeted checks (ground rules); report in the standard format, noting which simplifications required new characterization tests.

## Final phase — Ship

After all required gates pass, read and execute [the ship skill](../ship/SKILL.md) for the routine's completed changes. Stage only intended changes from this run; preserve unrelated work. Do not end at a plan, findings report, milestone, or request for shipping approval. If there are no changes, report that outcome without creating an empty commit. If a required gate remains blocked or fails after reasonable repairs, report the exact blocker and do not ship unverified changes.

Once the push succeeds, report the shipped HEAD SHA and subject per the ship skill and stop; do not monitor CI or deployment.
