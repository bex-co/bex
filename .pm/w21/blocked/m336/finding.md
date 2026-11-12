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

**Left (t004, t007), needs the user:** a read of the production cluster (`~/.ssh/id_bex` for `scripts/fetch-app-kubeconfig.sh`, as for w21/012) and one decision. The deploy flips the 11 `enforcing` rows to `enforced` and mails their owners the "compute suspended" notice, weeks late (`Enforce` already ran on every lease, so the workspaces were suspended all along). Decide before the deploy whether each of the 11 should be enforced, excluded or comped (`SetBillingException`, `ForceBillingRecovery`), then record the table here. Commands, once there is access:

```text
kubectl -n bex-system exec <bex-db pod> -- psql -c "SELECT l.workspace_id, l.reason, l.grace_deadline, l.attempt_count, (m.workspace_id IS NOT NULL) AS mapped FROM billing_lifecycles l LEFT JOIN billing_provider_mappings m USING (workspace_id) WHERE l.status='enforcing' ORDER BY l.attempt_count DESC"
kubectl -n bex-system logs deploy/bex-api --since=1h | grep -c 'billing lifecycle: no rows in result set'      # after the deploy: expect 0
kubectl -n bex-system exec <bex-db pod> -- psql -c "SELECT count(*) FROM billing_lifecycles WHERE status='enforcing' AND attempt_count > 100"   # after the deploy: expect 0
```
