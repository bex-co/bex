# w21 · m336 — Billing and push workers: one bad row cannot stop the pass

**Worker:** worker21 **Goal:** a billing lifecycle row whose provider mapping is missing completes instead of failing every tick, and neither the billing worker nor the push worker lets one failing row or stage starve the others in the same pass. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Billing store: completing a lifecycle row no longer requires a provider mapping | 45m | — |
| t002 | Billing worker: its three stages are independent, and a failed row is recorded and skipped | 40m | t001 |
| t003 | Push worker: a failed projection no longer starves send, receipts and the sweep | 45m | — |
| t004 | Settle the 11 stuck workspaces and check production after the deploy | 30m | t001, t002 |
| t005 | Simplify | 15m | t003, t004 |
| t006 | Test coverage | 20m | t005 |
| t007 | Closeout | 10m | t006 |

## Definition of done

- On a real Postgres, a lifecycle row in `enforcing` with no `billing_provider_mappings` row completes to `enforced`, writes its audit row and does not abort the pass; the test fails with the unconditional `livemode` read restored.
- In the billing worker, one row whose completion or failure record errors does not stop the next row, the notification drain or the event purge in the same tick; the error is recorded on the row with backoff, or returned only after every stage has run.
- In the push worker, a failing projection (`dispatch` or `dispatchAgentSessions`) still lets `send`, `checkReceipts` and `sweep` run, and the feed cursor does not advance past the failed page (no event is lost); a test with a failing `levels.admits` pins it.
- The outcome for the 11 `enforcing` workspaces is recorded in `finding.md` (what each should be, and why).
- After the deploy, `billing lifecycle: no rows in result set` is absent from the bex-api log for an hour and no `billing_lifecycles` row has `status='enforcing'` with `attempt_count` above 100. This bullet needs a read of the production cluster (the way the note was found: `kubectl -n bex-system logs deploy/bex-api`); without that access the milestone parks as BLOCKED on it, with the exact commands recorded, once t001–t003 are shipped.

## Source + Goal linkage

- **Source:** [`finding.md`](finding.md) (re-homed from `w10/012`, found 2026-11-08 and still open: no commit since then touches `store/billing_lifecycle.go` or `billing/worker.go`), plus the 2026-11-11 audit of the 22 `core.Poll` workers in `lego/backend/internal` (`/pm-brainstorm for w21`): only the billing lifecycle worker and the push worker abort a pass on one failing item.
- **Goal linkage:** the V0 roadmap's "multi tenant and usage metering" (`.pm/GOAL.md`): a workspace that does not pay is enforced and told so ([ADR046](../../../docs/ADR046-payment-onboarding-and-paid-gating.md)); today the lifecycle worker never finishes that for the 11 stuck rows and never reaches its notice or purge stages.
- **Expected outcome:** unmapped lifecycle rows settle; billing notices go out and old Stripe events are purged again; one poison feed row can no longer stop push delivery.
- **Why now:** the error has been logged about every 20 seconds since at least 2026-11-08, so each tick spent on the stuck rows is a tick the notice and purge stages never run.
- **Render parity task:** omitted. These are internal workers and store queries; no REST, GraphQL, MCP or dashboard surface changes, and Render has no counterpart for the dunning worker.
