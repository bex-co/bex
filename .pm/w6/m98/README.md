# w6 · m98 — Fix billing "Total month to date" collapsing to $0.00 when Stripe credit grants absorb real usage charges

**Worker:** worker6 **Goal:** the Billing page's headline total always shows the real current-period usage charge — never silently netted to zero by Stripe credit-grant consumption — with credit consumption itself shown as its own honest figure. **Status:** in-progress — t001–t006 done; t007 (live verification) blocked on deploy

## Background (found live, 2026-08-25/26 `/qa-find-bugs` hunt, 6th run of the day)

Signed in as the QA user (workspace `bex`, `tea-d98210cbbpdc73dcrkvg`, pro plan, a $1000 Stripe credit grant named "superadmin's privilege" active) and opened `https://dashboard.bex.co/billing`. The "Charges" category tree read Services $23.01 / Postgres $26.57 / Key Value $21.41 / Sandboxes $3.75 (sums to ~$74.74), but the "Total month to date" line directly beneath it read **$0.00 USD** — the page visibly contradicts its own itemization.

From inside the authenticated page (`fetch('https://api.bex.co/graphql', {credentials:'include'})`, not a bare script — per this hunt's own Phase-3 trap about non-browser UAs getting Cloudflare-blocked), the raw `Usage` GraphQL response for the same workspace/period showed:

```
estimatedCost.totalUsd        = "74.78"
billing.currentCost.amountUsd = "0.00"
billing.credits.availableUsd  = "1000.00"
billing.credits.grants        = [{ name: "superadmin's privilege", remainingUsd: "1000.00" }]
billing.invoices              = []
```

The itemized tree (`estimatedCost`) and the headline total (`billing.currentCost`) are two independently-sourced fields, and only one of them reflects the $1000 credit grant's consumption. The same `currentCost.amountUsd` value also drives `CreditBalanceCard`'s "Credits applied → amount due" line — so a period where **~$74.78 of credit was actually consumed** displays as "**$0.00 applied, $0.00 due**", i.e. the exact opposite of what happened. Two dashboard readouts break from one bad backend value simultaneously.

### Root cause

`lego/backend/internal/billing/read.go:255-272` (`currentInvoice()`), specifically line 265:

```go
amount, err := normalizedStripeAmount(inv.Total, inv.Currency)
```

`inv.Total` comes from Stripe's `Invoices.CreatePreview` (the upcoming/preview invoice). Under Stripe's Billing Credit Grants mechanism (the mechanism `docs/ADR071-tenant-billing-credits.md` documents and this workspace uses), eligible credit-grant consumption is applied directly on that same preview call — Stripe's own docs state grants apply "after discounts, but before taxes and the customer's `invoice_credit_balance`," and that this happens on preview invoices too, not only finalized ones. This is **the exact "invoice total is unreliable under flexible/credit billing" caveat the codebase already documents and works around** for the separate `Credits` block:

> `lego/backend/internal/billing/read.go:44-46` — _"Values come from Stripe's credit_balance_summary/credit_grants APIs — never derived from the invoice preview, whose total does not reliably reflect credit under flexible billing mode."_

That same caution was never applied to `currentCost`. `dashboard/src/features/usage/components/charges-card.tsx:211-219` then trusts any non-null `invoicedUsd` over the category-sum fallback **by design** (its own comment: "Stripe's real current-period amount... shown as the total when present"; locked in by `charges-card.test.tsx:234-250`, "prefers Stripe's real amount over the estimate when one exists") — so a net-of-credit `"0.00"` silently overrides the real $74.78 estimate instead of falling back to it.

**This is not simply "read the wrong Stripe field."** Stripe's own field documentation is ambiguous about whether `inv.Subtotal` is actually gross-of-credit for the Billing Credit Grants feature specifically — some Stripe documentation suggests credit grants can apply directly to eligible line items before `subtotal` itself is computed, in which case even `Subtotal` might already be net of credit. **This needs a live experiment against Stripe (test mode, a subscription with a credit grant and nonzero metered usage) before committing to a specific field**, not a confident one-line swap from `Total` to `Subtotal`.

### Blast radius (exhaustive, not an estimate)

`normalizedStripeAmount(inv.Total, ...)` is called from exactly **2** sites in `lego/backend/internal/billing/read.go`:

| call site | line | feeds |
| --- | --- | --- |
| `currentInvoice()` | 265 | `billing.currentCost.amountUsd` (confirmed broken live, above) |
| `finalizedInvoices()` | 288 | every historical `Invoice.amountUsd` in "Invoice history" — same bug class, **not yet observed live**: this workspace's `billing.invoices` is currently `[]` (no finalized invoices exist to check) |

Consumers, confirmed by grep — **GraphQL only**: `lego/backend/internal/usage/graphql.go:131` (`currentCost.amountUsd`) and `:143` (`invoices[].amountUsd`). `AmountUSD`/`amountUsd` has **no** REST or MCP wiring anywhere in `internal/usage/rest.go` or `internal/usage/mcp.go` — so this field is dashboard-GraphQL-only; no REST/MCP parity fix is required for the field itself (see Render parity task for what *is* in scope).

Dashboard consumers of `currentCost.amountUsd`, both of which must move together since they share the one backend value:

- `dashboard/src/features/usage/components/charges-card.tsx:211-219` — the "Total month to date" headline.
- `dashboard/src/features/usage/components/billing-summary-cards.tsx:59-88` (`CreditBalanceCard`) — the "Credits applied → amount due" line, computed as `applied: min(available, cost)` / `due: max(0, cost - available)` from the same value, currently reading `applied: 0.00` in the exact period real credit was consumed.

### Adjacent classes

Not applicable — this is a value-correctness bug in a billing computation, not an authz/existence boundary. No forbidden/unauthenticated/timeout distinction is affected.

### Unverified (carried forward, not asserted as fact)

- Whether Stripe's `inv.Subtotal` is actually gross-of-credit for the Billing Credit Grants mechanism as configured on this account — the spike task (t001) must settle this before t002 lands a fix.
- Whether the finalized "Invoice history" list (`finalizedInvoices()`, read.go:288) shows the same $0 symptom live — this workspace has zero finalized invoices to observe this run.
- Whether any workspace-level credit-consuming Stripe configuration besides the one observed "superadmin's privilege" grant exists.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Spike: determine Stripe's real gross-vs-net-of-credit invoice-preview semantics under Billing Credit Grants — **DONE** | 30m | — |
| t002 | Backend: `currentInvoice`/`finalizedInvoices` report the gross current-period charge; expose credit consumed this period as its own figure — **DONE** | 45m | t001 |
| t003 | Dashboard: reconcile `ChargesCard` total + `CreditBalanceCard` applied/due math with the corrected backend shape — **DONE** | 30m | t002 |
| t004 | Render parity — **DONE** | 20m | t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage — **DONE** | 30m | t004 |
| t007 | Closeout | 10m | t005, t006 |

## Definition of done

- Live on `dashboard.bex.co/billing` for a workspace with an active credit grant and nonzero metered usage this period: the "Total month to date" figure equals the sum of the category tree above it (modulo the documented sub-cent rounding difference between the backend's single rounding and the tree's per-resource rounding) — it never reads $0.00 while the categories sum to a nonzero amount.
- The "Credits applied → amount due" line reports a **nonzero "applied"** figure whenever credit was actually consumed this period (verified against the same live workspace used in this hunt, `tea-d98210cbbpdc73dcrkvg`) — it must stop reading "$0.00 applied" in a period where ~$74.78 of credit was consumed.
- `GET`/GraphQL read of `usage.billing.currentCost.amountUsd` for that workspace/period returns the gross figure (or the schema gains a distinct field for it) — verifiable by re-running this hunt's exact in-page `fetch` probe against the `Usage` query and comparing to `estimatedCost.totalUsd`.
- A new backend test stubs a Stripe invoice-preview response shaped like the real bug (an applied credit grant zeroing `Total` while metered line items are nonzero) and asserts the resolved `currentCost.amountUsd` is **not** silently zero — closing the gap `lego/backend/internal/billing/stripe_test.go` currently has (no existing test combines a nonzero-usage fixture with a credit-grant fixture).
- `finalizedInvoices()` (read.go:288) is fixed by the same change or explicitly re-verified separately once this workspace (or a test fixture) has a finalized invoice to check against — not left silently assumed-fixed.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co`, 2026-08-25/26 (6th run of the day). Evidence: `.playwright-mcp/qa-billing-1.png` (the $0.00 total against nonzero category rows, captured this run); raw `Usage` GraphQL response quoted above, captured via in-page `fetch` during this run.
- **Goal linkage:** [docs/ADR040-billing-metronome.md](../../../docs/ADR040-billing-metronome.md) (`currentCost` is defined there as "what the workspace is actually charged") and [docs/ADR071-tenant-billing-credits.md](../../../docs/ADR071-tenant-billing-credits.md) (documents the credit-grant mechanism whose interaction with the invoice preview causes this). A billing page that visibly contradicts its own numbers is a trust problem on the one surface where trust matters most.
- **Expected outcome:** a workspace with active credits sees an accurate, internally-consistent billing page — the gross charge, the credit consumed, and the net amount due are three honest numbers that add up, instead of one field silently absorbing the other two.
- **Why now:** live on production right now, affects every workspace with an active Stripe credit grant (not deploy-lag — confirmed still present at `main`@`bb93a00e` via direct source read), and is cheap to scope precisely (exhaustive 2-call-site blast radius, GraphQL-only, no REST/MCP surface to touch) even though the actual Stripe-semantics spike needs care before the fix lands.
- **Render parity:** included (t004) — this touches a GraphQL field (`usage.billing.currentCost`) and the dashboard UI; confirm REST/MCP genuinely have no equivalent surface to fix (grep in this hunt found none — the parity task should re-confirm rather than assume) and record the render.com comparison as n/a (bex's Stripe-based usage billing + ADR071 credits have no Render equivalent).
