---
name: routine-abstraction-police
description: >-
  Audits and fixes layering violations against the workspace import DAG (operator→types←backend; cli standalone; dashboard consumes the API, never reimplements it) and adds depguard rules so violations can't return. Use when the user asks to run the abstraction-police routine, audit module boundaries, or fix layering violations.
---

# Routine: Abstraction Police

Enforce the layering: `operator → types ← backend`; `types` imports no sibling; `cli` imports no lego sibling; the dashboard talks to the backend only through its API client and never reimplements server-authoritative logic.

## Routine ground rules

- NEVER `git commit` or `git push` — only the user-invoked `/ship` commits. End by reporting what changed so the user can review and `/ship`.
- Fix everything you find directly in the working tree. If a fix can't be finished safely in this run, revert the partial work and list it under **Deferred** in the report — never leave the tree half-refactored.
- Scope: parse `$ARGUMENTS` per Phase 1 — an optional path/module scope or skill-specific target. No argument = the skill's stated default (usually the whole repo via parallel agents per module: lego/types, lego/operator, lego/backend, lego/cli, dashboard).
- False-positive discipline: if a finding is intentional, load-bearing, or ambiguous, skip it with a one-line reason — don't argue it into a change.
- Every change must pass the verification gates for the modules it touched before being reported as done: operator `make test` + `make lint` (from `lego/operator/`), `cd lego/backend && go test ./...`, `cd lego/cli && go test ./...`, dashboard `yarn test` + typecheck.
- Run `npx prettier@3.4.2 --write` on any markdown touched.
- Report format: **Fixed** (file:line + one-line rationale) / **Deferred** (item + why) / **Skipped** (finding + reason) / **Gates** (commands run + outcomes).

## Phase 1 — Scope

Parse `$ARGUMENTS` as an optional module/boundary. Empty = all boundaries.

## Phase 2 — Mechanical audit

- Per Go module, diff actual imports against the law: `go list -deps ./...` or `grep -rn '"github.com/bex-co/bex/lego/'`.
- Within-module layer skips: `cmd/` reaching deep into internals instead of the intended package surface.
- Check whether `lego/backend/.golangci.yml` and `lego/operator/.golangci.yml` encode the DAG in depguard rules (backend's depguard currently guards the id convention). Missing DAG rules are themselves a finding.

## Phase 3 — Semantic audit

Parallel agents compare dashboard logic against `lego/backend/internal/apps`: blueprint validation and pricing are the likely offenders. Logic that must stay server-authoritative (pricing, plan diffs, authz decisions) may not live client-side; the dashboard renders API answers.

## Phase 4 — Fix

- Wrong-direction or sideways imports → move the shared code to `lego/types` (the only module both operator and backend may import) and migrate callers.
- Dashboard reimplementations → replace with API data (extend the GraphQL/REST response if a field is missing).
- Add depguard rules encoding the DAG to the golangci configs so `make lint` catches future violations.

## Phase 5 — Verify and report

Types moves ripple: run all Go gates; `make lint` must pass WITH the new depguard rules; dashboard gates if touched. Report per the standard format.
