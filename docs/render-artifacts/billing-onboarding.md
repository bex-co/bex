# Billing onboarding and self-service comparison

**Captured:** 2026-07-27 · **Scope:** w7/m51 · **Sources:** current public Render documentation and bex executable contracts

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
