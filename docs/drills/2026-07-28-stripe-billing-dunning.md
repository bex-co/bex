# 2026-07-28 — Stripe dunning and recovery drill

**Result:** PASS

**Scope:** production-hosted bex surfaces backed only by Stripe test-mode objects

**Implementation:** `127394891daf168d344759931e113e824ff24435`

## Preconditions and safety

- The runtime credential was a dedicated `rk_test_*` restricted key. Every inspected Customer, Subscription, Invoice, Test Clock, Event, and webhook endpoint reported `livemode=false`; no live key, object, charge, or tenant was used.
- The webhook API version was pinned to `2026-06-24.dahlia`. The final endpoint is `we_1Ty3GjEqsEqs2tLVWUJsPBMg` at `https://api.bex.co/v1/webhooks/stripe`, enabled for the exact ten Checkout, invoice, and Subscription lifecycle events required by m51–m52.
- The disposable workspace was `tea-d9k2ck9rui6s73fh3950`. It contained one initially running App and one App suspended by the user before the drill, so precise recovery could be distinguished from a blanket resume.
- The production defaults were restored after the accelerated drill: dunning enabled, grace `168h`, polling interval `5m`, seal horizon `48h`, and epoch `2026-07-27T00:00:00Z`. Both bex-api replicas were Ready and `/healthz` returned 200.

## Stripe fixtures

- Test Clock: `clock_1Ty20AEqsEqs2tLVrF5TJMnE`;
- Customer: `cus_UxxwoxzvL0yaGg`;
- Subscription: `sub_1Ty20uEqsEqs2tLV4YEGM0QV`;
- failed-then-paid Invoice: `in_1Ty2K6EqsEqs2tLVoJWzSkx2`, amount due and paid 121 cents;
- running App: `tea-d9k2ck9rui6s73fh3950-m52-run-h3950`;
- pre-suspended App: `tea-d9k2ck9rui6s73fh3950-m52-pre-h3950`.

The Test Clock advanced across two renewals. The first 21-cent renewal fell below Stripe's minimum charge and rolled into the Customer balance. A 100-cent test invoice item made the next renewal 121 cents. The attached `pm_card_chargeCustomerFail` failure method moved the Subscription to `past_due`; `pm_card_visa` later paid the same Invoice successfully.

## Lifecycle evidence

1. The test endpoint was deliberately disabled for the first failed renewal. No webhook reached bex, but polling created durable event `poll:in_1Ty2K6EqsEqs2tLVoJWzSkx2:past_due:open:1`, entered grace, and enforced after the accelerated one-minute deadline. This proves the missed-webhook backstop.
2. In grace, REST, GraphQL, and MCP returned the same normalized status, reason, deadline, actions, and timestamps. The running App was still running and the pre-suspended App remained suspended.
3. At enforcement, the three public surfaces agreed on `enforced`. The running App was suspended with the billing ownership marker; the pre-suspended App remained suspended without that marker. Creating another billable service returned 409. No database, key-value resource, secret, usage row, or tenant data was deleted.
4. The audited control-plane recovery command moved `enforced → recovering → healthy`. Audit rows recorded actor `codex-m52-test-drill`, the required reason, and before/after state; the worker recorded the matching recovery audit.
5. A second ordinary Stripe failure, `evt_1Ty2jtEqsEqs2tLVkiQOrT4L`, delivered through the version-pinned endpoint and re-entered grace. An audited ten-minute extension preserved the verification window.
6. For reordering, older failure A `evt_1Ty2neEqsEqs2tLVEXwfeILI` was generated with the endpoint disabled, then newer failure B `evt_1Ty2njEqsEqs2tLVLMO2O2qK` was generated and delivered first. The older real test Event payload was then signed with the endpoint's test secret and accepted with HTTP 204. Its durable row was marked applied without replacing B as the lifecycle source. Replaying A again returned 204, left lifecycle state byte-identical, and the event ledger still contained exactly one A row.
7. The second accelerated grace expired naturally. The worker again recorded `billing.EnforceWorkspace`, and the public verifier passed `enforced` across REST, GraphQL, MCP, and both App intents.
8. Paying the failed Invoice with `pm_card_visa` produced `invoice.payment_succeeded` event `evt_1Ty31gEqsEqs2tLVSws1s2id` and `invoice.paid` event `evt_1Ty31gEqsEqs2tLVsoeIV59K`; both were `livemode=false` with zero pending webhooks. The public surface observed `recovering → healthy`, then the verifier passed on all three APIs. Only the billing-owned App resumed and lost its marker; the pre-suspended App stayed suspended and unmarked.

The final database evidence before cleanup was:

- lifecycle `healthy`, transition version 12, with both `enforced_at` and `recovered_at` populated and no worker error;
- nine applied webhook/poll event rows, including reordered A/B and webhook/poll success;
- six logical notifications (`grace`, `enforced`, `healthy` twice), all `livemode=false`, delivered once with attempt count 1;
- one enforcement row for the running App, marked recovered;
- worker audit rows for both natural enforcements and recoveries, plus the reasoned force-recovery and grace-extension controls.

## Endpoint custody and setup guard

The first drill delivery exposed stale local webhook-secret custody: Stripe's normal delivery succeeded against the production Secret, while a locally signed replay failed. The macOS Keychain entry and `bex-system/bex-stripe` were compared without printing either secret, repaired, and confirmed equal.

Stripe CLI resend of an Event created while its endpoint was disabled continued to report `pending_webhooks=1`, while the immutable account Event object reported the account's older API version. The CLI/API exposed no successful delivery receipt, so that resend was not counted as product evidence; the handler's incompatible-version gate remains fail-closed. The drill instead used an explicitly versioned, correctly signed replay for the reorder assertion. After fixture cleanup, the intermediate endpoint was deleted and a fresh final endpoint was installed. A new foreign-contract `invoice.payment_failed` Event, `evt_1Ty3JAEqsEqs2tLV5fS1G69J`, verified the final endpoint at `pending_webhooks=0`; its generated Customer was immediately deleted. The old Event's historical pending counter remains 1, but the associated endpoint no longer exists.

The drill also found that `scripts/stripe-billing-secret.sh` accepted a reconcile interval below bex-api's one-minute startup minimum. The setup guard now parses composite Go durations, rejects intervals below `1m`, and has offline regression coverage for `59.999s`, `1m`, and `1m30s`.

## Cleanup evidence

- The Checkout Session was expired, the Subscription was canceled, and its `customer.subscription.deleted` Event drained to zero pending webhooks.
- Both drill-generated Customers returned `deleted=true`; the Test Clock lookup returned `No such billingclock` after deletion.
- The obsolete and intermediate webhook endpoints were deleted. Exactly one bex production-URL endpoint remains: `we_1Ty3GjEqsEqs2tLVWUJsPBMg`, enabled, version-pinned, ten-event inventory, and `livemode=false`.
- Public `deleteWorkspace` returned the disposable workspace id. Direct Postgres counts were zero for tenant, membership, Apps, usage, provider mapping, lifecycle, Stripe events, notifications, and enforcement rows. Its 179 audit rows remain intentionally under the audit-retention contract.
- The projector pruned both App CRs and the operator removed their Deployments, Services, and Pod.
- The disposable Kratos identity returned 200 before deletion, DELETE returned 204, and the final lookup returned 404.
- No verifier pod or port-forward remained, the credential-bearing shell history was cleared, both bex-api replicas were Ready, and public health returned 200.

## Residual

Live-mode collection and real-tenant enforcement remain deliberately out of scope. Stripe Tax also remains fail-closed until an accountable operator confirms a canonical product tax code, tax behavior, and active collecting registration.
