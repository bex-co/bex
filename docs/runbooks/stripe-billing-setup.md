# Runbook — Stripe Billing setup and activation

**Owner:** billing workstream · **Source:** [ADR040](../ADR040-billing-metronome.md) · **Status:** live

This runbook provisions and activates direct Stripe usage billing. Run every step in **test mode first**. Test and live objects are separate; a successful test setup does not configure live mode.

## 0. Preconditions

- Stripe CLI installed and authenticated to the intended account.
- Python 3.11+.
- The checked-in price sheet at [`lego/backend/internal/pricing/pricing.yaml`](../../lego/backend/internal/pricing/pricing.yaml).
- A public HTTPS bex-api origin for the webhook.
- `BEX_CP_DB_URI` enabled: the billing outbox lives in the control-plane database.
- A decided billing-start instant. Do not improvise the live epoch during deployment.

Confirm the Stripe account and mode before changing anything. Do not paste API keys into a command line, shell history, issue, or log.

## 1. Provision the test catalog

The script defaults to Stripe **test mode**:

```bash
python3 scripts/stripe-billing-setup.py
```

It reads `pricing.yaml` and creates or validates:

- 13 active usage meters and 13 monthly metered Prices;
- a Product for each newly created Price;
- the perpetual 100%-off coupon `bex-comp-100`;
- deactivation of superseded kind-level shadow meters.

Run it a second time. Every active object and the coupon must report `exists`; the script must not create duplicates. If an active Price with a required lookup key has the wrong meter, currency, cadence, or amount, the script stops. Correct/deactivate the stale Stripe object deliberately, then rerun—never work around a catalog mismatch in bex code.

The lookup/event names are:

- `instance_seconds.service.{starter,standard,pro,pro-plus,pro-max,pro-ultra}`;
- `instance_seconds.postgres.{basic-256mb,basic-1gb}`;
- `instance_seconds.key_value.{starter,standard}`;
- `egress_gib`, `build_seconds`, and `storage_gb_hours`.

Free tiers have no paid meter. Bytes are rebased to GiB and storage GB-seconds to GB-hours so the decimal Price fits Stripe's supported precision.

## 2. Create the restricted runtime key

Use a separate setup/admin credential for the script. For bex-api, create a Stripe **restricted API key** with only:

| Stripe resource | Runtime access | Why |
| --- | --- | --- |
| Customers | Write | search, create, and recover `bex_workspace=tea-…` Customers |
| Subscriptions | Write | list/create the complete metered contract and apply the comp coupon |
| Prices | Read | validate/resolve all active lookup keys before subscription creation |
| Billing meter events | Write | export sealed usage |
| Invoices | Read | current invoice preview and finalized invoice history |

No runtime write access is needed for Products, Prices, meters, Coupons, payment methods, payouts, balances, disputes, or account settings. If Stripe's permission UI groups a read operation with a broader Billing category, choose the narrowest category that makes the documented calls succeed and record that exception in the credential inventory.

Store the value out of band as `BEX_STRIPE_SECRET_KEY` using the same custody pattern as [ADR019](../ADR019-infra-credentials.md). Never commit or log it. Rotate by adding a new restricted key, deploying it, verifying successful calls, then revoking the old key.

## 3. Create and custody the webhook

The receiver uses stripe-go v82's API version `2025-08-27.basil`. Create the endpoint in test mode with that version and only the event m50 accepts:

```bash
stripe webhook_endpoints create \
  --url https://api.example.com/v1/webhooks/stripe \
  --api-version 2025-08-27.basil \
  -d 'enabled_events[0]=invoice.payment_failed'
```

Capture the returned signing `secret` once and store it out of band as `BEX_STRIPE_WEBHOOK_SECRET`. It is a distinct credential from the restricted API key. Do not paste it into git, `.env.example`, logs, or a ticket.

For local signature verification, use Stripe CLI forwarding and its temporary signing secret:

```bash
stripe listen \
  --events invoice.payment_failed \
  --forward-to localhost:8090/v1/webhooks/stripe
```

An invalid/missing `Stripe-Signature` must return 400. A compatible, correctly signed event returns 204. With `BEX_STRIPE_WEBHOOK_SECRET` unset, the public route is not mounted. The handler only verifies and records payment failure today; it does not suspend resources.

## 4. Verify test-mode behavior

Deploy bex-api with the test restricted key, test webhook secret, and an explicit recent test epoch:

```text
BEX_STRIPE_SECRET_KEY=<out-of-band restricted test key>
BEX_STRIPE_EPOCH=2026-07-26T00:00:00Z
BEX_STRIPE_SEAL_HOURS=48
BEX_STRIPE_WEBHOOK_SECRET=<out-of-band test endpoint secret>
BEX_STRIPE_COMP_COUPON_ID=bex-comp-100
```

Leave `BEX_STRIPE_API_URL` unset outside stub tests.

Verify all of the following before live activation:

1. Startup logs say Stripe Billing is enabled without printing either secret.
2. A non-excluded workspace with sealed paid usage gains exactly one Customer with `bex_workspace=tea-…` and one live `bex_billing_contract=true` Subscription containing all 13 items.
3. Meter events use the expected per-tier/rebased event names and deterministic 64-character identifiers.
4. The usage REST, GraphQL, MCP, and dashboard surfaces retain the same fields and show a Stripe invoice preview; a simulated Stripe read failure falls back to `estimatedCost`.
5. A Mode-A excluded workspace creates no Customer or Subscription.
6. Applying Mode B produces a Subscription discount using `bex-comp-100` and a net-zero preview.
7. Replaying a signed `invoice.payment_failed` event returns 204 and logs trusted intake without enforcing suspension.
8. Removing `BEX_STRIPE_SECRET_KEY` and restarting produces no Stripe network traffic and returns estimate-only usage.

## 5. Activate live mode

Live catalog mutation is explicit and irreversible enough to require a second operator:

```bash
python3 scripts/stripe-billing-setup.py --live
python3 scripts/stripe-billing-setup.py --live
```

The second run must reuse every object. Then create a **live** restricted key and webhook endpoint with the same permissions/version/event, and store their live secrets separately from test secrets.

Before deploying:

1. Compare all 13 live lookup keys and rates with `pricing.yaml`.
2. Confirm `bex-comp-100` is valid, perpetual, and exactly 100% off.
3. Confirm the webhook URL, API version, and enabled event.
4. Confirm Stripe's account-level invoice, retry/dunning, and email settings.
5. Confirm customers expected to pay have an approved payment-method onboarding path. m50 does not build that UX.
6. Set an explicit `BEX_STRIPE_EPOCH`; understand that sealed rows after that instant and within the 34-day safety window can be backfilled and charged.

Deploy the live secrets during a staffed window. Watch provisioning errors, event rejects, outbox backlog/stamp failures, invoice-read degradation, duplicate Customer/Subscription alarms, and webhook signature failures.

## 6. Rollback and incident response

To stop new Stripe activity without affecting operational metering:

1. Remove `BEX_STRIPE_SECRET_KEY` and `BEX_STRIPE_WEBHOOK_SECRET` from bex-api.
2. Restart bex-api and verify estimate-only usage plus no Stripe calls.
3. Preserve `usage_hourly`, `emitted_at`, Customers, meters, Prices, coupon, and invoice evidence.
4. Decide separately whether live Subscriptions should remain active, pause, or cancel. Disabling emission does not cancel collection policy in Stripe.

Do not delete Stripe objects during an incident. A stamp failure after Stripe accepted an event is ambiguous; retrying within Stripe's documented identifier-uniqueness window is deduplicated, but older ambiguity requires reconciliation before replay. A permanent 4xx dead-letter requires correcting the catalog/event in Stripe or an explicit billing adjustment.

For a suspected credential leak, disable billing, roll the affected key/endpoint secret, deploy the replacement, verify it, then revoke the old credential. Record the incident without recording secret values.
