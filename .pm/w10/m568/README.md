# w10 · m568 — A model added to the platform catalog stays off in a workspace whose admins narrowed its list, as Devin's allowlist leaves a model it does not list off

**Worker:** worker10 **Goal:** the workspace model list becomes Devin's two modes. A workspace whose admins have turned no model off allows every catalog model, including one added later. Once an admin turns any model off, the list is an allowlist: a model added to the catalog afterwards is off there until an admin turns it on. Settings → Agents and `bex agent env` show such a model as off because it is new in the catalog. No workspace's offer changes when this ships. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Design: storage of the allowlist, when a workspace allows every model, the read field naming new models, the legacy-row conversion and the rolling deploy; ADR047, ADR090 and ADR018 amended | 45m | — |
| [t002](t002.md) | Backend: the migration, the store, and the one-shot conversion of rows narrowed under the opt-out | 1h15m | t001 |
| [t003](t003.md) | Backend: the offered set reads the allowlist, and the environment read names the models that are new in the catalog on REST, GraphQL and MCP | 1h30m | t002 |
| [t004](t004.md) | CLI: `bex agent env` shows a new catalog model as off, "new in the catalog" | 45m | t003 |
| [t005](t005.md) | Dashboard: Settings → Agents shows a new catalog model off with a "New in the catalog" label | 1h | t003 |
| [t006](t006.md) | Live check: a catalog gaining a model on `dev-10`, then production's offers unchanged by the deploy | 45m | t003, t004, t005 |
| [t007](t007.md) | Render parity (Devin) | 30m | t006 |
| [t008](t008.md) | Simplify | 20m | t007 |
| [t009](t009.md) | Test coverage | 45m | t007, t008 |
| [t010](t010.md) | Closeout | 10m | t009 |

Total: **10 tasks, about 7h45m**.

## Definition of done

1. **Every model, new ones included, until an admin narrows the list.**
   - These workspaces offer the whole platform catalog (`BEX_AGENT_PLATFORM_MODELS`, the default first): one with no `agent_environments` row, one whose row turns no model off, and one whose admins turned every model back on.
   - A model added to the catalog is offered there from bex-api's next start. Devin's "Allow all models" does the same.
2. **An allowlist once any model is off.**
   - Once an admin's write leaves a catalog model off, the workspace offers the models that write left on.
   - A model added to the catalog afterwards is off there. The capabilities read's claude `models`, `bex agent models` and the pickers in the composer, the steering composer and the automation form leave it out.
   - A session create, a message's `model` or an automation create or update naming it is refused with `400 AGENT_SESSION_MODEL_NOT_OFFERED`. The message says the model is turned off in this workspace and that an admin can turn it on. `defaultModel` naming it is refused with `AGENT_ENVIRONMENT_MODEL_NOT_OFFERED`.
   - An admin turns it on through `enabledModels` as any other model, on REST, GraphQL, MCP, `bex agent env set --models` and Settings → Agents.
   - The catalog default is always on, as today.
3. **Shown as new.**
   - REST `GET /v1/agent-environment`, GraphQL `agentEnvironment` and MCP `get_agent_environment` name the catalog models that are off because the list predates them. They are listed apart from the ones an admin turned off. t001 chooses the field's name.
   - `bex agent env` prints them under "Models off" as "(new in the catalog)", beside "(turned off by an admin)".
   - Settings → Agents shows them switched off with a "New in the catalog" label, and the section's skeleton keeps the ready state's geometry.
4. **No workspace's offer changes on deploy.**
   - Every row narrowed under migration 0315's opt-out keeps exactly the models it offered before the migration.
   - A write by a previous-release bex-api Pod during the rollout leaves a row the new code reads the same way.
   - A model that later leaves the catalog drops out of the stored list as it does today.
5. **Said everywhere.**
   - ADR047 D7's w10/m527 amendment, ADR090 § Model pin ("A workspace narrows the catalog") and the ADR018 agent-model row state the two modes. They cite Devin's matching rule and drop "a model added to the catalog later starts on" and the w10/036 divergence.
   - The rule is also named in the MCP tool descriptions, the GraphQL field descriptions, `bex agent env set --models`'s help, the `BEX_AGENT_PLATFORM_MODELS` row of `lego/backend/AGENTS.md` and the new migration's comment.
6. **Tested.** Backend tests cover four cases:
   - A catalog model added after a narrowing write: the capabilities `models` leave it out, and a create naming it is refused with `AGENT_SESSION_MODEL_NOT_OFFERED`.
   - The same added model in a workspace that never narrowed its list is offered.
   - A narrowed workspace turning the added model on, and turning every model back on, which allows all again.
   - A legacy narrowed row converted with its offer unchanged.
7. **Live.**
   - On `dev-10`, a bex-api restarted with one more catalog model offers it to an un-narrowed workspace and not to a narrowed one, as the capabilities read and a refused create show.
   - On production, once the pin carries the milestone, `tian-personal` still offers the four catalog models. Every narrowed row, counted read-only with hashed ids, offers the same set it offered before the deploy.

## Source + Goal linkage

- **Source:** inbox note [w10/036](../done/036.md), filed by w10/m527 t007 (Devin comparison) on 2026-11-20 and promoted by `/loopx w10` on 2026-11-22.
- **Triage evidence (2026-11-22, `da137839b8`):**
  - **The premise holds.** `modelOfferPatch` (`lego/backend/internal/agentsessions/environment.go:585`) stores every catalog model a write leaves out as `disabled_models`. `leftOn` (`environment.go:853`), behind `offeredModels` (`:837`), offers every catalog model not in that column. A model appended to `BEX_AGENT_PLATFORM_MODELS` after a narrowing write is therefore on. Migration `0315_agent_environment_models.up.sql` says so in its comment.
  - **Devin's behavior is documented, so no Devin team account is needed.**
    - [List Models](https://docs.devin.ai/desktop/accounts/api-reference/list-models) (read 2026-11-22) describes the allowlist as "Model UIDs on your team's allowlist, as stored" and "Model tags on your team's allowlist, as stored, including provider tags such as `provider-anthropic`". Its rule is "A model is allowed when its UID is listed or it carries a listed tag".
    - A newly released model's UID is on no stored list, so under a by-model allowlist it stays off.
    - [Devin CLI Team Settings](https://docs.devin.ai/cli/enterprise/team-settings) (read 2026-11-22) offers that "Allowlist specific models" option ("Restrict users to a curated list of approved models") beside "Allow all models" ("Give users access to all available models").
    - Allowing by provider tag, the other half of the rule, is [w10/037](../037.md).
  - **Legacy rows convert exactly today.** Production's `BEX_AGENT_PLATFORM_MODELS` (`lego/operator/config/api/deployment.yaml:652`) last changed in `1988962c01` (w10/m516, 2026-11-19), before w10/m527 shipped the list (2026-11-20). Every row narrowed so far was written against today's catalog, so converting it with the catalog bex-api starts with keeps its offer. Re-check that history before shipping.
  - **Surfaces to touch:**
    - Backend: `Environment`/`EnvironmentInput` (`environment.go:73-186`), `environmentView` (`:675`) and `retiredTemplateModel` (`automation_scheduler.go:248`).
    - CLI: the "Models off" row (`lego/cli/internal/agent/config.go:610`).
    - Dashboard: `dashboard/src/features/agent-environment/components/enabled-models-section.tsx`.
  - **It is more than one sitting.** It touches a migration, the store and a startup conversion, three API surfaces, the CLI, the dashboard, three ADRs and two live checks.
- **Goal linkage:** cloud coding-agent sessions ([ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md), ADR008 pillar 5), where Devin is the reference product. An admin who narrows the list to control spend must not find the next expensive model on for every member.
- **Expected outcome:** a workspace that narrowed its list keeps exactly the models its admins chose when the catalog grows. A workspace that never narrowed it gets new models at once, as Devin's "Allow all models" does.
- **Why now:** the catalog is about to grow (w10/037 waits on this item), and every model added before this ships is silently on in every narrowed workspace. The milestone includes the Render parity task because it changes the REST, GraphQL, MCP, CLI and dashboard reads of the agent environment.
