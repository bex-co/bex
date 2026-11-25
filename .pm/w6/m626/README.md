# w6 · m626 — A workspace suspended for an unpaid invoice pays it from the dashboard and comes back at once

**Worker:** worker6 **Goal:** while a workspace is in billing grace or enforcement, the Billing page offers Pay now for the overdue invoice, and a newly bound card pays the open invoice at once, so billing-suspended services come back within one reconcile interval instead of at Stripe's next scheduled retry. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Reproduce recovery in Stripe test mode: card update, portal update, time to healthy](t001.md) | 45m | — |
| t002 | [A card bound during grace or enforcement pays the open invoice at once](t002.md) | 1h | t001 |
| t003 | [The lifecycle alert offers Pay now and Update payment method](t003.md) | 45m | t002, w6/m625/t002 |
| t004 | [Test-mode check: an enforced workspace recovers within one reconcile interval](t004.md) | 30m | t003 |
| t005 | [Render parity](t005.md) | 30m | t004 |
| t006 | [Simplify](t006.md) | 20m | t005 |
| t007 | [Test coverage](t007.md) | 45m | t005 |
| t008 | [Closeout](t008.md) | 10m | t006, t007 |

## Definition of done

- In Stripe test mode, a workspace in `enforced` whose Admin or Billing member binds a working card through the Billing page's Update payment method returns to `healthy`, and every service billing enforcement suspended is Running again, within one `BEX_STRIPE_RECONCILE_INTERVAL`, without waiting for Stripe's `next_payment_attempt`.
- The grace and enforced alerts carry Pay now (the overdue invoice's hosted page) and Update payment method; paying through Pay now recovers the same way.
- While an invoice is overdue, the lifecycle read on REST, GraphQL and MCP names it (id and hosted URL).
- A card that is declined again leaves the state unchanged and records the reason, and bex does not retry the same invoice with the same card.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w6` 2026-11-24 (billing & invoicing), item 3, filed by `/pmx them all to w6`. Today the grace and enforced alerts (`billing-lifecycle-alert.tsx`) are text only, and `bindDefaultPaymentMethod` (`lego/backend/internal/billing/sessions.go`) makes a new card the default without paying the open invoice: `Invoices.Pay` runs only in deletion settlement (`settle.go`) and agent-model charges. Recovery therefore waits for Stripe's schedule: Smart Retries make up to 8 attempts in 2 weeks by default, and after a hard decline "retries only execute after detecting a new payment method", still on the schedule (docs.stripe.com/billing/revenue-recovery/smart-retries, read 2026-11-24).
- **Render:** its terms reserve the right to "suspend your access ... until all outstanding amounts are paid in full" (render.com/terms), and each invoice's "Pay Now" pays it on Stripe at once (community answers read through search summaries on 2026-11-24 (community.render.com redirected to render.discourse.group, which did not resolve)). Render publishes no recovery timing, and this milestone does not claim to copy one ([billing-onboarding.md](../../../docs/render-artifacts/billing-onboarding.md) § Payment-failure lifecycle comparison).
- **Recommendation, not a user decision:** pay at bind time, off-session, once per invoice and card. It charges the customer's own new card for what the workspace already owes and nothing else.
- **Portal path:** a card changed inside the Stripe portal reaches bex only as `customer.updated`, which the live endpoint does not send until the user runs the live step of [w2/070](../../w2/blocked/070.md). t002 covers the dashboard path fully and the portal path once that event arrives.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)); [ADR040](../../../docs/ADR040-billing-metronome.md) §7 dunning.
- **Expected outcome:** a workspace suspended for non-payment is back minutes after its customer pays, not at Stripe's next retry days later.
- **Why now:** dunning runs on production (w4/m81 lifted the test-only fence; w21/098 fixed a production dunning bug), so a customer who has already fixed their card can stay suspended for days today.
- **Render parity:** included: the lifecycle read on REST, GraphQL and MCP and the Billing page change.
