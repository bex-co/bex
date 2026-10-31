# w10 · m14 — Platform model key: OpenRouter for an allowlisted workspace now, metered rollout to every workspace

**Worker:** worker10 **Goal:** Agent sessions can run on a bex-owned OpenRouter key without the tenant supplying a model key. The first step serves the allowlisted workspace `tea-d98210cbbpdc73dcrkvg`, which unblocks `https://dashboard.bex.co/agents/ags-dbdh96fsf6rc738q771g`, failed today with "this workspace's model API key is missing". It then widens to every workspace once token usage is metered, priced, billed and capped. **Status:** todo (t001–t003, t006 done; pricing decided in [ADR091](../../../docs/ADR091-agent-model-pricing.md) 2026-10-30; t007 widen due 2026-11-06)

## Tasks (in order)

| id   | title                                                                                  | est  | depends_on |
| ---- | -------------------------------------------------------------------------------------- | ---- | ---------- |
| t001 | Workspace-allowlisted platform OpenRouter route (platform key, model pin, never in sandbox) — **DONE** | 2h   | —          |
| t002 | Production rollout: Secret, deploy, verify the allowlisted session runs — **DONE**                | 45m  | t001       |
| t003 | Meter model tokens per workspace in the gateway model proxy → `usage_hourly` — **DONE**           | 2h   | t002       |
| t004 | Cost-based price and bill: provider cost × 1.3 → `agent_model_cost_microusd` → one Stripe meter (ADR091 D1–D3) | 2h | t003 |
| t005 | Monthly USD spend caps + trust ramp; OpenRouter credit limit 80% alert (ADR091 D5, D10.2) | 1.5h | t004 |
| t006 | Data policy for platform-key traffic (OpenRouter provider routing, terms copy) — **DONE**         | 45m  | t002       |
| t012 | BYO model key write path + model source precedence (ADR091 D7, absorbs w10/009) | 1.5h | t004 |
| t013 | Customer UI: usage line, Settings → Agents spend, composer cap banners, per-turn cost (ADR091 D6) | 2h | t005, t012 |
| t014 | Threshold billing: charge at $10 Hobby / $50 Pro+Scale (ADR091 D10.1) | 1.5h | t004 |
| t015 | Settle before delete + card fingerprint dedupe (ADR091 D10.3–D10.4) | 1.5h | t014 |
| t007 | Widen the platform route to every workspace — **due 2026-11-06** (gates: live Stripe meter, OpenRouter credit limit, terms copy) | 30m | t004–t006, t012–t015 |
| t008 | Render parity (capabilities/model-key readiness across REST/GraphQL/MCP/dashboard)    | 20m  | t007       |
| t009 | Simplify                                                                               | 20m  | t008       |
| t010 | Test coverage                                                                          | 30m  | t008       |
| t011 | Closeout                                                                               | 10m  | t009, t010 |

## Definition of done

- A claude agent session in any workspace without a BYO model key runs on the platform OpenRouter route. The key lives only in the `bex-system` Secret → bex-api env → gateway mint hop and never in a sandbox, API response, log or audit event (tests).
- Every platform-key request is priced at provider cost × 1.3 (ADR091), recorded with its cost source, metered as `agent_model_cost_microusd` in `usage_hourly`, billed through one Stripe meter, and capped by a monthly USD spend cap (with the $10 trust ramp) that returns `AGENT_MODEL_SPEND_CAP_REACHED` when reached.
- Admins can set a write-only BYO model key; BYO workspaces are not billed for tokens.
- Customers see agent model spend on Usage/Billing, Settings → Agents, the composer (80%/100% banners) and session detail.
- Unbilled agent-model spend is charged at the plan threshold, and deletion settles the balance first or refuses with `BILLING_BALANCE_DUE`.
- The production `BEX_AGENT_PLATFORM_MODEL_WORKSPACES` allowlist is removed (or replaced by "all") on or before 2026-11-06.

## Source + Goal linkage

- **Source:** user request 2026-10-23: "这个阶段上线 ags-dbdh96fsf6rc738q771g 最重要，如果有可能的安全问题，不让其他 tenant 用即可". The follow-up discussion (same day) established three things:
  - the ADR062 placeholder + gateway injection already keeps the key out of tenant reach;
  - the real gaps are unmetered tokens (billing today meters only `sandbox_compute_seconds`), pricing, quotas and data policy;
  - the dev-only route from `w10/blocked/m13` t016 is refused in production.
- **Goal linkage:** ADR008 AI-native pillar; ADR047/ADR090 agent sessions; ADR040 billing.
- **Expected outcome:** tenants can start agent sessions without bringing a model key, and the platform recovers the token cost through billing.
- **Why now:** the production session above is unusable without a model key, and the AGENTS.md rule requires a dated widen-to-every-workspace task for any allowlist.
- **Render parity included (t008):** model-key readiness is reported on REST/GraphQL/MCP and the dashboard composer.
- **Pricing decision (2026-10-30):** [ADR091](../../../docs/ADR091-agent-model-pricing.md) — provider cost × 1.3, every plan pays per use, USD spend caps, non-payment protection. Moved out of `blocked/` the same day; t012 absorbs `w10/009`.
