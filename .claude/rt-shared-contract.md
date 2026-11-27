# Shared Contract for `rt-*` Maintenance Routines

Every `.claude/skills/rt-*/SKILL.md` routine follows this shared contract. Each skill adds only the scope, candidate-finding, proof, and verification rules specific to its defect class.

## Execution & Shipping

- A user request to run a routine authorizes the full cycle: plan, fix, verify, then invoke `$land` (Codex) or `/land` (Claude) directly in the same run. Honor explicit user limits such as audit-only or no-ship. A request to edit a skill does not authorize running the routine.
- Keep a concise execution plan in the conversation and update it as findings are confirmed. Continue into implementation without a planning approval pause. Do not create or file a `.pm` milestone/task as a prerequisite or substitute for fixing; use the board only if the user requests it.
- After verification passes, follow the skill's land phase without asking the user to invoke land separately. Preserve the land skill's branch, conflict-resolution, and safety rules.
- Fix confirmed findings directly in the working tree. If a fix cannot be finished safely in this run, revert only your partial work for that fix and record it under **Deferred** in the report — never leave the tree half-refactored.

## Value Bar & Anti-Noise Discipline

- **A run that ships nothing is a success.** Never pad a run with marginal edits, test-only PRs, or synthetic edge cases just to produce a commit or PR. Zero shipped changes beats a cosmetic or low-signal PR.
- **False-positive discipline:** if a finding is intentional, load-bearing, or ambiguous, skip it with a one-line reason — do not argue it into a change.
- **Never bend production code to appease a flawed test:** when a fix breaks an existing CI/unit test, determine whether the test fixture is stale/flawed (e.g., reusing mutated state across subtests) or whether intent genuinely conflicts. Fix the test fixture or stop and report the conflict — **never** add spurious production behavior (such as emitting audit/activity-feed events on a no-op, or magic-number carve-outs) solely to keep an old test green.
- **Commit hygiene:** keep commit and PR headlines ≤72 characters (no mid-word `…` wraps) and use `refactor(...)` / `chore(...)` — never `fix(...)` — for pure refactors or constant deduplications.
- **Value bar (mandatory for tidy/maintenance routines: `rt-unify-duplicates`, `rt-flatten-abstractions`, `rt-normalize-abstractions`, `rt-simplify-logic`, `rt-prune-useless-tests`, `rt-remove-dead-code`):** A finding clears the bar only when it:
  1. fixes or prevents a concrete bug, or
  2. removes a real drift/regression hazard (copies that must change together and can drift apart, a structure that already caused bugs, or a genuinely hollow test/dead symbol sweep), or
  3. makes a unit that is genuinely hard to change clearly safer to change. Never ship on their own: a rename, removing a harmless alias, binding a repeated expression to a local, extracting a single-use helper, moving code without reducing coupling, or deduplicating a few stable lines (especially when a drift test already pins parity).
- **Batch cleanup & minimum PR threshold:** for pure cleanup/refactoring runs (no user-visible bug fix), sweep the full target scope and deliver the findings in **one batched PR/change** instead of opening separate tiny PRs per file. If the entire sweep yields only a single isolated cosmetic/cleanup edit of ≤15 changed lines with no bug risk, skip it and ship nothing.
- **Fix the whole invariant surface:** when fixing a logic invariant (e.g., rounding/total alignment, plan-vs-apply gating, redaction, authz enforcement), inspect **every sibling output, caller, and client consumer** of the same struct/contract in the same run (including `dashboard/`, `mobile/`, CLI, MCP, and sibling auth/validation helpers) so one invariant is fixed completely in one PR instead of split across follow-up PRs. Never copy a sibling pattern without verifying the sibling itself obeys the invariant.

## Ledger Rules (Never Open Ledger-Only PRs)

- Read `ledger.md` beside the skill before auditing. It carries earlier runs' **Deferred** and **Skipped** findings. Do not re-audit a Skipped entry unless the code it cites has changed, and prefer finishing an open Deferred entry over hunting new ones.
- Update `ledger.md` **only alongside a real code or qualifying test change**: add this run's Deferred and Skipped findings, and remove entries this run fixed or whose code is gone. One line each, `- YYYY-MM-DD <path or symbol> — <finding>; <why deferred or skipped>`, newest first, at most 40 entries (drop the oldest Skipped first).
- **Never commit or open a PR that only touches `ledger.md` — even for Deferred entries.** If a run produces no code or qualifying test changes, leave the working tree clean, make no commit, open no PR, and list any Deferred/Skipped notes in the final report only.
- **Out-of-class findings:**
  - If you find a **real bug** outside this routine's class (user/caller-observable wrong behavior proved by a failing test), fix it in this run as its own `fix` commit with that test: a proven bug outranks maintenance findings.
  - Otherwise, if you are already shipping code in this run, you may record an out-of-class Deferred entry in `../<routine>/ledger.md` in the same change. If you are not shipping code, report it and ship nothing.

## Board & Branch Freshness

- **Never work a `.pm` board item.** The board is `/loopx`'s queue, and a routine that takes a board item races the worker it was filed for. Find candidates in the code and ledger; if a bug you found is already filed on the board, leave it to that worker and name the item in the report.
- **Check freshness before working a ledger entry:** read the cited line on `origin/main` and run `git log origin/main --since=24.hours --oneline` for the files it names. If it is already fixed or gone, skip it with that reason.
- **Never fold a main CI fix into a routine PR.** A check that fails on `origin/main` without your change is main's failure, not this run's. Per `AGENTS.md`, a main CI fix ships alone as `fix(ci)`, and only after `git log origin/main --since=24.hours --grep='fix(ci)'` confirms nobody already fixed it.

## Default Scope (Whole-Repo Production Code, Not a Fixed Module List)

- **Scope by exclusion, not a closed manifest or module list:** when `$ARGUMENTS` is empty, the default scope is **all production code in the repository** — across every language and directory (Go services/plugins, JS/TS apps/drivers, Python/shell scripts, and any future module or language).
- **Exclude non-production trees:** skip `examples/`, `**/testdata/**`, `**/fixtures/**`, `docs/`, `.pm/`, `.internal/`, `external/`, and generated code unless `$ARGUMENTS` explicitly targets them. Never rely on enumerating `go.mod` / `package.json` files to define scope (that both pulls in example/testdata fixtures and misses script or new-language trees).

## Verification & Pre-PR Gate

Verify every change with targeted checks before shipping, and leave module-wide suites to CI. Run one heavy command at a time — never two in parallel, never the same suite twice.

- **Pre-PR fast gate (required before pushing / opening a PR):** run `bash scripts/rt-pre-pr-check.sh origin/main`. It rejects ledger-only diffs, checks `gofmt -l`, runs `golangci-lint` on touched production Go packages (skipping `examples/` and `testdata/` fixtures; module root discovered via nearest `go.mod` when the pinned binary is present), runs `eslint` on touched production JS/TS packages (nearest `package.json`), runs `shellcheck` on touched shell scripts, checks skill layout on `.claude/skills/` edits, and runs `scripts/gitleaks.sh` when `gitleaks` is available.
- **Go:** `gofmt -l` on touched files; from each touched Go module root (nearest `go.mod`), run `go build`, `go vet`, and `go test -count=1` on touched packages and their importers (`grep -rl '"<import path>"' --include='*.go'`). Run the pinned linter per touched module: `GOWORK=off "$(git rev-parse --show-toplevel)/lego/operator/bin/golangci-lint" run ./<pkg>/...` (`make golangci-lint` in `lego/operator` installs it).
- **JS/TS apps & packages:** from each touched package root (nearest `package.json`), run its targeted test runner (e.g., `yarn vitest related --run <changed files>`), `yarn eslint <changed files>`, and `yarn typecheck` when TypeScript changed.
- **Python / Shell / other languages:** run the touched unit tests (`python3 -m unittest ...`, `bash <script>.test.sh`, `shellcheck`, or the ecosystem's targeted test/lint command).
- **Markdown:** run `npx prettier@3.4.2 --write` on any markdown touched.
- **Never run module-wide suites locally** (`make lint`, `make test`, `go test ./...`, full `yarn test` or `yarn lint`): CI runs them on the PR against real Postgres, OpenFGA, and OpenBao.

## Report Format

Before shipping (or when ending with no changes), summarize in commentary using:

- **Candidates** — every candidate examined, one line each (`file:line`, what it is, verdict — including ones dropped at a glance).
- **Fixed** — `file:line` + what changes for a user or maintainer (never just "cleaner").
- **Deferred** — item + why.
- **Skipped** — finding + reason.
- **Gates** — exact commands run + outcomes.
