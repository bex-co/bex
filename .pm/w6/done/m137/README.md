# w6 · m137 — Notifications follow destination access: one eligibility policy at projection, delivery, and inbox (ADR087)

**Worker:** worker6 **Goal:** a notification is delivered, stored, counted, and shown only to recipients who can currently access its destination — closing the static path that today lets a Viewer/Billing member receive agent-session metadata their session read would refuse. **Status:** done — 2026-09-07. One policy (destinationRequiredRelation) applied at fan-out, at send time via the claim query's own tenant_members join (review fix: no per-device membership reads, no extra store seam), and at inbox list+count via event-type exclusions derived from the same table (review fix: closes the operate-vs-create tier drift). Push + inbox disclosure tests proven red pre-fix; lock-screen payloads generic (no repo/PR metadata); full backend suite + lint + deadcode green.

## Tasks (in order)

| id   | title                                                                    | est | depends_on |
| ---- | ------------------------------------------------------------------------ | --- | ---------- |
| t001 | Define one destination-eligibility policy in the notification backend — **DONE** | 45m | — |
| t002 | Enforce at push projection and immediately before deferred delivery — **DONE** | 45m | t001 |
| t003 | Enforce at stored inbox reads and unread counts — **DONE** | 45m | t001 |
| t004 | Keep lock-screen payloads generic; revalidate at open — **DONE** | 30m | t002 |
| t005 | Render parity: eligibility semantics aligned across notification surfaces — **DONE** | 30m | t002, t003 |
| t006 | Simplify — **DONE** | 30m | t005 |
| t007 | Test coverage — **DONE** | 45m | t005 |
| t008 | Closeout — **DONE** | 15m | t007 |

## Definition of done

One destination-eligibility policy (current access to the notification's target resource, e.g. agent events require the session-read gate `can_operate`) is applied when projecting recipients, immediately before deferred/retried delivery, and when returning stored inbox items and unread counts. An agent completion/failure/PR-ready event never reaches a Viewer/Billing recipient through any of those paths; a role downgrade removes unauthorized historic items from the API's visible projection and badge counts (durable records may be retained internally). An authorization outage defers or suppresses disclosure — it never broadens recipients. Lock-screen text stays generic (no repository names, prompts, log excerpts, credentials); the closed routing envelope and open-time revalidation are preserved. The push_worker/inbox regression tests are proven red against the pre-fix behavior.

## Source + Goal linkage

- **Source:** [docs/ADR087-mobile-role-views.md](../../../docs/ADR087-mobile-role-views.md) §Notifications follow destination access + research finding "Notification access can disagree with destination access"; materialized 2026-09-07 per user direction (`/pm for w6`). Gap verified against the checkout the same day: `notifications/push_worker.go` never sets `EligibleRoles`; `delivery_policy.go:494-497` treats nil eligibility as all roles; `inbox.go` authorizes only `RelCanView` (:65/:92/:117) without checking the destination resource's relation.
- **Goal linkage:** ADR052 notifications + ADR024 members/authz — this is a real information-disclosure bug independent of mobile, and a hard ADR087 launch gate ("Close the static agent fan-out/inbox mismatch … before claiming this policy complete").
- **Expected outcome:** delivery, inbox pages, counts, and open-time access all agree with current target eligibility, on every surface.
- **Why now:** m138's client-side filtering is only defense in depth; server disclosure and OS-rendered notifications can't be fixed from the client. Independent of m136, so it can land in parallel. Render parity task included — inbox/counts are user-facing API surfaces that must stay consistent across REST/GraphQL/MCP where exposed (the comparison target here is cross-surface consistency; Render has no equivalent agent-notification product).
