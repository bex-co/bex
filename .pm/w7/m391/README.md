# w7 · m391 — Muse on bex's Meta API key, contributor tier only

**Worker:** worker7 **Goal:** a workspace whose admin enabled Muse (m392) runs `muse` sessions with no key of its own: bex-api routes them to Meta's official API (`api.meta.ai`) with a bex-owned key, pinned to `muse-spark-1.3-contributor`, metered and billed at cost × 1.3 under the existing spend caps, within Meta's shared rate limit. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Amend ADR091 for Muse on Meta's API (price row, contributor data use) and update the agent-model terms page | 45m | w7/m390/t007 |
| t002 | Platform route for `muse`: `api.meta.ai` with a bex-owned key, model pinned to `muse-spark-1.3-contributor`, never a workspace key | 75m | t001 |
| t003 | Model proxy: pin the model in Responses bodies, read usage, compute cost into `agent_model_cost_microusd`, apply caps | 75m | t002 |
| t004 | Shared rate limit: platform cap on concurrent `muse` sessions and a readable failure for Meta 429s | 60m | t002 |
| t005 | Production check as the QA user | 45m | t003, t004, w7/m392/t004 |
| t006 | Render parity (Devin reference) | 30m | t005 |
| t007 | Simplify | 30m | t006 |
| t008 | Test coverage | 45m | t006 |
| t009 | Closeout | 10m | t007, t008 |

## Definition of done

- In a workspace where an admin enabled Muse (m392) and no workspace key is involved, a `muse` session started as the QA user on production completes a turn on `muse-spark-1.3-contributor` through `api.meta.ai` and opens a draft PR.
- A request for any other model is refused (`AGENT_SESSION_MODEL_PINNED` at create; 403 at the gateway). A workspace's own model key never applies to `muse`.
- Each request records input, cached-input and output tokens and a customer cost of $0.13 / $0.0026 / $0.26 per 1M tokens (Meta's contributor price × 1.3) into `agent_model_cost_microusd`; `bex agent cost` shows it; the monthly and per-session spend caps refuse with the existing typed codes.
- Starting a `muse` session beyond the platform concurrency cap is refused with a typed error; a 429 from Meta ends the turn with a readable failure reason.
- ADR091 is amended and `/legal/agent-models` (en + zh) discloses that Muse sessions use Meta's contributor tier, on which Meta may use prompts, code, files, context and responses to train its models.
- Backend real-DB suite, `make lint`, dashboard `yarn lint` + `yarn test` pass, and the change is live on production.

## Source + Goal linkage

- **Source:** `/pm-brainstorm add meta muse coding agent with muse-code-acp to our cloud agents`, 2026-11-13, item 2 as revised by the user's decisions the same day: **use Meta's official API instead of OpenRouter** and **use Muse Spark 1.3 on the contributor tier only**. Filed by `/pmx them all to w7`. Evidence: Meta's pricing page (https://dev.meta.ai/docs/pricing-rate-limits) — `muse-spark-1.3-contributor`, $0.10 input / $0.002 cached / $0.20 output per 1M tokens, 150 RPM / 3,000,000 TPM, content used to train future Meta models; Meta's API takes a Bearer key at `https://api.meta.ai/v1` (Responses API). Today the platform route serves only the `claude` profile through OpenRouter (`lego/backend/internal/agentsession/devroute.go` `routeFor`, `PinnedAgent`, `PlatformRouteServes`; `model.go` mint branches on `OpenRouterHost`), and a workspace key takes every profile off it.
- **Goal linkage:** ADR008 pillar 5; [ADR091](../../../docs/ADR091-agent-model-pricing.md) D1–D5 (cost × 1.3, metering, spend caps) and D8 (data and terms); [ADR062](../../../docs/ADR062-sandbox-credential-vault.md) (the key is injected only at the gateway).
- **Expected outcome:** Muse is usable without any Meta account of the tenant's, at a fraction of the Claude route's price, billed and capped like the existing platform model.
- **Why now:** m390 leaves `muse` with no route, and the user chose the contributor tier on Meta's own API as its only route.
- **Render parity included** (Devin reference): the route changes create refusals, capabilities readiness, cost read-outs and the terms page on REST, GraphQL, MCP and the dashboard.
- **User action needed (gate for t002's production secret and t005):** create an API key on bex's Meta Model API account (a team admin manages the payment method) and add it to `.env` as `META_MODEL_API_KEY` (mirrored in `.env.example`, pushed with the repo's secret scripts). Meta's pricing page shows the contributor tier as a separate model id with no enrollment step; this is not yet confirmed.
- **Recommendations recorded, not user sign-offs:** (1) handle Meta's shared 150 RPM per key with a platform concurrency cap sized from m390/t001's measurement, rather than asking Meta for a higher limit first; (2) the cost comes from a fixed price row because Meta returns no per-request cost — an amendment to ADR091 D1/D2's "no per-model price rows"; (3) Meta bills postpaid with no account balance to act as ADR091 D5's global backstop, so the ADR names the replacement (e.g. an alert on platform Muse spend).
- **Allowlist check (DO_NOT_DO):** the route covers the workspaces `BEX_AGENT_PLATFORM_MODEL_WORKSPACES` covers, whose dated widen to every workspace is `w10/m14` t007; access per workspace is then the admin's choice (m392), not an internal allowlist.
