# w6 · m362 — Reaching an agent-model charge threshold charges the card and the workspace keeps working

**Worker:** worker6 **Goal:** a new workspace using the platform model is charged at each trust-tier threshold and keeps working, climbing the tiers as its payments succeed; only a failed or unauthenticated charge stops its model spend (ADR091 D10.2a). **Status:** in progress — t001–t007 and t011–t016 done (t006 passed on production 2026-11-13: charged $10.50 in 4.9 s, the session kept running, T1 at once); t008–t010 remain, and t010 checks t016's window boundary on production after the batched deploy

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Backfill `billing_trust` from Stripe's paid invoices, and confirm new payments are counted](done/t001.md) — **DONE** | 1h30 | — |
| t002 | [Price sheet: no tier cap ceiling, an in-flight allowance per tier](done/t002.md) — **DONE** | 45m | — |
| t003 | [Every subscribed workspace's threshold reaches Stripe, so its threshold invoice is charged](done/t003.md) — **DONE** | 1h | t002 |
| t004 | [The mint keeps a workspace working while its charge is in flight, and stops it only on a failed or unauthenticated charge](done/t004.md) — **DONE** | 1h30 | t003 |
| t005 | [Settings → Billing, the agent environment (REST, GraphQL, MCP) and the CLI describe charge-to-continue](done/t005.md) — **DONE** | 1h30 | t004 |
| t011 | [bex charges the card the moment a workspace reaches its threshold, and the request waits for the result](done/t011.md) — **DONE** | 3h | — |
| t012 | [The month-end invoice bills only agent-model usage not already charged](done/t012.md) — **DONE** | 1h30 | t011 |
| t013 | [A failed agent-model charge stops only model spend; hosting billing is untouched](done/t013.md) — **DONE** | 1h | t011 |
| t014 | [Settings → Billing lists the mid-month agent-model charges, and the month-end invoice reads net of them](done/t014.md) — **DONE** | 1h30 | t012 |
| t015 | [Settle-before-delete charges unbilled agent-model usage through the same charger](done/t015.md) — **DONE** | 30m | t011, t012 |
| t016 | [A charge collects only usage no subscription invoice has billed](done/t016.md) — **DONE** | 2h30 | t011, t012 |
| t006 | [Production check: a T0 workspace reaches $10, is charged at once and keeps working](done/t006.md) — **DONE** | 1h | t001, t005, t011–t016 |
| t007 | [Render parity (Devin): spend and charge surfaces agree](done/t007.md) — **DONE** | 20m | t006 |
| t008 | [Simplify](t008.md) | 15m | t007 |
| t009 | [Test coverage](t009.md) | 45m | t007, t008 |
| t010 | [Closeout](t010.md) | 10m | t009 |

## Definition of done

- On production, a workspace at T0 whose platform-model spend reaches $10 is charged by bex at that moment (one invoice, paid off-session within seconds) and the request that crossed it is served; it reads T1 at once, and its next threshold is $25.
- Agent-model charges and hosting's monthly billing coexist on one customer, subscription and card: the cycle invoice bills only agent-model usage not already charged, a failed model charge stops only model spend, a failed hosting invoice still opens hosting's grace and holds model spend at T0, and Settings → Billing lists both kinds of charge.
- A charge that fails, or that needs 3-D Secure, stops platform-model spend with `BILLING_BALANCE_DUE` and a hosted payment link; nothing else refuses at a threshold. The workspace's own monthly cap still refuses at the cap.
- No subscription item keeps a Stripe `billing_thresholds`: bex's charge is the only mid-cycle mechanism.
- `billing_trust` carries every workspace's paid, non-zero invoices, including those before migration 0194.
- Settings → Billing and the agent environment on REST, GraphQL and MCP state the next charge point and no longer report a T0 ceiling.

## Source + Goal linkage

- **2026-11-13 user decisions (after t001–t005 shipped):** "应该是执行到 [上限] 然后扣钱，扣成功了就不阻断，否则打断" — charge at the moment the workspace would be stopped; only a failed charge interrupts (t011). Then `/pmx for w6 to implement such 两种扣法叠在一起，并且解决要守住几条边界`: agent-model charges coexist with hosting's monthly billing on one account, with the boundaries in t012–t015.
- **Recommendations, not user decisions:** the charge point is the tier's threshold ($10 at T0), and the $5 in-flight allowance retires with the race it covered; the user's sentence named $15, the stop point at the time, and moving the first charge to $15 is one `pricing.yaml` value. Netting a paid charge out of the cycle invoice with a negative pending invoice item on the subscription (t012, revised: not exporting it to the meter cannot undo hours already exported); it issues no value, so it is not `.pm/DO_NOT_DO.md`'s rejected ADR071 credit system.
- **Why the interim mechanism is replaced:** it reused Stripe's asynchronous threshold invoice (an hourly meter export plus 15 to 28 minutes) behind a $5 buffer, which a slow invoice could still exhaust and stop a workspace whose card was fine — the race the user ruled out.

- **Source:** [ADR091 D10.2a](../../../docs/ADR091-agent-model-pricing.md) (amended 2026-11-13, user decision: "到达 $10 的时候应该是扣钱以继续，而不是阻止 … 收到钱 / 升级 / 收到更多钱 / 然后逐渐升级"). Found while timing the routines in `w6/blocked/m354`: the bex workspace (Scale plan, card bound since 2026-08-17) reached $10.01 of platform-model spend on 2026-11-13, no threshold invoice came, `billing_trust` held no row for any workspace, and every routine failed with `AGENT_MODEL_SPEND_CAP_REACHED` until the month's reset.
- **Goal linkage:** ADR008 pillar 5 (hosted agent sessions) and ADR091's non-payment protection; agent sessions on the platform model become the default path once w10/m14 t007 widens it.
- **Expected outcome:** a new workspace is never stopped while its card works; the bex routines run again as soon as t001 or t003 is live.
- **Why now:** w10/m14 t007 widens the platform model to every workspace (due 2026-11-13). Widening first would stop every new workspace at its first $10 for an hour or more, or for the rest of the month where no threshold is set.
- **Recommendations, not user decisions:** the in-flight allowance's initial value ($5 at T0, t002) and the retryable refusal past it (t004) are this milestone's recommendation; the rule they rest on (charge to continue, stop only on failure) is the user's.
- **The widening itself stays in w10/m14 t007** (dated 2026-11-13, gated on the OpenRouter credit limit, Radar rules, terms copy and a Stripe test key), so this milestone files no second widen task. t006 amends t007's pre-widening threshold check to this mechanism.
- **Render parity:** included (t007). Customer-facing billing and agent-environment fields change; the reference for agent sessions is Devin, and render.com has no agent-model billing.
