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

Parse `$ARGUMENTS`: `billing` | `authz` | `blueprint-plan` | `agreement` | `agentsessions` | `sandbox` | `storage` | `deploys-webhooks` | a package path.

When empty, prefer **recently changed logic domains**: inspect `git log origin/main --since=7.days --name-only -- lego/backend/internal/ lego/operator/internal/` (excluding `*_test.go`) and pick the domain with active non-test changes that was least recently audited in [`ledger.md`](ledger.md). Fall back to the least-recently-audited domain if no domain changed in the past 7 days. Announce the choice.

Domain map:

- **billing** — `lego/backend/internal/pricing`, `lego/backend/internal/billing`, `lego/backend/internal/usage`, `lego/backend/internal/apps/blueprint_pricing.go`, `stripeBillingGate` in `lego/backend/cmd/api/billing_config.go`
- **authz** — `lego/backend/internal/authz`, `lego/backend/internal/core/scope.go`, `lego/backend/internal/api/scope_matrix.go`, `lego/backend/internal/twofactor`, `lego/backend/internal/managedoidc`
- **blueprint-plan** — `lego/backend/internal/apps/blueprint_plan.go`, `blueprint_state_plan.go` and the plan/apply cycle in `deploy.go`
- **agreement** — any place where one function predicts or previews what another performs, and the two must agree. Sites: Blueprint plan against `applyDatabase` / `applyKeyValue` / `updateStackService` in `lego/backend/internal/apps/deploy.go`; plan-time admission against write-time admission (`lego/backend/internal/postgres/admission.go`, `lego/backend/internal/keyvalue/tiers.go`); validation/preview verbs against write verbs; estimates against spend/payment gates.
- **agentsessions** — `lego/backend/internal/agentsessions`, `lego/backend/internal/agentsession`, `lego/backend/internal/sshgateway/agentcred` (session lifecycle, automation overlapPolicy/scheduling, spend-cap accounting, PR takeover, message queueing)
- **sandbox** — `lego/backend/internal/sandbox`, `lego/backend/internal/sandboxexec`, `lego/backend/internal/sandboxfiles`, `lego/backend/internal/sessionegress`, `lego/backend/internal/sshgateway` (ticket/grant expiry and narrowing, exec revalidation, egress policy)
- **storage** — `lego/backend/internal/s3gw`, `lego/backend/internal/objectgw`, `lego/backend/internal/objectstorage` (presign/ACL/bucket isolation, lifecycle/CORS rules)
- **deploys-webhooks** — `lego/backend/internal/deploys`, `lego/backend/internal/rollout`, `lego/backend/internal/webhooks`, `lego/backend/internal/notifications` (deploy state machine/rollback eligibility, webhook redaction and delivery)

Within a domain, take one resource kind or one invariant per run, and check the ledger for the ones earlier runs covered so each run digs somewhere new. Record what this run covered in the ledger's Skipped section (`<domain>/<kind or invariant> — audited, holds`) **only when shipping a code or qualifying property-test change**; otherwise report it without committing.

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
