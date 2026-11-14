# w7 · m392 — Admins choose which Cloud Agents are available; Muse needs admin opt-in

**Worker:** worker7 **Goal:** workspace admins decide which agent profiles members can use, as Devin admins do. Muse Code starts off in every workspace and an admin can turn it on only after acknowledging that Meta may train on what Muse sessions send. **Status:** done (2026-11-14)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | `enabledAgents` and the Muse data-use acknowledgement on the agent environment (REST, GraphQL, MCP; admin-only) — **DONE** | 75m | w7/m390/t003 |
| t002 | Enforcement: create, automation runs and child spawns refuse a disabled agent; capabilities filter it — **DONE** | 60m | t001 |
| t003 | Settings → Agents toggles with the Muse notice; composer hides disabled agents; `bex agent env` — **DONE** | 60m | t001 |
| t004 | Production check — **DONE** | 30m | t002, t003 |
| t005 | Render parity (Devin reference) — **DONE** | 30m | t004 |
| t006 | Simplify — **DONE** | 30m | t005 |
| t007 | Test coverage — **DONE** | 45m | t005 |
| t008 | Closeout — **DONE** | 10m | t006, t007 |

## Definition of done

- `GET /v1/agent-environment`, GraphQL `agentEnvironment` and MCP `get_agent_environment` return `enabledAgents`; `PATCH /v1/agent-environment`, GraphQL `updateAgentEnvironment` and MCP `update_agent_environment` change it, admin-only (403 otherwise). Claude Code, Codex and Gemini are enabled by default; Muse is disabled by default.
- Enabling `muse` is refused unless the same request acknowledges the contributor data use; the acknowledgement (who, when) is stored and readable.
- With an agent disabled, it is absent from the capabilities projection and the composer, and `createAgentSession`, an automation run and a child spawn using it are refused with `AGENT_SESSION_AGENT_DISABLED`; sessions already running can still be steered.
- Settings → Agents shows per-agent toggles and the Muse notice in en and zh; `bex agent env` shows the list and `bex agent env set` changes it.
- Backend real-DB suite, `make lint`, dashboard `yarn lint` + `yarn test` pass, and the change is live on production.

## Source + Goal linkage

- **Source:** `/pm-brainstorm add meta muse coding agent with muse-code-acp to our cloud agents`, 2026-11-13, item 3 (proposed for w1; the user filed every item to w7 with `/pmx them all to w7`). Devin: "Admins control which agents are available, so you may not see every option", and enterprise admins switch on the SWE-2 preview in Settings (https://docs.devin.ai/get-started/first-run.md). bex has no per-workspace control over agent profiles today (`agentProfiles()` in `lego/backend/internal/agentsessions/models.go` is fixed for every workspace).
- **Goal linkage:** ADR008 pillar 5; [ADR047](../../../../docs/ADR047-cloud-coding-agent-sessions.md) D9 (the product surface) and Devin parity for the agent picker; [ADR091](../../../../docs/ADR091-agent-model-pricing.md) D8 (data and terms) for the Muse acknowledgement.
- **Expected outcome:** a workspace's code goes to Meta's contributor tier only after its admin agreed; admins can remove any agent from their members' choices.
- **Why now:** m391's route sends tenant code to a tier on which Meta trains (https://dev.meta.ai/docs/pricing-rate-limits); bex cannot grant Meta that permission over a tenant's code without the tenant agreeing, so m391/t005 depends on this milestone.
- **Render parity included** (Devin reference): new fields and refusals on REST, GraphQL, MCP, the dashboard and the CLI.
- **Recommendation recorded, not a user sign-off:** Muse off by default with an admin acknowledgement. It rests on the tenant-consent point above and on Devin's admin-enabled preview agents; the alternative (on everywhere with only a terms disclosure) stays open for the user to choose instead. Other agents stay on by default.
- **Anti-goal check:** this is an admin setting, not an approval flow or a per-turn prompt (DO_NOT_DO `#APPROVAL-MODE`), and not an internal allowlist.

## Progress (2026-11-14)

- **t001:** migration 0252 adds `agent_environments.disabled_agents` and `agent_data_use_acks` (`{agent: {at, by}}`); the manifest marks `muse` `requiresDataUseAcknowledgement`. The agent environment returns `enabledAgents` and `agentDataUse` and takes `enabledAgents` (replace) and `acknowledgeAgentDataUse` on REST, GraphQL and MCP, admin-only. Unknown ids or a stray acknowledgement: 400 `AGENT_ENVIRONMENT_AGENT_INVALID`; enabling Muse without its acknowledgement: 400 `AGENT_ENVIRONMENT_DATA_USE_UNACKNOWLEDGED`. The first admin's acknowledgement is kept, and an acknowledgement alone turns nothing on. Stored as the turned-off list, so a profile added later starts on unless it needs an acknowledgement.
- **t002:** `resolveCreateModel` refuses a turned-off agent with 400 `AGENT_SESSION_AGENT_DISABLED`; every start path runs it (REST/GraphQL/MCP create, automation save and fire, child spawn, PR takeover), and steering a running session is unaffected. Capabilities list only enabled agents (one environment read, in parallel with GitHub).
- **t003:** Settings → Agents gains "Available agents": a switch per agent (disabled for members), a confirm with the Muse data-use notice and the terms link before Muse turns on, a matching skeleton block, en/zh. Saving refreshes the composer's agent list. `bex agent env` shows the agents and data-use state; `bex agent env set --agents a,b --acknowledge-data-use muse`.
- **t005 (Render parity, Devin reference):** REST, GraphQL and MCP share `Environment`/`EnvironmentInput` (the shape test and the MCP golden cover the new fields) and one create path; the CLI contract fixture carries them. Devin: "Admins control which agents are available" (https://docs.devin.ai/get-started/first-run.md); bex matches, with Muse off until acknowledged like Devin's admin-enabled previews. No ADR018 row changes.
- **t006:** `/simplify` applied: capabilities read the environment once inside the errgroup; the acknowledgement-alone bug fixed; the patch is filled in place; the terms link and CLI helpers reused; per-agent confirm copy; the dashboard's data-use list is held to the manifest by a test (and the dashboard CI paths include the manifest).
- **t007:** service (rules, admin-only, refusals, steer allowed, capabilities), real-DB store (round trip, first acknowledgement kept), dashboard (toggle off, confirm before Muse, no re-ask once acknowledged, members disabled, manifest parity) and CLI (flags, output, contract twin) tests.
- **t004 (production, 2026-11-14 06:56 UTC, deploy run 43566568833 at 706a389d5):** as the QA user (admin) in workspace `bex-canary` over `https://api.bex.co/graphql`:
  1. Default state: `enabledAgents` `[claude, codex, gemini]`, Muse unacknowledged; capabilities `claude, codex, gemini`.
  2. `updateAgentEnvironment` `enabledAgents: [claude, codex]` → saved; capabilities list `claude, codex` (the composer's agent list comes from this projection); `createAgentSession` with `gemini` → `AGENT_SESSION_AGENT_DISABLED` ("the gemini agent is turned off in this workspace; a workspace admin can turn it on in Settings → Agents").
  3. `enabledAgents: [claude, codex, muse]` without the acknowledgement → `AGENT_ENVIRONMENT_DATA_USE_UNACKNOWLEDGED`; with `acknowledgeAgentDataUse: [muse]` → saved, `agentDataUse` muse `acknowledgedAt 2026-11-14T06:56:38.603Z` with the QA user as `acknowledgedBy`, unchanged on re-read. Muse stays absent from capabilities and a Muse create answers `AGENT_SESSION_AGENT_UNAVAILABLE`: no Meta route serves the workspace until w7/m391 is unblocked.
  4. Restored `enabledAgents: [claude, codex, gemini]` (Muse off; the acknowledgement record stays); capabilities `claude, codex, gemini`. Logged out. The dashboard pages were not checked in a browser here; their behavior is covered by the t007 component tests.
- **t008:** closed 2026-11-14.
