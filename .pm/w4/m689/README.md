# w4 · m689 — A Pro or Scale workspace with no services and no activity in a month pays no plan fee for that month, as on Render

**Worker:** worker4 **Goal:** a workspace (or organization) on a paid plan that held no service and saw no activity for a whole month gets that month's plan fee waived on its invoice, and sees why on every surface, as Render's pricing page promises. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Decide what counts as a service and as activity, and record it in ADR030 §19 | 45m | — |
| [t002](t002.md) | The month's invoice carries a "Plan fee waived" line for a qualifying workspace or organization | 1h30m | t001 |
| [t003](t003.md) | The waiver shows on `GET /v1/usage`, GraphQL `usage`, MCP `get_usage` and the Billing page | 1h | t002 |
| [t004](t004.md) | Stripe test mode on dev-4: an empty Pro workspace's invoice carries the waiver, one with a suspended service does not | 45m | t003 |
| [t005](t005.md) | ADR018's Billing & invoicing section records the waiver | 20m | t003 |
| [t006](t006.md) | Render parity | 25m | t004, t005 |
| [t007](t007.md) | Simplify | 15m | t006 |
| [t008](t008.md) | Test coverage | 30m | t006, t007 |
| [t009](t009.md) | Closeout | 10m | t008 |

Total: **9 tasks, about 5h40m**.

## Definition of done

- ADR030 §19 states the rule, quoting Render's FAQ sentence, with bex's definitions of a service and of activity.
- A qualifying workspace's (or organization's) invoice carries exactly one negative line equal to that month's plan fee; a workspace that held any service, even suspended, or saw any member or API-key action other than sign-in, sign-out and billing reads, pays the fee.
- REST `GET /v1/usage`, GraphQL `usage`, MCP `get_usage` and the Billing page show the waived month alike.
- In Stripe test mode an empty Pro workspace's invoice nets the fee to zero and one with a suspended service does not.
- No ADR071 credit machinery is used.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w6`, 2026-11-26 (the aspect: what Render publishes outside its API pins that bex's parity ledger never read — its docs pages, its pricing page, and the Cloudflare headers its docs tell apps to use), filed to w4 by `/pm them to w4 and ship`. Re-checked at `origin/main` `f32c8164fb` before filing: no open, blocked or done item on any workstream owns it. Item 3.
- **Render:** render.com/pricing FAQ, read 2026-11-27: "If a workspace has no services (live or suspended) and no activity during a month, the workspace subscription fee for that month will be waived."
- **bex today:** no ADR, ledger row or code mentions a waiver (`waive` appears in none of ADR030, ADR040, `docs/ADR018/billing-invoicing.md` or `lego/backend/internal/billing/`). `docs/render-artifacts/pricing.md` (2026-07-13) and the plan-table fixtures captured the pricing tables, not the FAQ.
- **Gate:** t004 needs the Stripe test-mode key w6/blocked/m625 t004 waits for. t001–t003 and t005 do not.
- **Anti-goal check:** the waiver is an automatic invoice line, not an ADR071 credit (`.pm/DO_NOT_DO.md`, ADR071 line).
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first) for billing: a paid workspace left empty is not charged for it on Render.
- **Expected outcome:** a team that pauses a Pro workspace by deleting its services stops paying the plan fee, as it would on Render.
- **Why now:** bex's billing parity work (w6/m625–m628) covers the Billing page's invoices and payment; this rule is the one Render billing promise with no row at all.
- **Render parity:** included: three API surfaces, the Billing page and the invoice change.
