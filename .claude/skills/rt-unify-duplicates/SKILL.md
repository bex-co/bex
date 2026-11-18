---
name: rt-unify-duplicates
description: >-
  Finds duplicated implementations across modules and merges each into one canonical helper, respecting the workspace import DAG (operator→types←backend; cli imports no sibling); inherent two-sided duplication gets a drift-guard test instead. Use when the user asks to run the rt-unify-duplicates routine, deduplicate code, or unify parallel helpers.
---

# Routine: Dup Unifier

Find copies of one piece of logic that have drifted apart or will, fix the bug the drift caused, and merge them into one canonical home — or, where duplication is inherent to the design, guard it against drift. A merged pair of identical copies is the lesser result; a drifted pair, proved and fixed, is the one this routine exists for.

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
- A finding outside this routine's class is never only mentioned. If it is a real bug — behavior a user or caller can observe is wrong — and a failing test proves it, fix it in this run as its own `fix` commit with that test, whichever routine owns the class: a proven bug outranks this routine's own findings. Anything else that belongs to another routine goes into that routine's ledger (`../<routine>/ledger.md`) as a Deferred entry in the same change, so its next run starts there. The report is not a handoff: nobody reads it after the run. A run whose only output is Deferred entries, its own or another routine's, still ships that ledger change. Skipped entries alone do not: they ride along with the run's code or Deferred change, and a run that has only Skipped entries lists them in its report and ships nothing.
- Before working a `.pm` board item or a ledger entry, check it is still open: read its current file on `origin/main`, and `git log origin/main --since=24.hours --oneline` for its id and for the files it names. If it is closed or already fixed, skip it with that reason: another session may have taken it since the ledger or board was written.
- When a change fixes a board item, every bullet of its definition of done gets a test that fails on the code before the fix for exactly that bullet's scenario. Say in the report which test covers which bullet. A test that passes before the fix proves nothing, however close its name is to the bullet.
- Never fold a CI fix into this routine's change. A check that fails on `origin/main` without your change is main's failure, not this run's. Per `AGENTS.md` it ships alone as a `fix(ci)` change, and only after `git log origin/main --since=24.hours --grep='fix(ci)'` shows no one has fixed it already, so parallel runs do not each carry the same fix.
- Verify every change with targeted checks before reporting it done, and leave the full suites to CI. Run only what the change can break, one heavy command at a time: never two in parallel, never the same suite twice.
  - Go: `gofmt -l` on the touched files; `go build` and `go vet` of the touched packages; `go test` of those packages and of the packages that import them (find importers with `grep -rl '"<import path>"' --include='*.go' lego/`). Then lint the touched packages from each touched module with the pinned binary, `GOWORK=off "$(git rev-parse --show-toplevel)/lego/operator/bin/golangci-lint" run ./<pkg>/...` (`make golangci-lint` in `lego/operator` installs it). It takes seconds per package and catches what `go vet` does not (`unparam`, `lll`, depguard), which otherwise costs a CI round.
  - Dashboard: `yarn vitest related --run <changed files>`, `yarn eslint <changed files>`, and `yarn typecheck` when TypeScript changed.
  - Never the module-wide suites: `make lint`, `make test`, `go test ./...`, the whole `yarn test` or `yarn lint`. CI runs them on the pull request, against the real Postgres, OpenFGA and OpenBao a sandbox lacks. A failed check comes back to the session as its next turn (ADR047 D11). A run that ships straight to main gets them from the ship skill's gates and main's CI.
- Run `npx prettier@3.4.2 --write` on any markdown touched.
- Before shipping, summarize results in commentary using: **Candidates** (every candidate you examined, one line each: file:line, what it is, and its verdict; list them all, the ones you dropped at a glance included, because this list is how the run's search is judged) / **Fixed** (file:line + what it changes for a user or maintainer, not "cleaner") / **Deferred** (item + why, also written to the ledger) / **Skipped** (finding + reason) / **Gates** (commands run + outcomes).

## Phase 1 — Scope

Parse `$ARGUMENTS` as an optional path/module scope. Empty = whole repo; the backend↔operator pair is the hot spot.

## Phase 2 — Find duplicates

Start from the known candidates, then scan wider:

- `lego/backend/internal/id` vs `lego/operator/internal/identity` — two id/identity implementations split by module.
- Naming helpers in `lego/backend/internal/store/reconciler.go` vs `lego/operator/internal/controller/app_controller.go` — backend computes desired CRs, operator reconciles them.
- Scan: same-named functions across modules (`grep -rn "^func " lego/{backend,operator}/internal`), copy-paste blocks ≥10 lines, parallel dashboard/backend logic.

## Phase 3 — Classify

- **Drifted copies come first.** Diff every duplicate pair before classifying it. Copies that differ are either different on purpose (say why and skip) or drifted: one side gained a fix, a guard or a case the other never did. A drifted pair is usually a live bug. Known shapes: a plan that built its starting state twice and so compared a field with itself; an export with its own serializer that missed the escaping the shared one gained; two registrations under one global name, where the last to load wins. Write the test that fails on the stale copy, then unify onto the correct behavior and ship it as a `fix`. "Not identical, so not a duplicate" is not a reason to skip: it is the finding.
- **Identical copies** are worth merging only when they clear the value bar: they must change together, and a missed edit would be a bug (a retry or idempotency rule, a validation, a key both sides spell). A few stable lines that happen to match are not.
- **Accidental duplication** → unify.
- **Inherent two-sided design** (the store/reconciler mirroring is intentional) → do NOT merge; instead add or verify a cross-checking test that reads **both** sides' values in one assertion and fails when they disagree.
- **A spelling both modules share is not inherent.** Label and annotation keys, env names and error codes that operator and backend each hand-write are accidental duplication: unify them in `lego/types` (Phase 4), as `types/renderenv.LabelAppID` already does. That neither module may import the other is no reason to leave them split: both import `lego/types`.
- **What is not a drift guard.** A pair of tests that each compare one side's constant to a literal copied into that side's own test file guards nothing: either side changes together with its own test, and the other side's test stays green. rt-prune-useless-tests deletes such tests. Do not add one. Where a value truly cannot move to `lego/types` (a dashboard rule that mirrors a server rule), put the cases in one fixture file both sides' tests read, as `lego/backend/testdata/validation/*.json` does.

## Phase 4 — Unify

Pick the canonical home along the import DAG: code both operator and backend need goes to `lego/types` — the only module both import. `cli` stays standalone; never fold it into a sibling. Respect the depguard-fenced id convention (ids mint only via `lego/backend/internal/id`, `id.New(kind)`). Migrate all callers, delete the duplicate (a shared constant's callers include every hand-written copy of its value: find them with `git grep -n '"<value>"' -- lego`, and migrate them in the same change, not only the pair you started from), and confirm nothing dangles with `go build ./...` and `go vet ./...` in every module that used it. A helper left unused is caught by the deadcode pass in CI's `make lint`.

## Phase 5 — Verify and report

Types changes ripple everywhere: `go build ./...` and `go vet ./...` in every module (this compiles their tests without running them), then the targeted tests of the packages that changed and of their importers, plus the targeted dashboard checks if touched. The module-wide suites run in CI. Report per the standard format.

## Final phase — Ship

After all required gates pass, read and execute [the ship skill](../ship/SKILL.md) for the routine's completed changes. Stage only intended changes from this run; preserve unrelated work. Do not end at a plan, findings report, milestone, or request for shipping approval. If there are no changes, report that outcome without creating an empty commit. If a required gate remains blocked or fails after reasonable repairs, report the exact blocker and do not ship unverified changes.

Once the push succeeds, report the shipped HEAD SHA and subject per the ship skill and stop; do not monitor CI or deployment.
