# w1 · m62 — Payment-method onboarding + paid-tier gating (ADR046)

**Worker:** worker1 **Goal:** signup stays card-free while any paid intent (non-free tier create or plan change on service/Postgres/Key Value) requires a Stripe-bound payment method — enforced in the service layer across REST/GraphQL/MCP with dashboard just-in-time onboarding — and the meter-event emitter stops auto-provisioning Stripe Customers (and uncollectable invoices) for card-less workspaces. **Status:** todo

## Tasks (in order)

| id   | title                                                                          | est | depends_on   |
| ---- | ------------------------------------------------------------------------------ | --- | ------------ |
| t001 | Capture Render's card-less paid-create error dialect into render-artifacts     | 30m | —            |
| t002 | Marker: migration + `checkout.session.completed` stamp + store accessor        | 45m | —            |
| t003 | `PaymentGate` interface + composition-root wiring + `BEX_REQUIRE_PAYMENT_METHOD` | 45m | t002         |
| t004 | Gate paid intent in `apps` (create/plan change/Blueprint) with the 402 dialect | 45m | t001, t003   |
| t005 | Gate paid intent in `postgres` + `keyvalue` (create/plan update)               | 30m | t004         |
| t006 | Emitter: withhold meter export for card-less workspaces                        | 45m | t002         |
| t007 | Dashboard: paid-intent interception + readiness polling                        | 45m | t005         |
| t008 | Docs: ADR046 → Accepted, ADR018 parity row, env/table sync check               | 20m | t006, t007   |
| t009 | Render parity check (REST/GraphQL/MCP/UI vs render.com)                        | 30m | t008         |
| t010 | Simplify pass over the changed code                                            | 30m | t009         |
| t011 | Test coverage for gate, marker, emitter withholding, env-off invariance        | 45m | t009         |
| t012 | Closeout                                                                       | 15m | t011         |

## Definition of done

With `BEX_STRIPE_SECRET_KEY` and `BEX_REQUIRE_PAYMENT_METHOD=1` set:

- a card-less workspace's non-free-tier create or plan change is refused with `402` / `PAYMENT_REQUIRED` on REST, GraphQL, and MCP, while free-tier creates and every other verb succeed, and `billing_excluded` / `billing_comped` workspaces are exempt;
- completing the existing setup-mode Checkout (the `checkout.session.completed` webhook stamps `billing_provider_mappings.payment_method_bound_at`) makes the same request succeed with no other change;
- the emitter ships no meter events and creates no Stripe Customer/Subscription for card-less workspaces, and back-bills their sealed rows within the 34-day `BackfillHorizon` once a card binds;
- the dashboard intercepts the refusal, runs the `BillingOnboardingView` flow, polls `workspaceBillingReadiness` until `PaymentMethodReady`, and resumes the interrupted action.

With `BEX_REQUIRE_PAYMENT_METHOD` unset, behavior is byte-identical to today (guarded by tests); setting it without `BEX_STRIPE_SECRET_KEY` refuses startup.

## Source + Goal linkage

- **Source:** [docs/ADR046-payment-onboarding-and-paid-gating.md](../../../docs/ADR046-payment-onboarding-and-paid-gating.md) (Proposed 2026-07-31, from the signup-card investigation; user `/pm` handoff same day).
- **Goal linkage:** the monetization/billing pillar (ADR040 lineage: w7/m50–m53 shipped metering→Stripe, w1/m60–m61 billing authz + teardown) — this closes the gap between metering and *collectability*: today a paid tier can be created with zero payment info, and the first unpaid invoice is the first time the platform learns there is no card.
- **Expected outcome:** no uncollectable invoices and no dunning-suspension of free workspaces over cents of egress; no Stripe Customer sprawl from drive-by signups; Render-consistent card-at-paid-intent UX with signup conversion untouched.
- **Why now:** the emitter *currently* auto-provisions a Stripe Customer + Subscription for any workspace with sealed usage (`Emitter.ensureBillingSetup`) — free workspaces accrue real uncollectable egress/build/storage invoices and then walk the dunning path; the production Stripe test-mode path has been live since m53, so this exposure is active, and gating must land before live-mode billing. Render parity task **included**: the change touches the REST/GraphQL/MCP error surface and the dashboard UI.
