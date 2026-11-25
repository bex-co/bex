# w6 · m628 — Invoices bill the workspace by name, and Billing info adds company name, address and VAT or tax ID, as on Render

**Worker:** worker6 **Goal:** a workspace's invoices name the workspace, not its `tea-` id, and the Billing page's Billing info adds a company name, address and VAT or tax ID to every later invoice, as on Render. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [See what each Customer-creation path prints as Bill-to, and capture Render's billing info form](t001.md) | 30m | — |
| t002 | [Stripe Customers are named after the workspace and follow a rename](t002.md) | 45m | t001 |
| t003 | [Billing info: company name, address and tax ID on the Billing page and every API surface](t003.md) | 1h15m | t002 |
| t004 | [Backfill existing Customers still named by their tea- id](t004.md) | 30m | t002 |
| t005 | [Render parity](t005.md) | 30m | t003, t004 |
| t006 | [Simplify](t006.md) | 20m | t005 |
| t007 | [Test coverage](t007.md) | 45m | t005 |
| t008 | [Closeout](t008.md) | 10m | t006, t007 |

## Definition of done

- In Stripe test mode, the invoice of a workspace created through each path (`EnsureCustomer`, `prepareWorkspaceCreation`, a Checkout-created Customer) bills the workspace's display name, and after a rename the next invoice bills the new name.
- An Admin or Billing member enters a company name, address and a VAT or tax ID in Billing info (the tax ID field appears once a country is chosen); the next finalized test invoice shows them, and a later rename does not overwrite the company name.
- GraphQL, REST and MCP read and write the same billing info; members who cannot view billing get none of it, and writes need `can_manage_billing`.
- The backfill script lists every bex Customer still named by its `tea-` id; the live run renames them, or the item records the user's command it waits for.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w6` 2026-11-24 (billing & invoicing), item 5, filed by `/pmx them all to w6`. bex names Stripe Customers by workspace id: `Name: stripe.String(tenantID)` in `EnsureCustomer` (`lego/backend/internal/billing/stripe.go`) and `Name: stripe.String(workspaceID)` in `workspaceCreationCustomer` (`workspace_creation.go`); a Checkout-created Customer is named only when Stripe Tax is configured (`sessions.go`), which it is not ([ADR040](../../../docs/ADR040-billing-metronome.md) §6). Company, address and tax ID can be entered only in the Stripe portal.
- **Render:** "The billing page in the dashboard now has a button to add extra billing info that shows on all invoices" (feedback.render.com/features/p/display-company-name-and-address-on-render-invoice); the form reads "Provide additional billing info such as company name, address, or VAT number", and its VAT field appears once an EU country is chosen (community answers read through search summaries on 2026-11-24 (community.render.com redirected to render.discourse.group, which did not resolve)).
- **Recommendation, not a user decision:** the default Bill-to is the workspace's display name. What Render prints when no billing info is set is unverified; a `tea-` id is not a name a customer chose in either product.
- **Tax:** showing a customer's tax ID collects no tax. Stripe Tax stays off until the operator supplies its legal inputs (ADR040 §6); [w6/177](../177.md) records that open row.
- **Live step:** t004's live run writes bex's own Customers on the Stripe account bex shares with other products. If the agent is refused it, as in [w2/070](../../w2/blocked/070.md), t004 parks on the user's run and the rest ships.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)); ADR040 billing.
- **Expected outcome:** invoices a customer can file for expenses or a VAT reclaim, under their own name.
- **Why now:** every invoice bex has issued so far bills a `tea-` id.
- **Render parity:** included: GraphQL, REST, MCP and the Billing page change.
