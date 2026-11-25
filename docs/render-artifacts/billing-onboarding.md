# Billing onboarding and self-service comparison

**Captured:** 2026-07-28 · **Scope:** w7/m51–m52 · **Sources:** current public Render documentation and bex executable contracts

## Render observation

Render documents billing as a workspace-dashboard workflow. Its [Dashboard guide](https://render.com/docs/render-dashboard#manage-billing) says the Billing page can update a payment method, show current accrued usage, and show past invoices. Its [workspace roles guide](https://render.com/docs/team-members#role-permissions) grants payment-method editing to Admin and Billing roles, while Developers may view billing details but cannot edit the payment method. The public docs link general resource management to Render's API/CLI, but do not publish a billing-onboarding endpoint or official MCP action.

## bex contract and differences

| Concern | Render public contract | bex m51 contract |
| --- | --- | --- |
| Payment setup | Dashboard Billing page | Stripe-hosted setup-mode Checkout launched from dashboard, REST, GraphQL, or MCP; since 2026-08-29 also collected at sign-up on `/setup/payment`, driven by readiness `paymentMethodOnboardingRequired` (ADR075 D7 rev.) |
| Invoice self-service | Dashboard Billing page: past invoices, a PDF per invoice, Pay Now on an unpaid one ([capture](billing-invoices.md)) | Was a parity gap, not a deliberate difference: until w6/m625 a PDF or a payment meant opening the Stripe Customer Portal. Since [w6/m625](../../.pm/w6/blocked/m625/README.md) each invoice on the Billing page has Download PDF and, when open, Pay Now (Stripe's hosted invoice page), and REST, GraphQL and MCP carry the same `invoicePdfUrl` and `hostedInvoiceUrl` |
| Authorization | Admin and Billing can edit; Admin, Developer and Billing view | Billing role or admin (`can_manage_billing`, w1/m60) for both hosted actions; readiness and the usage `billing` object, invoice links included, need `can_view_billing` (Admin, Developer, Billing; w9/m620) |
| Machine surface | No public billing-onboarding verb found | Shared core with three thin adapters; dashboard consumes GraphQL |
| Provider details | Not specified publicly | Test/live mode, Customer/Subscription/payment readiness, and fail-closed Tax state are explicit; provider IDs stay private |

The API-first expansion is intentional under ADR008, not a claim that Render exposes matching endpoints. **Billing-management authorization reaches Render parity as of w1/m60:** the billing verbs gate on a dedicated, now-consumed `can_manage_billing` relation (`billing or admin`), so Render's **Billing** role manages billing without full workspace-admin — matching Render's payment-method edit split (admin + billing edit; others cannot). The earlier admin-only narrowing (a product-policy residual until the relation had a Go consumer and tests, never adapter drift) is closed; the reach is proven end-to-end against a real OpenFGA store (`TestMultiWorkspaceTargetingE2E`).

**Viewing billing matches Render since w9/m620.** This paragraph once flagged that a developer could not open the readiness panel while any `can_view` member, Contributor and Viewer included, read the invoices through the usage `billing` object. w9/m620 added `can_view_billing` (Admin, Developer and Billing, Render's "View billing details"): it gates readiness and the usage `billing` object on REST, GraphQL and MCP, so Contributor and Viewer see neither, and a developer sees both. The invoice links of w6/m625 ride the same gate.

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
| Paying to recover | Access is suspended "until all outstanding amounts are paid in full" ([terms](https://render.com/terms)); each unpaid invoice's Pay Now pays it on Stripe at once | Since [w6/m626](../../.pm/w6/blocked/m626/README.md): while an invoice is overdue the grace and enforcement alert offers Pay now (the invoice's hosted page) and Update payment method, and the lifecycle read names the invoice (`overdueInvoice`: `id`, `hostedInvoiceUrl`) on REST, GraphQL and MCP. A card bound during grace or enforcement, through Checkout or the Billing Portal, pays the unpaid invoices at once, once per invoice and card, and the recovery is recorded as soon as they are paid. Render publishes no recovery timing, so none is claimed |
| Machine visibility | No public billing-lifecycle API found | Identical status/reason/deadline/recovery fields on REST, GraphQL, MCP, and dashboard |
| Admin controls | Dashboard/support policy not publicly specified | Bearer-only control-plane override with actor, bounded reason, before/after audit, exclusion, comp, grace extension, and forced recovery |

This remains a deliberate bex-ahead contract. It borrows Render's observable product intent—unpaid workspaces may be restricted and deploys can be rejected—without inventing parity for undocumented timing or recovery mechanics. Eventual deletion and live enforcement are explicitly excluded.
