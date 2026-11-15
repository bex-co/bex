# w7 · m391 — Muse on bex's Meta API key, contributor tier only

**Worker:** worker7 **Goal:** a workspace whose admin enabled Muse (m392) runs `muse` sessions with no key of its own: bex-api routes them to Meta's official API (`api.meta.ai`) with a bex-owned key, pinned to `muse-spark-1.3-contributor`, metered and billed at cost × 1.3 under the existing spend caps, within Meta's shared rate limit. **Status:** todo — t001–t004 and t006–t008 done; t005 (live check) and t009 remain. The Meta key is installed on production (2026-11-14, see the unblock record).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Amend ADR091 for Muse on Meta's API (price row, contributor data use) and update the agent-model terms page — **DONE** | 45m | w7/m390/t007 |
| t002 | Platform route for `muse`: `api.meta.ai` with a bex-owned key, model pinned to `muse-spark-1.3-contributor`, never a workspace key — **DONE** | 75m | t001 |
| t003 | Model proxy: pin the model in Responses bodies, read usage, compute cost into `agent_model_cost_microusd`, apply caps — **DONE** | 75m | t002 |
| t004 | Shared rate limit: platform cap on concurrent `muse` sessions and a readable failure for Meta 429s — **DONE** | 60m | t002 |
| t005 | Production check as the QA user | 45m | t003, t004, w7/m392/t004 |
| t006 | Render parity (Devin reference) — **DONE** | 30m | t005 |
| t007 | Simplify — **DONE** | 30m | t006 |
| t008 | Test coverage — **DONE** | 45m | t006 |
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

## Progress (2026-11-14)

- **t001:** [ADR091](../../../docs/ADR091-agent-model-pricing.md) D8a (cost from a `pricing.yaml` `agentModel.upstreamPrices` row × markup; no account-balance backstop, so per-workspace caps plus a platform-wide session cap; the contributor tier's training use disclosed) and [ADR108](../../../docs/ADR108-muse-code-agent.md) D5 route details. `/legal/agent-models` (en + zh) gains the Muse exception, effective 2026-11-14.
- **t002:** `BEX_AGENT_PLATFORM_META_KEY` (Secret `bex-agent-platform-model`/`meta-api-key`, optional; `scripts/agent-platform-model-secret.sh` installs it from `.env` `META_MODEL_API_KEY`) arms the Meta route in the `BEX_AGENT_PLATFORM_MODEL_WORKSPACES` workspaces. The mint hands the gateway that key for `api.meta.ai`, Bearer, pinned to `muse-spark-1.3-contributor`, after the same balance, charge-threshold, cap and session-ceiling gates as the OpenRouter route, and never reads a workspace key for `muse`. Create refuses another model with `AGENT_SESSION_MODEL_PINNED`. Capabilities offer `muse` (ready) only where the route serves it.
- **t003:** the gateway meters `/v1/responses` on `api.meta.ai` (Responses usage from `response.completed` or a JSON body; cached tokens recorded as cache reads), sends no OpenRouter provider object there, and bex-api prices it at the contributor list price × 1.3. Muse sessions are metered, capped and trust-counted even in a workspace with its own key; `maxSpendUSD` is accepted.
- **t004:** `BEX_AGENT_META_ROUTE_MAX_SESSIONS` (default 8) caps live muse sessions across all workspaces; create, steer and resume past it answer 409 `AGENT_SESSION_AGENT_AT_CAPACITY` with `live`/`limit`. An upstream 429 from Meta is relayed unchanged to Muse; how Muse surfaces it is checked live in t005.
- **t006 (Render parity, Devin reference):** capabilities, create and steer refusals, and cost read-outs come from the same `Service` paths on REST, GraphQL and MCP; the terms page is the only UI change. No ADR018 row changes.
- **t007:** `/simplify` applied: the Meta route reuses `modelRoute`/`covers` and a shared `workspaceSet`; `PlatformRouteServes` is `PlatformRouteMeters(…, false)`; `sessionMetered` removed; the platform-only mint branch runs before the OpenRouter one; the capacity error carries `live`; the count query keeps two statement texts so the per-workspace plan keeps its index; Responses lines are buffered up to the JSON bound and `"usage":null` lines are not decoded.
- **t008:** pricing (row, bad rows), recorder (Muse priced from the row), meter (Responses stream/JSON/clamp), gateway end to end (pin 403, Meta host + Bearer key, no provider object, $0.39 for 1M+1M), mint (key, pin, BYO ignored, allowlist, spend gates), service (capabilities ready, pinned model, placeholder only, cross-workspace cap) and real-DB store (cross-workspace count) tests.

## Blocked (2026-11-14)

- **Gate 1 — user action:** create an API key on bex's Meta Model API account (a team admin manages the payment method), add it to `.env` as `META_MODEL_API_KEY`, and run `KUBECONFIG=… scripts/agent-platform-model-secret.sh` (or ask a worker to run it). Until then `muse` is offered nowhere.
- **Gate 2 — w7/m392:** install the key only after m392 has shipped, so no workspace gets Muse before its admin acknowledges the contributor data use. **Cleared 2026-11-14:** m392 is live on production (deploy run 43566568833) and verified there ([record](../done/m392/README.md)).
- **Release condition:** both gates clear ⇒ `/unblock w7` and run t005 (live check as the QA user; also re-measure requests per turn for the capacity default).

## 解阻记录（2026-11-14，`/unblock w7`）

- **涉及：** w7/m391（t005、t009）。核查基于 main `6ec9a62a5`。
- **D1（用户决定）：** 用户原话："现在是为了测试，都是我们自己在用，所以不用担心"。范围：bex 的 Meta key 目前只服务 bex 自己的 workspace（`BEX_AGENT_PLATFORM_MODEL_WORKSPACES` = `tea-d98210cbbpdc73dcrkvg`），用于内部测试。不为此拆分 Meta 路由的白名单，也不改为租户自带 key。对标：这是 bex 与 Meta 的合同问题，Devin 的做法决定不了。
- **仍开放的风险（复核证据，不是用户决定）：** 第三方条款追踪站 ConductAtlas 记录的 Meta Model API 条款（Meta 原文未核实）有两点。其一，未经 Meta 书面许可，不得把 API key 给第三方或经网关/代理转售访问权。其二，2026-10-02 新增 Exhibit C "Hosted Platform Access"，只适用于 Meta 授权的托管平台。ADR108 的开放项"对外开放前需核实 Meta 的托管使用条款"不变。Meta 路由与 OpenRouter 路由共用白名单，w10/m14 t007 放开白名单时 Muse 会对所有租户开放，届时需重新确认。
- **Gate 1，用户动作，已完成：** 用户已把 `META_MODEL_API_KEY` 写入本机 `.env`，并要求放到生产。
- **安装（worker 执行，复核证据）：**
  1. 本机 SSH host-key pin 过期，按 ADR019 的恢复流程处理。手动运行 `Verify production SSH trust`（run 43588025093），CI 的可信 pin 是一把通配的稳定 key `SHA256:W/o3ED3/xJrcoF0ONPn3Cb1S98X32ubnDcTeYtSVRyU`。4 个运行中的控制面节点逐一 keyscan，全部一致，用这把 key 生成本次运行的临时 pin，没有首次信任。本地 `.env` 和 pin 文件均未修改。
  2. 安装前核对：生产 Secret 原只有 `openrouter-api-key`，且与 `.env` 的 `OPENROUTER_API_KEY` 哈希一致。先 dry run，再运行 `scripts/agent-platform-model-secret.sh`。
  3. Secret 现在含 `meta-api-key`、`openrouter-api-key`。`bex-api` 滚动重启后，两个 pod 都在 11:03 UTC 打出 "PLATFORM Meta route active for the muse agent profile in 1 allowlisted workspace(s)"。临时 kubeconfig 已删除。
- **Gate 2：** 已于 2026-11-14 解除（m392 已上线并验证）。
- **key 可用（用户确认，2026-11-14）：** 用户原话："key 能用，你继续"。t005 的第一次真实调用仍要在生产上记录这把 key 调用 `muse-spark-1.3-contributor` 的结果。
- **worker 下一步（t005）：** 以 QA 用户登录（`bash scripts/qa-login.sh`），在 `bex` workspace 操作：
  1. 以管理员身份在 Settings → Agents 打开 Muse，并确认数据使用条款。本次 session 内容会被 Meta 用于训练，所以只用一次性测试仓库。
  2. 起一个 `muse` session，确认它开出 draft PR。
  3. 用 `bex agent cost` 和 usage 记录核对 token 数和金额，应为 contributor 价 × 1.3。
  4. 请求其他模型应被拒（`AGENT_SESSION_MODEL_PINNED`）。
  5. 记录每轮请求数，必要时调整 `BEX_AGENT_META_ROUTE_MAX_SESSIONS` 的默认值。
  6. 结束后关闭 Muse，退出登录。
  
  若 key 调用 contributor 模型被拒（例如需要报名），把 m391 放回 blocked，并写明需要用户在 Meta 后台完成的动作。之后做 t009 收尾。
