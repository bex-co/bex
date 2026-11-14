# w7 · m392 — Admins choose which Cloud Agents are available; Muse needs admin opt-in

**Worker:** worker7 **Goal:** workspace admins decide which agent profiles members can use, as Devin admins do. Muse Code starts off in every workspace and an admin can turn it on only after acknowledging that Meta may train on what Muse sessions send. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | `enabledAgents` and the Muse data-use acknowledgement on the agent environment (REST, GraphQL, MCP; admin-only) | 75m | w7/m390/t003 |
| t002 | Enforcement: create, automation runs and child spawns refuse a disabled agent; capabilities filter it | 60m | t001 |
| t003 | Settings → Agents toggles with the Muse notice; composer hides disabled agents; `bex agent env` | 60m | t001 |
| t004 | Production check | 30m | t002, t003 |
| t005 | Render parity (Devin reference) | 30m | t004 |
| t006 | Simplify | 30m | t005 |
| t007 | Test coverage | 45m | t005 |
| t008 | Closeout | 10m | t006, t007 |

## Definition of done

- `GET /v1/agent-environment`, GraphQL `agentEnvironment` and MCP `get_agent_environment` return `enabledAgents`; `PATCH /v1/agent-environment`, GraphQL `updateAgentEnvironment` and MCP `update_agent_environment` change it, admin-only (403 otherwise). Claude Code, Codex and Gemini are enabled by default; Muse is disabled by default.
- Enabling `muse` is refused unless the same request acknowledges the contributor data use; the acknowledgement (who, when) is stored and readable.
- With an agent disabled, it is absent from the capabilities projection and the composer, and `createAgentSession`, an automation run and a child spawn using it are refused with `AGENT_SESSION_AGENT_DISABLED`; sessions already running can still be steered.
- Settings → Agents shows per-agent toggles and the Muse notice in en and zh; `bex agent env` shows the list and `bex agent env set` changes it.
- Backend real-DB suite, `make lint`, dashboard `yarn lint` + `yarn test` pass, and the change is live on production.

## Source + Goal linkage

- **Source:** `/pm-brainstorm add meta muse coding agent with muse-code-acp to our cloud agents`, 2026-11-13, item 3 (proposed for w1; the user filed every item to w7 with `/pmx them all to w7`). Devin: "Admins control which agents are available, so you may not see every option", and enterprise admins switch on the SWE-2 preview in Settings (https://docs.devin.ai/get-started/first-run.md). bex has no per-workspace control over agent profiles today (`agentProfiles()` in `lego/backend/internal/agentsessions/models.go` is fixed for every workspace).
- **Goal linkage:** ADR008 pillar 5; [ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md) D9 (the product surface) and Devin parity for the agent picker; [ADR091](../../../docs/ADR091-agent-model-pricing.md) D8 (data and terms) for the Muse acknowledgement.
- **Expected outcome:** a workspace's code goes to Meta's contributor tier only after its admin agreed; admins can remove any agent from their members' choices.
- **Why now:** m391's route sends tenant code to a tier on which Meta trains (https://dev.meta.ai/docs/pricing-rate-limits); bex cannot grant Meta that permission over a tenant's code without the tenant agreeing, so m391/t005 depends on this milestone.
- **Render parity included** (Devin reference): new fields and refusals on REST, GraphQL, MCP, the dashboard and the CLI.
- **Recommendation recorded, not a user sign-off:** Muse off by default with an admin acknowledgement. It rests on the tenant-consent point above and on Devin's admin-enabled preview agents; the alternative (on everywhere with only a terms disclosure) stays open for the user to choose instead. Other agents stay on by default.
- **Anti-goal check:** this is an admin setting, not an approval flow or a per-turn prompt (DO_NOT_DO `#APPROVAL-MODE`), and not an internal allowlist.
