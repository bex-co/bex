---
name: rt-fix-logic-bugs
description: >-
  Picks a tricky logic domain — billing and pricing math, OpenFGA authz scope decisions, blueprint plan diffs — models it explicitly with invariants, truth tables, and property tests, and fixes the real bugs the model surfaces with regression tests. Use when the user asks to run the rt-fix-logic-bugs routine, hunt for logic bugs, or property-test a subsystem.
---

# Routine: Logic Bugfixer

Model tricky business logic explicitly, then let the model find the bugs. The tests written here persist as durable property coverage.

## Routine ground rules

Follow the shared contract in [`../../rt-shared-contract.md`](../../rt-shared-contract.md) (execution, anti-noise value bar, **never opening ledger-only PRs**, board isolation, freshness check, `scripts/rt-pre-pr-check.sh` pre-PR fast gate, and report format).

Logic-bugfixer additions:

- **Scope:** parse `$ARGUMENTS` per Phase 1. Empty = pick the highest-priority domain from Phase 1.
- **No-bug PR threshold:** if the audit finds **no real behavioral bug**, do **not** open a PR solely for a single small test case (<30 lines), a doc/comment tweak, or a ledger update. Ship a `test-only` PR without a code fix only when it adds durable property or cross-product matrix coverage for a previously unpinned invariant family.
- **Cover the whole invariant surface:** whenever you fix a logic invariant (rounding/total agreement, plan-vs-apply gating, redaction, authz enforcement), audit and fix **every sibling projection/consumer of the same struct or contract** in the same PR (e.g., all views of an estimate: `TotalUSD`, `Resources`, and `Meters`) so one invariant never splits across follow-up PRs.

## Phase 1 — Scope

Parse `$ARGUMENTS`: any path, module, package, or logic topic. Do not restrict discovery to a closed list of modules or domains — any module in the repository (`go.mod` / `package.json` trees, present or future) is eligible.

When `$ARGUMENTS` is empty, discover the target dynamically:

1. **Prefer recently changed non-test logic:** run `git log origin/main --since=7.days --name-only --diff-filter=AMCR` across the whole repo, filter to production source files (excluding tests, docs, generated files, and `.pm/`), group by package/feature directory, and pick the highest-risk logic package that was least recently audited in [`ledger.md`](ledger.md).
2. **Fallback when quiet:** if no logic-bearing package changed in the past 7 days, discover modules from the repo layout (`go.mod` / `package.json`) and pick the least-recently-audited high-risk logic package.
3. Announce the chosen package/invariant before modeling.

High-value logic shapes to prioritize (examples, not a closed list):

- **Agreement / preview-vs-execute pairs** — any place where one function predicts, validates, estimates, or previews what another performs, and the two must agree (plan vs apply, admission vs write, estimate vs spend/payment gate, client vs server rule).
- **Billing, pricing, quotas & caps** — monotonicity, cent/rounding alignment across all output projections, proration, limits, and cooldown windows.
- **Authz, scopes, credentials & tenant isolation** — deny-by-default matrices, grant/scope narrowing, expiry, redaction, and cross-tenant boundaries.
- **State machines, lifecycle & scheduling** — deploy/rollback eligibility, session/automation scheduling and overlap policies, queues, retries, idempotence, and storage/ACL rules.

Within the chosen package, take one resource kind or one invariant per run, and check [`ledger.md`](ledger.md) for what earlier runs covered so each run digs somewhere new. Record what this run covered in the ledger's Skipped section (`<package>/<kind or invariant> — audited, holds`) **only when shipping a code or qualifying property-test change**; otherwise report it without committing.

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
