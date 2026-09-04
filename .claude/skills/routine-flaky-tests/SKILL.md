---
name: routine-flaky-tests
description: >-
  Root-causes flaky CI tests by mining GitHub Actions run history for same-SHA flips, reproducing locally under race and stress, and fixing the actual race or timing bug — never by adding retries, sleeps, or skips. Use when the user asks to run the flaky-tests routine or investigate intermittent CI failures.
---

# Routine: Flaky-Test Fixer

A flaky test is a real bug in the test or the code. Find the flake, reproduce it, fix its root cause.

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

Parse `$ARGUMENTS` as an optional workflow or test name. Empty = scan the test gates: `backend-test.yml`, `operator-test.yml`, `cli-test.yml`, `dashboard-test.yml`.

## Phase 2 — Mine CI history

For each workflow: `gh run list --workflow <name> --limit 100 --json headSha,conclusion,databaseId,attempt`. Group by `headSha`; any SHA with both success and failure, or a rerun (`attempt > 1`) that flipped the outcome, is a flake candidate. Pull the failing test names: `gh run view <id> --log-failed`.

## Phase 3 — Reproduce locally

- `go test -race -count=20 -run 'TestName' ./pkg/...`; add `-timeout` pressure and vary `GOMAXPROCS`.
- Prime suspects: `require.Eventually` windows too short (~69 files use it), shared global state, port collisions, envtest resource races.
- Backend tests may need the same postgres:17 + openfga service containers `backend-test.yml` declares — replicate that docker setup before declaring "cannot reproduce".

## Phase 4 — Fix the root cause

Taxonomy: unsynchronized shared state → proper sync; time-dependent assertion → `Eventually` with an adequate window or a fake clock, not sleep; order dependence → isolate; external-resource collision → unique names/ports per test. NEVER fix with a retry loop, a longer sleep alone, or `t.Skip`.

## Phase 5 — Verify and report

The reproducer that failed must pass `go test -race -count=50`. Run the touched module's full gate. A flake you cannot reproduce after honest effort (including the CI-matching containers) goes under **Deferred** with its SHAs, run IDs, and log excerpts so it stays actionable. Report per the standard format.

## Final phase — Ship

After all required gates pass, read and execute [the ship skill](../ship/SKILL.md) for the routine's completed changes. Stage only intended changes from this run; preserve unrelated work. Do not end at a plan, findings report, milestone, or request for shipping approval. If there are no changes, report that outcome without creating an empty commit. If a required gate remains blocked or fails after reasonable repairs, report the exact blocker and do not ship unverified changes.

Once the push succeeds, report the shipped HEAD SHA and subject per the ship skill and stop; do not monitor CI or deployment.
