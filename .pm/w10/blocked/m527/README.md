# w10 · m527 — A workspace admin chooses which platform models its members may pick, as Devin's admins choose the agents and models on offer

**Worker:** worker10 **Goal:** a workspace on the platform model route can turn catalog models off. Every member picker and the capabilities read offer only the models left on. A create, a message or an automation naming a turned-off model is refused with `AGENT_SESSION_MODEL_NOT_OFFERED`. A session or automation already on one runs the workspace default from its next turn. The catalog default can never be turned off. REST, GraphQL, MCP, the CLI and Settings → Agents read and write the list. **Status:** blocked (t001–t005, t007–t009 done 2026-11-20 and shipped in `68e848a93`; t006, the production check, waits for a deploy carrying it, then t010 closes)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Design: the list's name and storage, which reads carry it, what happens to a default or a session on a turned-off model, when a change applies; ADR047, ADR090 and ADR018 amended — **DONE** | 45m | — |
| [t002](done/t002.md) | Backend: the agent environment stores the turned-off models and serves the list on REST, GraphQL and MCP — **DONE** | 1h30m | t001 |
| [t003](done/t003.md) | Backend: one offered-model set drives create, message, automation save and fire, the capabilities read, the default model and the next turn's fallback — **DONE** | 1h15m | t002 |
| [t004](done/t004.md) | CLI: `bex agent env set` changes the list, `bex agent env` shows it, `bex agent models` lists only offered models — **DONE** | 45m | t002 |
| [t005](done/t005.md) | Dashboard: a model list in Settings → Agents, the default-model picker limited to it, and the skeleton matching — **DONE** | 1h30m | t002 |
| [t006](t006.md) | Live check on production: a turned-off model leaves every picker, is refused on create, and a session on it runs the default from its next turn | 45m | t003, t004, t005 |
| [t007](done/t007.md) | Render parity (Devin) — **DONE** | 30m | t006 |
| [t008](done/t008.md) | Simplify — **DONE** | 20m | t007 |
| [t009](done/t009.md) | Test coverage — **DONE** | 45m | t007, t008 |
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

## Design record (t001, 2026-11-20)

- **Names.** The field is `enabledModels` on the REST and MCP read and input and on GraphQL `AgentEnvironment` and `AgentEnvironmentInput`. A write replaces the whole list, as `enabledAgents` does. The CLI flag is `bex agent env set --models ID,ID…`, which replaces the list like `--agents`; one flag with the wire's replace semantics beat an `--enable-model`/`--disable-model` pair. Settings → Agents labels it "Models members can choose".
- **Storage.** The opt-out `agent_environments.disabled_models text[] NOT NULL DEFAULT '{}'` (migration 0315, at most 64 entries), Go `store.AgentEnvironment.DisabledModels`. A workspace with no row, which bex-api reads as a zero-value row, every row from before the migration and a model added to the catalog later all offer the whole catalog. A write stores every catalog model the list leaves out, so a model that has left the catalog drops out of the column on the next write.
- **Which reads carry what.** The environment's `models` stays the whole catalog, so Settings can list what an admin may turn on. `enabledModels` is the subset members may choose, in catalog order with the catalog default first. The capabilities read's claude `models` becomes that subset, and its `model` is the workspace default while it is offered, else the catalog default. One helper, `offeredModels` in `lego/backend/internal/agentsessions/environment.go`, serves `sessionModel` (create, the message check, the turn dispatch, automation save), `agentModels`, `environmentView`, the `defaultModel` check and `retiredTemplateModel`. It always keeps the catalog default, even if a stored list names it after the catalog's default changed.
- **Refusals.**
  - `400 AGENT_ENVIRONMENT_MODELS_INVALID` (params `field: enabledModels`, `model`, `models`: the catalog) for a list that leaves the catalog default off (an empty list does, naming the default) or names a model outside the catalog. Off the route there is no `model` param.
  - `400 AGENT_SESSION_MODEL_NOT_OFFERED` is unchanged, and so is `AGENT_ENVIRONMENT_MODEL_NOT_OFFERED` for `defaultModel`, which is judged against the list as it stands after the same update. Both list only the offered models. For a model the catalog has, the message says the workspace turned it off and that an admin can turn it on in Settings → Agents.
- **A turned-off workspace default (Devin's rule).** Turning off the model `defaultModel` names is allowed. The default stays saved and reads back, but it stops taking effect and the catalog default runs, as Devin's pinned default falls back to the built-in default outside the allowlist. `bex agent env` marks it "turned off: sessions run <catalog default>", and Settings → Agents says the same. The alternative, refusing the list until the default changes, forces a two-step change that Devin does not ask for.
- **On the workspace's own key** there is no catalog, so the list has no effect. The reads give `enabledModels: []`, as they give `models: []`. A write naming it is refused (`AGENT_ENVIRONMENT_MODELS_INVALID`), judged by the key the workspace has after the update, so one update may clear the key and narrow the list. Settings hides the list. A stored list is kept and applies again if the workspace clears its key.
- **When a change applies.** New creates, messages and automation saves see it at once. A turn in flight keeps its model, because the mint pins the model the session recorded (`route.pin` still checks only the catalog). The next dispatch re-resolves through `sessionModel`: a stored model that is turned off runs the workspace default, or the catalog default when that is off too, and records it as the session's model. A queued message naming a model turned off while it waited runs the same fallback. An automation fire records "the template's model … is turned off in this workspace; this run used …".
- **Recorded in** [ADR047 D7](../../../../docs/ADR047-cloud-coding-agent-sessions.md#d7--agent-binary-byo-api-key-first-binary-pluggable) (w10/m527 amendment), [ADR090 § Model pin](../../../../docs/ADR090-agent-owned-delivery.md#platform-openrouter-route-for-an-allowlisted-workspace-2026-10-23) ("A workspace narrows the catalog") and the ADR018 agent-model row.

## Render parity (Devin) record (t007, 2026-11-20)

- **The surfaces agree.** REST `GET`/`PATCH /v1/agent-environment`, GraphQL `agentEnvironment`/`updateAgentEnvironment` and MCP `get_agent_environment`/`update_agent_environment` carry the same `enabledModels` list beside the whole-catalog `models`. Every surface reads the whole catalog for a workspace with no row. An admin's write on any surface reads back on the others, in catalog order. A member's write is refused on each as for the other fields: REST 403, a GraphQL error, an MCP tool error. Each refusal is the same typed 400 on all three, and nothing is written. These are pinned by `TestEnabledModelsOnEverySurface` and `TestEnabledModelsRefusalsOnEverySurface`. The capabilities read narrows `models` on all three surfaces (`TestCapabilitiesShowEachAgentsModelOnEverySurface`). `bex agent env` prints the models that are on, with the catalog default marked, plus a "Models off" row, and reads a field missing from an older bex-api as the whole catalog. `bex agent env set --models` sends only that field, and an empty value is a usage error. `bex agent models` lists what the server offers. Settings → Agents and the composer, the steering composer and the automation form read the same GraphQL fields.
- **Devin** (re-read 2026-11-20: [Your First Session](https://docs.devin.ai/get-started/first-run), [Devin Desktop admin controls](https://docs.devin.ai/desktop/controls), [Devin CLI Team Settings](https://docs.devin.ai/cli/enterprise/team-settings)):
  - **Who may change it:** Devin's Desktop admin permission; bex's workspace admins (`can_manage`). Same shape.
  - **What members see:** in Devin, a model outside the allowlist is hidden "from the relevant picker". Same here: the capabilities read, `bex agent models` and every dashboard picker.
  - **The default:** Devin says "the allowlist always takes precedence", and a pinned default outside it falls back to the built-in default. Same here.
  - **A running session:** a Devin switch "takes effect with your next message". Same here: the turn in flight keeps its model.
  - **A model added later:** Devin offers "allow all models" or "allowlist specific models", a "curated list of approved models", and the page does not say how a new model is treated. bex's list is an opt-out, so a new catalog model starts on. Filed as [w10/036](../../036.md).
  - **Filter by provider:** Devin Desktop can "filter by specific models or by provider". bex filters by model only. Filed as [w10/037](../../037.md).
  - **Not filed:** group-level lists, because a bex workspace has no member groups (members have roles only), so there is nothing to bind such a list to. Devin's enterprise defaults have no counterpart, because the workspace is bex's top tenancy level.

## Source + Goal linkage

- **Source:** inbox note [w10/028](../../done/028.md), filed by w10/m516's Devin comparison (t007) on 2026-11-19, promoted by `/loopx w10` on 2026-11-20.
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
- **Goal linkage:** ADR008 pillar 5, cloud coding agents ([ADR047](../../../../docs/ADR047-cloud-coding-agent-sessions.md), [ADR090](../../../../docs/ADR090-agent-owned-delivery.md), [ADR091](../../../../docs/ADR091-agent-model-pricing.md)). It completes [w10/m516](../../done/m516/README.md), which gave every session the platform catalog, and follows the enabled-agents list of w7/m392, whose shape it reuses.
- **Expected outcome:** an admin who wants routine runs kept on cheap models turns `anthropic/claude-opus-5.5` off once in Settings → Agents. From then on, no member's session, message or automation in the workspace runs it.
- **Why now:** since m516, any member can pick the most expensive catalog model for every session. That spend is billed to the workspace at provider cost × 1.3 (ADR091), and only the spend cap stops it. Devin's admins can limit the choice; bex's cannot.
- **Render parity included (reference product: Devin):** the list is a tenant-facing surface on REST, GraphQL, MCP, the CLI and the dashboard. t007 checks those surfaces against each other, and the behavior against Devin's agent and model allowlists.
- **Related:** [w10/029](../../029.md) gives the mobile composer a model control. Whichever ships second reads the offered models from the capabilities read, so the mobile picker honors this list too.
