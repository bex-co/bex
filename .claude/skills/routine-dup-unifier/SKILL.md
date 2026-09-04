---
name: routine-dup-unifier
description: >-
  Finds duplicated implementations across modules and merges each into one canonical helper, respecting the workspace import DAG (operator→types←backend; cli imports no sibling); inherent two-sided duplication gets a drift-guard test instead. Use when the user asks to run the dup-unifier routine, deduplicate code, or unify parallel helpers.
---

# Routine: Dup Unifier

Merge duplicated implementations into one canonical home — or, where duplication is inherent to the design, guard it against drift.

## Routine ground rules

- NEVER `git commit` or `git push` — only the user-invoked `/ship` commits. End by reporting what changed so the user can review and `/ship`.
- Fix everything you find directly in the working tree. If a fix can't be finished safely in this run, revert the partial work and list it under **Deferred** in the report — never leave the tree half-refactored.
- Scope: parse `$ARGUMENTS` per Phase 1 — an optional path/module scope or skill-specific target. No argument = the skill's stated default (usually the whole repo via parallel agents per module: lego/types, lego/operator, lego/backend, lego/cli, dashboard).
- False-positive discipline: if a finding is intentional, load-bearing, or ambiguous, skip it with a one-line reason — don't argue it into a change.
- Every change must pass the verification gates for the modules it touched before being reported as done: operator `make test` + `make lint` (from `lego/operator/`), `cd lego/backend && go test ./...`, `cd lego/cli && go test ./...`, dashboard `yarn test` + typecheck.
- Run `npx prettier@3.4.2 --write` on any markdown touched.
- Report format: **Fixed** (file:line + one-line rationale) / **Deferred** (item + why) / **Skipped** (finding + reason) / **Gates** (commands run + outcomes).

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
