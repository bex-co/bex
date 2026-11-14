---
name: rt-remove-dead-code
description: >-
  Deletes provably unreachable code using the repo's wired-in analyzers — whole-program deadcode for Go and knip for the dashboard — plus manual cross-language reachability checks before every deletion. Use when the user asks to run the rt-remove-dead-code routine or sweep for unreachable code.
---

# Routine: Dead-Code Removal

Delete code that provably cannot run, and only that.

## Routine ground rules

- A user request to run this routine authorizes the full cycle: plan, fix, verify, then invoke `$ship` (Codex) or `/ship` (Claude) directly in the same run. Honor explicit user limits such as audit-only or no-ship.
- Keep a concise execution plan in the conversation and update it as findings are confirmed. Continue into implementation without a planning approval pause. Do not create or file a `.pm` milestone/task as a prerequisite or substitute for fixing; use the board only if the user requests it.
- After the verification phase, follow the shipping phase below without asking the user to invoke ship separately. Preserve the ship skill's branch, conflict-resolution, and safety rules.
- Fix everything you find directly in the working tree. If a fix can't be finished safely in this run, revert only your partial work for that fix and list it under **Deferred** in the report — never leave the tree half-refactored.
- Scope: parse `$ARGUMENTS` per Phase 1 — an optional path/module scope or skill-specific target. No argument = the skill's stated default (usually the whole repo via parallel agents per module: lego/types, lego/operator, lego/backend, lego/cli, dashboard).
- False-positive discipline: if a finding is intentional, load-bearing, or ambiguous, skip it with a one-line reason — don't argue it into a change.
- Verify every change with targeted checks before reporting it done, and leave the full suites to CI. Run only what the change can break, one heavy command at a time: never two in parallel, never the same suite twice.
  - Go: `gofmt -l` on the touched files; `go build` and `go vet` of the touched packages; `go test` of those packages and of the packages that import them (find importers with `grep -rl '"<import path>"' --include='*.go' lego/`).
  - Dashboard: `yarn vitest related --run <changed files>`, `yarn eslint <changed files>`, and `yarn typecheck` when TypeScript changed.
  - Never the module-wide suites: `make lint`, `make test`, `go test ./...`, the whole `yarn test` or `yarn lint`. CI runs them on the pull request, against the real Postgres, OpenFGA and OpenBao a sandbox lacks. A failed check comes back to the session as its next turn (ADR047 D11). A run that ships straight to main gets them from the ship skill's gates and main's CI.
- Run `npx prettier@3.4.2 --write` on any markdown touched.
- Before shipping, summarize results in commentary using: **Fixed** (file:line + one-line rationale) / **Deferred** (item + why) / **Skipped** (finding + reason) / **Gates** (commands run + outcomes).

## Phase 1 — Scope

Parse `$ARGUMENTS`: `go` | `dashboard` | a path. Empty = both sides.

## Phase 2 — Run the analyzers

- **Go** — exactly what CI runs: `cd lego/operator && make deadcode && bash hack/deadcode.sh "$PWD/bin/deadcode"` (the pinned `deadcode`, run per module with `-tags=e2e -test` and the shared `types` findings intersected; see the script header). `make lint` runs the same script after golangci-lint.
- **Dashboard** — `cd dashboard && yarn lint:unused` (knip; config in `dashboard/knip.jsonc`).

## Phase 3 — Triage every hit

Before deleting anything:

- Public API surface, CRD types, generated code (`zz_generated*`, codegen outputs), and entry points → skip.
- Helpers used only by the env-gated `t.Skip` tests (envtest/e2e/live guards — there are ~87 of them) are NOT dead; check the skipped tests before believing the tool.
- Grep the symbol repo-wide, including cross-language edges the analyzers can't see: dashboard clients, `scripts/`, workflow YAML, docs examples.

## Phase 4 — Delete

Remove confirmed-dead symbols and files together with their now-orphaned tests and imports. Re-run both analyzers; repeat until the output is clean or every remaining hit has a one-line skip reason.

## Phase 5 — Verify and report

Run the targeted checks (ground rules) for the touched packages. Then run the whole-program analyzers once on the final tree: the CI-exact deadcode script from Phase 2, and a clean `yarn lint:unused` if the dashboard changed. They are this routine's own deliverable, so they stay whole-program. Do not run `make lint`: golangci-lint runs in CI. Report per the standard format.

## Final phase — Ship

After all required gates pass, read and execute [the ship skill](../ship/SKILL.md) for the routine's completed changes. Stage only intended changes from this run; preserve unrelated work. Do not end at a plan, findings report, milestone, or request for shipping approval. If there are no changes, report that outcome without creating an empty commit. If a required gate remains blocked or fails after reasonable repairs, report the exact blocker and do not ship unverified changes.

Once the push succeeds, report the shipped HEAD SHA and subject per the ship skill and stop; do not monitor CI or deployment.
