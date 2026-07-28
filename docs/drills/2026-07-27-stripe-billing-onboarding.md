# 2026-07-27 — Stripe customer-billing onboarding drill

**Result:** PASS

**Scope:** production-hosted bex surfaces backed only by Stripe test-mode objects

**Implementation:** `5c7d55be` (`stripe-go` v86.1.1, API/webhook version `2026-06-24.dahlia`)

## Preconditions and safety

- The runtime credential was an `rk_test_*` restricted key. Every inspected Stripe object reported `livemode=false`; no live key, object, registration, or charge was used.
- Stripe Tax remained fail-closed. The setup credential reported zero active test registrations, no product tax code or tax behavior was configured, API/dashboard readiness reported `product_tax_not_configured`, and the Subscription/invoice preview both reported automatic Tax disabled.
- Production normally used `BEX_STRIPE_EPOCH=2026-07-27T00:00:00Z` and a 48-hour seal. Because no window after that epoch was yet 48 hours old, the drill inserted one isolated non-hour fixture at `2026-07-28T01:01:37Z`, first proved that zero unrelated outbox rows would become eligible, temporarily set the epoch to that exact window and the seal to one hour, rolled bex-api, observed `emitted_at`, then restored the original epoch/48-hour seal and rolled bex-api again. Both replicas returned Ready and `/healthz` returned 200 after restoration.

## Public-path evidence

The disposable workspace was `tea-d9k0p374mq8s739bs8mg`. The same normalized readiness value was returned by production REST, GraphQL, and MCP before and after Checkout:

| Check | Evidence |
| --- | --- |
| Provider mode | `test` |
| Unique billing contract | one Customer and one active 13-item metered Subscription |
| Before Checkout | Customer/Subscription ready; payment method not ready |
| Hosted Checkout | setup-mode, dynamic payment methods, trusted return to `dashboard.bex.co/usage?billing=success` |
| After Checkout | Customer/Subscription/payment method ready on REST, GraphQL, MCP, and dashboard |
| Dashboard | authenticated production Usage page showed `Stripe Test Mode`, the payment-ready state, the Tax-unconfigured warning, and working Checkout/Portal actions |
| Customer Portal | payment-method and billing-information management plus invoice history present; trusted dashboard return present; Subscription cancellation and plan changes absent |
| Invoice preview | test-mode USD draft preview, automatic Tax disabled, amount due `$0.00` for the one-minute fixture |

Authoritative Stripe test objects were:

- Customer `cus_UxwEFLc8hdBKQv`;
- Subscription `sub_1Ty0M7EqsEqs2tLV0AHe8uG9`;
- Checkout Session `cs_test_c1kX6BV4iCfH6UxGSEBd9pqUVcG1u3OaTr6FFpf58X9V6OYq8GyqVoArls` (`mode=setup`, `status=complete`);
- SetupIntent `seti_1Ty0OaEqsEqs2tLVSl6ZhH2J` (`status=succeeded`, `usage=off_session`);
- PaymentMethod `pm_1Ty0QGEqsEqs2tLVeOZoPp0W`, which matched both the Customer invoice default and Subscription default;
- completion event `evt_1Ty0QIEqsEqs2tLVgtEIsCOZ` (`livemode=false`, `pending_webhooks=0`).

The signed completion event was resent to test endpoint `we_1TxxsbEqsEqs2tLVGQIdiQpP`. Delivery drained to zero pending webhooks; Customer and active billing-Subscription cardinality remained one, both defaults remained the same PaymentMethod, and every readiness surface remained payment-ready.

## Cleanup evidence

- All four fixture Checkout Sessions ended non-open: one complete and three explicitly expired; all were test mode.
- The Customer returned `deleted=true`; its Subscription returned `status=canceled`; active Customer search count was zero.
- The public `deleteWorkspace` mutation succeeded. Direct Postgres checks returned `0|0|0` for the tenant, membership, and fixture usage row.
- The Kratos admin lookup returned 404 after deleting the disposable identity.
- No verifier pod remained, the Stripe Secret again contained the original epoch/48-hour seal, both bex-api replicas were Ready, and the public health check passed.

## Residual

Tax activation remains intentionally out of scope until an accountable operator confirms a canonical Stripe product tax code and tax behavior and records an active collecting registration. At that time the runtime restricted key must also be rechecked for the documented `Tax registrations: Read` permission before setting either Tax environment variable.
