# w6 · m400 — Agent spend surfaces count billing credits

**Worker:** worker6 **Goal:** the agent environment and the composer show what billing credits cover and warn against the spend a charge would collect, matching the mint's credit-first rule (ADR091 D10.2a, w6/m362 t017). **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Agent environment: the credit the spend read counted and the unbilled spend, on REST, GraphQL and MCP | 1h | — |
| [t002](t002.md) | Record the credit when the Billing page or an invoice webhook reads it | 1h | t001 |
| [t003](t003.md) | Settings → Agents shows what billing credits cover; the composer warns against the unbilled spend | 1h30 | t001 |
| [t004](t004.md) | Production check on the bex workspace | 30m | t002, t003 |
| [t005](t005.md) | Render parity (Devin): credit and spend surfaces agree | 20m | t004 |
| [t006](t006.md) | Simplify | 15m | t005 |
| [t007](t007.md) | Test coverage | 45m | t005, t006 |
| [t008](t008.md) | Closeout | 10m | t007 |

## Definition of done

REST, GraphQL and MCP return `agentModelCreditUsd` and `agentModelUnbilledUsd` equal to the mint's values. Settings → Agents shows the credit. A credited workspace sees no charge warning before its credit plus threshold. The values are known before the first crossing.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w6` (2026-11-14), item 5. Follow-up to w6/m362 t017. `dashboard/src/features/agent-environment/lib/spend.ts` warns on month-to-date ÷ cap, and no surface returns the credit or the unbilled spend the mint uses.
- **Goal linkage:** ADR091 D10.2a (credits pay first), ADR008 pillar 5.
- **Expected outcome:** a workspace with credits sees what they cover and is warned only when its card is about to be charged.
- **Why now:** t017 goes live with the next batched deploy, and the bex workspace holds credits.
- **Anti-goal check:** read-only credit display, which `.pm/DO_NOT_DO.md` (ADR071 issuance) leaves allowed, like w5/m70.
- **Render parity:** included (Devin): the agent environment and composer change.
