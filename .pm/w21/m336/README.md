# w21 · m336 — Billing and push workers: one bad row cannot stop the pass

**Worker:** worker21 **Goal:** a billing lifecycle row whose provider mapping is missing completes instead of failing every tick, and neither the billing worker nor the push worker lets one failing row or stage starve the others in the same pass. **Status:** todo (t001–t003, t005 and t006 done and shipped; t004 and t007 remain: the production read and the closeout)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Billing store: completing a lifecycle row no longer requires a provider mapping — **DONE** | 45m | — |
| t002 | Billing worker: its three stages are independent, and a failed row is recorded and skipped — **DONE** | 40m | t001 |
| t003 | Push worker: a failed projection no longer starves send, receipts and the sweep — **DONE** | 45m | — |
| t004 | Settle the 11 stuck workspaces and check production after the deploy | 30m | t001, t002 |
| t005 | Simplify — **DONE** | 15m | t003, t004 |
| t006 | Test coverage — **DONE** | 20m | t005 |
| t007 | Closeout | 10m | t006 |

## Definition of done

- On a real Postgres, a lifecycle row in `enforcing` with no `billing_provider_mappings` row completes to `enforced`, writes its audit row and its notice (labeled test mode, as `SetBillingException` already did) and does not abort the pass; the test fails with the unconditional `livemode` read restored. **Met 2026-11-11** (`TestPGStoreBillingCompletionWithoutProviderMapping`).
- In the billing worker, one row whose completion or failure record errors does not stop the next row, the notification drain or the event purge in the same tick; the error is recorded on the row with backoff, or returned only after every stage has run. **Met 2026-11-11** (`TestWorkerLifecycleRowFailureDoesNotStarveTheTick`, `TestWorkerNotificationCompletionFailureDoesNotStarveThePurge`; both fail on the old worker).
- In the push worker, a failing projection (`dispatch` or `dispatchAgentSessions`) still lets `send`, `checkReceipts` and `sweep` run, and the feed cursor does not advance past the failed page (no event is lost); a test with a failing feed read pins it. **Met 2026-11-11** (`TestPushWorkerFailedProjectionStillSendsQueuedDeliveries`; the test fails on the old `RunOnce`).
- The outcome for the 11 `enforcing` workspaces is recorded in `finding.md` (what each should be, and why).
- After the deploy, `billing lifecycle: no rows in result set` is absent from the bex-api log for an hour and no `billing_lifecycles` row has `status='enforcing'` with `attempt_count` above 100. This bullet needs a read of the production cluster (the way the note was found: `kubectl -n bex-system logs deploy/bex-api`); without that access the milestone parks as BLOCKED on it, with the exact commands recorded, once t001–t003 are shipped.

## Source + Goal linkage

- **Source:** [`finding.md`](finding.md) (re-homed from `w10/012`, found 2026-11-08 and still open: no commit since then touches `store/billing_lifecycle.go` or `billing/worker.go`), plus the 2026-11-11 audit of the 22 `core.Poll` workers in `lego/backend/internal` (`/pm-brainstorm for w21`): only the billing lifecycle worker and the push worker abort a pass on one failing item.
- **Goal linkage:** the V0 roadmap's "multi tenant and usage metering" (`.pm/GOAL.md`): a workspace that does not pay is enforced and told so ([ADR046](../../../docs/ADR046-payment-onboarding-and-paid-gating.md)); today the lifecycle worker never finishes that for the 11 stuck rows and never reaches its notice or purge stages.
- **Expected outcome:** unmapped lifecycle rows settle; billing notices go out and old Stripe events are purged again; one poison feed row can no longer stop push delivery.
- **Why now:** the error has been logged about every 20 seconds since at least 2026-11-08, so each tick spent on the stuck rows is a tick the notice and purge stages never run.
- **Render parity task:** omitted. These are internal workers and store queries; no REST, GraphQL, MCP or dashboard surface changes, and Render has no counterpart for the dunning worker.

## Unblock (2026-11-16)

- **Source:** verified evidence and the user's standing instruction (2026-11-15: a "no access" gate is not a stop until the real paths were tried). It records no decision about any customer's billing state; the enforcement policy itself is unchanged.
- **Gate cleared, access (verified 2026-11-15 and 2026-11-16):** the production cluster is reachable from this checkout. Several workers read production logs and rows through it. Recipe: `HCLOUD_TOKEN="$(grep '^HCLOUD_TOKEN=' .env | cut -d= -f2-)" bash scripts/fetch-app-kubeconfig.sh <path outside the repo>`, then `KUBECONFIG=<path> kubectl …`; the key is `~/.ssh/bex` (the earlier notes named a different file; that name was stale and w21/089 fixed it). Never print the token, the kubeconfig or a customer's data beyond what the table needs, and delete the kubeconfig afterwards.
- **Gate dissolved, the 11 `enforcing` workspaces:** the earlier text asked for a decision "before the deploy mails their owners". The fix (`c79ba1f77`, 2026-11-11) has been on production through several deploys since, so that point has passed: the 11 rows either already completed to `enforced` (and their owners were mailed) or something else is holding them. What the rows are now is a fact to read, not a choice to make first. The read comes first; a decision is needed only for a row that should not be enforced (first-party, comped, excluded).
- **Rule for that row, from the existing code and ADR046:** correct it through the sanctioned override (`SetBillingException`, `ForceBillingRecovery`), the way `finding.md` names. A first-party or test workspace that was suspended or mailed by mistake is recovered, not left. For a real customer workspace that is `enforced` for a real unpaid reason, leave it. If a row's right state is not decidable from the rows, the grace deadline, the provider mapping and the workspace's plan, park only that row's t004 step, naming the workspace and the question, and finish the rest of the milestone.
- **Remaining:** t004 (read the rows and the log, record the table, check the log count and the row count) and t007 (Closeout).
- **This milestone's next steps:** t004 steps 1 to 3 as rewritten below, then t007.
- **Reference-product basis:** none needed: this is a worker fix and a production check, and the enforcement policy stays ADR046's as shipped. I did not re-verify Render's failed-payment behavior for this record.
