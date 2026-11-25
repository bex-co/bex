---
name: rt-remove-dead-code
metadata:
  internal: "true"
description: >-
  Deletes provably unreachable code using the repo's wired-in analyzers — whole-program deadcode for Go and knip for the dashboard — plus manual cross-language reachability checks before every deletion. Use when the user asks to run the rt-remove-dead-code routine or sweep for unreachable code.
---

# Routine: Dead-Code Removal

Delete code that provably cannot run, and only that.

## Routine ground rules

Follow the shared contract in [`../../rt-shared-contract.md`](../../rt-shared-contract.md) (execution, tidy-routine value bar, batch cleanup & ≤15-line minimum threshold, **never opening ledger-only PRs**, board isolation, freshness check, `scripts/rt-pre-pr-check.sh` pre-PR fast gate, and report format).

- **Scope:** parse `$ARGUMENTS` per Phase 1 (optional path or ecosystem filter). Empty = all production code trees per the shared contract (excluding `examples/`, `**/testdata/**`, `**/fixtures/**`, `docs/`, `.pm/`); batch all confirmed removals into one change and skip shipping if the entire sweep yields only one trivial ≤15-line deletion.

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

After all required gates pass, read and execute [the land skill](../land/SKILL.md) for the routine's completed changes. Stage only intended changes from this run; preserve unrelated work. Do not end at a plan, findings report, milestone, or request for shipping approval. If there are no changes, report that outcome without creating an empty commit. If a required gate remains blocked or fails after reasonable repairs, report the exact blocker and do not ship unverified changes.

Once the push succeeds, report the shipped HEAD SHA and subject per the land skill and stop; do not monitor CI or deployment.
