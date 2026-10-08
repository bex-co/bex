# w5 · m136 — A billing notice mails each owner once and stops retrying what can never succeed

**Worker:** worker5 **Goal:** A billing notice reaches each workspace owner once across retries, and stops retrying owners it can never reach, without silently dropping owners whose address lookup failed. **Status:** done

## Tasks (in order)

| id   | title                                                                         | est | depends_on |
| ---- | ----------------------------------------------------------------------------- | --- | ---------- |
| t001 | Record on each billing notice the owners it already reached — **DONE** | 30m | —          |
| t002 | A retry mails only the owners the notice has not reached — **DONE** | 40m | t001       |
| t003 | Permanent SMTP rejections skip the owner, and attempts are capped — **DONE** | 40m | t002       |
| t004 | Email lookups report failure, so "no address" and "lookup failed" differ — **DONE** | 60m | t002       |
| t005 | Simplify — **DONE** | 20m | t003, t004 |
| t006 | Test coverage — **DONE** | 30m | t003, t004 |
| t007 | Closeout — **DONE** | 10m | t006       |

## Definition of done

- A notice where one owner's send fails transiently mails the other owner once across two worker ticks, and the second tick retries only the failing owner.
- An owner whose address the mail server rejects permanently (SMTP 5xx) is logged and skipped, not retried, and no notice is retried past a fixed attempt cap.
- `LookupEmails` reports a failed Kratos batch as an error rather than as absent addresses. The billing notifier retries the owners it could not resolve and skips only owners Kratos confirms have no address. The members, workspaces and webhooks callers keep their current omission behavior.
- With `BEX_KRATOS_ADMIN_URL` unset, the billing worker takes the "identity lookup unavailable" path (log and complete) instead of failing every notice forever.
- The backend suite passes against real Postgres, including the migration.

## Source + Goal linkage

- **Source:** promoted from inbox w5/159, found by w5/153's review (2026-10-07).
  - `notifications/service.go` `notifyBilling` mails every owner on each attempt. `billing/worker.go` `drainNotifications` fails or completes the notice as a whole.
  - `store/billing_lifecycle.go` `ClaimBillingNotifications` retries any undelivered notice with backoff, and nothing caps `attempt_count`. `last_error` is written but never read.
  - `workspaces/kratos.go` `lookupMany` drops a failed batch silently. `api/server.go` `identityEmailLookup` always wraps `d.Identities`, so the notifier's `Identities == nil` check never fires when Kratos is unwired.
- **Goal linkage:** ADR040 billing lifecycle reliability; ADR008 trust. Repeated payment-failure mail to every owner is a visible defect.
- **Expected outcome:** one transient or permanent failure no longer re-mails every owner forever. An identity outage delays mail instead of losing it.
- **Why now:** w5/153 stopped the "no address" loop, and this is the remaining half of the same defect. Each retry today re-mails every owner who was already reached.
- **Render parity omitted:** an internal email-delivery fix. No REST, GraphQL, MCP or dashboard surface changes.

## Result (2026-10-08)

- Migration 0147 adds `billing_notifications.mailed_to`. `RecordBillingNotificationMailed` appends an owner right after its send, so a retry, or a crash mid-loop, mails only the owners the notice has not reached. Delivery is at least once: a send whose record fails is mailed again.
- An SMTP recipient refusal (550, 551, 553) skips the owner like a missing address. A deferral or a relay-level 5xx (535 login failure) still retries, so a misconfigured relay cannot silently drop notices.
- `ClaimBillingNotifications` stops at `store.MaxBillingNotificationAttempts` (24, about 36 hours of backoff). The last failure parks the notice at `next_attempt_at = 'infinity'`, outside the claim scan, keeping its `last_error`, and the worker logs it as abandoned. 0147 parks rows already past the cap.
- `LookupEmails` returns `(map, error)`. A failed Kratos batch answers `core.ErrIdentityLookupFailed` beside the emails that resolved. The billing notifier retries the owners it could not read and skips only owners Kratos answers for without an address, which replaces w5/153's "nobody resolved, so retry" heuristic. Members, workspaces and the deploy fan-out keep omitting. Webhooks logs a failed lookup distinctly.
- With `BEX_KRATOS_ADMIN_URL` unset the notifier's lookup is now nil, not an always-present adapter, so notices are logged and completed instead of retried forever.
- Tests:
  - store: `TestPGStoreBillingNotificationRecordsReachedOwnersAndCapsAttempts` (real Postgres);
  - notifications: `TestABillingNoticeRetryMailsOnlyTheOwnersItMissed`, `TestABillingNoticeStopsOnAPermanentRejection`, and `TestABillingNoticeStillRetriesWhatARetryCanFix` (now the failed-lookup case);
  - api: `TestNotificationsHaveNoEmailLookupWithoutAnIdentityProvider`, plus `TestAnEmailLookupLoadsNoCredentials` asserting the failure error.
- Four mutants fail the tests: no skip of reached owners, no permanent classification, a failed lookup treated as no address, and the wrapped wiring.
- Docs: ADR040.
- Follow-up: w5/160. The Hobby billing-email match can now refuse while the account email is unreadable.
