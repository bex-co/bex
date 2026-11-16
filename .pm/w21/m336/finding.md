# Billing lifecycle worker fails every tick with "no rows in result set" (11 workspaces stuck in `enforcing`)

Why: the dunning worker's whole tick aborts on this error, so no lifecycle transition or billing notice is processed, and its log line fires about every 20s.

Found 2026-11-08 while checking the ADR091 meter release in production (`kubectl -n bex-system logs deploy/bex-api`):

- **The error.** `billing lifecycle: no rows in result set` was logged 1,147 times in 7h. It comes from `core.Poll("billing lifecycle", …)` → `Worker.RunOnce` (`lego/backend/internal/billing/worker.go`), and appears 30s after the pods start.
- **The stuck rows.** `bex-db` has 11 `billing_lifecycles` rows with `status = 'enforcing'` and `attempt_count` around 21,000. `last_error` is empty, and every tenant still exists.
  - Some query in the claim → `Enforcer.Enforce` → `FailBillingLifecycleWork` path returns `pgx.ErrNoRows` unclassified. That aborts `RunOnce`, so the error is never recorded on the row.
  - The row stays claimable and the loop repeats.
- **Not caused by w10/m14.** The attempt counts predate it.

Fix:

1. Find the query returning bare ErrNoRows on this path. Classify it (`store.ErrNotFound`), or record it on the row with backoff instead of failing the whole tick.
2. Make one bad row unable to abort the tick: log it and continue.
3. Add a regression test with a lifecycle row whose enforcement dependency is missing.
4. Decide what the 11 workspaces should be (healthy, or properly enforced), and record the outcome here.

**Re-homed 2026-11-11** to [w21/m336](README.md) by `/pm-brainstorm for w21`: the bare `ErrNoRows` is most likely the unconditional `SELECT livemode FROM billing_provider_mappings` in `CompleteBillingLifecycleWork` (`lego/backend/internal/store/billing_lifecycle.go:669`; a lifecycle row can exist without a mapping), and the push worker aborts its pass the same way (`lego/backend/internal/notifications/push_worker.go:215-220`). The origin of this note was `w10/012`.

## Shipped 2026-11-11 (w21 drain) and what is left

Reproduced on a real Postgres first: `CompleteBillingLifecycleWork` on an `enforcing` row with no `billing_provider_mappings` row failed with exactly `no rows in result set` (`TestPGStoreBillingCompletionWithoutProviderMapping` on the old code). The unconditional `SELECT livemode FROM billing_provider_mappings` is the cause; it now runs only when a notice is inserted, through `billingProviderLivemode`, which treats a missing mapping as test mode (the owners are still told). The billing worker's three phases and its per-row store errors no longer abort the tick, and the push worker sends, checks receipts and sweeps even when a feed projection failed.

**Left (t004, t007), needs the user:** a read of the production cluster (`~/.ssh/bex` for `scripts/fetch-app-kubeconfig.sh`, as for w21/012) and one decision. The deploy flips the 11 `enforcing` rows to `enforced` and mails their owners the "compute suspended" notice, weeks late (`Enforce` already ran on every lease, so the workspaces were suspended all along). Decide before the deploy whether each of the 11 should be enforced, excluded or comped (`SetBillingException`, `ForceBillingRecovery`), then record the table here. Commands, once there is access:

```text
kubectl -n bex-system exec <bex-db pod> -- psql -c "SELECT l.workspace_id, l.reason, l.grace_deadline, l.attempt_count, (m.workspace_id IS NOT NULL) AS mapped FROM billing_lifecycles l LEFT JOIN billing_provider_mappings m USING (workspace_id) WHERE l.status='enforcing' ORDER BY l.attempt_count DESC"
kubectl -n bex-system logs deploy/bex-api --since=1h | grep -c 'billing lifecycle: no rows in result set'      # after the deploy: expect 0
kubectl -n bex-system exec <bex-db pod> -- psql -c "SELECT count(*) FROM billing_lifecycles WHERE status='enforcing' AND attempt_count > 100"   # after the deploy: expect 0
```

## Production read 2026-11-16 (t004)

Read through the fetched kubeconfig (deleted afterwards): `SELECT` only against the `bex` database on the primary (`bex-db-2`, session `default_transaction_read_only=on`), `kubectl logs` on both bex-api replicas, and Loki's HTTP API (port-forward to `loki-0`) for the longer log window. Nothing was written to production. The "decide before the deploy mails the owners" ordering above is moot: the fix reached production on 2026-11-12, so what the 11 rows are now is a fact, recorded here.

**Deploys.** The first deploy carrying `c79ba1f77` is `deploy.yml` run 43167265536 (workflow_dispatch, `10acf5314`, pin commit `84c4489a3`, completed 2026-11-12T06:46:10Z). The measurement below is against run 43816470794 (workflow_dispatch, `3c6654df6`, pin commit `d3e0b99e4`, completed 2026-11-16T06:46:41Z), which also contains `c79ba1f77`. The digest in the live `bex-api` Deployment is the one that pin commit wrote, and both replicas have run since 2026-11-16 06:46 and 06:59 with 0 restarts.

### The 11 rows now

All 11 are `enforced` (transition_version 3, `attempt_count` 0, empty `last_error`, no claim held), `reason = subscription_canceled`, plan `hobby`, `billing_excluded` and `billing_comped` both false, with **no** `billing_provider_mappings` row, **no** `billing_enforcements` row, no `apps` row and no tenant namespace on the cluster. Each has exactly one member and no other workspace; none shares a member with a first-party, excluded or comped workspace. Apart from 163 `billing.Status` reads across the 11, the only audit rows are one `billing.Checkout` each (2026-09-06 to 2026-09-29) and one `billing.EnforceWorkspace` each (2026-11-12).

| workspace | created | `customer.subscription.deleted` (2026-10-03 UTC) | grace deadline (2026-10-10 UTC) | completed to `enforced` (2026-11-12 UTC) |
| --- | --- | --- | --- | --- |
| tea-daekrutstb9s73801gcg | 09-06 | 07:03:06 | 07:03:06 | 06:46:39 |
| tea-daf4cqnco25s73fkr3fg | 09-07 | 07:02:47 | 07:02:47 | 06:46:04 |
| tea-dafeuj3h2g1c73dgc6m0 | 09-07 | 07:02:44 | 07:02:44 | 06:45:59 |
| tea-dafjdgoendjc738s7b7g | 09-07 | 07:02:41 | 07:02:41 | 06:47:34 |
| tea-daftut9c7cos73dm2lag | 09-08 | 07:02:51 | 07:02:51 | 06:46:14 |
| tea-dahsl87nf85c739nikng | 09-11 | 07:02:54 | 07:02:54 | 06:46:34 |
| tea-daj4rcggsm7s73f63ni0 | 09-13 | 07:02:57 | 07:02:57 | 06:46:44 |
| tea-dal61gdurvac73dms4fg | 09-16 | 07:03:00 | 07:03:00 | 06:46:24 |
| tea-dalq9onatu4c73cb8qvg | 09-17 | 07:03:00 | 07:03:00 | 06:47:04 |
| tea-daqmsijbdpcs73f5egkg | 09-24 | 07:03:03 | 07:03:03 | 06:46:29 |
| tea-datmop9bmrjc73fp31b0 | 09-29 | 07:03:06 | 07:03:06 | 06:47:09 |

Cardinalities of the rest of the table: 11 of 11 have `mapped = false`, `last_error = ''`, `attempt_count = 0`.

### What actually happened

1. **2026-10-03 07:02:41 to 07:03:07 UTC, bex cancelled them itself.** Each workspace's live Stripe Subscription was deleted within 26 s, about three seconds apart. That is the abandoned-checkout reclaimer that [w3/m172](../../w3/done/m172/README.md) shipped: the pin commit carrying it (`c78510d88`) landed at 06:58:41Z, four minutes before the first cancel, and its t006 records 13 cardless bex Customers reclaimed that day ("Subscription cancelled, Customer deleted, mapping dropped", 11 of them `unbound_no_checkout`). Its own guards say these customers owed nothing: it deletes a Customer only when Stripe confirms no payment method, no invoice that ever carried money and no accrued charges (`ReclaimUnboundCustomer`, `billing/reclaim.go`), then cancels the workspace's subscriptions first (`cancelWorkspaceSubscriptions`). The 11 here match it in count, minute and effect: no mapping survives, and each one's Stripe history in `stripe_billing_events` is a subscription created at checkout, paid invoices (`invoice.paid`; no amount was read, and m172's t006 notes that Stripe backdates a new subscription and shows an immediate `$0.00` invoice) and no `invoice.payment_failed`. I did not read Stripe; the match is by timing and the rows.
2. **`customer.subscription.deleted` is always a failure** (`billing/lifecycle.go:147`), so each workspace went to `grace` (deadline +168 h) and its owner got the grace notice (delivered 2026-10-03, live mode; no per-owner record exists for it because `mailed_to` arrived on 2026-10-08).
3. **2026-10-10 the deadline passed** and each row became `enforcing`. `Enforce` had nothing to suspend (no resource of theirs exists), and `Complete` failed on the missing mapping every tick: this is the `no rows in result set`. About 147,000 such lines are in the 30 days Loki holds (2026-10-24 11:06:56, the start of retained bex-api logs, to 2026-11-12 06:45:32 UTC, about 1,955 per six hours from two replicas).
4. **2026-11-12 06:45:59 to 06:47:34 UTC the deploy completed them**, one per tick, after the last error line (06:45:32). Each completion wrote its audit row and its notice, and the notice drain ran in the same tick. All 11 notices were delivered on the first attempt with no error: each reached its one owner (`mailed_to` has one subject per notice, 0 `not emailed` log lines name any of the 11 workspaces). The owners were mailed **"[bex Test Mode] Workspace compute suspended for billing"** about 33 days after enforcement was due, labeled Test Mode because the mapping is gone (the t001 rule). Nothing was suspended, because nothing existed.

### Decision for each row

**Each of the 11 should be `healthy`, not `enforced`:** none owes anything (the cancel was bex's own cleanup, not a failed collection), none holds a resource, and each owner has now been told twice, wrongly, that their payment failed and then that compute was suspended. The correction is **not applied**, and the milestone parks on it, for three reasons the unblock record's rule does not settle:

- They are not first-party, test, comped or excluded workspaces (one member each, no link to an internal workspace, no sign they are internal), so the record's "recover a first-party or test workspace" case does not cover them, and recovering a real customer's billing state is the user's call.
- The only sanctioned path back to `healthy`, `ForceBillingRecovery` (`POST /v1/tenants/{id}/billing-override` with `{"action":"recover","actor":…,"reason":…}` on bex-api's control-plane listener, `:8091`, `Authorization: Bearer $BEX_CP_TOKEN`), makes the worker mail each owner **"Billing recovered: Payment recovered and resources changed by billing enforcement were restored"**, which is also false for a workspace that never paid. That is a third wrong mail to a customer, so the wording or a silent path is a decision too.
- Invoking it needs the shared control-plane bearer (`BEX_CP_TOKEN`), which is a credential, not something the repository and the kubeconfig give.

Leaving them is not harmful in the meantime: with the payment gate on for every plan (production's `BEX_REQUIRE_PAYMENT_METHOD=all`, per m172's t006) a cardless workspace cannot create anything anyway, and the row recovers by itself on its first successful Stripe event (a successful card bind: `transitionBillingState` moves `enforced` to `recovering`, and the stale-event guard only drops events older than the 2026-10-03 cancel). That second part is read from the code, not exercised.

**Question for the user:** for the 11 ids above, (a) recover all to `healthy` and accept the "Billing recovered" mail, (b) first make that notice say what is true for a workspace that never paid (or recover without a mail), then recover, or (c) leave them to recover on their own card bind. My recommendation is (b); the reclaim path that caused it is filed as [w21/098](../done/098.md).

### The two checks (2026-11-16, 08:16 to 08:19 UTC)

| check | result |
| --- | --- |
| `billing lifecycle: no rows in result set` in the bex-api log, last hour, `kubectl logs --since=1h`, replica `…-nq7bm` | **0** |
| the same, replica `…-rlkmb` | **0** |
| the same, all lines each pod still holds (57 lines since 06:46:29, 31 lines since 06:59:48) | **0** and **0** |
| the same in Loki, both replicas, last hour | **0** |
| the same in Loki since the first deploy carrying the fix (2026-11-12T06:46:10Z, about 4 days) | **0** (the last line is 2026-11-12 06:45:32) |
| the longest window the logs hold (Loki, 30 days; first retained bex-api line 2026-10-24 11:06:43) | about 147,000 before 2026-11-12 06:45:32, **0** after |
| `SELECT count(*) FROM billing_lifecycles WHERE status='enforcing' AND attempt_count > 100` | **0** (no row is `enforcing` at all; `max(attempt_count)` over every row is 0, `last_error` is empty everywhere, no claim is held) |

Whole table today: 32 `excluded`, 11 `enforced` (the 11 above), 11 `healthy`, 1 `grace`. The one `grace` row is a live-mode scale workspace whose payment failed on 2026-11-16 01:02Z; its notice was delivered and mailed to its one owner two seconds later, so the grace, notice and drain path is working on production today. The notice queue holds 6 undelivered rows, all 2026-08 `excluded` notices parked at `infinity` because no owner has an email address (by design, w5/m136); none is new.

**The push worker shows no failing-projection symptom since the deploy.** The only `notifications: push worker` lines in 30 days are three one-minute bursts of `list active push destinations: push delivery store operation failed` (2026-11-02 07:29, 2026-11-12 09:35, 2026-11-14 03:36), the destinations read, not a projection. The 2026-11-14 burst coincides with `billing lifecycle: failed to connect to user=bex database=bex` lines at 03:36:39 to 03:36:44, so it is a database blip; I did not trace the other two bursts separately, but they carry the same message, and no line in 30 days names `dispatch` or `dispatchAgentSessions`. I changed nothing in the push worker.
