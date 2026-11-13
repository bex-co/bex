# w6 · m362 — Reaching an agent-model charge threshold charges the card and the workspace keeps working

**Worker:** worker6 **Goal:** a new workspace using the platform model is charged at each trust-tier threshold and keeps working, climbing the tiers as its payments succeed; only a failed or unauthenticated charge stops its model spend (ADR091 D10.2a). **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Backfill `billing_trust` from Stripe's paid invoices, and confirm new payments are counted](t001.md) | 1h30 | — |
| t002 | [Price sheet: no tier cap ceiling, an in-flight allowance per tier](t002.md) | 45m | — |
| t003 | [Every subscribed workspace's threshold reaches Stripe, so its threshold invoice is charged](t003.md) | 1h | t002 |
| t004 | [The mint keeps a workspace working while its charge is in flight, and stops it only on a failed or unauthenticated charge](t004.md) | 1h30 | t003 |
| t005 | [Settings → Billing, the agent environment (REST, GraphQL, MCP) and the CLI describe charge-to-continue](t005.md) | 1h30 | t004 |
| t006 | [Production check: a T0 workspace crosses $10, is charged once and keeps working](t006.md) | 1h | t001, t005 |
| t007 | [Render parity (Devin): spend and charge surfaces agree](t007.md) | 20m | t006 |
| t008 | [Simplify](t008.md) | 15m | t007 |
| t009 | [Test coverage](t009.md) | 45m | t007, t008 |
| t010 | [Closeout](t010.md) | 10m | t009 |

## Definition of done

- On production, a workspace at T0 whose platform-model spend reaches $10 is charged once (one threshold invoice, finalized at once and paid) and its sessions keep running; it reads T1 within a minute of the payment, and its next threshold is $25.
- A charge that fails, or that needs 3-D Secure, stops platform-model spend with `BILLING_BALANCE_DUE` and a hosted payment link; nothing else refuses at a threshold. The workspace's own monthly cap still refuses at the cap.
- Every subscribed workspace's agent-model item carries its tier's `billing_thresholds`, and the cycle invoice bills only usage not yet charged.
- `billing_trust` carries every workspace's paid, non-zero invoices, including those before migration 0194.
- Settings → Billing and the agent environment on REST, GraphQL and MCP state the next charge point and no longer report a T0 ceiling.

## Source + Goal linkage

- **Source:** [ADR091 D10.2a](../../../docs/ADR091-agent-model-pricing.md) (amended 2026-11-13, user decision: "到达 $10 的时候应该是扣钱以继续，而不是阻止 … 收到钱 / 升级 / 收到更多钱 / 然后逐渐升级"). Found while timing the routines in `w6/blocked/m354`: the bex workspace (Scale plan, card bound since 2026-08-17) reached $10.01 of platform-model spend on 2026-11-13, no threshold invoice came, `billing_trust` held no row for any workspace, and every routine failed with `AGENT_MODEL_SPEND_CAP_REACHED` until the month's reset.
- **Goal linkage:** ADR008 pillar 5 (hosted agent sessions) and ADR091's non-payment protection; agent sessions on the platform model become the default path once w10/m14 t007 widens it.
- **Expected outcome:** a new workspace is never stopped while its card works; the bex routines run again as soon as t001 or t003 is live.
- **Why now:** w10/m14 t007 widens the platform model to every workspace (due 2026-11-13). Widening first would stop every new workspace at its first $10 for an hour or more, or for the rest of the month where no threshold is set.
- **Recommendations, not user decisions:** the in-flight allowance's initial value ($5 at T0, t002) and the retryable refusal past it (t004) are this milestone's recommendation; the rule they rest on (charge to continue, stop only on failure) is the user's.
- **The widening itself stays in w10/m14 t007** (dated 2026-11-13, gated on the OpenRouter credit limit, Radar rules, terms copy and a Stripe test key), so this milestone files no second widen task. t006 amends t007's pre-widening threshold check to this mechanism.
- **Render parity:** included (t007). Customer-facing billing and agent-environment fields change; the reference for agent sessions is Devin, and render.com has no agent-model billing.
