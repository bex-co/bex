---
name: rt-prune-useless-tests
description: >-
  Audits hollow, duplicated, and implementation-coupled tests, then deletes or strengthens them using behavioral evidence and mutation spot-checks while preserving independent contracts. Use when the user asks to run the rt-prune-useless-tests routine, prune worthless tests, or audit test quality.
---

# Routine: Useless-Test Pruner

A test that cannot detect a meaningful regression costs CI time and buys false confidence. Find weak tests, establish what they actually protect, then delete or strengthen them.

Informed by [OpenClaw's test-audit skill](https://github.com/openclaw/openclaw/blob/main/.agents/skills/test-audit/SKILL.md); use bex's scope, verification, and shipping rules below.

## Routine ground rules

Follow the shared contract in [`../../rt-shared-contract.md`](../../rt-shared-contract.md) (execution, tidy-routine value bar, batch cleanup & ≤15-line minimum threshold, **never opening ledger-only PRs**, board isolation, freshness check, `scripts/rt-pre-pr-check.sh` pre-PR fast gate, and report format).

- **Scope:** parse `$ARGUMENTS` per Phase 1. Stay within that scope; do not turn a test audit into a general production refactor.
- **Batching:** sweep the target scope completely and land all confirmed test deletions/strengthenings in one batched change; do not open a PR to prune a single ≤15-line file unless it also strengthens a missing behavioral contract.
- **Explicit limits:** audit-only means read-only findings; no-ship means fixes and verification without committing or pushing.

## Phase 1 — Scope

Parse `$ARGUMENTS` as an optional path/module scope. Empty = every discovered Go module (`go.mod`) and JS/TS app/package (`package.json`) test tree in the repository. Read root and scoped `AGENTS.md` files and record the initial working-tree state before editing.

For whole-repo runs, use parallel discovery agents per module when available; otherwise cover the modules sequentially. Assign disjoint files for edits. Coordinate mutation probes centrally: no agent may mutate files another agent is editing or testing. Use an isolated checkout when probes cannot be serialized safely.

## Phase 2 — Collect suspects

- Tests with zero `require`/`assert`/`t.Error`/`t.Fatal`/`expect` calls. Follow helper calls and deferred checks before judging them; panic, race, compilation, and fuzz checks can fail without those tokens.
- Assertions comparing a value to itself or to the constant assigned two lines up.
- Tests whose only assertions are on a mock they themselves configured (mock-echo).
- `require.Eventually` or dashboard polling whose condition is already satisfied before the action under test, or becomes true without that action succeeding.
- Tests rendered unreachable by unconditional skips or impossible guards. Environment gates (`BEX_TEST_DB_URI`, `BEX_TEST_OPENFGA_URL`, `KUBECONFIG`, live-smoke inputs) are legitimate; a skipped test is not mutation evidence.
- Expected results computed by the subject; copied inventories; source-text assertions; private call-shape checks; repeated coverage across layers.
- Fixtures or mocks that manufacture the outcome; negative cases rejected by the wrong guard; names that overstate exercised behavior.

These are search leads, not deletion rules. Read the complete test, production path, callers, neighboring coverage, CI selection, and relevant history. Inspect dependency implementation when the claim relies on it.

Retain independent API, security, persistence, generated-code, release, and architecture contracts, including observable ordering. Slowness or source inspection alone does not justify removal. Keep static checks that independently enforce a contract through behavior-preserving refactors.

## Phase 3 — Establish evidence

Before editing, record the exact test/location, detectable regression, production callers, historical purpose, retained coverage (or why none is needed), risk, and focused validation command. Summarize confirmed findings in commentary without pausing for approval.

For hollow assertions outside audit-only runs, use a controlled mutation spot-check:

1. Run the exact test unchanged and confirm it actually executes and passes. Use an uncached run (`go test -count=1 -run ...` or `yarn test <file> -t ...`), checking the selected test names and skip output. A missing environment, empty selection, or baseline failure makes the probe inconclusive.
2. Temporarily introduce one realistic violation of the behavior the test claims to check, such as omitting the write it should observe. Keep the code compilable and show that the selected test reaches the changed path.
3. Run the same focused command. A surviving mutation establishes only that the test misses this regression, not that it has no value. A compile error, harness failure, or unrelated assertion failure is not a successful detection. If the intended assertion fails, retain that protection unless stronger remaining coverage demonstrably duplicates it.
4. Immediately restore only the probe edit, even if the command fails or is interrupted. Preserve pre-existing edits; do not use a blanket checkout/reset. Check the diff and rerun the focused baseline before continuing.

For duplicates, show that the retained test exercises the same inputs and behavior and catches the relevant mutation; the candidate may legitimately fail too. A tautology without a production path needs direct assertion/data-flow evidence, not an unrelated production mutation. If proof remains ambiguous, skip or defer the candidate.

## Phase 4 — Delete or strengthen

- **Delete** when the test is tautological, mock-echo, or a strict duplicate of a stronger test.
- **Strengthen** when the scenario matters but the assertions are hollow — especially when the test's name promises coverage the suite otherwise lacks. Add real behavioral assertions; for weak `Eventually` conditions, strengthen the condition rather than deleting the test.
- Any added or strengthened test must name its observable contract, plausible regression, and unique contribution. Prefer extending existing cases at the responsible boundary; avoid production hooks added solely for tests.
- Remove now-unused test helpers or production seams only after proving no production callers remain. Respect `operator → types ← backend` and standalone `cli`.

Reapply the relevant mutation after strengthening: the improved test must fail for the intended reason, then pass after restoration. Preserve useful baseline failures as product-bug evidence; defer unresolved repairs rather than deleting the failing coverage. Audit-only runs stop at findings without mutation or implementation edits.

## Phase 5 — Verify and report

Run the focused tests first, then the targeted checks for what changed:

- Go: `gofmt -l` on the touched files, `go vet` of each touched package, and `go test -count=1` of each changed test package and of any package whose helpers you removed. Then lint those packages from each touched module with the pinned binary, `GOWORK=off "$(git rev-parse --show-toplevel)/lego/operator/bin/golangci-lint" run ./<pkg>/...` (`make golangci-lint` in `lego/operator` installs it): a removed helper often leaves an `unused` or `unparam` finding behind.
- Types: also `go build ./...` and `go vet ./...` in the operator and backend modules, which compiles their tests without running them.
- Dashboard: `yarn vitest run <each changed test file>`, `yarn eslint <changed files>`, and `yarn typecheck`.
- Never the module-wide suites (`make lint`, `make test`, `go test ./...`, the whole `yarn test`): CI runs them on the pull request.

Use the scoped guides for integration-test prerequisites. Report skipped environment-dependent tests explicitly; they do not verify changes to those scenarios. Finish with Markdown formatting and `git diff --check`, and confirm no mutation probes remain.

Report **Fixed / Deferred / Skipped / Gates**, including deletion evidence, before/after mutation outcomes for strengthened tests, exact commands, and failures or skipped coverage. Distinguish production/helper removals from test removals; deletion count is not the success criterion.

## Final phase — Ship

After all required gates pass, read and execute [the ship skill](../ship/SKILL.md) for the routine's completed changes. Stage only intended changes from this run; preserve unrelated work. Do not end at a plan, findings report, milestone, or request for shipping approval. If there are no changes, report that outcome without creating an empty commit. If a required gate remains blocked or fails after reasonable repairs, report the exact blocker and do not ship unverified changes.

Once the push succeeds, report the shipped HEAD SHA and subject per the ship skill and stop; do not monitor CI or deployment.
