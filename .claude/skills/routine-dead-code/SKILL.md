---
name: routine-dead-code
description: >-
  Deletes provably unreachable code using the repo's wired-in analyzers — whole-program deadcode for Go and knip for the dashboard — plus manual cross-language reachability checks before every deletion. Use when the user asks to run the dead-code routine or sweep for unreachable code.
---

# Routine: Dead-Code Removal

Delete code that provably cannot run, and only that.

## Routine ground rules

- NEVER `git commit` or `git push` — only the user-invoked `/ship` commits. End by reporting what changed so the user can review and `/ship`.
- Fix everything you find directly in the working tree. If a fix can't be finished safely in this run, revert the partial work and list it under **Deferred** in the report — never leave the tree half-refactored.
- Scope: parse `$ARGUMENTS` per Phase 1 — an optional path/module scope or skill-specific target. No argument = the skill's stated default (usually the whole repo via parallel agents per module: lego/types, lego/operator, lego/backend, lego/cli, dashboard).
- False-positive discipline: if a finding is intentional, load-bearing, or ambiguous, skip it with a one-line reason — don't argue it into a change.
- Every change must pass the verification gates for the modules it touched before being reported as done: operator `make test` + `make lint` (from `lego/operator/`), `cd lego/backend && go test ./...`, `cd lego/cli && go test ./...`, dashboard `yarn test` + typecheck.
- Run `npx prettier@3.4.2 --write` on any markdown touched.
- Report format: **Fixed** (file:line + one-line rationale) / **Deferred** (item + why) / **Skipped** (finding + reason) / **Gates** (commands run + outcomes).

## Phase 1 — Scope

Parse `$ARGUMENTS`: `go` | `dashboard` | a path. Empty = both sides.

## Phase 2 — Run the analyzers

- **Go** — exactly what CI runs: `deadcode` (pinned in `lego/operator/Makefile`) with `-tags=e2e -test -filter='^github.com/bex-co/bex/lego/' ./backend/... ./operator/... ./cli/...`. Easiest: `cd lego/operator && make lint` and capture the deadcode section, or invoke `lego/operator/bin/deadcode` with those flags for a raw list.
- **Dashboard** — `cd dashboard && yarn lint:unused` (knip; config in `dashboard/knip.jsonc`).

## Phase 3 — Triage every hit

Before deleting anything:

- Public API surface, CRD types, generated code (`zz_generated*`, codegen outputs), and entry points → skip.
- Helpers used only by the env-gated `t.Skip` tests (envtest/e2e/live guards — there are ~87 of them) are NOT dead; check the skipped tests before believing the tool.
- Grep the symbol repo-wide, including cross-language edges the analyzers can't see: dashboard clients, `scripts/`, workflow YAML, docs examples.

## Phase 4 — Delete

Remove confirmed-dead symbols and files together with their now-orphaned tests and imports. Re-run both analyzers; repeat until the output is clean or every remaining hit has a one-line skip reason.

## Phase 5 — Verify and report

Run gates for touched modules (deadcode runs inside operator `make lint`); dashboard also needs a clean `yarn lint:unused`. Report per the standard format.
