---
name: rt-unify-duplicates
metadata:
  internal: "true"
description: >-
  Finds duplicated implementations across modules and merges each into one canonical helper, respecting the workspace import DAG (operator→types←backend; cli imports no sibling); inherent two-sided duplication gets a drift-guard test instead. Use when the user asks to run the rt-unify-duplicates routine, deduplicate code, or unify parallel helpers.
---

# Routine: Dup Unifier

Find copies of one piece of logic that have drifted apart or will, fix the bug the drift caused, and merge them into one canonical home — or, where duplication is inherent to the design, guard it against drift. A merged pair of identical copies is the lesser result; a drifted pair, proved and fixed, is the one this routine exists for.

## Routine ground rules

Follow the shared contract in [`../../rt-shared-contract.md`](../../rt-shared-contract.md) (execution, tidy-routine value bar, batch cleanup & ≤15-line minimum threshold, **never opening ledger-only PRs**, board isolation, freshness check, `scripts/rt-pre-pr-check.sh` pre-PR fast gate, and report format).

- **Scope:** parse `$ARGUMENTS` per Phase 1. Empty = whole repo; the backend↔operator pair is the hot spot.

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

After all required gates pass, read and execute [the land skill](../land/SKILL.md) for the routine's completed changes. Stage only intended changes from this run; preserve unrelated work. Do not end at a plan, findings report, milestone, or request for shipping approval. If there are no changes, report that outcome without creating an empty commit. If a required gate remains blocked or fails after reasonable repairs, report the exact blocker and do not ship unverified changes.

Once the push succeeds, report the shipped HEAD SHA and subject per the land skill and stop; do not monitor CI or deployment.
