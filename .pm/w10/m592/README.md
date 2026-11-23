# w10 · m592 — A workspace admin chooses "Allow all models" or an allowlist, so an allowlist listing every current catalog model still leaves the next one off, as Devin's modes do

**Worker:** worker10 **Goal:** the workspace model list gets Devin's two modes as an explicit setting, not one that follows the list. An admin who turns "Allow all models" off keeps an allowlist even while it lists every current catalog model, so a model added to the catalog later is off there and reads as new. With it on, the workspace offers the whole catalog, a later model included. REST, GraphQL, MCP, `bex agent env set --models all|ID,…` and Settings → Agents read and write it, and no workspace's offer changes when this ships. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Design: the mode's name and storage, what a write naming only `enabledModels` or `enabledProviders` does, its read off the route, the refusals and the rolling deploy; ADR047, ADR090 and ADR018 amended | 45m | — |
| [t002](t002.md) | Backend: the mode on REST, GraphQL and MCP, and the offered set honoring it | 1h30m | t001 |
| [t003](t003.md) | CLI: `bex agent env set --models all` and `bex agent env` show the mode | 45m | t002 |
| [t004](t004.md) | Dashboard: Settings → Agents offers "All models" beside "By model" and "By provider" | 1h15m | t002 |
| [t005](t005.md) | Live check: a catalog gaining a model on `dev-10` in an allowlist that lists every model, then production's offers unchanged by the deploy | 45m | t002, t003, t004 |
| [t006](t006.md) | Render parity (Devin) | 30m | t005 |
| [t007](t007.md) | Simplify | 20m | t006 |
| [t008](t008.md) | Test coverage | 45m | t006, t007 |
| [t009](t009.md) | Closeout | 10m | t008 |

Total: **9 tasks, about 6h45m**.

## Definition of done

1. **An explicit mode on every surface.**
   - REST `GET`/`PATCH /v1/agent-environment`, GraphQL `agentEnvironment`/`updateAgentEnvironment` and MCP `get_`/`update_agent_environment` read and write whether the workspace allows every catalog model. t001 names the field; the source note proposes `allowAllModels`.
   - `bex agent env set --models all` turns it on, and `bex agent env` prints the mode.
   - Settings → Agents offers "All models" beside "By model" and "By provider", and the section's skeleton keeps the ready state's geometry at 1280 px and 390 px.
2. **An allowlist that lists every model stays an allowlist.**
   - With the mode off and every catalog model listed, a bex-api restarted with one more catalog model leaves it off there.
   - It is absent from `enabledModels`, the capabilities read's claude `models`, `bex agent models` and the pickers, and it is listed in `newModels`.
   - A session create, a message's `model` or an automation save naming it is refused with `400 AGENT_SESSION_MODEL_NOT_OFFERED`, and `defaultModel` with `AGENT_ENVIRONMENT_MODEL_NOT_OFFERED`.
3. **"Allow all models" allows all.**
   - With the mode on, `enabledModels` is the whole catalog, `newModels` is `[]`, and a model added to the catalog later is offered from bex-api's next start.
   - Turning it on from an allowlist or from a filter by provider allows all. Turning it off from "allow all" leaves the workspace an allowlist of the models on now, so its offer does not change until the catalog does.
   - A write naming only `enabledModels` or `enabledProviders` does what t001 records, and every surface's description and `--models` help say it.
4. **No workspace's offer changes on deploy.**
   - Every existing row reads the mode and the offer it had before the deploy.
   - A write by a previous-release bex-api Pod during the rollout leaves a row the new code reads as that Pod does while the catalog is unchanged.
   - The deploy that first ships this adds no catalog model.
5. **Said everywhere.**
   - ADR047 D7 replaces "No explicit mode", and ADR090 § Model pin ("A workspace narrows the catalog") states the mode.
   - The ADR018 agent-model row drops "Divergence filed: Devin can keep an allowlist that lists every current model, which bex reads as 'allow all' (w10/046)".
   - The mode is also named in the MCP tool descriptions, the GraphQL field descriptions, `bex agent env set --models`'s help and the `BEX_AGENT_PLATFORM_MODELS` row of `lego/backend/AGENTS.md`, which drops "Every model back on allows all again".
6. **Tested.**
   - Backend tests cover:
     - both modes on all three surfaces;
     - the writes that switch between them, from a filter by provider too;
     - a catalog model added under each mode;
     - the refusals (off the route, on the workspace's own key, and any combination t001 refuses);
     - the rollout read rule.
   - CLI and dashboard tests cover their reads and writes.
   - The GraphQL clients are regenerated: the dashboard's and `mobile/src/generated-graphql/index.ts`.
7. **Live.**
   - On `dev-10`, a bex-api restarted with one more catalog model offers it to an "allow all" workspace. In a workspace whose allowlist lists every earlier model it is off, read in `newModels`, and refused at create.
   - On production, once the pin carries the milestone, `tian-personal` still offers the four catalog models. Every row, counted read-only with hashed ids, offers the set it offered before the deploy.

## Source + Goal linkage

- **Source:** inbox note [w10/046](../done/046.md), filed by w10/m568 t001 (design) on 2026-11-22 and promoted by `/loopx w10` on 2026-11-23.
- **Triage evidence (2026-11-23, `23b3c97413`):**
  - **The premise holds.**
    - `modelOfferPatch` (`lego/backend/internal/agentsessions/environment.go:636`) stores `known_models` NULL whenever an `enabledModels` write leaves no catalog model off (`:679-682`).
    - `leftOn` (`:971`) reads a row with `known_models` NULL, or with `disabled_models` empty, as "allow all" (`:973`).
    - So no write can keep an allowlist that lists every current model. The next catalog model is on in such a workspace.
  - **Devin's behavior.**
    - [Devin CLI Team Settings](https://docs.devin.ai/cli/enterprise/team-settings) (re-read 2026-11-23) offers "Allow all models" ("Give users access to all available models") beside "Allowlist specific models" ("Restrict users to a curated list of approved models") as two modes.
    - [List Models](https://docs.devin.ai/desktop/accounts/api-reference/list-models) (re-read 2026-11-23) stores the allowlist as `allowed_model_uids` ("Model UIDs on your team's allowlist, as stored") and `allowed_model_tags`, with the rule "A model is allowed when its UID is listed or it carries a listed tag".
    - A model released after an admin listed every current UID is therefore on no list and is not allowed. Its API has no explicit mode flag, so bex's field is its own wire name for the settings page's mode.
  - **The note's "ship only if admins would miss it".**
    - Production's catalog has not grown since `1988962c01` (2026-11-19): `deepseek/deepseek-v4.1-flash-1017` then three `anthropic/claude-*-5.5` models, read from the live `bex-api` Deployment on pin `ac882b5f557f`.
    - Its `agent_environments` has 3 rows, all bex's own fixtures. None is narrowed, has `known_models` set or filters by provider, and none is on its own key (read-only on both `bex-db` Pods, 2026-11-23). So no admin misses the mode today.
    - It ships anyway. ADR018's agent-model row records it only as "Divergence filed", without the user's sign-off, and a Devin divergence stands only with that sign-off ([ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first); `/loopx` worker rules).
    - The catalog grows with every model release worth offering. An admin who approved today's models to control spend or data handling would otherwise find the next model on for every member.
  - **Storage may need no migration.** t001 decides.
    - The row `known_models` = catalog with `disabled_models` `{}` can carry "an allowlist that lists every model" once `leftOn`'s pre-m568 clause (`len(env.DisabledModels) == 0`, `:973`) is retired.
    - That clause read a pre-m568 Pod's all-on write. Production's Pods run m568 and m573 (pin `ac882b5f557f`), whose all-on write stores `known_models` NULL, and 0 rows have `known_models` set.
    - A previous-release Pod reads such a row as "allow all": the same offer while the catalog is unchanged.
  - **Surfaces to touch:**
    - Backend: `Environment` (`environment.go:73-110`), `EnvironmentInput` (`:174`, its model fields `:199-206`), `modelOfferPatch`, `environmentView` (`:776`, the model read `:791-806`), `leftOn` and `providerFilter` (`:1034`). GraphQL `graphql.go:191-197`, `:283`, `:337`, and the MCP descriptions in `mcp.go:288`, `:293`.
    - CLI: `lego/cli/internal/agent/config.go:384-393` (the read) and `:464-477`, `:549-563` (`env set --models`/`--providers`: their help and handling). The REST PATCH decodes strictly (`rest.go:113`), so a CLI sending the new field to an older bex-api is refused.
    - Dashboard: `enabled-models-section.tsx`, `MODEL_FILTERS` in `lib/environment.ts:42`, `agent-environment-card-skeleton.tsx:377-430`, `api/agent-environment.graphql`, the eight locales and `test/mocks/agent-environment.ts`. The mobile app reads no agent environment, but its generated types are regenerated all the same.
  - **It is more than one sitting.** It touches three API surfaces, the CLI, Settings → Agents and its skeleton, three ADRs and two live checks.
- **Goal linkage:** cloud coding-agent sessions ([ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md), ADR008 pillar 5), where Devin is the reference product. It completes the m568 ([design record](../done/m568/README.md#design-record-t001-2026-11-22)) and m573 model lists.
- **Expected outcome:** an admin who keeps an allowlist keeps exactly the models it lists when the catalog grows, even when it lists all of today's. An admin who wants every model, the next ones included, says so with "Allow all models".
- **Why now:** w10's model-list line is otherwise complete (m516, m527, m568, m573), and this is its last recorded Devin divergence. It must land before the catalog's next model: every model added before then is on in a workspace whose admins listed every model to keep the list closed. The milestone includes the Render parity task because it changes the REST, GraphQL, MCP, CLI and dashboard reads and writes of the agent environment.
