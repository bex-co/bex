# w10 · m527 — A workspace admin chooses which platform models its members may pick, as Devin's admins choose the agents and models on offer

**Worker:** worker10 **Goal:** a workspace on the platform model route can turn catalog models off. Every member picker and the capabilities read offer only the models left on. A create, a message or an automation naming a turned-off model is refused with `AGENT_SESSION_MODEL_NOT_OFFERED`. A session or automation already on one runs the workspace default from its next turn. The catalog default can never be turned off. REST, GraphQL, MCP, the CLI and Settings → Agents read and write the list. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Design: the list's name and storage, which reads carry it, what happens to a default or a session on a turned-off model, when a change applies; ADR047, ADR090 and ADR018 amended | 45m | — |
| [t002](t002.md) | Backend: the agent environment stores the turned-off models and serves the list on REST, GraphQL and MCP | 1h30m | t001 |
| [t003](t003.md) | Backend: one offered-model set drives create, message, automation save and fire, the capabilities read, the default model and the next turn's fallback | 1h15m | t002 |
| [t004](t004.md) | CLI: `bex agent env set` changes the list, `bex agent env` shows it, `bex agent models` lists only offered models | 45m | t002 |
| [t005](t005.md) | Dashboard: a model list in Settings → Agents, the default-model picker limited to it, and the skeleton matching | 1h30m | t002 |
| [t006](t006.md) | Live check on production: a turned-off model leaves every picker, is refused on create, and a session on it runs the default from its next turn | 45m | t003, t004, t005 |
| [t007](t007.md) | Render parity (Devin) | 30m | t006 |
| [t008](t008.md) | Simplify | 20m | t007 |
| [t009](t009.md) | Test coverage | 45m | t007, t008 |
| [t010](t010.md) | Closeout | 10m | t009 |

Total: **10 tasks, about 8h15m**.

## Definition of done

1. **A list, with the whole catalog on by default.** The workspace agent environment carries the models its admins left on (`enabledModels`; the final name is t001's). A workspace with no row, and every workspace after the migration, offers the whole platform catalog (`BEX_AGENT_PLATFORM_MODELS`, the default first). A workspace admin changes it through REST `PATCH /v1/agent-environment`, GraphQL, MCP `update_agent_environment`, `bex agent env set` and Settings → Agents. Every read shows it: REST `GET /v1/agent-environment`, GraphQL, MCP `get_agent_environment`, `bex agent env` and Settings → Agents. A non-admin cannot change it, exactly as for the other environment fields. The change is audited as the other environment changes are.
2. **The catalog default stays on.** A write that leaves the catalog default off, names a model outside the catalog, or leaves the list empty is refused with a typed 400 naming the field and the offending model. Nothing is written.
3. **Turned off means not offered.** For a turned-off model:
   - The capabilities read's claude `models`, `bex agent models`, and the pickers in the composer, the steering composer and the automation form leave it out.
   - A session create, a message's `model` and an automation create or update naming it are refused with `400 AGENT_SESSION_MODEL_NOT_OFFERED`, listing the offered models. This holds on REST, GraphQL, MCP and the CLI.
   - `defaultModel` naming it is refused with `AGENT_ENVIRONMENT_MODEL_NOT_OFFERED`.
4. **Nothing is stranded.**
   - A session whose stored model is turned off keeps that model for the turn in flight, then runs the workspace default from its next turn, or the catalog default when the workspace default is off too. The session read reports the model that ran.
   - A queued message naming a model that was turned off while it waited runs the same fallback.
   - An automation whose template names a turned-off model runs the workspace default and records a note saying so, as a model retired from the catalog does today.
   - A workspace default that an admin turns off stops taking effect, and the catalog default runs, as Devin's pinned default does when it falls outside the allowlist. Settings → Agents says so.
5. **Said everywhere.** ADR047, ADR090 § Model pin and the ADR018 agent-model row describe the list: its default, who may change it, where it is enforced and when a change applies. The MCP tool descriptions, the GraphQL field descriptions and the CLI help name it.
6. **Live.** On production, once a deploy carries the milestone, in the QA user's `tian-personal` workspace:
   - An admin turns `anthropic/claude-opus-5.5` off. `bex agent models` and the dashboard composer no longer offer it. `bex agent run --model anthropic/claude-opus-5.5` is refused with `AGENT_SESSION_MODEL_NOT_OFFERED`. Turning off `deepseek/deepseek-v4.1-flash-1017` (the catalog default) is refused.
   - One short session runs a turn on `anthropic/claude-haiku-5.5`. The admin then turns that model off, and the session's next message runs on the workspace default, as its usage events and session read show.
   - The list is set back to the whole catalog, and the session is deleted.

## Source + Goal linkage

- **Source:** inbox note [w10/028](../done/028.md), filed by w10/m516's Devin comparison (t007) on 2026-11-19, promoted by `/loopx w10` on 2026-11-20.
- **Triage evidence (2026-11-20, `e9a2a96f3`):**
  - **No list exists.** The agent environment row has the default model and template, the disabled agents, the data-use acknowledgements and the `gh` opt-out, and nothing that narrows the models (`lego/backend/internal/store/agent_environments.go:31`).
  - **Every caller reads the whole catalog.** `agentsession.PlatformModels` (`lego/backend/internal/agentsession/devroute.go:356`) returns it with no workspace input. Five places take it as the choice:
    - `sessionModel` (`lego/backend/internal/agentsessions/environment.go:763`), used by create through `resolveCreateModel` (`service.go:644`), which is also the automation save path, and by the message check `steerModel` (`environment.go:806`) and the turn dispatch (`service.go:2196`).
    - `agentModels`, the capabilities read (`environment.go:838`).
    - The environment view's `models` (`environment.go:584`).
    - `validateEnvironmentInput`'s `defaultModel` check (`environment.go:493`).
    - `retiredTemplateModel` for automation fires (`automation_scheduler.go:247`).
  - **The fallback already exists.** `sessionModel` keeps a session's stored model "while the catalog still offers it and otherwise falls back" to the workspace default, else the catalog default. Narrowing the offered set per workspace therefore gives the next-turn fallback without a new mechanism. The mint pins the model the session recorded (`route.pin`, `lego/backend/internal/agentsession/model.go:405`), so a turn in flight keeps its model.
  - **It is more than one sitting.** It needs a migration and the store, three API surfaces, the enforcement points, the CLI, the dashboard, three ADRs and a live check.
- **Reference product (Devin):**
  - Devin's cloud agent picker: "Admins control which agents are available, so you may not see every option", and "the agent picker next to the message box lists only the agents available to you". A switch "takes effect with your next message" ([Your First Session](https://docs.devin.ai/get-started/first-run), read 2026-11-20).
  - Devin Desktop and the Devin CLI have an "Allowlisted models" team setting. It covers "which models are available to members", is held by the Devin Desktop admin permission, and hides "a model outside the allowlist" from the picker ([Devin Desktop admin controls](https://docs.devin.ai/desktop/controls), read 2026-11-20).
  - Their default model "must be in the allowlist to take effect". "If the pinned default is not present in the allowed models list above, Devin CLI falls back to the built-in default", because "the allowlist always takes precedence" ([Devin CLI Team Settings](https://docs.devin.ai/cli/enterprise/team-settings), read 2026-11-20).
  - This milestone ships the workspace-wide list, which is Devin's organization-level control. Devin's group-level lists and its filter by provider are narrower variants: t007 files them if users would miss them, and does not hold this milestone for them.
- **Goal linkage:** ADR008 pillar 5, cloud coding agents ([ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md), [ADR090](../../../docs/ADR090-agent-owned-delivery.md), [ADR091](../../../docs/ADR091-agent-model-pricing.md)). It completes [w10/m516](../done/m516/README.md), which gave every session the platform catalog, and follows the enabled-agents list of w7/m392, whose shape it reuses.
- **Expected outcome:** an admin who wants routine runs kept on cheap models turns `anthropic/claude-opus-5.5` off once in Settings → Agents. From then on, no member's session, message or automation in the workspace runs it.
- **Why now:** since m516, any member can pick the most expensive catalog model for every session. That spend is billed to the workspace at provider cost × 1.3 (ADR091), and only the spend cap stops it. Devin's admins can limit the choice; bex's cannot.
- **Render parity included (reference product: Devin):** the list is a tenant-facing surface on REST, GraphQL, MCP, the CLI and the dashboard. t007 checks those surfaces against each other, and the behavior against Devin's agent and model allowlists.
- **Related:** [w10/029](../029.md) gives the mobile composer a model control. Whichever ships second reads the offered models from the capabilities read, so the mobile picker honors this list too.
