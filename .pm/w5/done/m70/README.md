# w5 · m70 — Billing page: rename Usage→Billing + remaining-credit display

**Worker:** worker5 **Goal:** the dashboard's money surface is named what Render users expect (Billing), and a workspace with promotional Stripe credit can see its remaining balance, per-grant expiry, and the credit-adjusted amount due — without any wire-surface rename **Status:** done

## Tasks (in order)

| id   | title                                                                        | est | depends_on |
| ---- | ---------------------------------------------------------------------------- | --- | ---------- |
| t001 | Rename the nav/route surface: /billing real, /usage shim, deliberate-rename notes — **DONE** | 45m | —          |
| t002 | Restructure the page: Billing section above Usage section — **DONE** | 30m | t001       |
| t003 | Backend credits block: Stripe credit reads → REST/GraphQL/MCP — **DONE** | 60m | —          |
| t004 | Dashboard credits UI: stat card, applied-credit line, expiry note, gate copy — **DONE** | 45m | t002, t003 |
| t005 | Render parity — **DONE** | 30m | t004       |
| t006 | Simplify — **DONE** | 30m | t005       |
| t007 | Test coverage — **DONE** | 45m | t005       |
| t008 | Closeout — **DONE**                                                           | 15m | t007       |

## Progress log

- 2026-08-18 — t001–t007 shipped (`3a98d44b`): `/billing` real page, `/usage` query-preserving shim, Billing-above-Usage layout, read-only `credits` block on REST/GraphQL/MCP, degrade-by-omission, ADR046 gate copy unchanged. t008 held for a live Stripe credit grant on a real workspace.
- 2026-08-19 — closeout. Live grant remaining **$1000 USD** (paid, not voided, no expiry); Stripe `credit_balance_summary` metered applicability_scope matches. Prod `bex-api` `:8091/metrics`: `bex_billing_enabled 1`, `credit_read` success=8, no error series. Prod dashboard `/usage` → 307 `/billing` (query preserved); `/billing/update-plan` still opens change-plan. Backend billing/usage tests + dashboard billing-redirects/credits UI tests green. Authenticated screenshot omitted (login-gated); live `credit_read` + remaining balance is the observable credits-block proof.

## Definition of done

All of the following hold on a real workspace:

1. The sidebar item reads "Billing" (credit-card-class icon), `/billing` renders the page, `/usage` redirects to `/billing` preserving query/deep links, and `/billing/update-plan` still opens the change-plan dialog. REST `/v1/usage`, GraphQL `usage`, and MCP `get_usage` are byte-identical to before.
2. The page renders a Billing section (readiness/payment/lifecycle/invoice preview) above a Usage section (metered detail + resource caps).
3. A workspace with an active Stripe credit grant sees a "Credits remaining $X" stat card (hidden at zero), a "Credits applied −$X → amount due $Y" line in the invoice-preview card, and the earliest grant expiry when one exists; the values come from `credit_balance_summary`/`credit_grants`, never derived from `create_preview`.
4. The same `credits` block appears identically on REST, GraphQL, and MCP; a Stripe credit-read failure omits the block (readiness/invoice preview unaffected), and a runtime key without the credit read permission degrades the same way.
5. The ADR046 payment gate is unchanged: a credit-holding cardless workspace is still 402-gated, and the onboarding copy explains a card is still required.
6. ADR023's "Usage is the deliberate counterpart" note and the `/billing.$` route comment describe the post-rename architecture; runbook §2's permission inventory records the added credit read permission.
7. Backend + dashboard suites green; prettier clean on touched markdown.

## Source + Goal linkage

- **Source:** 2026-08-17 discussion following the Stripe live cutover (runbook §6 executed 2026-08-16, first live tenant card-bound 2026-08-17): user request to learn from Render's workspace billing page (credit balance display) and to rename Usage→Billing for consistency. Credits are currently granted manually via Stripe Dashboard credit grants (the ADR071 product mechanism remains Proposed/unbuilt; this milestone is read-only display, not grant management).
- **Goal linkage:** revenue viability + Render parity (ADR008): the billing plane went live and now needs a self-explanatory tenant-facing surface; credit visibility is the prerequisite for using credits commercially (promos, compensation) without support tickets.
- **Expected outcome:** Render-shaped IA (`/billing` is the real page, as on dashboard.render.com), and a tenant granted $N credit can see balance/expiry/adjusted amount due on dashboard, REST, GraphQL, and MCP.
- **Why now:** live billing shipped this week — the first credits will be granted against real invoices (first live invoice ~2026-09-16); rename-before-build keeps the credits UI from landing on a page whose name is about to change. Render parity task **included**: the credits block is a new REST/GraphQL/MCP/UI surface (bex extension — Render has no public credits API; compare dashboard behavior, not wire).
- **Out of scope (recorded):** ADR071's grant/approval/idempotent-operations machinery, credit-first payment-gate semantics, and any collection-management UI. Wire renames of the usage API (ADR023 extension) are explicitly excluded.
