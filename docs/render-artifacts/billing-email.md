# Render's billing email: who sets it, on which plan, and what reaches it

**Captured:** 2026-11-25 · **Scope:** [w6/m627](../../.pm/w6/blocked/m627/README.md) · **Sources:** Render's public docs and feedback board, read on 2026-11-25; the community forum through search summaries where noted

## What Render does

| Claim | Source (read 2026-11-25) | Verified |
| --- | --- | --- |
| A new workspace's form has a required **Billing Email**, prefilled from the signed-in account. Hobby "resets billing email to the account email and disables editing"; Pro and Scale "allow a different valid email". | [new-workspace.md](new-workspace.md) (Render's form, captured for w5/m90) | yes |
| A separate billing email exists. Render's team closed "Add billing email address" (filed 2021-08-26: "I need this so invoices are automatically sent to my acountant") as complete with "We have separate billing and team emails now!" | [feedback.render.com/features/p/add-billing-email-address](https://feedback.render.com/features/p/add-billing-email-address) | yes |
| Render does not offer it on the individual plan. Its team wrote on 2023-08-03: "we don't plan to make this feature available on the Individual plan", "which (by definition) assumes a single individual with a single email". | same page | yes |
| The request "Separate email for billing and failure notifications" (2022-02-04: "Our accountant needs to receive invoices, dev team needs failure notices") was merged into it. | same page | yes, the merge; what each address receives is not stated |
| "Billing email address is available on the Pro plan and above"; it is "not available for hobby plan customers - hobby plan is intended for individual usage where the customers == the billing contact". | `community.render.com/t/how-to-add-billing-mailadres/39286`, a staff reply, through a search summary | **no** (summary only) |
| Render sends payment-failure reminders by email before it suspends a workspace's services. In 2021 a team's reminders went to its team email, which was misspelled, so nobody saw them. | `community.render.com/t/everything-suspended-because-of-invoice-failed/1663`, through a search summary | **no** (summary only) |
| The Billing role "Has full access to the workspace's billing settings, including updating payment method", but edits no other workspace setting. Admin and Billing edit billing; Developer views billing details without editing them; Contributor and Viewer do neither. | [render.com/docs/team-members](https://render.com/docs/team-members) | yes |
| Render's audit log records no billing or email event. Its event list has no name mentioning billing, payment or email. | [render.com/docs/audit-logs](https://render.com/docs/audit-logs) | yes |
| Render emails a receipt, "Your Render receipt", which carries the receipt number the Billing page does not show. A user wrote so on 2020-09-02; to whom Render sends it is not stated. | [feedback.render.com/features/p/download-invoice-as-pdf](https://feedback.render.com/features/p/download-invoice-as-pdf) | yes, as of 2020 |

**The forum could not be read.** `community.render.com` answers `301` to `render.discourse.group`, which did not resolve on 2026-11-25 (`getaddrinfo ENOTFOUND`), as on 2026-11-24. The Pro-and-above rule rests on the new-workspace form and the feedback board. The forum adds only the staff wording.

**Unverified:**

- Which of Render's emails go to the billing email rather than the team's members: invoices and receipts, payment failures and suspension notices. The merged request asks for both, and nothing Render published says which it sends where.
- What Render does to a workspace's billing email when it moves from Pro to Hobby. No source covers it.
- Where Render's dashboard shows the setting after creation. No public page names it.

## Stripe's own customer emails

Stripe emails a Customer's `email` its receipts and finalized invoices only when the account's **Settings → Customer emails** say so. bex's Stripe account is shared with other products, and that setting is read in the Stripe Dashboard: the API exposes no read of it. This capture did not read it and changes nothing there.

## What bex does (w6/m627)

- A Pro, Scale or Enterprise workspace's Admin or Billing member sets its billing email on the Billing page, over GraphQL (`setWorkspaceBillingEmail`), REST (`PUT /v1/workspaces/{workspaceId}/billing/email`) and MCP (`update_billing_email`). bex changes the Stripe Customer's email first, then its own record, so a Stripe failure changes nothing.
- Admin, Developer and Billing read it (`workspaceBillingEmail`, `GET /v1/workspaces/{workspaceId}/billing/email`, `get_billing_email`), as Render's "View billing details". On Hobby the read is the owner's account email and `configurable` is false.
- On Hobby the write is refused with `PLAN_FEATURE_REQUIRED` (`feature: billingEmail`, `requiredPlan: pro`). A move to Hobby clears the stored address and sets the Stripe Customer's email back to the owner's account email.
- bex's own billing notices also go to a Pro or higher workspace's billing email, once, when no Admin who gets them already has that address: grace, suspension and recovery, and the Free instance hours, build minutes, included bandwidth and Free database expiry notices.
- bex's Stripe portal no longer offers an email change (`scripts/stripe-billing-setup.py`, whose `--check-portal` is the drift check). The live configuration was updated on 2026-11-25; the test-mode one waits for a test-mode key.
- Render's audit log has no event for the change. bex's lists it under its own verb, `workspaces.SetBillingEmail`, as it lists every write outside Render's event enum.
