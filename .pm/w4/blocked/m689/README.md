# w4 · m689 — A Pro or Scale workspace with no services and no activity in a month pays no plan fee for that month, as on Render

**Worker:** worker4 **Goal:** a workspace (or organization) on a paid plan that held no service and saw no activity for a whole month gets that month's plan fee waived on its invoice, and sees why on every surface, as Render's pricing page promises. **Status:** blocked (t001 and t005 done; t002–t004 wait for the user's decision to charge the plan fee, and t004 also for a Stripe test-mode key)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Decide what counts as a service and as activity, and record it in ADR030 §19 — **DONE** | 45m | — |
| [t002](t002.md) | The month's invoice carries a "Plan fee waived" line for a qualifying workspace or organization | 1h30m | t001 |
| [t003](t003.md) | The waiver shows on `GET /v1/usage`, GraphQL `usage`, MCP `get_usage` and the Billing page | 1h | t002 |
| [t004](t004.md) | Stripe test mode on dev-4: an empty Pro workspace's invoice carries the waiver, one with a suspended service does not | 45m | t003 |
| [t005](done/t005.md) | ADR018's Billing & invoicing section records the waiver — **DONE** | 20m | t003 |
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
- **Goal linkage:** [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first) for billing: a paid workspace left empty is not charged for it on Render.
- **Expected outcome:** a team that pauses a Pro workspace by deleting its services stops paying the plan fee, as it would on Render.
- **Why now:** bex's billing parity work (w6/m625–m628) covers the Billing page's invoices and payment; this rule is the one Render billing promise with no row at all.
- **Render parity:** included: three API surfaces, the Billing page and the invoice change.

## Parked 2026-11-26 (`/loopx w4` triage)

- **Render's rule holds.** render.com/pricing, re-read 2026-11-26, still says "If a workspace has no services (live or suspended) and no activity during a month, the workspace subscription fee for that month will be waived." `render.com/docs/llms.txt` has no "waive".
- **The fee this milestone waives is not charged.** bex lists the Pro and Scale fees ($17.50 and $349.30, ADR030 §2) and charges neither:
  - `lego/backend/internal/pricing/pricing.yaml` says attaching them "as licensed subscription items is a billing follow-up".
  - `docs/runbooks/stripe-billing-setup.md` creates no Price for them.
  - `EnsureContract` puts only metered Prices on a Subscription, and `lego/backend/internal/billing/contracts.go` refuses any other.
  - `Sheet.WorkspaceUSDPerMonth` (`internal/pricing/pricing.go`) has no caller outside its package.
  - `workspaces.Service.ChangePlan` only writes the tenant's plan.
  - w7/m464 found the same thing on 2026-11-17 ("bex charges no plan fee at all today"), and w7/082 is parked on it. No board item owns charging the fee.
- **So every Pro and Scale workspace already pays no plan fee, empty or not.** A negative "Plan fee waived" line on an invoice that bills no fee would discount the workspace's usage, or carry a negative balance to its next invoice. That is a credit, which the definition of done rules out. t002's own step 3 skips a workspace whose fee was not billed, and today that is every workspace. t003's field would be absent everywhere, and t004 would have no fee to net. None of the three can ship before the fee exists.
- **Done now.** t001: [ADR030](../../../../docs/ADR030-pricing.md) §19 states the rule for the day the fee is charged, with the definitions of a service and of activity, the organization rule, the calendar month and the next-invoice line. It also states that no fee is charged today. t005: the [ADR018 Billing & invoicing](../../../../docs/ADR018/billing-invoicing.md) row records the waiver, its five unsigned choices (charging no plan fee among them) and this item as owner.
- **Gate (the user, `/unblock w4`):** decide whether bex charges the listed plan fee on Stripe.
  - _Yes:_ the plan-fee Subscription item (licensed, once per organization on the primary, ADR030 §2) ships first, as its own item. Then a worker runs t002–t003 against it, then t004 with a Stripe test-mode restricted key (`rk_test_…`, the key w6/m625 t004 waits for). Neither `.env` nor the Stripe CLI login on this host holds one today. t006–t009 close it.
  - _No:_ sign off ADR018's difference (5) in `docs/ADR018/billing-invoicing.md`, that bex charges no plan fee. The waiver is then moot, and this milestone closes on t001 and t005 with ADR030 §19 trimmed to that fact.
