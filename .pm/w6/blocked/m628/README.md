# w6 · m628 — Invoices bill the workspace by name, and Billing info adds company name, address and VAT or tax ID, as on Render

**Worker:** worker6 **Goal:** a workspace's invoices name the workspace, not its `tea-` id, and the Billing page's Billing info adds a company name, address and VAT or tax ID to every later invoice, as on Render. **Status:** blocked (t001–t007 done; t009, the Stripe test-mode invoice check, needs a test-mode key from the user)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [See what each Customer-creation path prints as Bill-to, and capture Render's billing info form](done/t001.md) — **DONE** | 30m | — |
| t002 | [Stripe Customers are named after the workspace and follow a rename](done/t002.md) — **DONE** | 45m | t001 |
| t003 | [Billing info: company name, address and tax ID on the Billing page and every API surface](done/t003.md) — **DONE** | 1h15m | t002 |
| t004 | [Backfill existing Customers still named by their tea- id](done/t004.md) — **DONE** | 30m | t002 |
| t009 | [Stripe test-mode check: each path's invoice bills the workspace's name, a rename follows, and billing info prints on the next invoice](t009.md) | 45m | t002, t003, t004 |
| t005 | [Render parity](done/t005.md) — **DONE** | 30m | t003, t004 |
| t006 | [Simplify](done/t006.md) — **DONE** | 20m | t005 |
| t007 | [Test coverage](done/t007.md) — **DONE** | 45m | t005 |
| t008 | [Closeout](t008.md) | 10m | t006, t007, t009 |

## Definition of done

- In Stripe test mode, the invoice of a workspace created through each path (`EnsureCustomer`, `prepareWorkspaceCreation`, a Checkout-created Customer) bills the workspace's display name, and after a rename the next invoice bills the new name.
- An Admin or Billing member enters a company name, address and a VAT or tax ID in Billing info (the tax ID field appears once a country is chosen); the next finalized test invoice shows them, and a later rename does not overwrite the company name.
- GraphQL, REST and MCP read and write the same billing info; members who cannot view billing get none of it, and writes need `can_manage_billing`.
- The backfill script lists every bex Customer still named by its `tea-` id; the live run renames them, or the item records the user's command it waits for.

## Progress (2026-11-25)

- t001–t007 are done. Every Customer bex creates is named after its workspace (usage export, workspace creation, and a Checkout-created Customer once its card binds), and a rename renames it unless its name is a member's own (a company name set in Billing info, marked `bex_company_name`, or a name typed into the portal). Billing info (company name, address, one VAT or tax ID typed as a Stripe tax ID type) is read by Admin, Developer and Billing and set by Admin and Billing on the Billing page, GraphQL, REST and MCP, kept on the Stripe Customer; a tax ID Stripe refuses answers `BILLING_INFO_INVALID` naming its field, and a workspace with no Customer answers `BILLING_CUSTOMER_NOT_READY`. The dialog shows the tax ID field once a country with a Stripe tax ID type is chosen.
- Live backfill (t004, 2026-11-25): `scripts/stripe-customer-names.py --live` saw 16 bex Customers among 139 on the shared account; 2 were still named by their `tea-` id and `--apply` renamed them (3 retired skipped, 2 whose workspace's only name is its id left as they are, 9 left with their own name). A re-run lists none to rename.
- t001's live reading: those 9 are Customers Checkout created, each billing its cardholder's name with the card's address; they keep it, an open decision filed as [w6/180](../180.md).
- The definition of done holds in code and in the store-backed `TestBillingInfoOnEverySurfaceE2E` (real Postgres and OpenFGA, the real Stripe client against a Stripe stand-in), and the backfill's live run has happened. Its Stripe test-mode bullets (a finalized invoice per path, the billing info on the next invoice, the restricted key's tax ID permission) need a Stripe test-mode key, the one [m625](../m625/README.md), [m626](../m626/README.md) and [m627](../m627/README.md) wait for: [t009](t009.md) says what to run, and t008 closes the milestone after it.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w6` 2026-11-24 (billing & invoicing), item 5, filed by `/pmx them all to w6`. bex names Stripe Customers by workspace id: `Name: stripe.String(tenantID)` in `EnsureCustomer` (`lego/backend/internal/billing/stripe.go`) and `Name: stripe.String(workspaceID)` in `workspaceCreationCustomer` (`workspace_creation.go`); a Checkout-created Customer is named only when Stripe Tax is configured (`sessions.go`), which it is not ([ADR040](../../../../docs/ADR040-billing-metronome.md) §6). Company, address and tax ID can be entered only in the Stripe portal.
- **Render:** "The billing page in the dashboard now has a button to add extra billing info that shows on all invoices" (feedback.render.com/features/p/display-company-name-and-address-on-render-invoice); the form reads "Provide additional billing info such as company name, address, or VAT number", and its VAT field appears once an EU country is chosen (community answers read through search summaries on 2026-11-24 (community.render.com redirected to render.discourse.group, which did not resolve)).
- **Recommendation, not a user decision:** the default Bill-to is the workspace's display name. What Render prints when no billing info is set is unverified; a `tea-` id is not a name a customer chose in either product.
- **Tax:** showing a customer's tax ID collects no tax. Stripe Tax stays off until the operator supplies its legal inputs (ADR040 §6); [w6/177](../../done/177.md) records that open row.
- **Live step:** t004's live run writes bex's own Customers on the Stripe account bex shares with other products. If the agent is refused it, as in [w2/070](../../../w2/blocked/070.md), t004 parks on the user's run and the rest ships.
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)); ADR040 billing.
- **Expected outcome:** invoices a customer can file for expenses or a VAT reclaim, under their own name.
- **Why now:** every invoice bex has issued so far bills a `tea-` id.
- **Render parity:** included: GraphQL, REST, MCP and the Billing page change.
