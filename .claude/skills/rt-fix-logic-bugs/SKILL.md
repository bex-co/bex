---
name: rt-fix-logic-bugs
description: >-
  Picks a tricky logic domain — billing and pricing math, OpenFGA authz scope decisions, blueprint plan diffs — models it explicitly with invariants, truth tables, and property tests, and fixes the real bugs the model surfaces with regression tests. Use when the user asks to run the rt-fix-logic-bugs routine, hunt for logic bugs, or property-test a subsystem.
---

# Routine: Logic Bugfixer

Model tricky business logic explicitly, then let the model find the bugs. The tests written here persist as durable property coverage.

## Routine ground rules

- A user request to run this routine authorizes the full cycle: plan, fix, verify, then invoke `$ship` (Codex) or `/ship` (Claude) directly in the same run. Honor explicit user limits such as audit-only or no-ship.
- Keep a concise execution plan in the conversation and update it as findings are confirmed. Continue into implementation without a planning approval pause. Do not create or file a `.pm` milestone/task as a prerequisite or substitute for fixing; use the board only if the user requests it.
- After the verification phase, follow the shipping phase below without asking the user to invoke ship separately. Preserve the ship skill's branch, conflict-resolution, and safety rules.
- Fix everything you find directly in the working tree. If a fix can't be finished safely in this run, revert only your partial work for that fix and list it under **Deferred** in the report — never leave the tree half-refactored.
- Scope: parse `$ARGUMENTS` per Phase 1 — an optional path/module scope or skill-specific target. No argument = the skill's stated default (usually the whole repo via parallel agents per module: lego/types, lego/operator, lego/backend, lego/cli, dashboard).
- False-positive discipline: if a finding is intentional, load-bearing, or ambiguous, skip it with a one-line reason — don't argue it into a change.
- A run that changes nothing is a success. Ship only findings that clear this routine's bar, and never pad a run with marginal items so it has something to show: one finding finished completely beats several half-justified ones. The routine runs often, so what is left today is still there tomorrow.
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

Parse `$ARGUMENTS`: `billing` | `authz` | `blueprint-plan` | `agreement` | a package path. Empty → pick the least-recently-audited domain (`git log -1 --format=%cd` on each candidate below) and announce the choice.

Domain map:

- **billing** — `lego/backend/internal/apps/blueprint_pricing.go`, `stripeBillingGate` in `lego/backend/cmd/api/billing_config.go`
- **authz** — `lego/backend/internal/authz` and `lego/backend/internal/core/scope.go` (OpenFGA relationship model)
- **blueprint-plan** — `lego/backend/internal/apps/blueprint_plan.go`, `blueprint_state_plan.go` and the plan/apply cycle in `deploy.go`
- **agreement** — any place where one function predicts or previews what another performs, and the two must agree. This family has produced the most real bugs: a plan that promised an update its apply refuses (a database's name), a plan that reported a change and named no field (domains, the build strategy). Sites: the Blueprint plan against `applyDatabase` / `applyKeyValue` / `updateStackService` in `lego/backend/internal/apps/deploy.go`; plan-time admission against the admission the write runs (`lego/backend/internal/postgres/admission.go`, `lego/backend/internal/keyvalue/tiers.go`); validation and preview verbs against the verbs they preview; an estimate against the gate that enforces it.

Within a domain, take one resource kind or one invariant per run (service, database, key value, environment, env group; refusals, changed-field names, idempotence), and check the ledger for the ones earlier runs covered, so each run digs somewhere new. Record what this run covered in the ledger's Skipped section when it found nothing, as `<domain>/<kind or invariant> — audited, holds`.

## Phase 2 — Model the logic

Before hunting bugs, state the intended invariants in the conversation: e.g. "pricing is monotonic in replica count and disk size", "scope checks are deny-by-default", "plan(apply(plan)) is empty", "compile is deterministic", "the plan refuses exactly what the apply refuses", "a plan that says update names every field the apply moves". Build truth tables for boolean decision functions. Read the governing ADRs (`docs/ADR030-pricing.md`, `docs/ADR012-auth.md`, `docs/ADR024-members.md`) — the model must capture intent, not implementation.

## Phase 3 — Encode the model as tests

Write property/table tests in the package's `_test.go`: `testing/quick` or hand-rolled generators for pure functions; the full scope × role matrix for authz; plan/apply round-trips for blueprint-plan. These tests persist as durable coverage.

## Phase 4 — Judge each failure

A failing property is one of:

- **Model error** — the code is right and your invariant was wrong. Fix the test and record the true invariant in a test comment.
- **Real bug** — root-cause and fix the code, keeping the failing case as a named regression test.

## Phase 5 — Verify and report

Run the targeted checks (ground rules); report per the standard format, listing invariants encoded, bugs fixed, and model errors learned.

## Final phase — Ship

After all required gates pass, read and execute [the ship skill](../ship/SKILL.md) for the routine's completed changes. Stage only intended changes from this run; preserve unrelated work. Do not end at a plan, findings report, milestone, or request for shipping approval. If there are no changes, report that outcome without creating an empty commit. If a required gate remains blocked or fails after reasonable repairs, report the exact blocker and do not ship unverified changes.

Once the push succeeds, report the shipped HEAD SHA and subject per the ship skill and stop; do not monitor CI or deployment.
