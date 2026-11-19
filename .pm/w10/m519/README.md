# w10 · m519 — A session's sandbox size is chosen in the dashboard composer and defaults to the workspace's, as on Devin

**Worker:** worker10 **Goal:** a person picks the sandbox template (name and size) a session runs on from the dashboard composer, the automation form or the CLI, and a workspace admin sets the default that every session and automation naming none runs on. bex-api serves the template list, so no client hard-codes `agent` and `agent-4c-8g` or their sizes, and every session read reports the template it ran on. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Design: the workspace default template, where an empty choice resolves, and the template list's shape; ADR047 amended | 45m | — |
| [t002](t002.md) | Backend: the workspace agent environment's default template, used by a create or an automation run that names none | 2h | t001 |
| [t003](t003.md) | Backend: the capabilities and environment reads serve the agent templates (name, CPU, memory, the default) on REST, GraphQL and MCP | 1h | t001 |
| [t004](t004.md) | CLI: `bex agent env set --default-template`, the templates in `bex agent env` and `bex agent models`, no hard-coded sizes | 1h | t002, t003 |
| [t005](t005.md) | Dashboard: a Sandbox picker in the new-session composer and the automation form, the default in Settings → Agents, the sandbox in the session header | 2h30m | t002, t003 |
| [t006](t006.md) | Live check on production: the default and a composer choice run on the chosen template, and every surface shows it | 45m | t004, t005 |
| [t007](t007.md) | Render parity (Devin) | 30m | t006 |
| [t008](t008.md) | Simplify | 20m | t007 |
| [t009](t009.md) | Test coverage | 45m | t007, t008 |
| [t010](t010.md) | Closeout | 10m | t009 |

Total: **10 tasks, about 9h45m**.

## Definition of done

1. **The list is served.** The capabilities read and the agent environment read (REST, GraphQL, MCP) list every agent template a session can name, smallest first, each with its name, CPU and memory, and say which one a session that names none runs on (the workspace default). The dashboard and the CLI read it from there; neither hard-codes a template name or size.
2. **Workspace default.** A workspace admin sets `defaultTemplate` to any registered agent template through REST `PATCH /v1/agent-environment`, GraphQL, MCP `update_agent_environment`, `bex agent env set --default-template` and Settings → Agents; `""` returns to `agent`. A name that is not an agent template is refused with a typed 400 listing the templates. A session create, or an automation run, that names no template runs on the workspace default, and its session read reports that template in `agentConfig.template`.
3. **Choose in the dashboard.** The `/agents` new-session composer and the automation form offer the templates with their sizes, preselect the workspace default, and the session (or automation) is saved with the one picked. The session detail header shows the template the session runs on.
4. **Errors keep naming the choices.** The create's refusal of an unknown template and the out-of-memory failure reason still name the registered templates; the out-of-memory reason also says how to pick a larger one in the dashboard, not only with `bex agent run --template`.
5. **Live.** On production, once a deploy carries the milestone: in the `bex` workspace, a default set to `agent-4c-8g` makes a session that names none run on it, a composer pick of `agent` runs on `agent`, each session read and header shows its template, and the default is set back.

## Source + Goal linkage

- **Source:** inbox note [w10/023](../done/023.md), filed by w10/017's worker on 2026-11-18 as that note's stated gap, promoted by `/loopx w10` on 2026-11-19.
- **Triage evidence (2026-11-19, `bf637f3fc`):**
  - **No workspace default.** `agent_environments` has `default_model` but no template column (`lego/backend/internal/store/migrations/0169_agent_environments.up.sql:17`). A session naming none runs on the hard-coded `sandbox.DefaultAgentTemplate` (`lego/backend/internal/sandbox/service.go:1079`, used at `:1212` and by the out-of-memory reason, `lego/backend/internal/agentsessions/completion.go:254`). Its session read shows no template.
  - **The list is hidden.** The capabilities read "never returns model endpoints, templates, egress, or credentials" (`agentsessions/mcp.go:283`, `service.go:1129`, written for w11/m6's mobile composer when every sandbox template was internal). The agent templates exist only as `sandbox.AgentSessionLifecycle.AgentTemplates()` (`sandbox/service.go:1113`), used by the create's refusal (`agentsessions/service.go:624`). The CLI's `--template` help hard-codes `agent` (2 CPU, 4 GB) and `agent-4c-8g` (4 CPU, 8 GB) (`lego/cli/internal/agent/agent.go:545`).
  - **No dashboard control.** The create mutation passes a `template` through (`dashboard/src/features/agent-sessions/hooks/use-agent-session-mutations.ts:280`), but `new-session-composer.tsx` offers none; the mapper reads `agentConfig.template` (`lib/mapper.ts:108`) but `session-detail-header.tsx` shows only the model; the automation form carries a stored template through untouched with no field (`agent-automations/lib/form.ts:118,181`); Settings → Agents has no default.
- **Reference product (Devin):** a session's machine is chosen when it starts ("Pick the outpost under **Configuration → Virtual environment** when starting a session"), and an admin sets the organization default ("make it the organization default under **Settings → Devin → Default platform**", also an API), which applies to new sessions ([Outposts](https://docs.devin.ai/cloud/outposts/overview), read 2026-11-18 and again 2026-11-19). DoD 2-3 match that. The default applies to sessions created afterwards; running sessions keep their template.
- **Goal linkage:** ADR008 pillar 5, cloud coding agents ([ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md)); continues [w10/017](../done/017.md), which gave the CLI and API the per-session choice.
- **Expected outcome:** a dashboard user whose session ran out of memory picks `agent-4c-8g` in the composer instead of needing the CLI, and a workspace whose repositories need the larger machine sets it once as the default.
- **Why now:** three ad-hoc sessions on `bex-co/beancount-io` failed out of memory on 2026-11-17/18 where the automations on `agent-4c-8g` finished (w10/017's evidence); the out-of-memory reason now tells a dashboard user about a choice the dashboard does not offer.
- **Render parity included (reference product: Devin):** sessions, automations and the agent environment are tenant-facing on REST, GraphQL, MCP, the CLI and the dashboard; t007 walks each against Devin's virtual-environment picker and organization default. The mobile composer is a known candidate follow-up, as w10/029 is for the model.
