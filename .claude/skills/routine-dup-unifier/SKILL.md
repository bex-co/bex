---
name: routine-dup-unifier
description: >-
  Finds duplicated implementations across modules and merges each into one canonical helper, respecting the workspace import DAG (operator→types←backend; cli imports no sibling); inherent two-sided duplication gets a drift-guard test instead. Use when the user asks to run the dup-unifier routine, deduplicate code, or unify parallel helpers.
---

# Routine: Dup Unifier

Merge duplicated implementations into one canonical home — or, where duplication is inherent to the design, guard it against drift.

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

Parse `$ARGUMENTS` as an optional path/module scope. Empty = whole repo; the backend↔operator pair is the hot spot.

## Phase 2 — Find duplicates

Start from the known candidates, then scan wider:

- `lego/backend/internal/id` vs `lego/operator/internal/identity` — two id/identity implementations split by module.
- Naming helpers in `lego/backend/internal/store/reconciler.go` vs `lego/operator/internal/controller/app_controller.go` — backend computes desired CRs, operator reconciles them.
- Scan: same-named functions across modules (`grep -rn "^func " lego/{backend,operator}/internal`), copy-paste blocks ≥10 lines, parallel dashboard/backend logic.

## Phase 3 — Classify

- **Accidental duplication** → unify.
- **Inherent two-sided design** (the store/reconciler mirroring is intentional) → do NOT merge; instead add or verify a cross-checking test that fails when the two sides drift.

## Phase 4 — Unify

Pick the canonical home along the import DAG: code both operator and backend need goes to `lego/types` — the only module both import. `cli` stays standalone; never fold it into a sibling. Respect the depguard-fenced id convention (ids mint only via `lego/backend/internal/id`, `id.New(kind)`). Migrate all callers, delete the duplicate, and confirm nothing dangles via `make lint` (whole-program deadcode is wired in).

## Phase 5 — Verify and report

Types changes ripple everywhere: run all Go gates (operator `make test` + `make lint`, backend and cli `go test ./...`), plus dashboard gates if touched. Report per the standard format.

## Final phase — Ship

After all required gates pass, read and execute [the ship skill](../ship/SKILL.md) for the routine's completed changes. Stage only intended changes from this run; preserve unrelated work. Do not end at a plan, findings report, milestone, or request for shipping approval. If there are no changes, report that outcome without creating an empty commit. If a required gate remains blocked or fails after reasonable repairs, report the exact blocker and do not ship unverified changes.

Once the push succeeds, report the shipped HEAD SHA and subject per the ship skill and stop; do not monitor CI or deployment.
