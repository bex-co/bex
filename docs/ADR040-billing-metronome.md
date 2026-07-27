# ADR040 — Direct Stripe usage billing

**Status:** Accepted · **Owner:** backend/control plane · **Originally accepted:** 2026-07-17 · **Revised:** 2026-07-26 (w7/m50)

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
- a stable perpetual 100%-off coupon, `bex-comp-100`.

Free and unknown tiers produce no paid meter event. The setup script validates existing meter mappings and Price currency, cadence, meter, and amount before reusing them; catalog drift fails closed. Runtime subscription provisioning independently resolves the complete set of active Price lookup keys and refuses to create a partial contract.

Stripe owns authoritative rating. `pricing.yaml` remains visible because it also powers the low-latency estimate; an invoice may legitimately differ because of billing periods, discounts, taxes, credits, or provider rounding.

### 2. Workspace identity and Subscriptions

Each billable workspace maps to one Stripe Customer carrying metadata `bex_workspace=tea-…`. The client searches that metadata after a restart, caches the result, and creates with a deterministic idempotency key only when absent. Multiple matching Customers are an error rather than an arbitrary choice.

Each Customer gets one bex Subscription containing every active catalog Price. Subscription metadata carries both `bex_workspace` and `bex_billing_contract=true`; list-before-create plus a deterministic idempotency key makes reconciliation restart-safe. Multiple live bex Subscriptions are rejected to prevent double rating. The Subscription uses `charge_automatically`, so invoicing, payment attempts, and Stripe's configured retry policy are native Stripe behavior.

The billing epoch is also the optional Subscription backdate start. This aligns accepted backfill with the first billed service period.

### 3. Seal, emit, then stamp

The m47 outbox remains:

1. roll usage into `usage_hourly`;
2. wait for the rewrite horizon (`BEX_STRIPE_SEAL_HOURS`, default 48 hours);
3. select non-excluded, unstamped rows at or after `max(BEX_STRIPE_EPOCH, now − 34 days)`;
4. ensure the Customer and Subscription;
5. send one Stripe meter event per paid row;
6. stamp `emitted_at` only after the batch succeeds.

The 34-day cap stays inside Stripe's 35-calendar-day past-timestamp limit and prevents first enable from billing unbounded history. A future timestamp is never manufactured.

The meter-event `identifier` is the SHA-256 hash of normalized resource kind, service id, meter kind, tier, and UTC hour. Stripe guarantees identifier uniqueness for a rolling window of at least 24 hours. Because the emitter retries hourly, an ordinary crash between event acceptance and the local stamp is deduplicated. This is a bounded provider guarantee, not mathematically strict exactly-once delivery: if the local stamp remains broken longer than Stripe's identifier window, a later retry could be counted twice. Operators must alert on stamp failures/backlog and reconcile before retrying an ambiguity older than that window. Stripe exposes no permanent receipt lookup that can close this two-system commit gap.

A retryable network/5xx/429 failure leaves rows unstamped for the next cycle. A permanent non-429 4xx is logged as a dead-letter and consumed so one malformed event cannot hot-loop the entire outbox; operators reconcile and correct it in Stripe if necessary.

### 4. Invoice reads preserve the m48 public contract

The existing public `billing` shape does not change:

- `currentCost` comes from Stripe's current Subscription invoice preview;
- `invoices` comes from non-draft invoices for that Customer and Subscription;
- Stripe minor-unit USD totals are normalized to the existing major-unit strings;
- failures degrade to estimate-only rather than turning the usage endpoint into a 500.

REST `GET /v1/usage`, GraphQL `usage`, MCP `get_usage`, and the dashboard continue to share the same service result. The requested calendar usage period still controls the quantities and estimate; Stripe's own invoice period is returned inside each billing amount.

### 5. Exclusions and comps

- **Mode A — structural exclusion:** `tenants.billing_excluded=true` removes a workspace from the outbox selection. Usage reads never create a Customer, so the workspace has no Stripe Customer, Subscription, events, or collection risk. It still sees `estimatedCost`.
- **Mode B — rated but free:** `CompCustomer` ensures the Customer/Subscription and applies `bex-comp-100` with no proration. Stripe still produces rated line items and invoice history, while the perpetual discount makes the net charge zero.

Both controls are admin-only and audited. Tenants cannot edit their own billing status.

### 6. Webhook intake, not enforcement

When both Stripe runtime and webhook secrets are configured, `POST /v1/webhooks/stripe` is mounted outside OAuth because `Stripe-Signature` is its credential. The handler reads a bounded body, verifies the signature and compatible Stripe API version, and accepts `invoice.payment_failed`. m50 records the trusted event for operators but does not suspend or delete anything.

The dunning enforcement ladder remains deferred: payment failure → grace → reversible suspension → eventual audited termination. Stripe owns invoice state and payment retries; bex will own only the later resource reaction. A polling backstop, notifications, billing portal, and enforcement state are separate work.

### 7. Runtime configuration

| Variable | Meaning |
| --- | --- |
| `BEX_STRIPE_SECRET_KEY` | Restricted runtime API key. Unset disables all Stripe behavior and network access. Requires `BEX_CP_DB_URI`. |
| `BEX_STRIPE_API_URL` | Test/stub endpoint override; production leaves it unset. |
| `BEX_STRIPE_SEAL_HOURS` | Finality horizon in hours; default 48, minimum 1. Parsed only when Stripe is enabled. |
| `BEX_STRIPE_EPOCH` | RFC3339 billing-start floor/backdate; unset while enabled defaults to `now − 34d`. Malformed while enabled fails startup. |
| `BEX_STRIPE_WEBHOOK_SECRET` | Endpoint signing secret. Unset means no public Stripe webhook route. |
| `BEX_STRIPE_COMP_COUPON_ID` | Optional coupon override; default `bex-comp-100`. |

`BEX_METRONOME_*` variables are retired and ignored. The Metronome SDK, client, setup runbook, and reconciliation script are removed.

## Activation and rollback

Follow [`docs/runbooks/stripe-billing-setup.md`](runbooks/stripe-billing-setup.md). Test mode is mandatory before live mode. Provision and validate the complete catalog and coupon, create a restricted runtime key, create the version-pinned webhook endpoint, choose an explicit epoch, then deploy the secrets.

Rollback is non-destructive: remove `BEX_STRIPE_SECRET_KEY` and `BEX_STRIPE_WEBHOOK_SECRET` and restart bex-api. Metering and advisory estimates continue; Stripe calls stop. Do not delete Customers, Subscriptions, Prices, meters, or the coupon during rollback. Decide separately whether to pause/cancel live Subscriptions, because disabling emission alone leaves collection policy in Stripe.

## Security and operations

- Setup and runtime credentials are separate. Runtime uses a restricted key with only the Customer, Subscription, Price-read, meter-event-write, and Invoice-read permissions listed in the runbook.
- API keys and webhook secrets stay out of git, logs, command history, audit payloads, and tenant-visible state.
- Test and live Stripe objects are independent. The setup script defaults to test mode and requires explicit `--live` for live mutation.
- Customer/subscription ambiguity and catalog mismatch fail closed.
- Alert on billing emitter backlog, provisioning failures, permanent event rejects, invoice-read degradation, and signature failures.

## Consequences

- `internal/billing` has one Stripe client for customer/subscription reconciliation, event emission, invoice reads, comps, and verified webhook intake.
- The m47 `emitted_at` migration and store API remain provider-neutral and unchanged.
- `pricing.yaml` now has two deliberate consumers: advisory estimates and Stripe catalog setup. Stripe invoices remain authoritative.
- The operator and CRD contract remain unchanged.
- Native Stripe collection removes the planned Metronome→Stripe bridge, but payment-method onboarding, tax, customer portal, and non-payment enforcement remain product work.

## Render parity

Render exposes billing in its dashboard but no public REST/GraphQL/MCP billing API. bex's raw usage quantities and normalized `billing` object remain a deliberate bex-ahead extension. The Stripe pivot changes the source of those values, not their adapter shapes or the parity verdict; see [ADR018](ADR018-render-parity.md).

## Alternatives considered

**Keep Metronome between bex and Stripe.** Rejected: it duplicates catalog, customers, contracts, credentials, and operational failure modes while Stripe already performs rating and collection.

**Send usage directly from the rollup loop.** Rejected: provider availability must not affect usage correctness. The outbox isolates operational metering from external billing.

**Build invoices and payments in bex.** Rejected: rating periods, collection, discounts, retries, tax, and payment reconciliation belong to a billing provider, not the Kubernetes operator or control plane.

**Claim strict exactly-once delivery.** Rejected as an unverifiable promise across the Postgres/Stripe commit boundary. Deterministic identifiers provide bounded deduplication; backlog monitoring and reconciliation cover ambiguity outside Stripe's documented uniqueness window.
