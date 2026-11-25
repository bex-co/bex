# Render's invoice list: PDF download, Pay Now, and who sees them

**Captured:** 2026-11-24 · **Scope:** [w6/m625](../../.pm/w6/m625/README.md) · **Sources:** Render's public docs and feedback board, read on 2026-11-24; the community forum where noted

## What Render shows

| Claim | Source (read 2026-11-24) | Verified |
| --- | --- | --- |
| The Billing page is under **Workspace → Billing** and lets a member "View and update your plan", "Update your payment method" and "View invoices for past months". | [render.com/docs/render-dashboard](https://render.com/docs/render-dashboard) | yes |
| Each past invoice downloads as a PDF from the Billing page. Render's team closed the request "Download invoice as PDF" (filed 2019-11-07) on 2021-09-02 with "Users can now download their .pdf invoices at https://dashboard.render.com/billing." | [feedback.render.com/features/p/download-invoice-as-pdf](https://feedback.render.com/features/p/download-invoice-as-pdf) | yes |
| The list shows no invoice number. A user wrote on 2020-09-02 that "there is no indication of invoice/receipt number" on the Billing page. | same page | yes, as of 2020 |
| An unpaid invoice has **Pay Now** in its row's three-dot menu, and it opens Stripe for that invoice. | community answers read through search summaries on 2026-11-24 | **no** (see below) |
| An invoice is paid on Stripe's page. A member who tried to pay a past invoice there read Stripe's own refusal: "This invoice can no longer be paid on Stripe. Please contact Render (support@render.com) who will arrange an alternative method to pay this invoice, or issue you a new invoice." | `community.render.com/t/payment-failed-cannot-pay-previous-month-bill/15763`, through a search summary | **no** (summary only) |
| "View billing details" is granted to **Admin, Developer and Billing**; **Contributor** and **Viewer** are not. Contributor "Can't view sensitive fields (billing info, connection strings, environment variables, and so on)", and so does Viewer. | [render.com/docs/team-members](https://render.com/docs/team-members#role-permissions) | yes |
| "Edit payment method" is Admin and Billing only. | same page | yes |
| Render's public API has no invoice or billing endpoint: the 166 paths of `api-docs.render.com/openapi/render-public-api-1.json` name neither. | the OpenAPI document, fetched 2026-11-24 | yes |

**The forum could not be read.** `community.render.com` now answers `301` to `render.discourse.group`, which did not resolve on 2026-11-24 (`curl: (6) Could not resolve host`). So the Pay Now menu and the Stripe refusal above rest only on search summaries. Which roles see Pay Now is unverified too. Render's docs list no role that may view invoices but not pay them, so bex gives both links to every role that can view billing details.

## What bex does (w6/m625)

- Each invoice in the Billing page's history has a row menu. **Download PDF** opens Stripe's `invoice_pdf`. **Pay Now** opens Stripe's `hosted_invoice_url` in a new tab. It appears on an invoice that still owes: `open`, or `uncollectible`, which bex's own settlement still collects (`unpaidInvoiceStatuses`, `lego/backend/internal/billing/settle.go`) and Stripe's page still takes payment for.
- REST `GET /v1/usage` (`billing.invoices[]`), GraphQL `BillingInvoice` and MCP `get_usage` carry the same two links as `hostedInvoiceUrl` and `invoicePdfUrl`. A link Stripe does not supply is left out, and is `null` on GraphQL. This goes beyond Render, whose public API has no invoice read. bex has served the invoice list on all three surfaces since m48/m50, under ADR006's one core with three adapters.
- Both links ride the usage read's `can_view_billing` gate ([w9/m620](../../.pm/w9/done/m620/README.md)): Admin, Developer and Billing, as Render's "View billing details". Contributor and Viewer get no `billing` object and so no link.
- Statuses read in the dashboard's language instead of Stripe's raw words: in English, Unpaid (`open`), Paid, Void and Uncollectible.
