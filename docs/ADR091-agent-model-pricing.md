# ADR091 — Agent model pricing: provider cost × 1.3, spend caps, and non-payment protection

**Status:** Accepted, 2026-10-30. Implementation is tracked in `w10/m14`. This ADR defines the pricing and spend controls that [ADR090](ADR090-agent-owned-delivery.md)'s 2026-10-23 platform model route left as follow-ups. It supersedes the per-model token-meter billing mechanism shipped in `952b9e8fa`; the token meters stay for display. It amends [ADR030](ADR030-pricing.md): the "Render price × 0.70" rule does not cover agent model tokens, because Render has no equivalent SKU.

**Source:** user decisions 2026-10-30 (`/plan`):

- the price is about **1.3× the OpenRouter model's raw price**, taken from the **actual per-request cost**;
- **Hobby also pays per use**;
- the user asked how the industry handles customers who run up usage and delete their account before the invoice.

## Context

- **Where it runs today.** Agent sessions run on a bex-owned OpenRouter key through the platform model route, currently only in allowlisted workspaces (ADR090). Widening it to every workspace is `w10/m14` t007, due 2026-11-06.
- **What is metered.** bex already meters `agent_model_{input,output,cache_read,cache_write}_tokens` per request: the gateway tee-parses the provider usage, and bex-api stores idempotent `agent_model_usage_events` (ADR023). Nothing is priced or capped yet.
- **No BYO path.** Customers cannot set their own model key: there is no write path for `ModelKeySecretPath` (`w10/009`). So the platform model is the only way a customer can use agent sessions at all.
- **Time-of-day prices.** OpenRouter prices `deepseek/deepseek-v4.1-flash-1017` by time of day: $0.15/M prompt, $0.60/M completion and $0.003/M cache read. Prices double on weekdays 01:00–04:00 and 06:00–10:00 UTC. A fixed per-token price either loses money at peak or overcharges off-peak.
- **Card on file.** Every workspace, Hobby included, already needs a bound card before any usage: [ADR075](ADR075-user-onboarding.md) D7, `BEX_REQUIRE_PAYMENT_METHOD=all`, live.
- **Billing in arrears.** Metered usage is invoiced monthly in arrears through Stripe Billing ([ADR040](ADR040-billing-metronome.md)). Usage reaches Stripe only after a 48h seal horizon. [ADR086](ADR086-account-deletion.md) account deletion keeps Stripe records, but nothing settles unbilled usage before the account disappears.

## Decision

### D1 — Price formula

The customer price of each platform-route request is **upstream cost (USD) × `markup`**, with `markup = 1.30`. It is a single value in `pricing.yaml` (`agentModel.markup`) that applies to every platform-route model. There are no per-model price rows.

Gross margin is 30% on every request. That holds automatically across time-of-day prices, model changes and OpenRouter price changes, with no catalog to maintain.

### D2 — Cost source

1. **Reported cost (primary).** OpenRouter's own per-request cost. The gateway model proxy already tee-parses usage (`sshgateway/modelproxy/usage.go`); it also reads `usage.cost`, and asks for usage accounting in the request if the Anthropic-format `/api/v1/messages` endpoint needs it. The first implementation task confirms this with one live probe.
2. **Computed cost (fallback).** When no cost is reported, bex-api computes it from the OpenRouter `/api/v1/models` price table. It applies the `overrides` (UTC day/hour windows) in effect when the request started. The table is refreshed hourly, and the last good copy is kept.
3. **Recording.** Each usage event records the raw cost, its source (`reported` | `computed`) and the resulting customer price, so an invoice line can be traced back to a request.

### D3 — Metering unit

- **New meter.** One new meter, `agent_model_cost_microusd`, carries the customer price per request in micro-USD, rounded half-up per request and summed in `usage_hourly`.
- **Stripe.** It maps to **one model-agnostic Stripe meter** priced at $0.000001 per unit through the ADR040 outbox. `scripts/stripe-billing-setup.py` creates that one meter and price.
- **Superseded.** The per-model token Stripe meters from `952b9e8fa` are no longer used for billing. The token meters remain the usage breakdown by model and token kind.

### D4 — Who pays

- **Every plan.** Hobby, Pro and Scale all pay per use, with no included allowance (user decision).
- **Admission.** The ADR075 D7 card requirement already gates every workspace. Agent-session admission keeps its synchronous 402 `PAYMENT_REQUIRED`.
- **BYO workspaces.** A workspace using its own model key (D7) is never billed for tokens; it pays only for sandbox compute.

### D5 — Spend caps

These replace the token quota mechanism, which shipped disabled in `952b9e8fa`.

- **The cap.** Each workspace has a monthly spend cap in USD. Plan defaults live in `pricing.yaml` (`agentModel.monthlySpendCapUsd[]`): **Hobby $20, Pro $200, Scale $2,000** as initial values.
- **Admin control.** Workspace admins can lower or raise the cap in Settings → Agents, up to 10× the plan default.
- **Enforcement.** The model mint compares month-to-date `agent_model_cost_microusd` with the effective cap (D10.2 can lower it). Over the cap, it refuses with the typed code `AGENT_MODEL_SPEND_CAP_REACHED`. This reuses the shipped refusal path: HTTP 429 at the mint, an Anthropic-shaped 429 to the agent, and a readable session failure with `failureReasonCode`.
- **Overshoot bound.** A request is priced after it completes. So the cap can be exceeded by at most the requests already in flight, one per session.
- **Global backstop.** The OpenRouter key carries a credit limit, set in the OpenRouter console as an ops step. An ops alert fires when platform spend passes 80% of it.

### D6 — What the customer sees

- **Usage and Billing.**
  - An "Agent model" line with month-to-date USD.
  - An expandable breakdown by model and token kind (input, output, cache).
  - REST, GraphQL and MCP usage reads expose the same data.
- **Settings → Agents.**
  - The model source: bex-provided, or your own key (D7).
  - The statement "billed at the model provider's cost + 30%".
  - Month-to-date spend against the cap, editable by admins.
- **Composer.**
  - At 80% of the cap, a warning banner.
  - At 100%, the session is blocked with the typed error, and admins see an "increase cap" action.
- **Session detail.** The cost of each turn, from the usage events.

### D7 — Model source and BYO key

This absorbs `w10/009`.

- **Precedence.** A workspace with a BYO key uses it, and bex bills only sandbox compute. Otherwise it uses the platform model, billed per D1.
- **Write path.** A write-only BYO key mutation on REST, GraphQL and MCP, admin-only, stores the key at `ModelKeySecretPath`. The key is never readable back; reads only report whether one is set. Settings → Agents gets the field.
- **Composer copy.** The copy that sends users to a non-existent settings field is fixed.

### D8 — Data and terms

- **Routing policy.** Platform-route requests keep OpenRouter provider routing `data_collection: "deny"` (shipped in `952b9e8fa`; see ADR090), and a client cannot override it.
- **Disclosure.** Prompts and code are sent to third-party model providers through OpenRouter, under a routing policy that excludes providers that store or train on them. The final legal wording on the terms page is a user decision.

### D9 — Rollout

1. Implement D1–D7 and D10.
2. Create the Stripe meter on test Stripe, then live Stripe.
3. Set the OpenRouter credit limit.
4. Widen `BEX_AGENT_PLATFORM_MODEL_WORKSPACES` to every workspace (`w10/m14` t007), by 2026-11-06.

### D10 — Non-payment and delete-before-invoice protection

**The risk.** Usage is invoiced monthly in arrears and reaches Stripe only after the 48h seal. Account deletion does not settle unbilled usage. So a workspace can burn model spend and delete itself before the period invoice.

**How the industry handles it:**

- **Prepaid credits.** The OpenAI, Anthropic and OpenRouter APIs take money up front, and usage draws down a balance that stops at zero. This is the strongest option, and the most friction.
- **Threshold billing.** Vercel, Twilio and Stripe `billing_thresholds.amount_gte` charge the card as soon as unbilled usage crosses an amount. Exposure is bounded by the threshold, not the month.
- **Trust tiers.** OpenAI usage tiers and cloud quota ramps start new accounts with low limits, which rise after successful payments.
- **Settle on close.** AWS requires a paid account before it can be closed. Heroku and Vercel issue a final invoice at deletion.
- **Card hygiene.** Card verification (3DS, an authorization hold), Stripe Radar risk scoring, optionally refusing prepaid or virtual cards, and deduplicating repeat abusers by card fingerprint across accounts.
- **Dunning and suspension.** A failed charge leads to a grace period, then suspension, then collections. bex already has this (ADR040 §7).

**bex's decision** (the amounts are initial values in `pricing.yaml`):

1. **Threshold billing.** bex charges immediately when a workspace's unbilled `agent_model_cost_microusd` reaches **$10 (Hobby) or $50 (Pro, Scale)**, using an early invoice for the meter or Stripe billing thresholds.
2. **Trust ramp.** A new workspace's effective spend cap starts at **$10** until its first successful agent-model charge, then rises to the plan default (D5).
3. **Settle before delete.** Workspace and account deletion (ADR086) first flushes all unbilled usage for that workspace, bypassing the seal horizon, then invoices and charges it. If the charge fails, deletion is refused with typed code `BILLING_BALANCE_DUE` and a payment link, and the identity is kept until the balance is settled.
4. **Card fingerprint deduplication.** Binding a card whose Stripe fingerprint belongs to a workspace with unpaid, written-off agent-model debt is refused.
5. **Unchanged.** The card requirement for every workspace (ADR075 D7) and dunning with suspension (ADR040 §7).

**Exposure.** With these, the most an abusive account can cost bex is roughly one threshold, about $10 of customer price on Hobby, plus at most one in-flight request per session.

## Alternatives considered

| Option | Why not |
| --- | --- |
| Off-peak list price × 1.3, fixed | Loses money at peak, where revenue is 0.65× cost. |
| Peak list price × 1.3, fixed | Off-peak it is 2.6× cost, which is uncompetitive. |
| Per-model token meters with list prices (the `952b9e8fa` mechanism) | Cannot express time-of-day prices, and every model change needs new Stripe meters and a new price table. |
| A free Hobby allowance | The user chose pay-per-use for every plan. |
| Prepaid credits | The strongest protection against unpaid usage, but it adds the most friction and needs a new balance ledger. Revisit if threshold billing shows losses. |

## Consequences

- Invoices are cost-plus and cannot be predicted per token. They are made transparent instead: "provider cost + 30%", with a breakdown by model and token kind.
- Billing depends on OpenRouter cost reporting, with a computed fallback from its price table, and each event records which source was used.
- The rollout needs one new Stripe meter and price on test and live Stripe, plus an OpenRouter credit limit.
- Account deletion gains a payment step that can refuse deletion while a balance is due. ADR086 is amended accordingly in implementation.
- There is no render.com equivalent, so this is a bex-native surface. [ADR018](ADR018-render-parity.md) records it as such, not as parity drift.
