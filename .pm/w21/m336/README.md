# w21 · m336 — Billing and push workers: one bad row cannot stop the pass

**Worker:** worker21 **Goal:** a billing lifecycle row whose provider mapping is missing completes instead of failing every tick, and neither the billing worker nor the push worker lets one failing row or stage starve the others in the same pass. **Status:** todo (t001–t003, t005 and t006 done and shipped; t004's production read and both production checks are done; t004's correction of the 11 `enforced` workspaces and t007 Closeout remain, with w21/098 fixed first)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Billing store: completing a lifecycle row no longer requires a provider mapping — **DONE** | 45m | — |
| t002 | Billing worker: its three stages are independent, and a failed row is recorded and skipped — **DONE** | 40m | t001 |
| t003 | Push worker: a failed projection no longer starves send, receipts and the sweep — **DONE** | 45m | — |
| t004 | Settle the 11 stuck workspaces and check production after the deploy — **BLOCKED (user decision on the 11 rows; the reads and checks are done)** | 30m | t001, t002 |
| t005 | Simplify — **DONE** | 15m | t003, t004 |
| t006 | Test coverage — **DONE** | 20m | t005 |
| t007 | Closeout | 10m | t006 |

## Definition of done

- On a real Postgres, a lifecycle row in `enforcing` with no `billing_provider_mappings` row completes to `enforced`, writes its audit row and its notice (labeled test mode, as `SetBillingException` already did) and does not abort the pass; the test fails with the unconditional `livemode` read restored. **Met 2026-11-11** (`TestPGStoreBillingCompletionWithoutProviderMapping`).
- In the billing worker, one row whose completion or failure record errors does not stop the next row, the notification drain or the event purge in the same tick; the error is recorded on the row with backoff, or returned only after every stage has run. **Met 2026-11-11** (`TestWorkerLifecycleRowFailureDoesNotStarveTheTick`, `TestWorkerNotificationCompletionFailureDoesNotStarveThePurge`; both fail on the old worker).
- In the push worker, a failing projection (`dispatch` or `dispatchAgentSessions`) still lets `send`, `checkReceipts` and `sweep` run, and the feed cursor does not advance past the failed page (no event is lost); a test with a failing feed read pins it. **Met 2026-11-11** (`TestPushWorkerFailedProjectionStillSendsQueuedDeliveries`; the test fails on the old `RunOnce`).
- The outcome for the 11 `enforcing` workspaces is recorded in `finding.md` (what each should be, and why). **Recorded 2026-11-16**: the rows were completed to `enforced` by the 2026-11-12 deploy, and each should be `healthy` (bex's own reclaimer cancelled them, none owes anything); the correction is not applied, see Parked below.
- After the deploy, `billing lifecycle: no rows in result set` is absent from the bex-api log for an hour and no `billing_lifecycles` row has `status='enforcing'` with `attempt_count` above 100. This bullet needs a read of the production cluster (the way the note was found: `kubectl -n bex-system logs deploy/bex-api`); without that access the milestone parks as BLOCKED on it, with the exact commands recorded, once t001–t003 are shipped. **Met 2026-11-16** against `deploy.yml` run 43816470794 (`3c6654df6`, which contains `c79ba1f77`): 0 lines in the last hour on both replicas and in Loki, none since 2026-11-12 06:45:32Z (the first deploy carrying the fix is run 43167265536), and 0 rows `enforcing` with `attempt_count` above 100 ([`finding.md`](finding.md)).

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

## Parked (2026-11-16)

Everything this milestone shipped is verified on production; one decision remains, and it is the user's, because it changes 11 real customers' billing state and mails them.

- **The 11 workspaces:** `tea-daekrutstb9s73801gcg`, `tea-daf4cqnco25s73fkr3fg`, `tea-dafeuj3h2g1c73dgc6m0`, `tea-dafjdgoendjc738s7b7g`, `tea-daftut9c7cos73dm2lag`, `tea-dahsl87nf85c739nikng`, `tea-daj4rcggsm7s73f63ni0`, `tea-dal61gdurvac73dms4fg`, `tea-dalq9onatu4c73cb8qvg`, `tea-daqmsijbdpcs73f5egkg`, `tea-datmop9bmrjc73fp31b0`. All `enforced` since 2026-11-12, none has a resource, a payment method or a mapping, and bex's own abandoned-checkout reclaimer caused their `subscription_canceled` failure on 2026-10-03 ([`finding.md`](finding.md); the reducer defect is [w21/098](../098.md)).
- **The question:** (a) recover all 11 to `healthy` and accept that the worker mails each owner "Billing recovered: Payment recovered…", (b) first make that notice true for a workspace that never paid (or recover without a mail), then recover, or (c) leave them, since they recover on their own at the first successful Stripe event. Recommendation: (b).
- **How it is applied:** `POST /v1/tenants/{id}/billing-override` with `{"action":"recover","actor":…,"reason":…}` on bex-api's control-plane listener (`:8091`, bearer `BEX_CP_TOKEN`), one call per id; nothing else writes this state, and no table is edited by hand. The worker completes each to `healthy` on its next tick.
- **What clears it:** the user's answer. Then t004's last criterion is applied or declined and recorded, and t007 closes the milestone (the other criteria already hold). `/unblock w21` carries it.

## Unblock (2026-11-16, the user's decision)

- **Source:** explicit user decision in session on 2026-11-16: "就这么做，解除 m336 停靠，连同 098 一起派下去", answering the plan in the preceding reply. The user's earlier remark was that a problem should simply be fixed and that the affected users are alpha testers.
- **Decision:** fix the cause and correct the 11 rows now, in this order, without waiting on anything else:
  1. **w21/098 first** (the reducer must not read bex's own reclaimer cancelling a cardless Subscription as a payment failure; a Subscription Stripe cancels after dunning for a card-bound workspace still enters grace; both sides tested against the store, the first failing on the current reducer). It ships and deploys before the rows are corrected, so the repair cannot be undone by a repeat.
  2. **Then recover all 11 workspaces** listed under Parked to `healthy`, through the only sanctioned path: `POST /v1/tenants/{id}/billing-override` with `{"action":"recover","actor":…,"reason":…}` on bex-api's control-plane listener (`:8091`, bearer `BEX_CP_TOKEN`), one call per id. No table is edited by hand.
  3. **The notice:** the worker's "Billing recovered" mail (`lego/backend/internal/notifications/service.go:297`) goes to the 11 owners. This is accepted: they are alpha users, the earlier two mails were wrong, and a recovery notice corrects them. Making that notice silent or truthful for a workspace that never paid is an optional polish, not a gate: file it as its own inbox note if the worker thinks it worth doing, and do not block on it.
- **Gate dissolved, `BEX_CP_TOKEN`:** the earlier park said the token "is a credential, not something the repository and the kubeconfig give". That was never tried. The token is a Secret in the production cluster that bex-api reads, so the kubeconfig does reach it (`scripts/fetch-app-kubeconfig.sh`, then find where the `bex-api` Deployment's `BEX_CP_TOKEN` comes from with `kubectl -n bex-system get deploy bex-api -o json` and read that Secret key). Treat it as a credential: read it into a shell variable without echoing it, pass it to curl through a config file or stdin the way `scripts/stripe-billing-prod-test-fixtures.sh` does, port-forward `:8091` only for the calls, delete temp files and `unset` the variable afterwards. Never print it, log it, put it in a commit or a note.
- **Remaining gates:** none. If the Secret turns out not to hold the key or the call is refused, record exactly what was tried and what answered, and park only the recovery step.
- **Reference-product basis:** none needed (a worker bug and a data repair). Render has no counterpart for this dunning worker.
- **This milestone's next steps:** w21/098, then t004's correction (steps 5 and 6 below in t004), then t007 Closeout.
