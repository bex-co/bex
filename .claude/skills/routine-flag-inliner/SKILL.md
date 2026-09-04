---
name: routine-flag-inliner
description: >-
  Removes gates for fully-shipped features — stale growthbook-shaped dashboard flags and always-on backend config booleans — by inlining the enabled branch and deleting flag machinery when the last flag goes; subset/beta-targeted flags are out of scope and skipped with a note. Use when the user asks to run the flag-inliner routine, clean up feature flags, or inline shipped gates.
---

# Routine: Shipped-Feature Inliner

A flag whose answer never changes is dead weight. Inline the winning branch, delete the machinery.

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

Parse `$ARGUMENTS` as an optional flag/gate name. Empty = inventory everything.

## Phase 2 — Inventory the gate families

- **Dashboard flags** — the growthbook-shaped local evaluator: `dashboard/src/config/growthbook.ts` + `use-growthbook.ts` (+ tests). Today the only flag is `dashboard-agents`, force-true for one beta workspace.
- **Backend config booleans** — `lego/backend/cmd/api/config.go`: `StripeEnabled`, `StripeDunningEnabled` (via `stripeBillingGate` in `billing_config.go`), `BEX_REQUIRE_PAYMENT_METHOD`.

## Phase 3 — Judge shipped-ness

- A growthbook flag is shipped when `defaultValue` is true with no targeting rules AND it has been that way a while (`git log -p` on the flag lines).
- A subset/beta-targeted flag (like `dashboard-agents` today) is NOT shipped — skip with a note; shipping a beta is a product decision, not this routine's call.
- A config boolean is a stale gate only when it is unconditionally the same value in every environment (check `deploy.yml` and manifests). Booleans that legitimately vary per environment (Stripe off in dev) are KEEP.
- Anything ambiguous and product-visible → skip with a reason; never guess.

## Phase 4 — Inline

For each confirmed-shipped flag: take the enabled branch; delete the disabled branch, the flag definition, hook/call sites, and tests of the disabled path. If the last growthbook flag goes, delete `growthbook.ts`, `use-growthbook.ts`, and their tests entirely — knip (`yarn lint:unused`) will confirm nothing still references them.

## Phase 5 — Verify and report

Dashboard gates (`yarn test` + typecheck + `yarn lint:unused`) if dashboard touched; backend `go test ./...` + operator `make lint` if backend touched. Report per the standard format.

## Final phase — Ship

After all required gates pass, read and execute [the ship skill](../ship/SKILL.md) for the routine's completed changes. Stage only intended changes from this run; preserve unrelated work. Do not end at a plan, findings report, milestone, or request for shipping approval. If there are no changes, report that outcome without creating an empty commit. If a required gate remains blocked or fails after reasonable repairs, report the exact blocker and do not ship unverified changes.

Once the push succeeds, report the shipped HEAD SHA and subject per the ship skill and stop; do not monitor CI or deployment.
