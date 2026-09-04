---
name: routine-abstraction-flattener
description: >-
  Finds over-engineered abstractions — single-implementation interfaces, pass-through layers, generics instantiated at one type, wrapper hooks that add nothing — and flattens them to direct code; module-boundary seams are left to routine-abstraction-police. Use when the user asks to run the abstraction-flattener routine, remove over-engineering, or flatten needless indirection.
---

# Routine: Abstraction Flattener

Indirection has to earn its keep. Flatten what doesn't.

## Routine ground rules

- NEVER `git commit` or `git push` — only the user-invoked `/ship` commits. End by reporting what changed so the user can review and `/ship`.
- Fix everything you find directly in the working tree. If a fix can't be finished safely in this run, revert the partial work and list it under **Deferred** in the report — never leave the tree half-refactored.
- Scope: parse `$ARGUMENTS` per Phase 1 — an optional path/module scope or skill-specific target. No argument = the skill's stated default (usually the whole repo via parallel agents per module: lego/types, lego/operator, lego/backend, lego/cli, dashboard).
- False-positive discipline: if a finding is intentional, load-bearing, or ambiguous, skip it with a one-line reason — don't argue it into a change.
- Every change must pass the verification gates for the modules it touched before being reported as done: operator `make test` + `make lint` (from `lego/operator/`), `cd lego/backend && go test ./...`, `cd lego/cli && go test ./...`, dashboard `yarn test` + typecheck.
- Run `npx prettier@3.4.2 --write` on any markdown touched.
- Report format: **Fixed** (file:line + one-line rationale) / **Deferred** (item + why) / **Skipped** (finding + reason) / **Gates** (commands run + outcomes).

## Phase 1 — Scope

Parse `$ARGUMENTS` as an optional path/module scope. Empty = whole repo via parallel per-module agents.

## Phase 2 — Find candidates

- Go interfaces with exactly one non-test implementation (`grep -rn "^type .* interface" lego/`, then find implementers). A test fake doesn't count as a second implementation — but interfaces that exist specifically to enable envtest/controller-runtime fakes in `lego/operator` are often legitimate; skip those.
- Functions that only forward to one other function with the same arguments.
- Generic type parameters instantiated at exactly one type.
- Dashboard wrapper hooks/components adding no behavior over what they wrap.

## Phase 3 — Check recorded intent

`git log` the abstraction: added recently for a planned second implementation is different from untouched for a year. Check `docs/` ADRs and `.pm/DO_NOT_DO.md` for recorded intent. Module-boundary seams (types↔operator↔backend contracts) are `/routine-abstraction-police` territory — keep them even when single-impl.

## Phase 4 — Flatten

Inline the single implementation, replace interface parameters with the concrete type, delete pass-through layers, collapse single-use generics. Run the package's tests after each flatten. If a flatten forces test rewrites beyond mechanical type substitution, the abstraction was load-bearing — revert and skip with a note.

## Phase 5 — Verify and report

Full gates for touched modules. Report per the standard format.
