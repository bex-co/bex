# w10 · m516 — An agent session or automation can choose its model from the platform catalog, and every surface shows the model that will run, as on Devin

**Worker:** worker10 **Goal:** on a workspace that runs agents on the platform model (no model key of its own), a person picks the model a `claude` session runs, per session, per automation and as the workspace default, from a catalog bex offers. A session can switch models from its next message. `bex agent models`, the capabilities read and the dashboard show the model each agent will run and the choices. The gateway still refuses any request model other than the one its session chose, and every request is still priced at its OpenRouter cost × 1.3 (ADR091 D1). **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Design: the platform model catalog, the per-session pin and the switch on the next message; ADR090, ADR047 and ADR091 amended | 1h | — |
| [t002](t002.md) | Backend: a session, an automation and the workspace default choose a catalog model; the mint and the sandbox run that model | 2h30m | t001 |
| [t003](t003.md) | A message can switch the session's model from its next turn, as Devin's agent toggle | 1h30m | t002 |
| [t004](t004.md) | The capabilities read and `bex agent models` show each agent's effective model and its choices | 1h15m | t002 |
| [t005](t005.md) | Dashboard: a model picker in the new-session composer, the steering composer, the automation form and Settings → Agents | 2h30m | t003, t004 |
| [t006](t006.md) | Live check on production: a non-default catalog model runs, is metered at its own cost, and every surface shows it | 45m | t005 |
| [t007](t007.md) | Render parity (Devin) | 30m | t006 |
| [t008](t008.md) | Simplify | 20m | t007 |
| [t009](t009.md) | Test coverage | 45m | t007, t008 |
| [t010](t010.md) | Closeout | 10m | t009 |

Total: **10 tasks, about 11h**.

## Definition of done

1. **A catalog.** bex-api reads a platform model catalog for the claude profile: the current `BEX_AGENT_PLATFORM_MODEL` (the default, first) plus the models t001 picks, at least one stronger Claude model among them. Each catalog model is checked live to answer OpenRouter's `/api/v1/messages` under the gateway's `provider.data_collection: deny` policy, and ADR090 records the check. The catalog is configuration, not code: changing it needs no release.
2. **Choose per session and per automation.** On a platform-route workspace, a session create (REST, GraphQL, MCP, `bex agent run --model`) or an automation (create, update, `bex agent automations apply`) naming any catalog model records it, the sandbox runs it (`ANTHROPIC_MODEL` and the three `ANTHROPIC_DEFAULT_*_MODEL` slots), the mint pins it, and the gateway meters its OpenRouter cost × 1.3. A model outside the catalog is refused at create with a typed 400 that lists the offered models, never a 403 mid-turn. The gateway still refuses any request model other than its session's.
3. **Workspace default.** A workspace admin sets the default model to any catalog model (REST `PATCH /v1/agent-environment`, GraphQL, MCP, Settings → Agents); a session or automation that names no model runs it. A model outside the catalog is refused with a typed 400.
4. **Switch on the next message.** A message to a running session (REST, GraphQL, MCP, `bex agent send`, the steering composer) may name a catalog model; the session's next turn runs it and the session read reports it, as Devin's agent toggle does. Without a model, a message keeps the session's model.
5. **Visible.** The capabilities read (REST, GraphQL, MCP) gives every offered agent its effective model and the models it can choose. `bex agent models` prints them next to each agent (`claude — Claude Code — ready — deepseek/deepseek-v4.1-flash-1017 (default) …`). The dashboard composer, steering composer, automation form and Settings → Agents show the model that will run and let the person pick another. A profile with one fixed model (`muse`, `deepseek`) shows that model and offers no choice. A workspace on its own key keeps today's free-form model.
6. **Live.** On production, once a deploy carries the milestone: a session in the `bex` workspace on a non-default catalog model completes a turn, its usage events record that model at its own cost, and `bex agent models` and the dashboard composer show the choice.

## Source + Goal linkage

- **Source:** inbox note [w10/019](../done/019.md), filed from the agent-automation runs of 2026-11-15..18, promoted by `/loopx w10` on 2026-11-19.
- **Triage evidence (2026-11-19, `7d71e9cfd`):**
  - **One model per workspace.** `agentsession.PlatformPinnedModel` (`lego/backend/internal/agentsession/devroute.go:311`) returns the one route model for every workspace without its own key. `sessionModel` (`lego/backend/internal/agentsessions/environment.go:649`) refuses any other explicit claude model with `AGENT_SESSION_MODEL_PINNED`, automations resolve through the same path (`resolveCreateModel`, `service.go:648`), and the workspace default is refused with `AGENT_ENVIRONMENT_MODEL_PINNED` (`environment.go:502`). The mint pins `route.model` (`agentsession/model.go:402`) and `ModelRouteEnv` injects it into every sandbox (`devroute.go:344`).
  - **The choice is hidden.** `bex agent models` prints agent, name and readiness only (`lego/cli/internal/agent/config.go:506`); capabilities `agents[]` carry `id`, `label` and `modelKeyReady` (`agentsessions/models.go:122`). The dashboard shows the pinned model read-only in Settings → Agents only (`agent-environment-card.tsx:284`); the composer has no model control.
  - **Why the pin existed is gone.** ADR090 pinned the model on 2026-10-23 so a sandbox could not spend the platform key on a more expensive model while nothing was priced. ADR091 (2026-10-30) prices every platform request at its own reported cost × 1.3, with no per-model rows, under per-workspace spend caps and trust tiers. A per-session pin keeps the security property (the sandbox cannot pick a model its session did not) while the person chooses.
- **Reference product (Devin):** "The agent selector in the message box controls which agent runs your session"; it can be set when the session starts and switched mid-session, "the change takes effect with your next message", and "Admins control which agents are available" ([Your First Session](https://docs.devin.ai/get-started/first-run), read 2026-11-19). The API takes `devin_mode` (`lite`, `normal`, `ultra`, `fusion`, …) at create, defaulting to the organization's, and the session read reports the resolved mode ([Create Session](https://docs.devin.ai/api-reference/v3/sessions/post-organizations-sessions)); Devin CLI takes `--model` and `devin models list` lists the choices ([Models](https://docs.devin.ai/cli/models)). DoD 2-5 match that. An admin allow-list narrowing the catalog per workspace is the stricter variant: t007 files it as its own item instead of gating this milestone on it.
- **Goal linkage:** ADR008 pillar 5, cloud coding agents ([ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md)), priced by [ADR091](../../../docs/ADR091-agent-model-pricing.md).
- **Expected outcome:** a workspace runs bug-hunting routines on a stronger model and routine work on the cheap default, and anyone reading `bex agent models` or the composer knows which model a session will run.
- **Why now:** a controlled comparison of routine runs on 2026-11-17 found model quality mattered most for bug hunting, and 17 scheduled automations in the `bex` workspace cannot act on it; `bex agent run --model` returns 400 today.
- **Render parity included (reference product: Devin):** sessions, automations and the agent environment are tenant-facing on REST, GraphQL, MCP, the CLI and the dashboard; t007 walks each against Devin's agent selector.
