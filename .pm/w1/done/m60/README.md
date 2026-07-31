# w1 · m60 — Billing-role authz: make `can_manage_billing` real

**Worker:** worker1 **Goal:** Render's BILLING workspace role can actually manage billing — the modelled-but-dead `can_manage_billing` FGA relation gains its Go consumers, so billing-role members reach the billing verbs (Status/Checkout/Portal and whichever reads the sweep assigns) while staying denied everywhere else. **Status:** done (2026-07-31) — `core.RelCanManageBilling` added + classified as an audited write; `billing.authorize()` flipped off admin-only `RelCanManage`, so the three billing verbs (Status/Checkout/Portal) let a billing-role member manage billing on REST/GraphQL/MCP + dashboard while staying denied every resource mutation/sensitive read. Proven against real OpenFGA (`TestMultiWorkspaceTargetingE2E` §9) + unit pins + audit-tier pin; ADR012/024/040/018 + the dated comparison updated; `w7/014` closed; backend suite (real-FGA) + backend lint + dashboard suite/typecheck/lint all green (see Closeout).

## Tasks (in order)

| id   | title                                                                                                        | est | depends_on | status     |
| ---- | ------------------------------------------------------------------------------------------------------------ | --- | ---------- | ---------- |
| t001 | Add `core.RelCanManageBilling` + flip `billing.authorize()` to it                                             | 30m | —          | — **DONE** |
| t002 | Role-matrix sweep: billing-verb gating vs Render's BILLING semantics across REST/GraphQL/MCP + dashboard      | 45m | t001       | — **DONE** |
| t003 | Real-FGA E2E: extend the w7/m61 role-ladder matrix — billing allowed on billing verbs, denials unchanged      | 45m | t001, t002 | — **DONE** |
| t004 | Docs: ADR012/ADR024/ADR040 role semantics + close `w7/014`                                                    | 20m | t002, t003 | — **DONE** |
| t005 | Render parity: billing-role behavior consistent on every surface; compare against render.com and flag drift   | 30m | t004       | — **DONE** |
| t006 | Simplify pass over the changed code                                                                           | 20m | t005       | — **DONE** |
| t007 | Test coverage: unit-level authz + dashboard gating tests beyond the t003 E2E                                  | 45m | t005       | — **DONE** |
| t008 | Closeout                                                                                                     | 10m | t007       | — **DONE** |

## Verb → relation matrix (t002)

The billing feature exposes exactly three authorized verbs, and every surface routes through the one shared `billing.authorize()` guard, so REST/GraphQL/MCP grant identically. All three now gate on `can_manage_billing` (`billing or admin`):

| Service verb | REST | GraphQL | MCP | Relation |
| --- | --- | --- | --- | --- |
| `Status` (read) | `GET /v1/workspaces/{id}/billing` | `workspaceBillingReadiness` | `get_billing_readiness` | `can_manage_billing` |
| `Checkout` | `POST …/billing/checkout-session` | `createBillingCheckoutSession` | `create_billing_checkout_session` | `can_manage_billing` |
| `Portal` | `POST …/billing/portal-session` | `createBillingPortalSession` | `create_billing_portal_session` | `can_manage_billing` |

Decisions recorded:

- **`Status` (readiness + invoice preview) opens to the billing role**, not just admin — it is billing-management visibility (Render's BILLING role views invoices/billing status), and it funnels through the same `authorize()` as Checkout/Portal. The invoice reads in `read.go` (`StripeClient.BillingFor`) are called internally by `Status`, never a separately authorized verb, so they inherit its gate.
- **Usage reads (`internal/usage`) stay on `can_view`** (unchanged) — the billing role already holds `can_view`, so a billing member sees the Usage page and its embedded `billing` object without any billing-specific change; no re-gating needed.
- **Platform comp/exclusion (`store.setBillingExcluded` / `Admin.OverrideBilling`) stays admin/operator-only** — it is not a tenant-facing billing verb and is not exposed on any REST/GraphQL/MCP surface, so it is out of the billing role's reach by design.
- **Dashboard:** the Usage/billing page is shown to every authenticated member (`requireAuth()` only, no role gate) and the `BillingOnboardingCard` surfaces the backend's authorization decision (readiness success → actions; a 403 → the "unavailable / no billing access" state). No admin-only client gate exists to remove; the copy was corrected from "workspace-admin access" to "billing access (billing role or admin)".

## Render parity (t005)

Compared against Render's live-captured role docs (`docs/render-artifacts/{team-members.graphql,billing-onboarding.md}`, render.com/docs/team-members):

- **Match:** Render grants payment-method **editing** to Admin + Billing; bex now gates `Checkout`/`Portal` on `can_manage_billing` (`billing or admin`), so the edit split is identical. Contributor/viewer are denied on both.
- **Match:** billing/invoice **data visibility** — Render shows invoices/accrued usage to developers; bex exposes the same Stripe invoice preview + finalized invoices to any `can_view` member through the usage `billing` object (`GET /v1/usage`), so a developer still sees billing data.
- **Flagged residual divergence (deliberate, recorded — not silently accepted):** Render lets a **developer** open the billing **view** but bex's onboarding **readiness** verb (`Status`) is `can_manage_billing`-gated, so a developer can't open the readiness panel (only the invoice data via usage). bex has no separate `can_view_billing` relation; a finer split is a possible future refinement, not in w1/m60 scope. Documented in `docs/render-artifacts/billing-onboarding.md`.
- **Surface consistency:** all three API surfaces route through the one `billing.authorize()` guard, so REST/GraphQL/MCP grant identically; a denied billing verb returns the same Render error dialect (`{id,message}`, 403) a viewer already gets — the deny path's shape is unchanged, only membership. Proven by `TestMultiWorkspaceTargetingE2E` (real OpenFGA) + `TestBillingVerbsGateOnCanManageBilling` (unit) + the fake-checker `roleladder_test.go` pins.

## Definition of done

- A **billing-role** member completes billing management (`billing.Service.Status`/`Checkout`/`Portal`, plus any read verb t002 assigns to the role) on every surface that exposes it (REST/GraphQL/MCP + dashboard), proven against a real OpenFGA store — not a unit fake.
- The same member remains **denied** all resource mutations and sensitive reads: the w7/m61 role-ladder deny matrix passes with only the intended billing-verb rows flipped to allowed.
- `can_manage_billing` has at least one Go consumer (`git grep RelCanManageBilling` non-empty in `lego/backend/`); no verb still gates billing management on `RelCanManage`.
- ADR012/ADR024/ADR040 record the role's billing reach; `w7/014` is closed into this milestone.
- Backend suite + lint green.

## Source + Goal linkage

- **Source:** `w7/014` (filed 2026-07-31 by the w7/m61 insider role-ladder sweep: `deploy/gitops/authz/model.fga` defines `can_manage_billing: billing or admin` but no Go verb checks it — `billing.authorize()` at `lego/backend/internal/billing/service.go:185` gates on admin-only `RelCanManage`), promoted via `/pm-brainstorm more work for w1` 2026-07-31 with the user picking Option 1 (align to Render) over Option 2 (drop the relation). Closed into this milestone m57-style (w7 is drained; note moved to `w7/done/014.md`).
- **Goal linkage:** Render parity on the members/roles ladder (docs/ADR024-members.md — Render's five roles include BILLING with billing management) + billing correctness (docs/ADR040-billing-metronome.md). Multi-tenant pillar: roles behaving as modelled is part of "multi tenant" (GOAL.md V0 #5).
- **Expected outcome:** the last modelled-but-dead authz relation is exercised; a billing-role member can run billing onboarding/portal without being granted admin; the m61 matrix documents the exact intended reach of every role.
- **Why now:** the finding is a day old with the m61 evidence and tooling fresh (roleGrants pinned to `model.fga`, completeness-guarded matrix, real-FGA E2E harness — all reusable here); a dead relation is drift-in-waiting (a future model edit could widen it unnoticed with no test consuming it); w7 is drained and w1 is the board's active lane (m57 precedent).
- **Render parity:** INCLUDED (t005) — billing spans REST/GraphQL/MCP (`internal/billing/{rest,graphql,mcp}.go`) and the dashboard; the role's reach must land identically on all of them and match Render's documented BILLING-role behavior.

## Closeout (t008)

**Shipped 2026-07-31.** The last modelled-but-dead FGA relation now has a live Go consumer.

- **Code (3 lines of behavior):** `core.RelCanManageBilling = "can_manage_billing"` (`internal/core/base.go`), classified as an audited **write** relation (`internal/core/audit.go` `writeRelations`), and `billing.authorize()` flipped from admin-only `core.RelCanManage` to it (`internal/billing/service.go`). All three billing verbs (`Status`/`Checkout`/`Portal`) funnel through that one guard, so REST/GraphQL/MCP + dashboard grant identically. Wire-facing copy corrected: MCP tool descriptions + REST doc-comment ("billing role or admin"), dashboard `usage.billingUnavailable` (en+zh).
- **Tests:** `TestMultiWorkspaceTargetingE2E` §9 (real OpenFGA) — gwen/billing allowed all three (200/201/201), carl/viewer + fred/contributor + erin/non-member all 403, admin retained; anti-tautology verified (reverting the guard fails exactly the billing-allowed rows). Unit pins: `TestBillingVerbsGateOnCanManageBilling` (each verb asks exactly `can_manage_billing`; granting only `can_manage` denies — bites on revert) + `TestCanManageBillingIsAuditedWrite` (audit-tier pin). Fake-checker matrix `roleladder_test.go` (`representativeVerbRelations` + `modelRelations` + `roleGrants`) re-pinned to the model. Dashboard `billing-onboarding.test.tsx` gating states.
- **Docs:** ADR012 (matrix enforcement + audit tier), ADR024 (billing role now live), ADR040 (authorization prose ×3 + Render-parity), ADR018 (parity ledger), `render-artifacts/billing-onboarding.md` (authorization row + flagged developer-view residual). `w7/014` closed in `w7/done/014.md`.
- **Gates green:** backend `go test -p 1 ./...` against ephemeral Postgres + OpenFGA (all packages ok); backend `make lint-backend` 0 issues; dashboard `yarn test` (1767 tests), `yarn typecheck`, `yarn lint`; markdown prettier clean.
- **Flagged residual (deliberate, recorded — not silently accepted):** Render lets a *developer* view the billing readiness panel; bex gates `Status` on `can_manage_billing`, so a developer sees invoice data via the usage `billing` object but not the onboarding readiness. A `can_view_billing` split is a possible future refinement, out of m60 scope. Documented in `render-artifacts/billing-onboarding.md`.
- **Not committed** — awaiting `/ship` per repo rules.
