# w6 · m627 — A Pro or higher workspace sets its own billing email; Hobby's stays the owner's account email, as on Render

**Worker:** worker6 **Goal:** Pro, Scale and Enterprise Admins and Billing members set the workspace's billing email on the Billing page and over GraphQL, REST and MCP, and billing notices reach it; a Hobby workspace's billing email is its owner's account email and cannot be changed anywhere, the Stripe portal included. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Capture Render's billing email rules and what Render sends there](t001.md) | 30m | — |
| t002 | [Read and set the billing email, Pro and above](t002.md) | 1h | t001 |
| t003 | [Billing notices also reach the billing email](t003.md) | 45m | t002 |
| t004 | [The Billing page shows the billing email, editable on Pro and above](t004.md) | 45m | t002 |
| t005 | [The Stripe portal stops offering an email change](t005.md) | 30m | t002 |
| t006 | [Render parity](t006.md) | 30m | t003, t004, t005 |
| t007 | [Simplify](t007.md) | 20m | t006 |
| t008 | [Test coverage](t008.md) | 45m | t006 |
| t009 | [Closeout](t009.md) | 10m | t007, t008 |

## Definition of done

- A Pro workspace's Admin or Billing member sets a billing email on the Billing page; the Stripe Customer's email becomes it, and GraphQL, REST and MCP read it back.
- On Hobby the write is refused with `PLAN_FEATURE_REQUIRED` (`feature: billingEmail`, `requiredPlan: pro`) on every surface, the read returns the owner's account email, and the page shows it read-only with a "Pro and above" note.
- A downgrade to Hobby sets the stored and Stripe emails back to the owner's account email.
- In Stripe test mode, a grace notice of a Pro workspace with a billing email of its own also goes to that address, once.
- bex's portal configuration no longer allows an email change (`scripts/stripe-billing-setup.py` and its drift check), and the live configuration matches once the live step has run.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w6` 2026-11-24 (billing & invoicing), item 4, filed by `/pmx them all to w6`. Today the billing email is asked only at creation (`PrepareWorkspaceCreation`, `normalizeBillingEmail` in `lego/backend/internal/workspaces/service.go`), and Hobby is held to the account email there ([w5/160](../../w5/done/160.md)). Nothing in bex changes it afterwards, while the Stripe portal configuration (`scripts/stripe-billing-setup.py`, `allowed_updates` includes `email`) lets every plan change it, Hobby included. bex's own billing notices go to the members `ListBillingOwnerSubjects` returns (`lego/backend/internal/notifications/service.go`), never to the billing email.
- **Render:** its new-workspace form "resets billing email to the account email and disables editing" on Hobby, while Pro and Scale "allow a different valid email" ([new-workspace.md](../../../docs/render-artifacts/new-workspace.md)); Render's team: "We have separate billing and team emails now!", not offered on the individual plan (feedback.render.com/features/p/add-billing-email-address); a staff answer: "Billing email address is available on the Pro plan and above" (community answers read through search summaries on 2026-11-24 (community.render.com redirected to render.discourse.group, which did not resolve)).
- **Recommendations, not user decisions:** (1) billing notices also go to the billing email, since Render merged the request for a separate billing and failure-notification address into this feature; its exact notice list is forum-level evidence, which t001 records. (2) A downgrade to Hobby resets the email to the owner's, following Render's Hobby rule; Render's own downgrade behavior is unverified.
- **Live step:** t005 changes bex's live portal configuration on the Stripe account bex shares with other products. If the agent is refused that write, as in [w2/070](../../w2/blocked/070.md), t005 parks on the user's run of the script and the rest ships.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)); ADR030 §9 plan gates.
- **Expected outcome:** a finance contact who is not a member gets the workspace's invoices and billing notices, and Hobby can no longer move its billing email away from its owner.
- **Why now:** the portal lets a Hobby workspace bypass w5/160's rule today, and [w6/177](../177.md)'s ledger row needs its owner.
- **Render parity:** included: GraphQL, REST, MCP and the Billing page change.
