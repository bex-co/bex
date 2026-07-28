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
python3 scripts/stripe-billing-setup.py \
  --dashboard-url https://dashboard.example.com
```

It reads `pricing.yaml` and creates or validates:

- 13 active usage meters and 13 monthly metered Prices;
- a Product for each newly created Price;
- the perpetual 100%-off coupon `bex-comp-100`;
- deactivation of superseded kind-level shadow meters;
- one metadata-tagged Customer Portal configuration with payment-method update, billing-information update, and invoice history enabled, and Subscription cancellation/plan changes disabled.

Run it a second time. Every active object and the coupon must report `exists`; the script must not create duplicates. If an active Price with a required lookup key has the wrong meter, currency, cadence, or amount, the script stops. Correct/deactivate the stale Stripe object deliberately, then rerun—never work around a catalog mismatch in bex code.

The lookup/event names are:

- `instance_seconds.service.{starter,standard,pro,pro-plus,pro-max,pro-ultra}`;
- `instance_seconds.postgres.{basic-256mb,basic-1gb}`;
- `instance_seconds.key_value.{starter,standard}`;
- `egress_gib`, `build_seconds`, and `storage_gb_hours`.

Free tiers have no paid meter. Bytes are rebased to GiB and storage GB-seconds to GB-hours so the decimal Price fits Stripe's supported precision.

### Optional Stripe Tax gate

Do not guess a Product tax code or create a registration as a software-deployment step. An accountable operator must first choose the canonical `txcd_*` classification, inclusive/exclusive behavior, and jurisdictions with tax counsel as needed, then create the collecting **test-mode** registration in Stripe. Only after those decisions run:

```bash
python3 scripts/stripe-billing-setup.py \
  --dashboard-url https://dashboard.example.com \
  --tax-code txcd_OPERATOR_CONFIRMED \
  --tax-behavior exclusive
```

The script refuses a partial choice, malformed code, missing active same-mode registration, incompatible existing Price behavior, or mismatched Product code. Omitting both Tax arguments is a safe supported state: catalog provisioning succeeds, the API/dashboard report `product_tax_not_configured`, and automatic tax stays off.

## 2. Create the restricted runtime key

Use a separate setup/admin credential for the script. For bex-api, create a Stripe **restricted API key** with only:

| Stripe resource | Runtime access | Why |
| --- | --- | --- |
| Customers | Write | search, create, and recover `bex_workspace=tea-…` Customers |
| Subscriptions | Write | list/create the complete metered contract, bind its default payment method, enable gated automatic tax, and apply the comp coupon |
| Prices | Read | validate/resolve all active lookup keys before subscription creation |
| Products | Read | verify the operator-confirmed Product tax code when the Tax gate is configured |
| Billing meter events | Write | export sealed usage |
| Invoices | Read | current invoice preview and finalized invoice history |
| Checkout Sessions | Write | create setup-mode hosted sessions and retrieve completed sessions |
| Payment Intents | Read | allow Checkout to resolve account-enabled dynamic payment methods for setup mode |
| Setup Intents | Read | verify the succeeded setup and owned payment method after the webhook |
| Payment Methods | Read | verify Customer ownership before binding defaults |
| Customer Portal | Write | create a short-lived session for the resolved Customer |
| Tax registrations | Read | verify an active same-mode registration when the Tax gate is configured |

No runtime write access is needed for Products, Prices, meters, Coupons, Setup Intents, payment methods, Tax registrations, payouts, balances, disputes, or account settings. Portal **configuration** is setup/admin work; runtime only creates sessions against its `bpc_*` id. If Stripe's permission UI groups a read operation with a broader Billing category, choose the narrowest category that makes the documented calls succeed and record that exception in the credential inventory.

Store the value out of band as `BEX_STRIPE_SECRET_KEY` using the same custody pattern as [ADR019](../ADR019-infra-credentials.md). Never commit or log it. Rotate by adding a new restricted key, deploying it, verifying successful calls, then revoking the old key.

## 3. Create and custody the webhook

The receiver uses stripe-go v86.1.1's API version `2026-06-24.dahlia`. Create the endpoint in test mode with exactly the events consumed by payment onboarding and the m52 lifecycle:

```bash
stripe webhook_endpoints create \
  --url https://api.example.com/v1/webhooks/stripe \
  --api-version 2026-06-24.dahlia \
  -d 'enabled_events[0]=checkout.session.completed' \
  -d 'enabled_events[1]=invoice.payment_failed' \
  -d 'enabled_events[2]=invoice.payment_action_required' \
  -d 'enabled_events[3]=invoice.payment_succeeded' \
  -d 'enabled_events[4]=invoice.paid' \
  -d 'enabled_events[5]=customer.subscription.created' \
  -d 'enabled_events[6]=customer.subscription.updated' \
  -d 'enabled_events[7]=customer.subscription.deleted' \
  -d 'enabled_events[8]=customer.subscription.paused' \
  -d 'enabled_events[9]=customer.subscription.resumed'
```

Capture the returned signing `secret` once and store it out of band as `BEX_STRIPE_WEBHOOK_SECRET`. It is a distinct credential from the restricted API key. Do not paste it into git, `.env.example`, logs, or a ticket.

For local signature verification, use Stripe CLI forwarding and its temporary signing secret:

```bash
stripe listen \
  --events checkout.session.completed,invoice.payment_failed,invoice.payment_action_required,invoice.payment_succeeded,invoice.paid,customer.subscription.created,customer.subscription.updated,customer.subscription.deleted,customer.subscription.paused,customer.subscription.resumed \
  --forward-to localhost:8090/v1/webhooks/stripe
```

An invalid/missing `Stripe-Signature`, incompatible API version, or mode mismatch must return 400. A compatible, correctly signed event returns 204 only after its normalized ledger/state transaction commits. With `BEX_STRIPE_WEBHOOK_SECRET` unset, the public route is not mounted. `checkout.session.completed` retrieves and verifies authoritative objects before binding defaults; lifecycle events require the immutable bex Subscription metadata.

## 4. Verify test-mode behavior

Create a dedicated test restricted key in Stripe Workbench, then put the test key, test webhook secret, and an explicit recent test epoch in the gitignored `.env`:

```text
BEX_STRIPE_SECRET_KEY=<out-of-band restricted test key>
BEX_STRIPE_EPOCH=2026-07-26T00:00:00Z
BEX_STRIPE_SEAL_HOURS=48
BEX_STRIPE_WEBHOOK_SECRET=<out-of-band test endpoint secret>
BEX_STRIPE_COMP_COUPON_ID=bex-comp-100
BEX_STRIPE_PORTAL_CONFIGURATION_ID=<non-secret bpc_* id from setup>
BEX_STRIPE_DUNNING_ENABLED=1
BEX_STRIPE_GRACE_PERIOD=168h
BEX_STRIPE_RECONCILE_INTERVAL=5m
# Set these only after the optional Tax gate above passes:
# BEX_STRIPE_TAX_CODE=<operator-confirmed txcd_*>
# BEX_STRIPE_TAX_BEHAVIOR=exclusive
```

Leave `BEX_STRIPE_API_URL` unset outside stub tests.

Install the values into the cluster selected by `KUBECONFIG`/the current kubectl context. The installer rejects unrestricted `sk_*` keys, malformed webhook secrets, a missing/naive epoch, and a zero seal horizon; it never prints secret bytes:

```bash
DRY_RUN=1 scripts/stripe-billing-secret.sh
scripts/stripe-billing-secret.sh
```

This creates or updates `bex-system/bex-stripe` and rolls bex-api when its Deployment exists. The Deployment references every Secret key as optional, so deleting/omitting the Secret preserves estimate-only behavior. Do not use the Stripe CLI login key as the durable runtime credential: create the dedicated restricted key from §2 even for a long-lived test environment. On macOS, the installer may read a missing test `BEX_STRIPE_WEBHOOK_SECRET` from the login keychain item `bex-stripe-test-webhook`; this fallback is never used with an `rk_live_*` runtime key.

Verify all of the following before live activation:

1. Startup logs say Stripe Billing is enabled without printing either secret.
2. A non-excluded workspace with sealed paid usage gains exactly one Customer with `bex_workspace=tea-…` and one live `bex_billing_contract=true` Subscription containing all 13 items.
3. Meter events use the expected per-tier/rebased event names and deterministic 64-character identifiers.
4. The usage REST, GraphQL, MCP, and dashboard surfaces retain the same fields and show a Stripe invoice preview; a simulated Stripe read failure falls back to `estimatedCost`.
5. A Mode-A excluded workspace creates no Customer or Subscription.
6. Applying Mode B produces a Subscription discount using `bex-comp-100` and a net-zero preview.
7. REST, GraphQL, and MCP return identical workspace-admin billing readiness. A non-admin is denied before any Stripe call, and every hosted return URL outside `BEX_DASHBOARD_URL` is rejected.
8. Setup-mode Checkout uses the existing Customer, USD currency, dynamic payment methods (no `payment_method_types`), no line items, and no Subscription creation. Completing it with a Stripe test payment method makes both Customer and existing Subscription defaults match; replaying the signed completion is harmless.
9. If the optional Tax gate is absent, readiness and invoice preview expose the explicit unconfigured reason and automatic tax stays off. If configured, the catalog and active test registration match and the resulting Subscription/invoice exposes Stripe's tax result.
10. The scoped Portal opens for the same Customer, permits payment/billing-information updates and invoice history, returns only to the trusted dashboard origin, and cannot cancel or change the metered Subscription.
11. With dunning enabled in test mode, a failed/action-required invoice creates one visible grace deadline; duplicate and older events are retained without repeating the transition.
12. Grace expiry suspends only running App/Postgres/Key Value resources, never static content or tenant data; each changed resource has the exact billing ownership marker.
13. A newer successful payment enters recovery and resumes only resources whose exact marker remains. A pre-suspended, deleted, or independently re-marked resource remains untouched.
14. Polling repairs one deliberately missed webhook, owner notification failures retry independently, and the same lifecycle fields appear on REST, GraphQL, MCP, and the dashboard.
15. Removing `BEX_STRIPE_DUNNING_ENABLED` stops lifecycle processing without enabling live enforcement. Removing `BEX_STRIPE_SECRET_KEY` and restarting produces no Stripe network traffic and returns estimate-only usage.

For a disposable production-hosted test workspace, the cross-surface and hosted-session portion is reproducible without placing a Stripe credential in the verifier:

```bash
verify_dir="$(mktemp -d)"
BEX_VERIFY_SESSION_TOKEN=<out-of-band disposable admin session> \
BEX_VERIFY_WORKSPACE_ID=tea-... \
BEX_VERIFY_HOSTED_URL_FILE="$verify_dir/hosted.json" \
  scripts/stripe-billing-verify.sh
```

The script refuses a non-`tea-*` target, requires readiness `mode=test` with one Customer and one complete Subscription, compares normalized REST/GraphQL/MCP results, and validates only the returned Stripe HTTPS hosts. It never prints the Kratos session token or hosted-session URLs. When `BEX_VERIFY_HOSTED_URL_FILE` is set, the script exclusively creates that previously nonexistent file with mode `0600`; use its `checkoutUrl` in a private browser, complete Checkout with a documented Stripe test payment method, then `unlink "$verify_dir/hosted.json" && rmdir "$verify_dir"`. Re-run without the output-file variable and with `BEX_VERIFY_REQUIRE_PAYMENT_READY=1` to prove webhook completion bound the default payment method, then clean up the disposable workspace, Customer, and Subscription.

## 5. Activate live mode

Live catalog mutation is explicit and irreversible enough to require a second operator:

```bash
python3 scripts/stripe-billing-setup.py --live
python3 scripts/stripe-billing-setup.py --live
```

The second run must reuse every object. Then create a **live** restricted key and webhook endpoint with the same permissions/version/events, and store their live secrets separately from test secrets. Tax still requires a separately confirmed live registration and the same explicit code/behavior gate; test registration does not authorize live collection.

Before deploying:

1. Compare all 13 live lookup keys and rates with `pricing.yaml`.
2. Confirm `bex-comp-100` is valid, perpetual, and exactly 100% off.
3. Confirm the webhook URL, API version, and both enabled events.
4. Confirm Stripe's account-level invoice, retry/dunning, and email settings.
5. Confirm Checkout setup and the scoped Customer Portal round trip in test mode, including replay-safe completion and trusted returns.
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
