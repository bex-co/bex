# Render's billing info: company name, address and VAT number on every invoice

**Captured:** 2026-11-25 · **Scope:** [w6/m628](../../.pm/w6/blocked/m628/README.md) · **Sources:** Render's feedback board and Stripe's documentation, read on 2026-11-25; the community forum through search summaries where noted

## What Render does

| Claim | Source (read 2026-11-25) | Verified |
| --- | --- | --- |
| A request filed 2019-12-03, "Display company name and address on Render invoice" ("Certain countries (like France) need the customer name and address displayed on the invoice"), is complete. Render's CEO wrote on 2020-06-15: "The billing page in the dashboard now has button to add additional billing info to be displayed on all invoices." | [feedback.render.com/features/p/display-company-name-and-address-on-render-invoice](https://feedback.render.com/features/p/display-company-name-and-address-on-render-invoice) | yes |
| A comment on that request (2019-12-09) cites the EU's invoicing rules, which list the "Customer's full name & address" and the "Customer's VAT identification number". | same page | yes |
| The billing info form reads "Provide additional billing info such as company name, address, or VAT number". Render's staff wrote that the VAT number field appears only once an EU country is chosen, and later fixed it for Czechia, where it had not appeared. | `community.render.com/t/unable-to-enter-vat-number-for-billing/1399`, through a search summary | **no** (summary only) |
| Two later threads, "VAT is missing in the invoice" (2025-03-06) and "Unable to Register Thailand Tax ID/ VAT Number" (2024-11-02), suggest the tax ID field is still EU-only. | the same thread's related topics, through a search summary | **no** (summary only) |
| Who may edit it: the Billing role "Has full access to the workspace's billing settings"; Developer views billing details without editing them. | [render.com/docs/team-members](https://render.com/docs/team-members), as captured in [billing-email.md](billing-email.md) | yes |

**The forum could not be read.** `community.render.com` answers `301` to `render.discourse.group`, which did not resolve on 2026-11-25 (`getaddrinfo ENOTFOUND`), as on 2026-11-24.

**Unverified:**

- What Render's invoices print as the Bill-to when no billing info is set: the workspace's name, its owner's name or its email. No public source shows a Render invoice.
- The full field list of Render's form beyond "company name, address, or VAT number", and whether a non-EU tax ID can be entered.
- Whether billing info is offered on every plan. Nothing published restricts it.

## What Stripe prints

- An invoice's Bill-to is the Customer's name, address and email: "Until the invoice is finalized, this field will equal customer.name. Once the invoice is finalized, this field will no longer be updated" (`customer_name`, and likewise `customer_address` and `customer_tax_ids`; [docs.stripe.com/api/invoices/object](https://docs.stripe.com/api/invoices/object)). A change therefore reaches the next invoice Stripe finalizes, never one already finalized.
- "A customer's tax IDs display in the header of invoice and credit note PDFs", and "The invoice includes the customer tax ID whether or not it's valid" ([docs.stripe.com/billing/customer/tax-ids](https://docs.stripe.com/billing/customer/tax-ids)). Neither depends on Stripe Tax, which bex keeps off ([ADR040 §6](../ADR040-billing-metronome.md)). Stripe checks each tax ID's format against its type, and checks EU VAT, GB VAT and AU ABN numbers against the government registers.
- With tax ID collection on, Checkout "saves the business name as the customer's name" ([docs.stripe.com/tax/checkout/tax-ids](https://docs.stripe.com/tax/checkout/tax-ids)). bex turns that collection on only once Stripe Tax is configured.

## What each of bex's Customer paths printed (live account, 2026-11-25)

Read on bex's live Stripe Customers (those tagged `bex_workspace`, retired ones left out), counting without printing any name:

| Path | Bill-to before w6/m628 | Live Customers |
| --- | --- | --- |
| Usage export (`EnsureCustomer`) | the workspace's `tea-` id | 4: 2 renamed to their workspace's name by `scripts/stripe-customer-names.py --live --apply` on 2026-11-25, 2 whose workspace's only name is its `tea-` id |
| Workspace creation (`workspaceCreationCustomer`) | the workspace's `tea-` id (the code) | none |
| A Customer Checkout created | the cardholder's name, which Checkout copies from the card, with the card's billing address | 9 |

The Stripe test-mode readings of an invoice PDF per path wait for a test-mode key ([w6/m628](../../.pm/w6/blocked/m628/README.md)).

## What bex does (w6/m628)

- Every Customer bex creates is named after its workspace: its display name, else its slug. A Customer Checkout creates is renamed to it when the card binds, unless Checkout collected a business name with a tax ID, which stays as the company name.
- A workspace rename renames its Customer, so the next invoice bills the new name, unless the Customer's name is a member's own: a company name set in Billing info, or a name typed into the Stripe portal. A Stripe failure does not fail the rename; it is logged.
- **Billing info** on the Billing page, and over REST (`GET`/`PUT /v1/workspaces/{workspaceId}/billing/info`), GraphQL (`workspaceBillingInfo`, `setWorkspaceBillingInfo`) and MCP (`get_billing_info`, `update_billing_info`), a bex extension: a company name, an address and one VAT or tax ID, typed as one of Stripe's customer tax ID types. The dialog shows the tax ID field once a country with a Stripe tax ID type is chosen. Admin, Developer and Billing read it; Admin and Billing set it. A tax ID Stripe refuses answers `BILLING_INFO_INVALID` naming the field.
- It is kept on the Stripe Customer, the same fields the portal edits. A workspace with no Customer yet (no payment method ever added) reads `customerReady: false`, and a save answers `BILLING_CUSTOMER_NOT_READY`.
- Without a company name the invoices bill the workspace's name. Showing a tax ID collects no tax.
- The 9 Customers Checkout created before w6/m628 still bill the cardholder's name; whether they move to the workspace's name is [w6/180](../../.pm/w6/180.md).
