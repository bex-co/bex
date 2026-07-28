# w7 · m51 — Stripe customer billing: Checkout payment setup + Tax + Customer Portal

**Worker:** worker7 **Goal:** make an existing bex Stripe Customer and metered Subscription payment-ready through Stripe-hosted test-mode Checkout, tax-aware invoice semantics, and a self-service Customer Portal, exposed API-first across REST, GraphQL, MCP, and the dashboard **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Upgrade and pin the Stripe SDK/API/webhook contract — **DONE** | 45m | — |
| t002 | Define payment-readiness and hosted-session core contracts — **DONE** | 45m | — |
| t003 | Create scoped Checkout setup sessions for the existing Customer — **DONE** | 1h | t001, t002 |
| t004 | Complete payment setup idempotently and bind the default payment method — **DONE** | 1h | t003 |
| t005 | Create scoped Customer Portal sessions — **DONE** | 45m | t001, t002 |
| t006 | Add the fail-closed Stripe Tax test-mode activation gate — **DONE** | 1h | t001, t002 |
| t007 | Expose checkout, portal, and billing-readiness across REST · GraphQL · MCP — **DONE** | 1h | t003, t004, t005, t006 |
| t008 | Build the dashboard billing onboarding and portal experience — **DONE** | 1h | t007 |
| t009 | Verify payment setup, tax, portal, and invoice preview in prod test mode — **DONE** | 1h | t008 |
| t010 | Render parity — **DONE** | 30m | t009 |
| t011 | Simplify — **DONE** | 30m | t010 |
| t012 | Test coverage — **DONE** | 45m | t011 |
| t013 | Closeout — **DONE** | 10m | t012 |

## Definition of done

From a production-hosted bex workspace while Stripe remains in test mode, an authorized workspace admin can obtain a short-lived Stripe-hosted Checkout URL, attach a test payment method to the workspace's one metadata-keyed Customer and one complete metered Subscription, observe the same payment-readiness state over REST, GraphQL, MCP, and the dashboard, and open a scoped Customer Portal session. Checkout never receives a hard-coded `payment_method_types` list, browser code never receives a Stripe server credential, duplicate completion callbacks are harmless, and a workspace cannot open another workspace's sessions. Tax is enabled only after an operator-confirmed product tax code and a collecting test registration pass a fail-closed activation check; otherwise the UI and API report tax as unconfigured rather than claiming it is collected. The prod verification records Checkout → payment-method binding → tax-aware invoice preview → Portal round trips with test objects only. No `rk_live_*`, live catalog mutation, real charge, or live tax registration is in scope.

## Source + Goal linkage

- **Source:** User request on 2026-07-27 to continue entirely in Stripe test mode until the full billing path runs in prod; direct continuation of `w7/done/m50` and ADR040's recorded payment-method, tax, and portal residuals.
- **Goal linkage:** Advances ADR008's API-first and machine-readable product goals by making the hosted offering's billing onboarding operable by humans and agents through one core contract, while Stripe-hosted pages keep payment details outside bex.
- **Expected outcome:** A real prod workspace can become payment-ready and self-manage its test billing details without an operator editing Stripe objects, and every bex surface reports the same readiness/tax state.
- **Why now:** m50 already creates Customers and Subscriptions and prod already exports to Stripe test mode; without payment onboarding, tax gating, and Portal access, that pipeline cannot exercise collection end to end.
- **Render parity:** Included as t010 because this adds tenant-facing REST, GraphQL, MCP, and dashboard behavior; Stripe-specific bex extensions must stay cross-surface-consistent and any Render drift must be documented.
