# ADR040 — Direct Stripe usage billing

**Status:** Accepted · **Owner:** backend/control plane · **Originally accepted:** 2026-07-17 · **Revised:** 2026-07-28 (w7/m53)

> The filename is retained for stable links. The original decision used Metronome as an intermediate rating service. w7/m50 replaces that sink with Stripe Billing; Metronome is no longer a runtime dependency.

## Context

[ADR023](ADR023-usage-metering.md) makes `usage_hourly` the durable operational usage record. [ADR030](ADR030-pricing.md) adds an advisory estimate from [`internal/pricing/pricing.yaml`](../lego/backend/internal/pricing/pricing.yaml). m47/m48 then introduced a sealed-row outbox and a real billing object on the REST, GraphQL, MCP, and dashboard usage surfaces.

The intermediate Metronome path duplicated customer, catalog, contract, credential, and collection concerns. Stripe Billing provides usage meters, metered Prices, Subscriptions, invoice previews/history, collection, discounts, and dunning in the same system. The control plane therefore sends sealed usage directly to Stripe.

The architecture keeps three responsibilities separate:

1. **Metering:** bex owns quantities in `usage_hourly`/`usage_monthly` whether billing is enabled or not.
2. **Advisory estimate:** `pricing.yaml` produces the fast `estimatedCost` shown to every workspace.
3. **Authoritative money:** Stripe rates metered Subscription items, produces invoices, and collects payment. The operator never imports billing code.

## Decision

### 1. One catalog, provisioned into Stripe

`pricing.yaml` remains the checked-in rate source. [`scripts/stripe-billing-setup.py`](../scripts/stripe-billing-setup.py) reads it and idempotently provisions the active Stripe catalog:

- ten paid `instance_seconds.<resource_kind>.<tier>` dimensions;
- `egress_gib`, rebased from bytes;
- `build_seconds`;
- `storage_gb_hours`, rebased from GB-seconds;
- `sandbox_compute_seconds`, measured in milli-vCPU-equivalent seconds with memory already folded into the quantity at ADR047's AgentCore reference ratio;
- a stable perpetual 100%-off coupon, `bex-comp-100`.

Free and unknown tiers produce no paid meter event. The setup script validates existing meter mappings and Price currency, cadence, meter, and amount before reusing them; catalog drift fails closed. Runtime subscription provisioning independently resolves the complete set of active Price lookup keys and refuses to create a partial contract.

Stripe owns authoritative rating. `pricing.yaml` remains visible because it also powers the low-latency estimate; an invoice may legitimately differ because of billing periods, discounts, taxes, credits, or provider rounding.

### 2. Workspace identity and Subscriptions

Each billable workspace maps to one Stripe Customer carrying metadata `bex_workspace=tea-…`. The client searches that metadata after a restart, caches the result, and creates with a deterministic idempotency key only when absent. Multiple matching Customers are an error rather than an arbitrary choice.

Each Customer gets one bex Subscription containing every active catalog Price. Subscription metadata carries both `bex_workspace` and `bex_billing_contract=true`; list-before-create plus a deterministic idempotency key makes reconciliation restart-safe. Multiple live bex Subscriptions are rejected to prevent double rating. The Subscription uses `charge_automatically`, so invoicing, payment attempts, and Stripe's configured retry policy are native Stripe behavior.

The billing epoch is also the optional Subscription backdate start. This aligns accepted backfill with the first billed service period.

**Workspace deletion cancels the Subscription, keeps the Customer (w1/m61).** When a workspace is deleted, the local billing rows (`billing_provider_mappings`, `billing_lifecycles`, and the rest of §51's cascade) drop with the tenant row, but that says nothing to Stripe — left alone, the metered Subscription would stay active against a Customer whose workspace no longer exists. So workspace delete runs a pre-cascade purger that calls `CancelContract`: it resolves the one live bex Subscription (skipping already-cancelled ones, so a retry is a no-op) and cancels it immediately with `invoice_now=true`, billing already-exported metered usage one last time. The Customer is deliberately **not** deleted — its invoice history must stay readable after the workspace is gone (the same cancel-not-delete stance as rollback, above). The purger runs before the tenant-row cascade specifically so the provider-mapping row still resolves the ids; it is gated on `BEX_STRIPE_SECRET_KEY`, so with Stripe disabled workspace delete is byte-identical. **Accepted loss:** usage still inside the seal window (`BEX_STRIPE_SEAL_HOURS`, §3) at delete time was never exported as a meter event and is therefore never billed — a deliberate small forfeit consistent with the seal-then-emit outbox, not a flush. A Mode-A (`billing_excluded`) workspace never had a Customer or Subscription, so its delete cancels nothing.

Payment onboarding augments that existing contract; it never creates another Subscription. A billing-role member or workspace admin (`can_manage_billing`, w1/m60) asks the shared billing core for a setup-mode Checkout Session scoped to the metadata-keyed Customer and complete bex Subscription. Setup mode creates a SetupIntent and saves a future-use payment method without an initial charge. The server supplies `currency=usd` but deliberately omits `payment_method_types`, leaving Stripe's dynamic payment-method selection active. Success/cancel URLs must use the configured dashboard origin, and each request gets a bounded idempotency key plus a randomized eight-letter `integration_identifier` suffix.

The signed `checkout.session.completed` webhook is the source of truth, not the browser redirect. Its handler retrieves the Checkout Session, SetupIntent, PaymentMethod, Customer, and unique Subscription from Stripe; verifies test/live mode and both workspace/Subscription relationships; then idempotently sets the Customer and Subscription default payment method. Replayed delivery converges on the same defaults. A Customer Portal session is likewise created only after `can_manage_billing` authorization (billing role or admin) and unique Customer/Subscription resolution, using an operator-owned configuration that enables payment-method and invoice management but not Subscription cancellation or plan changes.

### 3. Seal, emit, then stamp

The m47 outbox remains:

1. roll usage into `usage_hourly`;
2. wait for the rewrite horizon (`BEX_STRIPE_SEAL_HOURS`, default 48 hours);
3. select non-excluded `pending` rows at or after `max(BEX_STRIPE_EPOCH, now − 34 days)`; when ADR046's gate is enabled, also require a webhook-stamped payment method or Mode-B comp;
4. ensure the Customer and Subscription;
5. persist the deterministic identifier, meter name, and immutable first-attempt timestamp before any provider call;
6. send one Stripe meter event per paid row; and
7. atomically stamp accepted rows `emitted`, hold permanent rejects, and leave retryable outcomes `pending`.

The 34-day cap stays inside Stripe's 35-calendar-day past-timestamp limit and prevents first enable from billing unbounded history. A future timestamp is never manufactured.

Under [ADR046](ADR046-payment-onboarding-and-paid-gating.md), cardless rows remain pending rather than being stamped or treated as errors. That prevents `ensureBillingSetup` from creating a Customer/Subscription for drive-by free workspaces; once the marker binds, still-in-horizon rows enter the ordinary idempotent path on the next pass. Mode-A exclusion and Mode-B rated-but-free comp behavior are unchanged.

The meter-event `identifier` is the SHA-256 hash of normalized resource kind, service id, meter kind, tier, and UTC hour. Stripe enforces identifier uniqueness within a rolling 24-hour period. Because the emitter retries hourly, an ordinary crash between event acceptance and the local stamp is deduplicated. This is a bounded provider guarantee, not mathematically strict exactly-once delivery. `billing_export_attempted_at` therefore never moves: once an unstamped attempt exceeds 24 hours, the row becomes `ambiguous`, a durable `stamp_ambiguity` issue is created, and automatic replay stops. An operator must compare the local row with Stripe meter summaries and invoice lines, then explicitly `mark_repaired` or `acknowledge`; ambiguity can never use the automatic `retry` action.

A retryable network/5xx/429 failure leaves its row pending for the next cycle while accepted siblings are stamped independently. A permanent non-429 4xx becomes a durable `permanent_reject` issue and the row becomes `rejected`; it is never falsely stamped emitted and cannot hot-loop the outbox. Repair is dry-run-first and audited. `retry` is allowed only for a definite permanent reject whose event timestamp remains inside the 34-day operational backfill horizon; `mark_repaired` records an externally reconciled outcome without manufacturing another provider event.

The internal control-plane operations API exposes bounded local rows and issue context, never secret values or payment payloads. [`scripts/stripe-billing-reconcile.sh`](../scripts/stripe-billing-reconcile.sh) compares each normalized paid dimension with Stripe's eventually consistent meter-event summaries and invoice-preview lines, fails on mismatch/reject/ambiguity/duplicate evidence, and refuses every live key.

### 4. Invoice reads preserve the m48 public contract

The existing public `billing` shape does not change:

- `currentCost` comes from Stripe's current Subscription invoice preview;
- `invoices` comes from non-draft invoices for that Customer and Subscription;
- Stripe minor-unit USD totals are normalized to the existing major-unit strings;
- each amount is reported as three figures rather than one net number (w6/m98): `amountUsd` is the **gross** rated charge (Stripe's invoice `subtotal` — before invoice-level discounts, before billing credit, before tax), `creditsAppliedUsd` is the [ADR071](ADR071-tenant-billing-credits.md) credit-grant consumption Stripe applied to that invoice, and `amountDueUsd` is Stripe's own `amount_due`. Reading the invoice `total` as the current cost made a period whose usage a credit grant fully covered render as `$0.00` beside a nonzero charge tree; credit consumption is reported beside the charge, never folded into it. Mode B comps stay netted out of `amountDueUsd` and are **not** counted as credit — Stripe lists both in `total_pretax_credit_amounts`, and only `credit_balance_transaction` entries are grant consumption;
- failures degrade to estimate-only rather than turning the usage endpoint into a 500.

REST `GET /v1/usage`, GraphQL `usage`, MCP `get_usage`, and the dashboard continue to share the same service result. The requested calendar usage period still controls the quantities and estimate; Stripe's own invoice period is returned inside each billing amount.

### 5. Exclusions and comps

- **Mode A — structural exclusion:** `tenants.billing_excluded=true` removes a workspace from the outbox selection. Usage reads never create a Customer, so the workspace has no Stripe Customer, Subscription, events, or collection risk. It still sees `estimatedCost`.
- **Mode B — rated but free:** `CompCustomer` ensures the Customer/Subscription and applies `bex-comp-100` with no proration. Stripe still produces rated line items and invoice history, while the perpetual discount makes the net charge zero.

Both controls are admin-only and audited. Tenants cannot edit their own billing status.

### 6. Stripe Tax is an explicit activation gate

Tax collection stays off unless all of these are true in the same Stripe mode:

1. an operator supplies a canonical Stripe Product tax code (`txcd_*`) and explicitly chooses inclusive or exclusive Price behavior;
2. every catalog Product and Price matches those choices;
3. Stripe reports at least one active Tax registration; and
4. payment setup completes for the Subscription.

No code path guesses the legal/product classification or creates a registration. Before the gate passes, REST, GraphQL, MCP, and the dashboard return an explicit unconfigured reason and never claim tax is collected. Once it passes, Checkout collects billing address and tax-ID inputs on Stripe's hosted page, and the completion handler enables `automatic_tax` on the existing Subscription. This is configuration enforcement, not legal advice, registration, filing, or remittance.

### 7. Durable dunning and reversible enforcement

When both Stripe runtime and webhook secrets are configured, `POST /v1/webhooks/stripe` is mounted outside OAuth because `Stripe-Signature` is its credential. The handler reads a bounded body, verifies the signature, stripe-go v86 API version `2026-06-24.dahlia`, and test/live mode before dispatch. It accepts Checkout completion plus invoice payment failed/action-required/succeeded/paid and Subscription created/updated/deleted/paused/resumed. Checkout completion performs the ownership-checked default-payment-method binding above.

Every accepted collection event is reduced to identifiers, mode, provider time, normalized outcome, and bounded reason in one Postgres transaction; the signed body and payment details are never stored. Event id is the idempotency key. A provider timestamp makes stale/reordered events ledger-only facts, while a bounded Stripe Subscription poll feeds the same transition engine when a webhook is missed. Events for objects without immutable `bex_workspace` and `bex_billing_contract=true` Subscription metadata are acknowledged and ignored rather than poisoning retries.

The state ladder is `healthy → grace → enforcing → enforced → recovering → healthy`. A failed/action-required invoice or non-collectible Subscription opens one durable grace deadline. Payment recovery before the deadline cancels enforcement; recovery after enforcement reverses it. Workers claim due rows with a database lease, so replicas and restarts cannot create parallel ownership. `excluded` and `comped` are terminal collection exceptions until an audited operator changes them.

The enforcement matrix is deliberately conservative and reversible:

| Resource | At grace expiry | Billing ownership | Recovery |
| --- | --- | --- | --- |
| web/private/worker/cron App | set `spec.suspended=true` | marker row plus `billing.bex.co/enforcement` annotation, only when previously running | resume only while the same annotation remains |
| managed Postgres | hibernate (`spec.suspended=true`) | same marker/annotation rule | unhibernate only billing-owned changes |
| managed Key Value | scale to zero (`spec.suspended=true`) | same marker/annotation rule | resume only billing-owned changes |
| static site | no serving mutation | none | none |
| PVC/object data, credentials, OpenBao secrets, usage, invoices, audit evidence | no touch and never delete | none | none required |

An already suspended resource gets no billing marker and therefore stays suspended after payment. A deleted or independently unmarked resource is treated as operator/user-owned and is never recreated or resumed. Billable creates, upgrades, deploys, and tenant resume attempts are gated while enforcement/recovery is active. Eventual deletion and debt collection are explicitly outside this milestone.

Each visible transition creates one durable owner notification. SMTP/provider failure retries independently and never rolls back billing state. Operator overrides require an actor and reason, are audit-recorded, and use the same marker-based recovery path; tenant credentials cannot invoke them.

### 8. Customer-billing API contract

One `can_manage_billing`-gated service (billing role or admin, w1/m60) owns readiness and hosted-session authorization — every verb funnels through the same `billing.authorize()` guard, so REST/GraphQL/MCP grant identically. REST exposes `GET /v1/workspaces/{workspaceId}/billing` plus Checkout/Portal session POSTs; GraphQL exposes `workspaceBillingReadiness`, `createBillingCheckoutSession`, and `createBillingPortalSession`; MCP exposes the equivalent `get_billing_readiness` and `create_billing_*_session` tools. All return the same provider-neutral readiness fields or a short-lived hosted URL. Stripe object IDs, server credentials, SetupIntent secrets, and payment details never enter the public result.

The dashboard consumes the GraphQL contract with its existing Kratos HttpOnly session, labels Stripe Test Mode explicitly, and re-reads authoritative readiness after hosted-page returns. A query parameter is presentation state only and cannot mark setup complete.

### 9. Runtime configuration

| Variable | Meaning |
| --- | --- |
| `BEX_STRIPE_SECRET_KEY` | Restricted runtime API key. Unset disables all Stripe behavior and network access. Requires `BEX_CP_DB_URI`. |
| `BEX_STRIPE_API_URL` | Test/stub endpoint override; production leaves it unset. |
| `BEX_STRIPE_SEAL_HOURS` | Finality horizon in hours; default 48, minimum 1. Parsed only when Stripe is enabled. |
| `BEX_STRIPE_EPOCH` | RFC3339 billing-start floor/backdate; unset while enabled defaults to `now − 34d`. Malformed while enabled fails startup. |
| `BEX_STRIPE_WEBHOOK_SECRET` | Endpoint signing secret. Unset means no public Stripe webhook route. |
| `BEX_STRIPE_COMP_COUPON_ID` | Optional coupon override; default `bex-comp-100`. |
| `BEX_STRIPE_PORTAL_CONFIGURATION_ID` | Optional operator-owned `bpc_*` configuration used for every Portal session. |
| `BEX_STRIPE_TAX_CODE` | Operator-confirmed canonical Product tax code. Requires `BEX_STRIPE_TAX_BEHAVIOR` and an active same-mode registration; otherwise Tax stays unconfigured. |
| `BEX_STRIPE_TAX_BEHAVIOR` | Explicit `exclusive` or `inclusive` Price behavior. Requires `BEX_STRIPE_TAX_CODE`. |
| `BEX_STRIPE_DUNNING_ENABLED` | `1` enables the durable grace/enforcement workers. Test mode only at m52; live-mode enable is refused. |
| `BEX_STRIPE_GRACE_PERIOD` | Go duration for the grace deadline; default `168h`, minimum `1m`, parsed only when dunning is enabled. |
| `BEX_STRIPE_RECONCILE_INTERVAL` | Stripe polling backstop cadence; default `5m`, minimum `1m`. |
| `BEX_DASHBOARD_URL` | Trusted first-party dashboard origin for Checkout success/cancel and Portal return URLs. Missing/invalid configuration makes hosted-session creation fail closed. |

`BEX_METRONOME_*` variables are retired and ignored. The Metronome SDK, client, setup runbook, and reconciliation script are removed.

## Activation and rollback

Follow [`docs/runbooks/stripe-billing-setup.md`](runbooks/stripe-billing-setup.md). Test mode is mandatory before live mode. Provision and validate the complete catalog and coupon, create a restricted runtime key, create the version-pinned webhook endpoint, choose an explicit epoch, then deploy the secrets.

Rollback is non-destructive: remove `BEX_STRIPE_SECRET_KEY` and `BEX_STRIPE_WEBHOOK_SECRET` and restart bex-api. Metering and advisory estimates continue; Stripe calls stop. Do not delete Customers, Subscriptions, Prices, meters, or the coupon during rollback. Decide separately whether to pause/cancel live Subscriptions, because disabling emission alone leaves collection policy in Stripe.

## Security and operations

- Setup and runtime credentials are separate. Runtime uses a restricted key with only the Customer/Subscription/session writes and catalog, SetupIntent, PaymentMethod, Tax-registration, and Invoice reads listed in the runbook.
- API keys and webhook secrets stay out of git, logs, command history, audit payloads, and tenant-visible state.
- Test and live Stripe objects are independent. The setup script defaults to test mode and requires explicit `--live` for catalog mutation; the runtime Secret installer additionally refuses `rk_live_*` unless a separate `BEX_STRIPE_ALLOW_LIVE=1` go-live decision is present.
- Customer/subscription ambiguity and catalog mismatch fail closed.
- `/metrics` exposes only account-wide gauges and bounded `operation`/`result` labels. Workspace ids, provider objects, transaction ids, invoices, payment data, request ids, and secrets never become labels.
- Tested alerts cover export backlog, permanent rejects, old ambiguity, local-stamp failure, Customer/Subscription duplicates, invoice-read degradation, webhook signature/version/mode drift, and provisioning failure. Every expression gates on `bex_billing_enabled == 1`, so disabled and retained idle state do not page.

## Consequences

- `internal/billing` has one Stripe client for customer/subscription reconciliation, hosted payment setup, Portal sessions, tax gating, event emission, invoice reads, comps, and verified webhook intake.
- `usage_hourly` remains the operational quantity source, while `billing_export_state`, first-attempt metadata, and `billing_export_issues` make the external commit boundary explicit and repairable.
- `pricing.yaml` now has two deliberate consumers: advisory estimates and Stripe catalog setup. Stripe invoices remain authoritative.
- The operator and CRD contract remain unchanged.
- Payment-method onboarding and Customer Portal access are Stripe-hosted and API-operable across all bex surfaces. Tax remains deliberately unconfigured until the operator supplies the legal/business inputs and same-mode registration. Test-mode non-payment now converges through a durable, reversible lifecycle; live enforcement and eventual termination remain product decisions.
- With ADR046 enforcement enabled, Customer/Subscription provisioning and meter export wait for a bound payment method (or Mode-B comp), while up to 34 days of sealed pending usage remains eligible for later backfill.

## Render parity

Render exposes payment-method updates, accrued charges, and invoice history in its dashboard but no public REST/GraphQL/MCP billing API. bex's raw usage quantities, normalized `billing` object, readiness state, and hosted-session verbs remain deliberate bex-ahead extensions. Since w1/m60 bex gates these financial actions on `can_manage_billing` (`billing or admin`), so — like Render — a dedicated **billing** role reaches billing management without full workspace-admin; the earlier admin-only limitation is closed (before w1/m60 the relation was modelled but had no Go consumer, so billing gated on admin-only `can_manage`). See [the dated comparison](render-artifacts/billing-onboarding.md) and [ADR018](ADR018-render-parity.md).

## Alternatives considered

**Keep Metronome between bex and Stripe.** Rejected: it duplicates catalog, customers, contracts, credentials, and operational failure modes while Stripe already performs rating and collection.

**Send usage directly from the rollup loop.** Rejected: provider availability must not affect usage correctness. The outbox isolates operational metering from external billing.

**Build invoices and payments in bex.** Rejected: rating periods, collection, discounts, retries, tax, and payment reconciliation belong to a billing provider, not the Kubernetes operator or control plane.

**Claim strict exactly-once delivery.** Rejected as an unverifiable promise across the Postgres/Stripe commit boundary. Deterministic identifiers provide bounded deduplication; backlog monitoring and reconciliation cover ambiguity outside Stripe's documented uniqueness window.
