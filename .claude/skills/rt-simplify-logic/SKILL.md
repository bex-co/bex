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
- A run that changes nothing is a success. Ship only findings that clear this routine's bar, and never pad a run with marginal items so it has something to show: one finding finished completely beats several half-justified ones. The routine runs often, so what is left today is still there tomorrow.
- Value bar, on top of the proof this routine demands. A finding counts only when it (a) fixes or prevents a concrete bug, (b) removes a real hazard — copies that must change together and can drift apart, or a structure that has already caused a bug — or (c) makes a unit that is genuinely hard to change clearly safer to change. These never count on their own: a rename, removing an alias, binding a repeated expression to a local, naming a condition, extracting a helper that is called once, moving code without reducing coupling, deduplicating a few stable lines. That a change provably preserves behavior is what makes it safe, not what makes it worth shipping. Spend the run searching for a finding that clears this bar instead of shipping the first provable one; zero findings beats a cosmetic one.
- Read [`ledger.md`](ledger.md) beside this skill before auditing. It carries earlier runs' Deferred and Skipped findings. Do not re-audit a Skipped entry unless the code it cites has changed, and prefer finishing an open Deferred entry to hunting new ones. Update the ledger in the same change: add this run's Deferred and Skipped findings, and remove the entries this run fixed or whose code is gone. One line each, `- YYYY-MM-DD <path or symbol> — <finding>; <why deferred or skipped>`, newest first, at most 40 entries (drop the oldest Skipped first).
- A finding outside this routine's class is never only mentioned. If it is a real bug — behavior a user or caller can observe is wrong — and a failing test proves it, fix it in this run as its own `fix` commit with that test, whichever routine owns the class: a proven bug outranks this routine's own findings. Anything else that belongs to another routine goes into that routine's ledger (`../<routine>/ledger.md`) as a Deferred entry in the same change, so its next run starts there. The report is not a handoff: nobody reads it after the run. A run whose only output is Deferred entries, its own or another routine's, still ships that ledger change.
- Never fold a CI fix into this routine's change. A check that fails on `origin/main` without your change is main's failure, not this run's. Per `AGENTS.md` it ships alone as a `fix(ci)` change, and only after `git log origin/main --since=24.hours --grep='fix(ci)'` shows no one has fixed it already, so parallel runs do not each carry the same fix.
- Verify every change with targeted checks before reporting it done, and leave the full suites to CI. Run only what the change can break, one heavy command at a time: never two in parallel, never the same suite twice.
  - Go: `gofmt -l` on the touched files; `go build` and `go vet` of the touched packages; `go test` of those packages and of the packages that import them (find importers with `grep -rl '"<import path>"' --include='*.go' lego/`). Then lint the touched packages from each touched module with the pinned binary, `GOWORK=off "$(git rev-parse --show-toplevel)/lego/operator/bin/golangci-lint" run ./<pkg>/...` (`make golangci-lint` in `lego/operator` installs it). It takes seconds per package and catches what `go vet` does not (`unparam`, `lll`, depguard), which otherwise costs a CI round.
  - Dashboard: `yarn vitest related --run <changed files>`, `yarn eslint <changed files>`, and `yarn typecheck` when TypeScript changed.
  - Never the module-wide suites: `make lint`, `make test`, `go test ./...`, the whole `yarn test` or `yarn lint`. CI runs them on the pull request, against the real Postgres, OpenFGA and OpenBao a sandbox lacks. A failed check comes back to the session as its next turn (ADR047 D11). A run that ships straight to main gets them from the ship skill's gates and main's CI.
- Run `npx prettier@3.4.2 --write` on any markdown touched.
- Before shipping, summarize results in commentary using: **Candidates** (every candidate you examined, one line each: file:line, what it is, and its verdict; list them all, the ones you dropped at a glance included, because this list is how the run's search is judged) / **Fixed** (file:line + what it changes for a user or maintainer, not "cleaner") / **Deferred** (item + why, also written to the ledger) / **Skipped** (finding + reason) / **Gates** (commands run + outcomes).

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
