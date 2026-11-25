# w6 · m625 — Each invoice on the Billing page downloads as a PDF, and an unpaid one has Pay Now, as on Render

**Worker:** worker6 **Goal:** the Billing page's invoice history gives each finalized invoice a PDF download and an unpaid invoice a Pay Now that opens Stripe's hosted invoice page, with the same links on REST, GraphQL and MCP, as Render's Billing page does. **Status:** blocked (t001–t003, t005–t007 done; t004 needs a Stripe test-mode key from the user)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Capture Render's invoice list: PDF download, Pay Now, who sees them](done/t001.md) — **DONE** | 30m | — |
| t002 | [Invoices carry Stripe's hosted page and PDF links on REST, GraphQL and MCP](done/t002.md) — **DONE** | 45m | t001 |
| t003 | [Invoice rows offer Download PDF and, when unpaid, Pay Now](done/t003.md) — **DONE** | 1h | t002 |
| t004 | [Test-mode check: download a PDF and pay an open invoice from the Billing page](t004.md) | 30m | t003 |
| t005 | [Render parity](done/t005.md) — **DONE** | 30m | t004 |
| t006 | [Simplify](done/t006.md) — **DONE** | 20m | t005 |
| t007 | [Test coverage](done/t007.md) — **DONE** | 45m | t005 |
| t008 | [Closeout](t008.md) | 10m | t006, t007 |

## Definition of done

- With a Stripe test-mode key, every finalized invoice in the Billing page's history has a working Download PDF link, and an `open` invoice has Pay Now; paying it on Stripe's page reads `paid` on the Billing page after a refresh.
- REST `GET /v1/usage` (`billing.invoices[]`), GraphQL `BillingInvoice` and MCP `get_usage` return the same `hostedInvoiceUrl` and `invoicePdfUrl` for the same invoice. A link Stripe does not supply is omitted (null on GraphQL), never invented.
- A member who cannot view billing (Contributor, Viewer; w9/m620) gets neither link on any surface.
- Invoice statuses are translated, not Stripe's raw words.

## Progress (2026-11-25)

- Shipped in `c3bd7a9887`: t001 (the capture, [billing-invoices.md](../../../../docs/render-artifacts/billing-invoices.md)), t002 (the links on REST, GraphQL and MCP), t003 (the row menu, translated statuses and a matching skeleton), and t005–t007 (parity, simplify, tests).
- The DoD's second to fourth bullets are met in code and tests: `TestInvoiceLinksAgreeAcrossSurfaces`, `TestInvoiceLinksWithheldFromMembersWhoCannotViewBilling` and the `InvoiceHistoryCard row menu` tests.
- The first bullet needs Stripe test mode, and no test-mode key is reachable ([t004](t004.md) lists where it looked and what to run). The milestone is parked on that key; t008 closes it after t004.
- Pay Now also shows on an `uncollectible` invoice, because bex still collects it (`unpaidInvoiceStatuses`, `lego/backend/internal/billing/settle.go`) and Stripe's hosted page still takes payment for it.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w6` 2026-11-24 (billing & invoicing), item 2, filed by `/pmx them all to w6`. Today `billing.Invoice` (`lego/backend/internal/billing/read.go`) and GraphQL `BillingInvoice` (`lego/backend/internal/usage/graphql.go`) carry only id, status, kind, amounts, currency and period, and `InvoiceHistoryCard` (`dashboard/src/features/usage/components/billing-summary-cards.tsx`) shows month, raw status and amount with no action, so a PDF or a payment means opening the Stripe Customer Portal. [billing-onboarding.md](../../../../docs/render-artifacts/billing-onboarding.md) called that a deliberate difference on 2026-07-28, before the Render parity first rule and with no user sign-off.
- **Render:** the Billing page lists past invoices (render.com/docs/render-dashboard); Render's team closed "Download invoice as PDF" with "users can now download their .pdf invoices at https://dashboard.render.com/billing" (feedback.render.com/features/p/download-invoice-as-pdf); each invoice's three-dot menu has "Pay Now", which opens Stripe for it (community answers read through search summaries on 2026-11-24 (community.render.com redirected to render.discourse.group, which did not resolve); t001 recaptures).
- **Recommendation, not a user decision:** everyone who can view billing (Admin, Developer, Billing; [w9/m620](../../../w9/done/m620/README.md)'s `can_view_billing`) gets both links, because Render shows its invoice list to the same roles and paying a hosted invoice changes no setting. Which roles see Render's Pay Now is unverified.
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)); ADR040 billing.
- **Expected outcome:** a member downloads an invoice and pays an unpaid one without leaving bex.
- **Why now:** w6/m626 reuses these links for the overdue invoice, and [w6/177](../../done/177.md)'s ledger row needs its owner.
- **Render parity:** included: REST, GraphQL, MCP and the Billing page change.
