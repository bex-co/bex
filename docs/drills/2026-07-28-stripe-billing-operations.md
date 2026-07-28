# 2026-07-28 — Stripe billing operations acceptance

**Result:** PASS — the production deployment and its dedicated runtime RAK completed reconciliation and operational acceptance using Stripe test mode only

**Scope:** production-hosted bex using Stripe test-mode objects only

**Implementation:** `1df14b2a`, precision/reconciliation follow-up `1408d64f`, deploy run `30336364685`, GitOps digest write-back `876f6d0d`

## Safety and deployment

- Every runtime credential and inspected Stripe object was test mode. No live key, live object, real payment method, or real charge was used.
- Backend, operator/envtest, dashboard, offline billing guards, lint, secret scan, image signing/SBOM, and both CRITICAL CVE gates passed. Production rolled onto `sha256:dc9ead16c949aeecd8f0df9b66ee9803bb5a8d13d6b2d31f6e6c39933399d7a4`; both bex-api replicas became Ready and public `/healthz` returned 200.
- Prometheus loaded all eight billing alert rules. At final handoff none was firing or pending; billing was enabled and the reject, ambiguity, sealed-backlog-age, and open-export-issue signals were all zero.
- The normal seal horizon was restored to `48` hours after the staffed fixture window. Dunning remained test-only with the production defaults: enabled, `168h` grace, `5m` reconciliation.

## Paid · excluded · comp fixtures

The mode-0600 fixture state tracked three disposable workspaces:

- paid `tea-d9k4t7h8ijpc73bo28ng`;
- structurally excluded `tea-d9k4t7h8ijpc73bo28o0`;
- comped `tea-d9k4t7p8ijpc73bo28og`.

At `2026-07-28T04:00:00Z`, each workspace received bounded instance, egress, build, and storage usage. The paid and comp workspaces each reached four `emitted` rows. The excluded workspace retained four local pending estimates and had no Stripe Customer or Subscription. The paid Subscription had no comp coupon; the comp Subscription had exactly one valid, perpetual, test-mode `bex-comp-100` coupon with `percent_off=100`. Its invoice preview had a 3-cent subtotal and a zero total/amount due.

Paid and comp meter summaries matched the local normalized quantities independently: build `30`, egress `1`, starter instance-seconds `3600`, and storage `0.5` GB-hours. All 13 invoice-preview lines were paginated; the four selected lines matched Stripe's integer quantity presentation, exact decimal rates, rounded cent amounts, and USD currency.

## Rejection discovery and repair

The acceptance harness found 84 historical `egress_gib` permanent rejects for test workspace `tea-d98210cbbpdc73dcrkvg`. Every row had the same bounded Stripe 4xx: the event value exceeded the provider's 12-decimal limit. The old emitter used a `float64` bytes→GiB conversion, producing values such as `1.4238451225683093`.

Follow-up `1408d64f` replaced that conversion with deterministic rational arithmetic rounded to 12 decimal places, made reconciliation apply the same per-event rule, paginated the preview's complete line collection, and hardened partial fixture cleanup/database credential handling. Full CI and the production rollout passed before repair.

All 84 issues were exact-classified and dry-run first. Each was then resolved with audited action `retry`, actor `codex-m53-test-operator`, and the reason that Stripe had rejected the original payload and the precision fix was deployed. No ambiguity used retry and no row was blindly marked repaired. After one ordinary bex-api restart, metrics reported `pending=0`, `rejected=0`, `ambiguous=0`, and the open-issue list was empty.

Full-window reconciliation then accounted for 434 emitted rows. Local and Stripe egress both totaled `49.955545777453` GiB; the invoice line showed quantity 49, amount 75 cents, and USD. Standard service usage matched at 129,600 seconds with an 86-cent line. The report had no problems.

## Disable and recovery drill

The exact `bex-system/bex-stripe` Secret was copied to a mode-0600 temporary file without printing it, deleted, and bex-api rolled normally. Both replicas stayed healthy, `/healthz` remained 200, and internal metrics showed `bex_billing_enabled=0` with no backlog/reject/ambiguity signal.

While disabled, the paid fixture received one egress row at `2026-07-28T05:00:00Z`, quantity `1528842059` bytes. It remained `pending`. The exact Secret was restored and byte-compared, bex-api rolled, and the row became one `emitted` transaction. Reconciliation over both fixture hours matched local and Stripe egress at `2.423845122568` GiB; the invoice line showed quantity 2, amount 4 cents, and USD. No Customer, Subscription, or meter quantity was duplicated. The temporary credential backup was then deleted.

## Webhook rotation

Webhook custody used add → deploy → verify → revoke. An incomplete first add produced orphan test endpoint `we_1Ty5LSEqsEqs2tLV2ZFUyWTd`; because the old endpoint and runtime Secret were unchanged, it caused no outage and was explicitly deleted before retrying.

The successful replacement was `we_1Ty5MlEqsEqs2tLVHF7GTVLG`, enabled at API version `2026-06-24.dahlia` for the exact ten Checkout/invoice/Subscription events. Its one-time secret was placed in the macOS Keychain and production Secret without output, both values compared equal, and bex-api rolled.

With both endpoints briefly present, test event `evt_1Ty5PvEqsEqs2tLVX0oA31xc` advanced the valid-webhook timestamp and was retained/applied exactly once; the old endpoint's now-invalid delivery accounted for its one pending webhook. After deleting old endpoint `we_1Ty3GjEqsEqs2tLVWUJsPBMg`, final event `evt_1Ty5RyEqsEqs2tLVOGCvHZvS` was `livemode=false`, reached `pending_webhooks=0`, and was retained/applied exactly once. Exactly one production-URL test endpoint remains.

## Lifecycle and Tax evidence

This acceptance composes the immediately preceding production drills rather than repeating real time in a second disposable lifecycle:

- [customer-billing onboarding](2026-07-27-stripe-billing-onboarding.md) proves setup-mode Checkout completion, default payment-method binding, Customer Portal scope, REST/GraphQL/MCP/dashboard readiness, replay safety, and cleanup;
- [dunning and recovery](2026-07-28-stripe-billing-dunning.md) proves test-clock renewal failure, polling repair, grace, precise enforcement, reordered/duplicate events, payment recovery, cross-surface convergence, and cleanup.

Stripe test Tax settings were active, but the account had zero registrations; the runtime Secret had neither tax code nor tax behavior. The runtime RAK's grouped `Tax Settings, Registrations Read` permission was verified directly against `/v1/tax/registrations`, which returned zero total/active registrations and no live objects. Tax therefore remained explicitly fail-closed, with no guessed classification or jurisdiction.

## Runtime-key acceptance and cleanup

The paid, excluded, and comp verifier passed immediately before cleanup. Cleanup deleted only the two mapped test Customers and three exact tenant ids, proved tenant absence, removed the mode-0600 state file, and retained invoices/audit evidence. No temporary secret or webhook-rotation state remains.

The production Secret's dedicated `rk_test_*` passed strict JSON-body probes for `Billing Meters Read` and grouped `Tax Settings, Registrations Read`; the probes rejected a CLI exit-code-only check because Stripe CLI can exit zero with an API error body. The same runtime key then reconciled `2026-07-27T00:00:00Z` through `2026-07-28T07:00:00Z`: all 434 selected rows were `emitted`, all five meter dimensions matched, all 13 preview lines were read, and `problems=[]`. Egress remained `49.955545777453` GiB/75 cents and standard service usage remained 129,600 seconds/86 cents.

The immediately following `07:00` window contained 11 new pending rows (four build, three egress, four instance) after the normal 48-hour seal horizon was restored. They were intentionally outside the closed reconciliation interval, had no rejected or ambiguous state, and remain ordinary mutable usage rather than an export backlog.
