# w7 · m52 — Stripe dunning lifecycle: trusted events → grace → reversible enforcement → recovery

**Worker:** worker7 **Goal:** turn Stripe's test-mode invoice lifecycle into a durable, idempotent bex billing state machine with notifications, a bounded grace period, reversible enforcement, and automatic recovery without deleting tenant data **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Decide and record the reversible workspace enforcement policy | 45m | — |
| t002 | Persist Stripe event and workspace billing lifecycle state | 1h | t001 |
| t003 | Process invoice and subscription webhooks idempotently | 1h | t002 |
| t004 | Add Stripe polling reconciliation as the webhook-loss backstop | 1h | t002, t003 |
| t005 | Implement configurable grace deadlines and transition scheduling | 1h | t002, t003 |
| t006 | Notify owners on failure, grace, enforcement, and recovery | 1h | t005 |
| t007 | Enforce the approved reversible policy without deleting tenant data | 1h | t001, t005 |
| t008 | Recover only resources changed by billing enforcement | 1h | t003, t004, t007 |
| t009 | Add audited admin override, exclusion, and comp recovery controls | 45m | t002, t007, t008 |
| t010 | Expose billing state and deadlines across REST · GraphQL · MCP · UI | 1h | t005, t006, t007, t008, t009 |
| t011 | Verify failure → grace → enforcement → payment → recovery in test mode | 1h | t010 |
| t012 | Render parity | 30m | t011 |
| t013 | Simplify | 30m | t012 |
| t014 | Test coverage | 45m | t013 |
| t015 | Closeout | 10m | t014 |

## Definition of done

Stripe test-mode `invoice.payment_failed`, payment-success, and subscription lifecycle events are signature-verified, deduplicated by event id, persisted, and safe under retries and reordering. A polling backstop converges missed events. The workspace enters a visible grace state, owners receive notifications, and expiry applies the approved reversible policy while preserving databases, key-value data, secrets, and billing evidence; eventual deletion is not part of this milestone. A later successful payment automatically restores only the resources that billing enforcement changed, and audited operator controls can override, exclude, or comp the workspace. REST, GraphQL, MCP, and dashboard expose the same status, reason, deadline, and recovery state. A production-hosted Stripe test-clock scenario proves failure → grace → enforcement → successful recovery with duplicate and out-of-order webhook delivery. Live mode and real tenant suspension are out of scope.

## Source + Goal linkage

- **Source:** User request on 2026-07-27 for complete prod-hosted test-mode billing; closes ADR040 §6's explicit deferred ladder and follows m51's payment-ready Customer/Subscription.
- **Goal linkage:** Advances ADR008's deterministic and machine-readable goals: external payment state converges to an explicit, auditable platform state that agents can inspect and safely retry.
- **Expected outcome:** Test-mode non-payment has an observable, reversible lifecycle rather than a log-only webhook, and payment recovery restores service without operator surgery or data loss.
- **Why now:** The production test endpoint already receives trusted `invoice.payment_failed` events, but m50 deliberately stops at logging; end-to-end billing cannot be accepted until that event has safe consequences and a tested recovery path.
- **Render parity:** Included as t012 because the lifecycle adds user-visible state and controls across REST, GraphQL, MCP, email, and dashboard surfaces.
