# w10 · m573 — A workspace admin allows the platform catalog's models by provider, as Devin's allowlist filters by provider

**Worker:** worker10 **Goal:** the workspace model list gains Devin's second filter type. An admin allows whole providers (`enabledProviders: ["anthropic"]`, the provider being a model id's prefix before `/`) instead of naming models one by one, as the one filter type in force. Every model of an allowed provider is offered, one added to the catalog later included, and the catalog default still always is. REST, GraphQL, MCP, `bex agent env set` and Settings → Agents read and write it, and no workspace's offer changes when this ships. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Design: storage of the provider filter, the one-filter-type rule, the refusals, the read fields and the rolling deploy; ADR047, ADR090 and ADR018 amended | 45m | — |
| [t002](t002.md) | Backend: the migration and the store | 45m | t001 |
| [t003](t003.md) | Backend: the offered set and the write honor the provider filter on REST, GraphQL and MCP | 1h30m | t002 |
| [t004](t004.md) | CLI: `bex agent env set --providers` and `bex agent env` show the filter by provider | 45m | t003 |
| [t005](t005.md) | Dashboard: Settings → Agents chooses models by model or by provider | 1h15m | t003 |
| [t006](t006.md) | Live check: a catalog gaining a model of an allowed provider on `dev-10`, then production's offers unchanged by the deploy | 45m | t003, t004, t005 |
| [t007](t007.md) | Render parity (Devin) | 30m | t006 |
| [t008](t008.md) | Simplify | 20m | t007 |
| [t009](t009.md) | Test coverage | 45m | t007, t008 |
| [t010](t010.md) | Closeout | 10m | t009 |

Total: **10 tasks, about 7h30m**.

## Definition of done

1. **Allow by provider.**
   - An admin's write of `enabledProviders` (REST `PATCH /v1/agent-environment`, GraphQL `updateAgentEnvironment`, MCP `update_agent_environment`, `bex agent env set --providers`, Settings → Agents) makes the workspace offer every catalog model whose id starts with `<provider>/` for a listed provider, in catalog order, the catalog default always first and always on.
   - A model added to the catalog later is offered there from bex-api's next start when its provider is listed, and is off when it is not.
   - The capabilities read's claude `models`, `bex agent models` and the composer, steering composer and automation form pickers list exactly that set. A create, a message's `model` or an automation create or update naming a model outside it is refused with `400 AGENT_SESSION_MODEL_NOT_OFFERED`, and `defaultModel` with `AGENT_ENVIRONMENT_MODEL_NOT_OFFERED`.
2. **One filter type at a time, as Devin Desktop's.**
   - A write naming both `enabledModels` and `enabledProviders` is refused with a 400 that names both fields. t001 chooses the code (a new `AGENT_ENVIRONMENT_PROVIDERS_INVALID` or `AGENT_ENVIRONMENT_MODELS_INVALID` with `field`).
   - So is a provider no catalog model carries, with the catalog's providers in the params, and any `enabledProviders` off the platform model route.
   - A later `enabledModels` write replaces the provider filter with a by-model one (m568's two modes), and a later `enabledProviders` write replaces a by-model list. t001 decides how a provider filter returns to "allow all".
3. **Read everywhere.**
   - REST `GET /v1/agent-environment`, GraphQL `agentEnvironment` and MCP `get_agent_environment` read the filter in force and the catalog's providers. t001 names the fields.
   - `enabledModels` stays the effective offered list. `newModels` is `[]` under a provider filter, since a model of an unlisted provider is off by the admin's filter, not because it is new.
   - `bex agent env` prints the allowed providers. Settings → Agents shows the filter type with a switch per provider, and its skeleton keeps the ready state's geometry at desktop and narrow-mobile widths.
4. **No workspace's offer changes on deploy.**
   - Every existing row keeps exactly the models it offered before the migration.
   - A write by a previous-release bex-api Pod during the rollout leaves a row the new code reads the same way that Pod does, including an `enabledModels` write on a row that had a provider filter.
5. **Said everywhere.**
   - ADR047 D7's amendments, ADR090 § Model pin ("A workspace narrows the catalog") and the ADR018 agent-model row state the provider filter and cite Devin. ADR018 drops the w10/037 divergence, and ADR047:305 drops "filed by t007 if users would miss them" for the provider filter.
   - The rule is also named in the MCP tool descriptions, the GraphQL field descriptions, `bex agent env set`'s help, the `BEX_AGENT_PLATFORM_MODELS` row of `lego/backend/AGENTS.md` and the migration's comment.
6. **Tested.** Backend tests cover:
   - the read under a provider filter on all three surfaces;
   - the write's refusals (an unknown provider, both filter types at once, off the route);
   - a catalog model added later of an allowed provider being offered, and one of an unlisted provider refused at create;
   - switching between the filter types;
   - the rollout read rule for a previous-release write.
   CLI and dashboard tests cover their reads and writes, and the GraphQL clients (dashboard and `mobile/src/generated-graphql/index.ts`) are regenerated.
7. **Live.**
   - On `dev-10`, a bex-api restarted with one more catalog model of an allowed provider offers it to the provider-filtered workspace. A model of an unlisted provider is refused there at create, as the capabilities read and a refused create show.
   - On production, once the pin carries the milestone, `tian-personal` still offers the four catalog models. Every row, counted read-only with hashed ids, offers the set it offered before the deploy.

## Source + Goal linkage

- **Source:** inbox note [w10/037](../done/037.md), filed by w10/m527 t007 (Devin comparison) on 2026-11-20 and promoted by `/loopx w10` on 2026-11-22.
- **Triage evidence (2026-11-22, `0641b3b97e`):**
  - **The premise holds.** `EnvironmentInput.EnabledModels` (`lego/backend/internal/agentsessions/environment.go:190`) is the agent environment's only list field. `modelOfferPatch` (`:594`) refuses any entry that is not a catalog model id. `leftOn` (`:875`), behind `offeredModels` (`:851`), matches by model id alone. No `enabledProviders` exists in `lego/backend/internal/api/testdata/schema.graphql`, `mcp.go`, `lego/cli/internal/agent/config.go` or the dashboard.
  - **The note's conditions are met.**
    - w10/036 shipped first, as w10/m568 (`343ae3da8d` on main; its production check waits for the pin). A narrowed list is now an allowlist that leaves every later model off, so an admin can no longer narrow and still get a trusted provider's next model. That is the case the note named ("together with w10/036").
    - The catalog is unchanged at four models from two providers: the default `deepseek/deepseek-v4.1-flash-1017` and three `anthropic/…` (`lego/operator/config/api/deployment.yaml:650-653`, last changed in `1988962c01`). Today a filter allowing `anthropic` offers what "allow all" offers. It first differs at the next catalog model.
  - **Devin's behavior, re-read 2026-11-22.**
    - [Devin Desktop admin controls](https://docs.devin.ai/desktop/controls): "Allowlisted models … Filter by specific models or by provider (one filter type at a time)", and a model outside the allowlist is hidden from the relevant picker or settings page.
    - [List Models](https://docs.devin.ai/desktop/accounts/api-reference/list-models): `allowed_model_tags` are "Model tags on your team's allowlist, as stored, including provider tags such as `provider-anthropic`", `allowed_model_uids` is "Empty when the team allows models by tag only", and "A model is allowed when its UID is listed or it carries a listed tag".
  - **Parity-first applies.** The ADR018 agent-model row (`docs/ADR018-render-parity.md:440`) records "Devin Desktop also filters by provider (w10/037)" as a filed divergence with no user sign-off. ADR047:305's "filed by t007 if users would miss them" is the older hedge this milestone amends (AGENTS.md Rules, ADR008 § Render parity first).
  - **It is more than one sitting.** It touches a migration, the store, the offer logic, three API surfaces, the CLI, the dashboard, two GraphQL clients, three ADRs and a live check.
- **Goal linkage:** cloud coding-agent sessions ([ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md), ADR008 pillar 5), where Devin is the reference product.
- **Expected outcome:** an admin who trusts a provider allows it in one step, and that provider's next model is offered to members as soon as the catalog adds it, while every other provider's new model stays off.
- **Why now:** m568 made every narrowed list an allowlist, so the provider filter is the only way left to narrow and keep a trusted provider current. Shipping it before the catalog next grows means no admin has to turn the new model on by hand. The milestone includes the Render parity task because it changes the REST, GraphQL, MCP, CLI and dashboard reads and writes of the agent environment.
