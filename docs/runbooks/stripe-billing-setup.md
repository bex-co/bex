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

- 14 active usage meters and 14 monthly metered Prices;
- a Product for each newly created Price;
- the perpetual 100%-off coupon `bex-comp-100`;
- deactivation of superseded kind-level shadow meters;
- one metadata-tagged Customer Portal configuration with payment-method update, billing-information update, and invoice history enabled, and Subscription cancellation/plan changes disabled.

Run it a second time. Every active object and the coupon must report `exists`; the script must not create duplicates. If an active Price with a required lookup key has the wrong meter, currency, cadence, or amount, the script stops. Correct/deactivate the stale Stripe object deliberately, then rerun—never work around a catalog mismatch in bex code.

The lookup/event names are:

- `instance_seconds.service.{starter,standard,pro,pro-plus,pro-max,pro-ultra}`;
- `instance_seconds.postgres.{basic-256mb,basic-1gb}`;
- `instance_seconds.key_value.{starter,standard}`;
- `egress_gib`, `build_seconds`, `storage_gb_hours`, and `sandbox_compute_seconds` (one unit is one milli-vCPU-equivalent second; memory is folded into the quantity at ADR047's AgentCore reference ratio).

Free tiers have no paid meter. Bytes are rebased to GiB and storage GB-seconds to GB-hours so the decimal Price fits Stripe's supported precision.

Workspace plan fees (`pricing.yaml` `workspace.usdPerMonth`: Hobby $0, Pro $17.50, Scale $349.30) are licensed monthly SKUs, not usage meters. This script does not create Stripe Prices for them; the dashboard reads the same YAML via the Go sheet / locale lockstep. Attaching those SKUs to a Customer Subscription is a billing follow-up.

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
| Billing meters | Read | reconcile event names to meter ids and read bounded event summaries |
| Billing meter events | Write | export sealed usage |
| Invoices | Read | current invoice preview and finalized invoice history |
| Checkout Sessions | Write | create setup-mode hosted sessions and retrieve completed sessions |
| Payment Intents | Read | allow Checkout to resolve account-enabled dynamic payment methods for setup mode |
| Setup Intents | Read | verify the succeeded setup and owned payment method after the webhook |
| Payment Methods | Read | verify Customer ownership before binding defaults |
| Customer Portal | Write | create a short-lived session for the resolved Customer |
| Tax Settings, Registrations | Read | verify an active same-mode registration when the Tax gate is configured (`tax_settings_read` in Stripe's restricted-key error) |
| Billing credit grants / credit balance | Read | read the remaining credit balance + active grants for the usage surface's `credits` block (w5/m70); a key without it degrades to credits-omitted, everything else unaffected |

No runtime write access is needed for Products, Prices, meters, Coupons, Setup Intents, payment methods, Tax settings/registrations, payouts, balances, disputes, account settings, or **credit grants** (granting/voiding credit stays operator-side per ADR071's future dedicated credit key; the runtime key only reads balances). Portal **configuration** is setup/admin work; runtime only creates sessions against its `bpc_*` id. If Stripe's permission UI groups a read operation with a broader Billing category, choose the narrowest category that makes the documented calls succeed and record that exception in the credential inventory.

Verify the two operational reads by inspecting the returned JSON, not only the Stripe CLI process status: the CLI can exit zero while the body contains `error.code=more_permissions_required`. `/v1/billing/meters` must succeed with **Billing Meters Read** (`billing_meter_read`), and `/v1/tax/registrations` must succeed with **Tax Settings, Registrations Read** (`tax_settings_read`). Keep the complete error body out of evidence because it identifies the account and restricted key.

Store the value out of band as `BEX_STRIPE_SECRET_KEY` using the same custody pattern as [ADR019](../ADR019-infra-credentials.md). Never commit or log it. The installer accepts only `rk_*`; `rk_live_*` is additionally refused unless `BEX_STRIPE_ALLOW_LIVE=1` records a separate go-live decision. Rotate by adding a new restricted key, deploying it, verifying successful calls, then revoking the old key.

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
2. A non-excluded workspace with sealed paid usage gains exactly one Customer with `bex_workspace=tea-…` and one live `bex_billing_contract=true` Subscription containing all 14 items.
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
16. `/metrics` reports `bex_billing_enabled=1`, bounded outbox/reject/ambiguity gauges, and only `operation`/`result` counter labels. No workspace, Stripe object, transaction, invoice, payment, request, or secret value is a label.
17. Every selected row reaches exactly one local terminal state: accepted rows become `emitted`, permanent 4xx rows become `rejected`, and an unstamped attempt older than 24 hours becomes `ambiguous` and cannot automatically replay.

For a disposable production-hosted test workspace, the cross-surface and hosted-session portion is reproducible without placing a Stripe credential in the verifier:

```bash
verify_dir="$(mktemp -d)"
BEX_VERIFY_SESSION_TOKEN=<out-of-band disposable admin session> \
BEX_VERIFY_WORKSPACE_ID=tea-... \
BEX_VERIFY_HOSTED_URL_FILE="$verify_dir/hosted.json" \
  scripts/stripe-billing-verify.sh
```

The script refuses a non-`tea-*` target, requires readiness `mode=test` with one Customer and one complete Subscription, compares normalized REST/GraphQL/MCP results, and validates only the returned Stripe HTTPS hosts. It never prints the Kratos session token or hosted-session URLs. When `BEX_VERIFY_HOSTED_URL_FILE` is set, the script exclusively creates that previously nonexistent file with mode `0600`; use its `checkoutUrl` in a private browser, complete Checkout with a documented Stripe test payment method, then `unlink "$verify_dir/hosted.json" && rmdir "$verify_dir"`. Re-run without the output-file variable and with `BEX_VERIFY_REQUIRE_PAYMENT_READY=1` to prove webhook completion bound the default payment method, then clean up the disposable workspace, Customer, and Subscription.

## 5. Operations: reconciliation and repair

All commands in this section run against the production deployment but permit only Stripe test-mode keys. Start a local port-forward to the authenticated control-plane API without printing its bearer token, then set `BEX_CP_URL`, `BEX_CP_TOKEN`, and the dedicated `rk_test_*` in a history-disabled shell.

Read-only reconciliation compares local quantities, normalized units, Stripe meter summaries, and the current rated invoice lines. Stripe summaries are eventually consistent, so rerun after a short bounded wait before declaring a mismatch:

```bash
scripts/stripe-billing-reconcile.sh report tea-... \
  2026-07-27T00:00:00Z 2026-07-28T00:00:00Z
scripts/stripe-billing-reconcile.sh issues
```

The report lists `instance_seconds.<resource_kind>.<tier>`, `egress_gib`, `build_seconds`, `storage_gb_hours`, and `sandbox_compute_seconds` separately and exits non-zero for mismatches, rejected/ambiguous rows, duplicate local transaction ids, or duplicate rated lines. It refuses `rk_live_*`/`sk_live_*`, never passes a key on the process command line, and never prints a key or webhook secret.

Stripe permits at most 12 decimal places in a meter-event value. bex therefore normalizes each bytes→GiB and GB-seconds→GB-hours event independently with exact rational arithmetic and half-up rounding to 12 places; reconciliation applies the identical per-event rule before summing. Do not aggregate unrounded values first. Stripe invoice-line `quantity` is an integer presentation even when the underlying meter aggregate is decimal, so the harness separately proves the decimal meter summary and recomputes the rounded cent `amount` from `pricing.unit_amount_decimal`.

An invoice preview embeds only its first page of lines (10 in the current API). The harness follows the preview's `lines.url`, paginates every line, and expands each Price before comparing lookup key, quantity, rate, amount, and currency. Reading only `preview.lines.data` produces a false missing-line result for bex's 14-price contract.

Repair is explicitly dry-run-first:

```bash
scripts/stripe-billing-reconcile.sh repair TRANSACTION_ID acknowledge OPERATOR "reason"
scripts/stripe-billing-reconcile.sh repair TRANSACTION_ID retry OPERATOR "catalog corrected"
scripts/stripe-billing-reconcile.sh repair TRANSACTION_ID mark_repaired OPERATOR "summary proved receipt"
APPLY=1 scripts/stripe-billing-reconcile.sh repair TRANSACTION_ID mark_repaired OPERATOR "summary proved receipt"
```

- `acknowledge` closes the issue while leaving the usage row held as evidence.
- `retry` is only for a definite permanent rejection after its cause is corrected and while its event remains inside the 34-day operational horizon.
- `mark_repaired` stamps a rejected/ambiguous row only after reconciliation proves the provider-side outcome or an explicit provider adjustment was made.
- An ambiguous row can never use `retry`. Every applied resolution requires actor/reason and creates `billing.ResolveExportIssue` audit evidence.

Prometheus scrapes the cluster-internal `bex-api.bex-system.svc:8091/metrics`; the public product listener does not expose process metrics. The tested alerts are `BillingExportBacklog`, `BillingPermanentReject`, `BillingExportAmbiguity`, `BillingLocalStampFailure`, `BillingProviderDuplicate`, `BillingInvoiceReadDegraded`, `BillingWebhookDrift`, and `BillingProvisioningFailure`. Disabled billing (`bex_billing_enabled=0`) suppresses all eight.

For the recurring production-hosted test, provision isolated paid/excluded/comp workspaces with explicit owner/24-hour expiry metadata, seed four bounded dimensions, and clean exactly those ids:

```bash
scripts/stripe-billing-prod-test-fixtures.sh plan

# Staffed test window only: make the fixture's now-2h rows seal without moving
# the account-wide billing epoch. This value is non-secret.
kubectl -n bex-system patch secret bex-stripe --type merge \
  -p '{"stringData":{"BEX_STRIPE_SEAL_HOURS":"1"}}'
kubectl -n bex-system rollout restart deployment/bex-api
kubectl -n bex-system rollout status deployment/bex-api --timeout=300s

scripts/stripe-billing-prod-test-fixtures.sh apply
scripts/stripe-billing-prod-test-fixtures.sh verify
scripts/stripe-billing-reconcile.sh report tea-PAID WINDOW_START WINDOW_END
scripts/stripe-billing-reconcile.sh report tea-COMP WINDOW_START WINDOW_END

# Restore the production rewrite horizon before cleanup/handoff.
kubectl -n bex-system patch secret bex-stripe --type merge \
  -p '{"stringData":{"BEX_STRIPE_SEAL_HOURS":"48"}}'
kubectl -n bex-system rollout restart deployment/bex-api
kubectl -n bex-system rollout status deployment/bex-api --timeout=300s
scripts/stripe-billing-prod-test-fixtures.sh cleanup
```

Use the exact paid/comp ids and one-hour window recorded in the state file; do not paste the file itself into evidence. Wait up to the documented emitter interval plus Stripe's eventual-consistency delay before declaring a reconciliation mismatch. Always restore `48` even if acceptance fails.

The state file is created before the first workspace and atomically updated after each successful create, is mode `0600`, and is gitignored. A failed partial apply is therefore recoverable with `cleanup`. Exclusion is applied before provisioning or seeding can export, so it must have no Customer, Subscription, or events. Paid and comp objects are tagged `bex_fixture=m53` with owner/expiry metadata; comp retains rated lines but uses the perpetual 100%-off coupon. Cleanup validates `livemode=false`, deletes only the two fixture Customers and three exact tenant ids, proves absence, and retains catalog/invoice/audit evidence.

### Rotation and disable drill

Use add → deploy → verify → revoke, never in-place guessing:

1. Create a replacement test restricted key with the exact §2 permission inventory. Keep the old key active.
2. Put the replacement in the out-of-band/keychain custody source, run `DRY_RUN=1 scripts/stripe-billing-secret.sh`, apply it, and wait for both bex-api replicas plus `/healthz` and `/metrics`.
3. Run the read-only reconciliation, create/resolve one disposable Customer/Subscription, and verify no duplicate object or event quantity.
4. Revoke the old restricted key only after the replacement passes. Record key ids/fingerprints and timestamps, never values.
5. For webhook rotation, create a second test endpoint with the exact ten events/API version, deploy its one-time signing secret, prove a new test event reaches `pending_webhooks=0`, then disable/delete the old endpoint. Do not revoke the only working secret first.

To disable, remove the out-of-band `bex-stripe` Secret and roll bex-api. Confirm `bex_billing_enabled=0`, estimate-only surfaces, continuing `usage_hourly` writes, zero provider traffic, and no alerts. Disabling emission does **not** pause or cancel existing Subscriptions. Restore the Secret, reconcile backlog before resuming, and confirm the unique Customer/Subscription counts and meter-summary quantities did not increase twice.

## 6. Activate live mode

Live catalog mutation is explicit and irreversible enough to require a second operator. This section is the executable go-live checklist (w4/m81): run it top to bottom in a staffed window; every prerequisite and decision is settled **before** the window opens.

### 6.0 Out-of-band prerequisites (days before)

1. **Stripe account live activation** — business verification complete and a payout bank account attached in the Stripe Dashboard. Without it, live charges fail regardless of anything below.
2. **Tax decision** — an accountable operator (with tax counsel as needed) confirms the canonical product tax code and behavior. Researched recommendation on record (2026-08-15): `txcd_10102000` (Platform as a Service — business use; bex is Render-class app hosting), alternative `txcd_10101000` (IaaS — business use) if positioned as raw metered compute; behavior `exclusive` (the USD/US/B2B convention). Both are effectively immutable once stamped on the catalog — the setup script refuses behavior changes after the fact. Then create the **live** Tax registration(s) per jurisdiction (Dashboard → Tax → Registrations; status must be Collecting). A test registration does not authorize live collection, and Stripe silently calculates **zero tax** in unregistered jurisdictions. Skipping tax entirely is a supported, explicitly-accepted-risk configuration: omit the pair and collection stays off.
3. **Epoch decision** — pick the exact cutover instant for `BEX_STRIPE_EPOCH` (RFC3339, with timezone). The emitter floor is `max(epoch, now − 34d)`: with epoch = cutover, every pre-cutover (test-period) `usage_hourly` row is permanently below the floor and is never billed to live customers. ⚠️ If the variable is omitted, the binary defaults the floor to `now − 34d` and would backfill up to 34 days of pre-cutover usage onto live cards — `scripts/stripe-billing-secret.sh` requires the variable, so **never** deploy the live Secret by raw `kubectl` edit, only through the installer.
4. **Dunning decision** — live enforcement is an operator choice (the w7/m52 test-only fence is lifted): `BEX_STRIPE_DUNNING_ENABLED=1` runs the grace → enforcement → recovery lifecycle against live non-payment; `0` means invoices are collected by Stripe's own retries only and nothing suspends a non-paying workspace. Record the choice.

Also note the cycle-anchor semantics: every subscription the client creates backdates its start to the **global epoch** (`BackdateStartDate`), so a workspace onboarded months after cutover still gets invoice periods aligned to the cutover anchor, not to its own signup. This bills only actual metered usage — it is a period-boundary alignment, not a retroactive charge.

### 6.1 Provision live Stripe objects (second operator present)

```bash
python3 scripts/stripe-billing-setup.py --live \
  --dashboard-url https://dashboard.bex.co \
  --webhook-url https://api.bex.co/v1/webhooks/stripe \
  # plus, if the tax decision is a go:
  --tax-code txcd_OPERATOR_CONFIRMED --tax-behavior exclusive
python3 scripts/stripe-billing-setup.py --live --dashboard-url ... --webhook-url ...   # identical second run
```

The second run must reuse every object (`exists` on all 14 meters/prices, the coupon, the portal configuration, and the webhook endpoint). The run emits two values to carry into the live `.env`: the scoped portal id (`BEX_STRIPE_PORTAL_CONFIGURATION_ID=bpc_…` — **required** in live mode; the installer refuses a live key without it because the account-default portal allows subscription cancellation) and the webhook signing secret (written to a mode-0600 file under `infra/local/`, never echoed — feed it to the installer as `BEX_STRIPE_WEBHOOK_SECRET`, then delete the file).

Separately create the **live** restricted key with the §2 permission inventory and store it out of band, never alongside test secrets.

### 6.2 Pre-deploy checklist

1. Compare all 14 live lookup keys and rates with `pricing.yaml`.
2. Confirm `bex-comp-100` is valid, perpetual, and exactly 100% off.
3. Confirm the webhook endpoint URL, API version, and exact ten-event allowlist from §3 (the `--webhook-url` provisioning enforces these; this is the human double-check).
4. Confirm Stripe's account-level invoice, retry, and email settings, and that the recorded dunning decision matches `BEX_STRIPE_DUNNING_ENABLED`.
5. Confirm Checkout setup and the scoped Customer Portal round trip in test mode, including replay-safe completion and trusted returns.
6. Confirm `BEX_STRIPE_EPOCH` in the live `.env` equals the decided cutover instant; understand that sealed rows after that instant and within the 34-day safety window can be backfilled and charged.

### 6.3 Cutover window (staffed)

1. **Reset the test-mode payment markers** so no workspace keeps paid access or usage export without a live card (the mode-flip upsert clears them lazily; this closes the window for workspaces not touched immediately):

   ```sql
   UPDATE billing_provider_mappings SET payment_method_bound_at = NULL WHERE livemode = false;
   ```

2. Fill the live `.env`: `BEX_STRIPE_SECRET_KEY=rk_live_…`, `BEX_STRIPE_WEBHOOK_SECRET=whsec_…`, `BEX_STRIPE_EPOCH=<cutover>`, `BEX_STRIPE_PORTAL_CONFIGURATION_ID=bpc_…`, the dunning decision, and the tax pair if activated. Export `BEX_STRIPE_ALLOW_LIVE=1` — the single deliberate live gate.
3. `DRY_RUN=1 bash scripts/stripe-billing-secret.sh`, review the key list and mode line, then run it for real (it writes `bex-system/bex-stripe` and rolls bex-api).
4. Verify startup: the log line reports Stripe Billing enabled with the decided epoch and webhook `true`; `bex_billing_enabled` is 1.

### 6.4 Post-cutover verification

1. Watch the eight `bex_billing_*` alerts through the window: provisioning errors, event rejects, outbox backlog/stamp failures, invoice-read degradation, duplicate Customer/Subscription alarms, and webhook signature/version/mode drift.
2. Confirm readiness self-healed: `workspaceBillingReadiness` reports `customerReady=false` for previously test-bound workspaces until they complete a live Checkout, and (if tax activated) `readiness.tax.configured=true` with `reason=payment_setup_required` until then.
3. Run the read-only drift report against live (allowed under the same flag; mutating `repair` still refuses live keys):

   ```bash
   BEX_STRIPE_ALLOW_LIVE=1 bash scripts/stripe-billing-reconcile.sh report <tea-workspace> <start> <end>
   ```

4. Drive one real workspace through live Checkout with a real card, confirm the portal round trip is scoped (no cancel/plan-change controls), and confirm the first invoice preview shows only post-epoch usage.

## 7. Rollback and incident response

To stop new Stripe activity without affecting operational metering:

1. Remove `BEX_STRIPE_SECRET_KEY` and `BEX_STRIPE_WEBHOOK_SECRET` from bex-api.
2. Restart bex-api and verify estimate-only usage plus no Stripe calls.
3. Preserve `usage_hourly`, `billing_export_state`, first-attempt metadata, `billing_export_issues`, Customers, meters, Prices, coupon, and invoice evidence.
4. Decide separately whether live Subscriptions should remain active, pause, or cancel. Disabling emission does not cancel collection policy in Stripe.

Do not delete Stripe objects during an incident. A stamp failure after Stripe accepted an event is ambiguous; retrying within Stripe's documented 24-hour identifier window is deduplicated, while the application automatically quarantines an older attempt and forbids blind replay. A permanent 4xx dead letter remains durable until an audited acknowledgement, corrected retry, or reconciled repair.

For a suspected credential leak, disable billing, roll the affected key/endpoint secret, deploy the replacement, verify it, then revoke the old credential. Record the incident without recording secret values.
