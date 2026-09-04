---
name: routine-useless-tests
description: >-
  Finds tests that cannot fail — assert-nothing, tautological, mock-echoing, or trivially-true polling — and deletes them or strengthens them into real behavioral tests, using mutation spot-checks as proof. Use when the user asks to run the useless-tests routine, prune worthless tests, or audit test quality.
---

# Routine: Useless-Test Pruner

A test that cannot fail is worse than no test: it costs CI time and buys false confidence. Find them, prove they're useless, then delete or strengthen.

## Routine ground rules

- A user request to run this routine authorizes the full cycle: plan, fix, verify, then invoke `$ship` (Codex) or `/ship` (Claude) directly in the same run. Honor explicit user limits such as audit-only or no-ship.
- Keep a concise execution plan in the conversation and update it as findings are confirmed. Continue into implementation without a planning approval pause. Do not create or file a `.pm` milestone/task as a prerequisite or substitute for fixing; use the board only if the user requests it.
- After the verification phase, follow the shipping phase below without asking the user to invoke ship separately. Preserve the ship skill's branch, conflict-resolution, and safety rules.
- Fix everything you find directly in the working tree. If a fix can't be finished safely in this run, revert only your partial work for that fix and list it under **Deferred** in the report — never leave the tree half-refactored.
- Scope: parse `$ARGUMENTS` per Phase 1 — an optional path/module scope or skill-specific target. No argument = the skill's stated default (usually the whole repo via parallel agents per module: lego/types, lego/operator, lego/backend, lego/cli, dashboard).
- False-positive discipline: if a finding is intentional, load-bearing, or ambiguous, skip it with a one-line reason — don't argue it into a change.
- Every change must pass the verification gates for the modules it touched before being reported as done: operator `make test` + `make lint` (from `lego/operator/`), `cd lego/backend && go test ./...`, `cd lego/cli && go test ./...`, dashboard `yarn test` + typecheck.
- Run `npx prettier@3.4.2 --write` on any markdown touched.
- Before shipping, summarize results in commentary using: **Fixed** (file:line + one-line rationale) / **Deferred** (item + why) / **Skipped** (finding + reason) / **Gates** (commands run + outcomes).

## Phase 1 — Scope

Parse `$ARGUMENTS` as an optional path/module scope. Empty = all four Go modules plus the dashboard test tree, via parallel agents.

## Phase 2 — Collect suspects

- Tests with zero `require`/`assert`/`t.Error`/`t.Fatal`/`expect` calls.
- Assertions comparing a value to itself or to the constant assigned two lines up.
- Tests whose only assertions are on a mock they themselves configured (mock-echo).
- `require.Eventually` (used in ~69 files) polling a condition that is trivially true.
- Skips whose condition can never be true. Note: the ~87 existing `t.Skip` calls are environment gates (`BEX_TEST_DB_URI`, `KUBECONFIG`, live-smoke inputs) — legitimate, not suspects.

## Phase 3 — Prove uselessness

For each suspect, run a mutation spot-check: temporarily break the code under test (invert the condition, return the zero value), run the test, and confirm it STILL passes — that is the proof. Revert the probe edit immediately, whatever the outcome.

## Phase 4 — Delete or strengthen

- **Delete** when the test is tautological, mock-echo, or a strict duplicate of a stronger test.
- **Strengthen** when the scenario matters but the assertions are hollow — especially when the test's name promises coverage the suite otherwise lacks. Add real behavioral assertions; for weak `Eventually` conditions, strengthen the condition rather than deleting the test.

## Phase 5 — Verify and report

Run gates for touched modules (strengthened tests must pass; deletions must not break compilation). Report per the standard format, including the mutation-check evidence for each deletion.

## Final phase — Ship

After all required gates pass, read and execute [the ship skill](../ship/SKILL.md) for the routine's completed changes. Stage only intended changes from this run; preserve unrelated work. Do not end at a plan, findings report, milestone, or request for shipping approval. If there are no changes, report that outcome without creating an empty commit. If a required gate remains blocked or fails after reasonable repairs, report the exact blocker and do not ship unverified changes.

Once the push succeeds, report the shipped HEAD SHA and subject per the ship skill and stop; do not monitor CI or deployment.
