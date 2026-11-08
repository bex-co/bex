# ADR094 — Cloud Agents MCP: the bex cloud coding agent as a task-shaped toolset at `/mcp/cloud-agents`

**Status:** Proposed, 2026-10-30. Scheduled 2026-11-03 as `w21/m257`–`m260` (one milestone per D10 phase). **Phase 1 shipped (w21/m257, 2026-11-03)**, **phase 2 shipped (w21/m258, 2026-11-03)** and **phase 3 shipped (w21/m259, 2026-11-05)**: `state`/`steerable`/`outcome`, the `create_`/`send_` names, the `/mcp/cloud-agents` mount, `bex.cloud_agents`, idempotent create, `title`/`tags`/optional `branch`, `wait_agent_sessions`, and schema-validated structured output (`outputSchema`, `submit_result`, `outcome.result`). **Phase 4 shipped (w21/m260, 2026-11-05)**: the completion webhooks, the per-session `maxSpendUSD` ceiling and `outcome.cost.modelUSD`; sandbox compute joined it as `computeUSD`/`totalUSD` in w9/m178 (2026-11-06, see D7 as built), with no raw `sandboxSeconds`. It amends [ADR025](ADR025-connect-an-agent.md) (a second, narrower MCP endpoint and one new scope) and builds on [ADR047](ADR047-cloud-coding-agent-sessions.md), [ADR051](ADR051-agent-session-transcript.md), [ADR065](ADR065-agent-session-archive.md), [ADR090](ADR090-agent-owned-delivery.md) and [ADR091](ADR091-agent-model-pricing.md). It does not change the session engine.

**Source:** user request 2026-10-30: deep-research Devin's MCP and our coding agent's APIs, then design a coding-agent MCP as part of our overall MCP offering. User decision 2026-11-02: the product line is **Cloud Agents**, at `/mcp/cloud-agents` (D1 § Naming).

**Related decisions:** [ADR006](ADR006-bex-api.md) (one core, thin REST/GraphQL/MCP adapters), [ADR012 §7](ADR012-auth.md) (OAuth 2.1 provider), [ADR020](ADR020-identifiers.md) (`ags-` ids), [ADR023](ADR023-usage-metering.md) (meters).

## Context

### What we already ship

The agent product is complete behind REST and GraphQL, and **MCP already has parity**: `lego/backend/internal/agentsessions/mcp.go` registers 15 tools (`spawn_agent_session`, `list_/get_agent_session`, `get_agent_session_capabilities`, `get_/update_agent_environment`, `get_agent_session_transcript`, `get_agent_session_tool_output`, `archive_/unarchive_/delete_agent_session`, `resume_agent_session`, `attach_agent_session`, `cancel_agent_session`, `steer_agent_session`). They sit on the one bex MCP server (`internal/api/server.go`, `modelcontextprotocol/go-sdk` v1.8.0, stateless streamable HTTP at `/mcp`, stdio via `api mcp-stdio`), next to 176 other tools.

Parity is the problem. The tools mirror CRUD verbs designed for the dashboard, which holds an SSE stream open. An MCP client is an LLM in a tool loop, and it hits five gaps:

1. **No way to wait.** `spawn_agent_session` returns `phase: creating` and nothing else. The only progress signal is polling `get_agent_session`, and every poll costs the caller model tokens. The attach ticket unlocks the gateway's AI-SDK SSE stream, which an MCP client cannot consume.
2. **No compact result.** To learn what the agent did, a caller pages `get_agent_session_transcript`. Those are raw UI-message parts (reasoning, tool calls, diffs), so finding the final answer costs thousands of tokens.
3. **No machine-readable output.** A calling agent that wants `{tests_passed, files_changed, risk}` has to parse prose.
4. **No idempotency.** A client that times out and retries spawns a second session and a second sandbox, up to the five-per-workspace live limit.
5. **Drift.** `spawn_agent_session` says it "returns its sandbox attach ticket" and `resume_agent_session` says it mints one. Neither does; only `attach_agent_session` mints (`service.go:1422`). The tool vocabulary is also uneven: `spawn_` here, `create_` everywhere else.

The rest of the bex MCP offering frames all of this. One server exposes 191 tools, which is already more than several clients load comfortably. A caller that only wants a coding agent, such as an orchestrator, a CI bot or another vendor's agent, has to load the whole catalog and request `bex.write`. That scope can also delete services.

### What Devin does (research 2026-10-30)

Devin's API and MCP were checked against primary sources: the v1 and v3 OpenAPI specs downloaded from `docs.devin.ai`, and live unauthenticated probes of `mcp.devin.ai` and `mcp.deepwiki.com`.

- **One server, two modes.** `https://mcp.devin.ai/mcp` and `https://mcp.deepwiki.com/mcp` run the same FastMCP server (`serverInfo: DeepWiki 2.14.3`). Without auth it serves only the public-repo wiki tools (`read_wiki_structure`, `read_wiki_contents`, `ask_wiki_question`). With a `cog_` service-user key, a personal token or an OAuth token it adds the `devin_*` tools. Transport is streamable HTTP only; `/sse` returns 410. ([Devin MCP](https://docs.devin.ai/work-with-devin/devin-mcp.md), [DeepWiki MCP](https://docs.devin.ai/work-with-devin/deepwiki-mcp.md))
- **Standards OAuth, undocumented.** A 401 carries `resource_metadata` (RFC 9728) pointing at `api.devin.ai` (PKCE S256, public clients, client-ID metadata documents, scope `devin`). Enterprise keys and PATs pick the org with an `X-Org-Id` header. The docs describe only API keys.
- **Few, multiplexed session tools.** `devin_session_create` (batch-capable: prompt, title, playbook, tags, ACU limit), `devin_session_search`, `devin_session_interact` (operation enum: `get`, `get_messages`, `get_attachments`, `message`, `sleep`, `terminate`, `archive`, `unarchive`, `set_tags`), `devin_session_events` (`list`, `details`, `search`), and **`devin_session_gather`**. Gather blocks until a set of sessions settles (finished, errored, sleeping or waiting) "instead of polling in a loop". Platform admin sits in `devin_*_manage` tools (playbooks, knowledge, automations, MCP installs, reviews). The exact `devin_*` input schemas are visible only after authentication, so they are **unverified**.
- **REST v3** (`/v3/organizations/{org_id}/sessions`). Create takes `prompt` (required), `repos`, `playbook_id`, `knowledge_ids`, `secret_ids`, inline `session_secrets`, `max_acu_limit`, `tags`, `devin_mode`, `resumable`, `structured_output_schema` (Draft 7, ≤64 KB, no external `$ref`) and `structured_output_required`. When the schema is required, the agent must call an internal `provide_structured_output` tool with `is_final=true` before its turn ends. The response carries `status`, `status_detail`, `acus_consumed`, `pull_requests[{pr_url, pr_state}]`, `structured_output` and `parent_session_id`/`child_session_ids`. Messages go to `POST …/messages`, which auto-resumes a sleeping session. Pagination is by cursor (`first`/`after` → `end_cursor`, `has_next_page`). ([v3 overview](https://docs.devin.ai/api-reference/v3/overview.md), [structured output](https://docs.devin.ai/api-reference/structured-output))
- **Two-level state.** `status` follows the lifecycle (`new → claimed → running → suspended ⇄ resuming → exit | error`). `status_detail` gives intent or the suspension reason: `working`, `waiting_for_user`, `waiting_for_approval`, `finished`, `inactivity`, `out_of_credits`, `usage_limit_exceeded`, and so on. A finished task usually reads `running/finished` and then sleeps, so "done" is never a terminal status.
- **Gaps.** v3 dropped v1's `idempotent` flag and its `is_new_session` signal; create documents an unexplained 409. There is no outbound completion webhook; the official sample polls every 10 s. Rate limits and the ACU price are unpublished. Tool names drift between the docs and the live server (`ask_question` vs `ask_wiki_question`).

The comparable offerings take the same task-API shape. Cursor Cloud Agents v1 uses a client-supplied `agentId`, follow-up `runs`, SSE `/runs/{id}/stream`, and the states `FINISHED|ERROR|CANCELLED|EXPIRED`. The GitHub MCP server's `assign_copilot_to_issue` / `create_pull_request_with_copilot` make the draft PR both the handle and the result. Jules v1alpha has `sessions:sendMessage`, `:approvePlan` and an `activities` feed. OpenAI's Agents API beta has durable sessions with streaming or webhooks.

**What to take:** a blocking wait tool, a two-level status that makes "needs a human" and "out of money" machine-readable, a schema-validated structured result, a per-session spend ceiling, a narrow toolset, and standards OAuth. **What to avoid:** an ambiguous "done", missing idempotency, missing completion push, operation-multiplexed tools whose read and write calls share a single annotation, and docs that drift from the server.

## Decision

### D1 — One server, a coding-agent toolset, two mount points

The coding agent stays inside the one bex MCP server (ADR006: one core, thin adapters). There is no second server binary and no second auth stack. Each tool now belongs to a named **toolset**. `cloud-agents` is the first toolset with a mount of its own:

| Endpoint | Serves | For |
| --- | --- | --- |
| `/mcp` | every toolset (unchanged) | an agent that operates the whole platform |
| `/mcp/cloud-agents` | the `cloud-agents` toolset (D2) plus `list_workspaces` and `list_repos` | an orchestrator, CI bot or third-party agent that only delegates coding work |
| `api mcp-stdio --toolset cloud-agents` | the same, over stdio | local dev |

Both mounts share the auth middleware, rate limiter, workspace binding (`workspaceId` per call, `api/mcp_workspace.go`), scope gate and core error mapping. `/mcp/cloud-agents` advertises the same RFC 9728 metadata with that resource URL. Toolsets are a registration tag (`mcputil.AddTool(..., mcputil.Toolset("cloud-agents"))`), not separate packages, so they cannot drift from `/mcp`. Later toolsets (`deploys`, `data`, …) can use the same mechanism, but none is decided here.

**Naming (user decision 2026-11-02).** The product line is **Cloud Agents**. Every customer-facing surface uses that one name:

| Surface                           | Name                |
| --------------------------------- | ------------------- |
| Product (docs, pricing, invoices) | Cloud Agents        |
| MCP mount                         | `/mcp/cloud-agents` |
| Toolset tag and stdio flag        | `cloud-agents`      |
| Server name in connect snippets   | `bex-cloud-agents`  |
| OAuth scope (D9)                  | `bex.cloud_agents`  |

The internal resource keeps its name, `agent_session` / `ags-`. The CLI keeps the short `bex agent …` noun, because in a terminal the local launchers already have their own names (`bex code`, `bex edit`). Its help text names Cloud Agents.

Why this name:

- **It is the established category name.** Cursor calls them Cloud Agents, GitHub calls them the Copilot cloud agent, and OpenAI has Codex cloud.
- **It separates bex's three kinds of agent:** local launchers, the desktop agent panel, and cloud sessions.
- **It tells the consent screen what is at stake:** an agent that runs on bex, spends money, and pushes a branch.
- **It is plural, like `/v1/agent-sessions` and the dashboard's `/agents`.** Customers run several cloud agents at once. A singular form only reads well behind a persona name, as in "Copilot cloud agent".

Rejected names:

- **`/mcp/agents` and `/mcp/agent`.** In MCP, every client is an agent, so "connect your agent to `/mcp/agent`" is self-referential, and neither name says which bex agent is meant.
- **`/mcp/bex-agents`.** The brand already sits in the host.
- **`/mcp/coding-agents`.** Too narrow: repo-less sessions exist, and FRFAQ002 points at non-coding work.
- **`/mcp/tasks`.** It collides with the MCP Tasks primitive (D6).
- **`/mcp/workers`.** It collides with Render's Background Worker service type.
- **`/mcp/sessions`.** That is internal vocabulary.

A persona name for the agent (the Devin and Jules pattern) and a separate `agents.bex.co` host stay open. Either would arrive as an additive alias; `/mcp/cloud-agents` is never removed.

A Devin-style **anonymous public mode** is rejected: every bex agent turn costs a sandbox and model tokens (Alternatives).

### D2 — The `cloud-agents` toolset: task-shaped, not CRUD-shaped

The toolset has nine tools. Each one has its own name, so `readOnlyHint`/`destructiveHint` annotations and the scope matrix stay per-tool. Devin's `operation` multiplexing is rejected because one annotation would have to cover both read and write calls (see Alternatives).

| Tool | Kind | What it does |
| --- | --- | --- |
| `get_agent_session_capabilities` | read | Preflight, unchanged: GitHub connected, model ready, agent profiles, `ready`. |
| `create_agent_session` | write | Starts a task (D3, D4, D5). It is the canonical name; `spawn_agent_session` stays on `/mcp` as an alias. |
| `wait_agent_sessions` | read | Blocks until sessions settle (D6). |
| `get_agent_session` | read | Session view plus the new `outcome` block (D7). |
| `list_agent_sessions` | read | Unchanged filters; adds `state` and `tag` filters (D7). |
| `send_agent_session_message` | write | A follow-up turn. It is the canonical name for `steer_agent_session` (alias kept) and also accepts a per-turn `outputSchema`. |
| `get_agent_session_transcript` | read | Unchanged; the escape hatch when `outcome.summary` is not enough. |
| `get_agent_session_tool_output` | read | Unchanged. |
| `cancel_agent_session` | destructive | Unchanged. |

**Left off `/mcp/cloud-agents`:** `attach_agent_session` (a ticket for an SSE stream an MCP client cannot read), `resume_agent_session` (a message already re-dispatches), archive, unarchive, delete, and the admin-only environment tools. All of them remain on `/mcp`.

**Description fixes ship first and alone.** `spawn_` and `resume_` stop claiming to return tickets. A new golden test pins every agent tool's name, description, input schema and annotations, so any later edit has to change the golden file and the docs together (Devin's drift is the lesson).

### D3 — `create_agent_session` input

The input is the current `CreateRequest` (`ownerId` becomes `workspaceId`; `repo`, `branch`, `agentConfig{agent, model, task, template}`, `egressAllowlist`) plus these fields:

| Field | Meaning |
| --- | --- |
| `idempotencyKey` | D4. |
| `outputSchema` | D5. JSON Schema; makes the turn's result machine-readable. |
| `outputRequired` | Default `true` when `outputSchema` is set. |
| `title` | Display title. Defaults to the first line of `task`. |
| `tags` | ≤ 20 tags, each `[a-z0-9:_-]{1,64}`, for filtering and cost attribution. Stored in a new `agent_sessions.tags text[]`. |
| `maxSpendUSD` | A per-session ceiling on the ADR091 customer price (D8). |
| `branch` | Becomes **optional**. When omitted, the server picks `bex-agent/<ags-id>`. An LLM should not have to invent a branch name that passes the `bex-agent/` prefix rule. |

Batch create (Devin's `devin_session_create` takes several prompts) is not added. MCP clients already issue parallel tool calls, and one session per call keeps idempotency and errors per session.

### D4 — Idempotent create

`idempotencyKey` (1–128 printable ASCII characters) is unique per workspace for 24 h, enforced by a partial unique index on `agent_sessions(tenant_id, idempotency_key)`.

- **Same key, same normalized request:** returns the existing session with `created: false`. No second sandbox is created.
- **Same key, different request:** `409 AGENT_SESSION_IDEMPOTENCY_KEY_REUSED`. Unlike Devin's bare 409, the code names the cause.

REST gets the same behavior from an `Idempotency-Key` header, and GraphQL from an `idempotencyKey` argument. `created` appears on every create response, on all three transports.

### D5 — Structured output through the in-sandbox `bex` MCP server

If a turn carries `outputSchema`, the driver adds a second tool to the in-sandbox `bex` stdio MCP server (`lego/agent-image/driver/src/mcp-bex.ts`, beside `open_or_update_draft_pr`):

`submit_result({ result, final })` validates `result` against the turn's schema in the driver and returns the validation errors to the agent, so it can fix the result and call again. It then posts the result through the gateway to a new internal verb, `/v1/agent-session-results`, which is authorized the same way as the ADR090 pull-request verb. The **last valid** submission of the turn wins. A non-final submission is a partial value and can be polled while the turn runs, which mirrors Devin's notepad model.

- **Schema limits:** JSON Schema 2020-12, ≤ 64 KiB, no external `$ref`, validated at create or message time (`400 AGENT_SESSION_OUTPUT_SCHEMA_INVALID`). Result ≤ 256 KiB.
- **Overlay:** the ADR090 D2 platform overlay gains one paragraph. When a schema is attached, the agent must call `submit_result` with `final: true` before ending the turn.
- **Missing result:** if `outputRequired` holds and the turn ends without a final valid result, the driver issues **one** bounded reminder prompt inside the same turn. If the result is still missing, the turn ends `failed` with `failureReasonCode: STRUCTURED_OUTPUT_MISSING`. Any PR the agent opened stays on the session either way.
- **Storage:** `agent_session_turns.output_schema jsonb` and `.result jsonb`. The result is exposed in `outcome.result` (D7) and on the transcript turn.

Repo-less (chat-only) sessions support structured output too, which makes "research this and answer in this shape" a first-class use.

**As built (w21/m259, 2026-11-05).**

- **Inputs.** `outputSchema` (a JSON object; GraphQL takes it as JSON text) and `outputRequired` ride `create_agent_session` and `send_agent_session_message` on REST, GraphQL and MCP. The schema is compiled as 2020-12 with a loader that refuses every reference leaving the document, so an external `$ref`, another `$schema` dialect, an oversize or uncompilable schema, or a NUL character is `400 AGENT_SESSION_OUTPUT_SCHEMA_INVALID`. `outputRequired` without a schema is `400 AGENT_SESSION_INPUT_INVALID`. The normalized contract is part of the idempotency digest. A follow-up turn never inherits the previous turn's contract.
- **Storage.** Migration 0200 adds `agent_session_turns.output_schema jsonb`, `.output_required`, `.result jsonb` and `.result_final`. `result_final` is what separates a partial value from the turn's answer.
- **Verb.** `submit_result` posts `{result, final}` to `POST /agent-results/<b64 ns>/<b64 session>` on the gateway's agent listener, the session-label-only Pod rule the share route uses, so repo-less sessions submit too. The gateway forwards a kind-tagged `submit-result` envelope with its own MAC domain to bex-api's `/v1/agent-session-results` (`BEX_AGENT_RESULT_API_URL`). bex-api re-reads the session, applies the current-sandbox rule, and validates the result against the schema stored on the turn before it keeps it; the driver's check only spares the agent a round trip. Refusals use the PR verb's vocabulary plus `not_expected` and `schema_mismatch`.
- **Last valid wins.** A later valid submission replaces an earlier one. A non-final submission is accepted but never replaces one already declared final.
- **Read side.** `outcome.result` is the current turn's final result, or absent; a partial value is readable on the transcript turn (`result`, `resultFinal`, `outputSchema`, `outputRequired`) while the turn runs. Like `summary` and `files`, `outcome.result` is computed on the single-session read and in `wait_agent_sessions`, not on list pages.
- **Missing result.** The driver reads the result record the MCP server writes, sends exactly one reminder inside the same turn before the delivery check, and reports `output` in its status file. The failure itself is decided by bex-api: the Completer fails a `succeeded` turn whose contract is required and has no stored final result with `failureReasonCode: STRUCTURED_OUTPUT_MISSING`, so a sandbox cannot claim a result it never submitted. The PR fields stay on the row.
- **Driver.** The schema reaches the sandbox as `BEX_AGENT_OUTPUT_SCHEMA` beside `BEX_AGENT_RESULT_URL` and `BEX_AGENT_OUTPUT_REQUIRED`, per turn, because every turn runs in a fresh sandbox. The driver validates with Ajv (2020-12, no remote loading). The overlay paragraph, which carries the schema text, and the `submit_result` tool exist only on a turn with a schema.

### D6 — `wait_agent_sessions`: server-side waiting

The input is `{ ids: [ags-…] (1–20), until: "any" | "all" (default "all"), timeoutSeconds (1–25, default 25) }`. The tool returns `{ sessions: [AgentSessionSummary], settled: [ids], timedOut: bool }`, where a summary is `id, state, outcome` (D7). A session counts as **settled** when its `state` is anything but `working`.

- **Mechanism:** the handler reads the sessions once, then waits on Postgres `LISTEN agent_session_state` (a `NOTIFY` is added to the store's phase and turn writes), with a 2 s re-read fallback. No pod affinity is needed, which keeps the transport stateless (ADR025).
- **Progress:** if the client sent a `progressToken`, each state change also emits `notifications/progress` with the summary line.
- **Why 25 s:** bex-api's `HTTPWriteTimeout` is 30 s (`internal/api/graphql_cost.go`), and several MCP clients time out tool calls at about 60 s. The description tells the model to call again when `timedOut` is true. A 25 s blocking call is a small fraction of the per-poll token cost. Raising the deadline for this one route with `http.ResponseController.SetWriteDeadline` is a later tuning knob, not part of this decision.
- **Rate limiting:** the call counts once against the caller's read budget, however long it blocks.

**MCP Tasks.** The 2025-11-25 MCP spec adds an experimental task-augmented `tools/call` for long-running work. Once go-sdk and the major clients support it, `create_agent_session` and `send_agent_session_message` should accept task augmentation, and `wait_agent_sessions` becomes the fallback for clients without it. That is a watch item; this ADR does not depend on it. go-sdk v1.8.0 support is **unverified**.

### D7 — `state` and `outcome`: one explicit answer to "is it done?"

`phase` (ADR047) stays the lifecycle field. Every session view on REST, GraphQL and MCP gains a derived, coarse **`state`**:

| `state` | When | Terminal for this turn? |
| --- | --- | --- |
| `working` | phase `creating`, `running`, `resuming`, `redispatching`, `canceling` | no |
| `needs_input` | reserved and never emitted: cloud agent sessions run unattended and never pause for a person ([`DO_NOT_DO.md` #APPROVAL-MODE](../.pm/DO_NOT_DO.md); ADR100 withdrawn) | — |
| `succeeded` | phase `completed` | yes |
| `failed` | phase `failed`; see `failureReasonCode` | yes |
| `canceled` | phase `canceled` | yes, session closed |

`steerable: bool` is reported beside it, so a session that finished its turn but still accepts messages is never ambiguous. This is the fix for Devin's `running/finished`. `state` is computed in `agentsessions.toView`, so all three adapters get it at once.

**`outcome`** is the compact result an LLM reads instead of the transcript:

```json
{
  "state": "succeeded",
  "steerable": true,
  "turn": 2,
  "summary": "Added retry with jitter to the webhook sender; unit tests pass.",
  "pullRequest": {
    "url": "…/pull/412",
    "number": 412,
    "headSha": "9f2c…",
    "draft": true
  },
  "result": { "tests_passed": true, "files_changed": 3 },
  "deliveryNote": "",
  "failureReasonCode": "",
  "cost": { "modelUSD": "0.42", "sandboxSeconds": 611 }
}
```

- **`summary`** is the final assistant text part of the latest completed turn, clipped to 4,000 characters with the same head-and-tail rule as tool output (ADR051). It is derived from the transcript, so no new agent behavior is needed.
- **`result`** is D5's validated output, or null.
- **`cost`** is null until ADR091's `agent_model_cost_microusd` meter ships. It is a decimal string, so float rounding never shows up in money.

**As built (w21/m260, 2026-11-05).** `outcome.cost` is `{modelUSD}` and is absent when the session made no priced request (a workspace's own model key, or nothing run yet). `modelUSD` is the sum of the session's recorded `agent_model_usage_events.price_microusd`, the events ADR091 bills from, written as an exact decimal string with two to six fractional digits (`"0.42"`, `"0.001234"`), so it matches the meter to the micro-dollar. (`get_agent_session_model_cost` rounds the same events to the cent for display.) Like `summary` and `result`, it is computed on the single-session read, in `wait_agent_sessions` and in the webhook payload, not on list pages. GraphQL spells it `outcome { cost { modelUSD } }`. **`sandboxSeconds` is not shipped.** bex meters sandbox compute per OpenSandbox sandbox in weighted milli-vCPU seconds (`sandbox_meter_states`, keyed by sandbox id), and every turn runs in a fresh sandbox whose id is not kept per session, so there is no per-session figure to report; wall-clock turn time would not be the metered quantity. Adding it needs a durable session-to-sandbox attribution first.

**As built (w9/m178, 2026-11-06; user decision 2026-11-06).** `outcome.cost` is `{modelUSD, computeUSD, totalUSD}`. Like Devin's single per-session consumption figure, `totalUSD` is the one number to compare with the bill, and the parts say where it went. No raw `sandboxSeconds` is exposed.

- `computeUSD` is the sandbox compute the meter attributed to the session's sandboxes, priced at the billed `sandbox_compute_seconds` rate (`pricing.yaml` `sandbox.usdPerWeightedSecond`, exact rational, rounded to the micro-dollar).
- `totalUSD` is `modelUSD + computeUSD`.
- All three are micro-dollar decimal strings in the same format, so `"0.00"` is a zero part. The cost is absent when the session has neither a priced model request nor attributed compute, or when a read fails (below).

**Attribution.** Migration 0206 adds `agent_session_sandboxes`, a durable sandbox → session map with a running `compute_milli_seconds` total.

- **Binding.** A trigger binds a sandbox whenever `agent_sessions.sandbox_id` adopts it, on first dispatch and on re-dispatch. The binding starts from what `usage_hourly` already holds for that sandbox.
- **Accrual.** Each meter accrual adds to the session's total under the meter's per-sandbox advisory lock, which the trigger takes too. A disappeared sandbox's final interval also counts.
- **Why a running total.** `usage_hourly` is pruned, so the figure cannot be summed from it later.
- **Older sessions.** Sandboxes reaped before migration 0206 are not attributed, and those sessions read `computeUSD` `"0.00"`. Two sessions live across the rollout can also undercount:
  - a sandbox metered by a pod still running the old binary after the migration;
  - hours already compacted out of `usage_hourly` before its binding.
- **Read failures.** If either part's read fails, the cost is absent. A total that silently left one part out would understate the bill while reading as authoritative.

GraphQL spells the fields `cost { modelUSD computeUSD totalUSD }`; REST, MCP and the webhook payload carry the same object.

### D8 — Cost and spend ceilings ride ADR091

`maxSpendUSD` on create is a **per-session** ceiling under ADR091's monthly workspace cap. The gateway model proxy checks the lower of the two before it forwards each request. A session at its own ceiling gets the ADR091 refusal code with a session qualifier, `AGENT_MODEL_SPEND_CAP_REACHED` plus `scope: "session"`, and its turn ends `failed`. This is the equivalent of Devin's `max_acu_limit`, priced in dollars rather than an opaque unit. It ships only after `w10/m14` lands the cost meter. Before that, the field is accepted and rejected with `400 AGENT_SESSION_SPEND_LIMIT_UNAVAILABLE`, never silently ignored.

**As built (w21/m260, 2026-11-05).** `w10/m14` t004 (cost meter) and t005 (caps) had shipped, so the ceiling is enforced.

- **Input.** `maxSpendUSD` is a number on `create_agent_session` (REST body, GraphQL `maxSpendUSD: Float`, MCP argument): more than 0 and at most 100000, else `400 AGENT_SESSION_SPEND_LIMIT_INVALID`. It is create-only and part of the idempotency digest. It is read back on the session as the decimal string `maxSpendUSD`. A follow-up turn cannot change it.
- **Where it applies.** Only the platform model route records a price. A create that cannot be metered (the workspace sets its own model key, the route does not cover the workspace, or the agent is not the route's `claude` profile) is refused `400 AGENT_SESSION_SPEND_LIMIT_UNAVAILABLE`, so a ceiling is never silently dropped.
- **Enforcement.** Migration 0201 adds `agent_sessions.max_spend_microusd`. The model mint (`ModelMinter.Mint`, which the gateway calls before every model request) reads the session's recorded price (`SUM(price_microusd)` over `agent_model_usage_events`, one indexed read, not through the 20 s workspace cache) after the ADR091 balance and monthly checks, so the lower of the two binds. At or over the ceiling it refuses with the new `ErrModelSessionSpendCapReached`, a 429 with refusal header `model-session-spend-cap-reached`. The proxy answers the agent in the Anthropic error shape with `code: AGENT_MODEL_SPEND_CAP_REACHED` and `scope: "session"`, and reports the refusal so the turn ends `failed` with `failureReasonCode: AGENT_MODEL_SPEND_CAP_REACHED` and a reason naming the session ceiling. A ceiling the minter cannot check (no session-spend reader wired) or a failed read fails the mint closed, never open.
- **Overshoot.** Usage is reported after each exchange, so a session overshoots its ceiling by at most the requests in flight (including one streaming response) when the check ran.

### D9 — Auth: a `bex.cloud_agents` capability scope, and a completion push

**Scope.** A new granular scope, **`bex.cloud_agents`**, is added to `core/scope.go` and advertised in `scopes_supported`. It satisfies the scope check for the `cloud-agents` toolset's tools **only**: create, message, cancel and the reads. An orchestrator can therefore be consented to run coding agents without `bex.write` on services, databases or env vars. `bex.read`/`bex.write` keep satisfying the agent tools as they do today. OpenFGA stays the authority: `bex.cloud_agents` never grants more than the caller's `can_create`/`can_operate` on `agent_session` (developer and contributor and up, `model.fga`). `api/scope_matrix_ops.go` gains the classification, and `TestAuthzGuardsEveryVerb` covers it.

**Completion push.** Devin has no outbound completion signal, so callers poll. bex already has workspace webhooks (`internal/webhooks`) and agent push notifications (`agent_pr_ready`, `agent_failed`, `agent_needs_decision`). The webhook vocabulary gains `agent_session_turn_succeeded`, `agent_session_turn_failed` and `agent_session_needs_input`. Each payload carries `{id, state, outcome}` without `summary`. A non-MCP orchestrator can then react without polling, and these are the same events that drive `wait_agent_sessions`.

**As built (w21/m260, 2026-11-05).**

- **Source.** Migration 0202 adds `agent_session_turn_events` and a trigger on `agent_sessions` that writes one row when the phase becomes `completed` or `failed` (`ON CONFLICT DO NOTHING` on `(session, turn, phase)`), so every path that terminalizes a turn is covered by one choke point. The webhook feed query (`ListWebhookEvents`) gained an `agent` arm over it; the push and alert-email consumers of the same feed ignore the new source. A cancel produces no event.
- **Vocabulary.** `agent_session_turn_succeeded` (phase `completed`), `agent_session_turn_failed` and `agent_session_needs_input` are subscribable bex extensions (the dashboard picker has a "Cloud Agents" group, and the Render vocabulary fixture lists them under `bexExtensions`). An endpoint with an empty event filter (all events) receives them like any new type. **`agent_session_needs_input` is advertised but never emitted**: no session phase waits for input, and none will (`state: needs_input` covers no phase; [`DO_NOT_DO.md` #APPROVAL-MODE](../.pm/DO_NOT_DO.md)).
- **Payload.** `{"type", "timestamp", "data": {"id", "state", "outcome"}}`: `id` is the session id, `state` the session `state` vocabulary for the phase the turn ended in, and `outcome` the same object as the session read minus `summary` and `files`. It is read from the session when the event is delivered, a second or two after the turn ended. If the session has meanwhile moved on to another turn, `outcome` is just `{state, steerable: false, turn}` so it never mixes in the newer turn's fields. `webhook-id` is the event id derived from the row key, so retries and the two replicas deduplicate.
- **Delivery.** The outcome is read only for an endpoint that wants the event, and a read failure fails the dispatch pass so the watermark stays and the event is retried. A deleted session drops its event.

### D10 — Phasing (proposed)

1. **Contract hygiene.** Description fixes; the golden test; `state`/`steerable` on all three transports; `outcome` without `result` and `cost`; `create_`/`send_` canonical names with aliases.
2. **The toolset.** The toolset tag, the `/mcp/cloud-agents` mount and stdio flag; `wait_agent_sessions` with LISTEN/NOTIFY and progress notifications; `idempotencyKey`; optional `branch`; `title`/`tags`; `bex.cloud_agents` scope. Docs: an ADR025 section on connecting an orchestrator to `/mcp/cloud-agents`.
3. **Structured output.** `submit_result` in `mcp-bex.ts`, the internal results verb, the overlay paragraph, the turn columns, `outcome.result`.
4. **Money and push.** `maxSpendUSD` and `outcome.cost` once `w10/m14` ships; the webhook event types.

Each phase is independently useful. Phase 1 and part of phase 2 need no engine changes.

## Consequences

- **The bex MCP offering gets two shapes.** "The whole platform" stays at `/mcp`. "Delegate a coding task" moves to `/mcp/cloud-agents`, where the coding agent is a nine-tool catalog behind a narrow consent scope, which is cheap in context and easy to approve.
- **The dashboard and mobile app benefit too.** `state` and `outcome` replace the client-side phase decoding they do today, and `idempotencyKey` fixes double-submit on flaky mobile networks.
- **New durable state:** `agent_sessions.idempotency_key`, `.tags`, `.title`; `agent_session_turns.output_schema`, `.result`; one `NOTIFY` channel.
- **The in-sandbox `bex` MCP server grows to two tools** and stays hand-written. Its error envelope (`structuredContent.error{code,message,hint}`) still differs from bex-api's `CODE: message` text. Unifying them is deferred.
- **`wait_agent_sessions` holds a request for up to 25 s.** The held requests are cheap: each pod shares one LISTEN connection rather than opening one per request, and the per-caller rate limit bounds how many a single caller can hold open. Phase 2 measured what a test can: 1,000 held waits on one pod share a single LISTEN connection and all wake on one notification (`TestManyHeldWaitsShareOneListenerAndAllWake`). What it did not measure is Postgres load under a real wake storm: every notification re-reads each waiter's own sessions, so reads scale with waiters × phase changes (bounded by the pool and the per-caller rate limit). Treat that ceiling as unmeasured until production traffic shows it.

## Alternatives considered

- **A separate `bex-agent` MCP server or binary.** Rejected. It would duplicate auth, workspace binding and the error mapping, and break ADR006's one-core rule. A mount point gives the same narrow catalog at none of that cost.
- **Devin-style multiplexed tools (`agent_session_interact` with an `operation` enum).** Rejected. MCP annotations are per tool, so a single `interact` tool must be marked destructive for its `get` calls too. Clients then prompt for approval on reads, and the per-tool scope matrix loses resolution. Nine named tools are within the context budget.
- **Anonymous public mode (the DeepWiki funnel).** Rejected for agents: every turn spends sandbox and model money, and abuse controls would need an unauthenticated meter. A free read-only "explain this public repo" tool could be its own future decision.
- **Polling only, with better descriptions.** Rejected. It is Devin's REST posture and their own MCP added `devin_session_gather` to escape it. Server-side waiting costs one handler; client polling costs the caller tokens on every session.
- **Expose the AI-SDK SSE stream as MCP resources with subscriptions.** Rejected for now. Resource subscription support across clients is uneven, and a coding transcript is not something an orchestrating LLM wants token by token. `outcome` plus the transcript tool covers it.
- **Parent and child sessions (Devin `parent_session_id`).** Deferred. bex agents cannot yet spawn agents; an in-sandbox `bex` tool for that, together with lineage on the session record, is a separate decision.
