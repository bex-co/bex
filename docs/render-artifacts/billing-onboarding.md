# Billing onboarding and self-service comparison

**Captured:** 2026-07-28 · **Scope:** w7/m51–m52 · **Sources:** current public Render documentation and bex executable contracts

## Render observation

Render documents billing as a workspace-dashboard workflow. Its [Dashboard guide](https://render.com/docs/render-dashboard#manage-billing) says the Billing page can update a payment method, show current accrued usage, and show past invoices. Its [workspace roles guide](https://render.com/docs/team-members#role-permissions) grants payment-method editing to Admin and Billing roles, while Developers may view billing details but cannot edit the payment method. The public docs link general resource management to Render's API/CLI, but do not publish a billing-onboarding endpoint or official MCP action.

## bex contract and deliberate differences

| Concern | Render public contract | bex m51 contract |
| --- | --- | --- |
| Payment setup | Dashboard Billing page | Stripe-hosted setup-mode Checkout launched from dashboard, REST, GraphQL, or MCP |
| Invoice self-service | Dashboard Billing page | Stripe-hosted Customer Portal plus normalized invoice reads on every API surface |
| Authorization | Admin and Billing can edit; Developer view only | Workspace admin required for readiness and both hosted actions |
| Machine surface | No public billing-onboarding verb found | Shared core with three thin adapters; dashboard consumes GraphQL |
| Provider details | Not specified publicly | Test/live mode, Customer/Subscription/payment readiness, and fail-closed Tax state are explicit; provider IDs stay private |

The API-first expansion is intentional under ADR008, not a claim that Render exposes matching endpoints. The admin-only rule is deliberately narrower than Render's Billing role until bex defines and tests a dedicated `can_manage_billing` relation. That is a product-policy residual, not adapter drift.

## Cross-surface evidence

- REST: `GET /v1/workspaces/{workspaceId}/billing`, `POST …/checkout-session`, `POST …/portal-session`.
- GraphQL: `workspaceBillingReadiness`, `createBillingCheckoutSession`, `createBillingPortalSession`.
- MCP: `get_billing_readiness`, `create_billing_checkout_session`, `create_billing_portal_session`.
- Dashboard: Usage → Billing setup, using the GraphQL operations and existing Kratos HttpOnly session.
- `internal/billing/service_test.go` drives all three adapters against one readiness value and verifies authorization before provider access.

No billing action introduced by m51 exists only in the dashboard.

## Payment-failure lifecycle comparison (m52)

Render's current public terms reserve the right to retry a failed charge, suspend access until amounts are paid, or terminate an account. Its FAQ separately says services that would exceed free included usage without a payment method are disabled for the current billing period, and its deploy-hook contract can return 409 when a workspace cannot deploy. The public product docs do not specify a paid-invoice grace duration, a per-resource ownership marker, an automatic recovery algorithm, or a public billing-state API. These observations support only a broad restriction/recovery comparison; they do not justify cloning an undocumented Render schedule.

Sources captured 2026-07-28:

- [Render Terms of Service — failed/declined payment remedies](https://render.com/terms)
- [Render FAQ — no-payment-method service disabling](https://render.com/docs/faq)
- [Deploy Hooks — workspace-level 409 restriction](https://render.com/docs/deploy-hooks)
- [Render Dashboard — billing remains a dashboard workflow](https://render.com/docs/render-dashboard#manage-billing)

| Concern | Render public contract | bex m52 contract |
| --- | --- | --- |
| Grace | No public duration found | One durable operator-configured deadline, default 168h |
| Restriction | Broad right to retry/suspend/terminate; deploys may 409 | Test-mode-only reversible suspension of running App/Postgres/Key Value; static sites and data remain untouched |
| Ownership | Not documented | Exact ledger marker + `billing.bex.co/enforcement` annotation; pre-suspended/re-marked/deleted resources are preserved |
| Recovery | Not documented as an API/state machine | Successful invoice/subscription state enters durable recovery and resumes only exact billing-owned changes |
| Machine visibility | No public billing-lifecycle API found | Identical status/reason/deadline/recovery fields on REST, GraphQL, MCP, and dashboard |
| Admin controls | Dashboard/support policy not publicly specified | Bearer-only control-plane override with actor, bounded reason, before/after audit, exclusion, comp, grace extension, and forced recovery |

This remains a deliberate bex-ahead contract. It borrows Render's observable product intent—unpaid workspaces may be restricted and deploys can be rejected—without inventing parity for undocumented timing or recovery mechanics. Eventual deletion and live enforcement are explicitly excluded.
