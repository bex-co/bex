# w10 · m573 — A workspace admin allows the platform catalog's models by provider, as Devin's allowlist filters by provider

**Worker:** worker10 **Goal:** the workspace model list gains Devin's second filter type. An admin allows whole providers (`enabledProviders: ["anthropic"]`, the provider being a model id's prefix before `/`) instead of naming models one by one, as the one filter type in force. Every model of an allowed provider is offered, one added to the catalog later included, and the catalog default still always is. REST, GraphQL, MCP, `bex agent env set` and Settings → Agents read and write it, and no workspace's offer changes when this ships. **Status:** in progress (t001–t005 and t007–t009 done 2026-11-22, the `dev-10` half of t006 too; t006's production read and t010 follow the deploy)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Design: storage of the provider filter, the one-filter-type rule, the refusals, the read fields and the rolling deploy; ADR047, ADR090 and ADR018 amended — **DONE** | 45m | — |
| [t002](done/t002.md) | Backend: the migration and the store — **DONE** | 45m | t001 |
| [t003](done/t003.md) | Backend: the offered set and the write honor the provider filter on REST, GraphQL and MCP — **DONE** | 1h30m | t002 |
| [t004](done/t004.md) | CLI: `bex agent env set --providers` and `bex agent env` show the filter by provider — **DONE** | 45m | t003 |
| [t005](done/t005.md) | Dashboard: Settings → Agents chooses models by model or by provider — **DONE** | 1h15m | t003 |
| [t006](t006.md) | Live check: a catalog gaining a model of an allowed provider on `dev-10`, then production's offers unchanged by the deploy | 45m | t003, t004, t005 |
| [t007](done/t007.md) | Render parity (Devin) — **DONE** | 30m | t006 |
| [t008](done/t008.md) | Simplify — **DONE** | 20m | t007 |
| [t009](done/t009.md) | Test coverage — **DONE** | 45m | t007, t008 |
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

## Design record (t001, 2026-11-22)

Against [Devin Desktop admin controls](https://docs.devin.ai/desktop/controls) ("Filter by specific models or by provider (one filter type at a time)"; a model outside the allowlist is hidden "from the relevant picker or settings page") and [List Models](https://docs.devin.ai/desktop/accounts/api-reference/list-models) (`allowed_model_tags` "including provider tags such as `provider-anthropic`"; `allowed_model_uids` "Empty when the team allows models by tag only"; "A model is allowed when its UID is listed or it carries a listed tag"), both re-read 2026-11-22, and m568's read rule ([design record](../blocked/m568/README.md#design-record-t001-2026-11-22)).

- **A model's provider** is its catalog id's prefix before the first `/` (`anthropic/claude-opus-5.5` → `anthropic`), Devin's `provider-anthropic` tag. An id with no `/` has no provider, so no provider filter allows it unless it is the catalog default.
- **Storage.** Migration 0333 adds a nullable `agent_environments.enabled_providers text[]` (at most 64). NULL filters by model, m568's two modes. A provider write stores three columns together:
  - `enabled_providers`: the listed providers, folded and in catalog order;
  - `known_models`: the catalog it saw, never NULL;
  - `disabled_models`: every catalog model but the default whose provider is not listed.
  - So a previous-release Pod offers the same set under the same catalog. m568 reads `known_models − disabled_models`, or every model when `disabled_models` is empty. The code before m568 reads `catalog − disabled_models`.
  - An `enabledModels` write stores `enabled_providers` NULL, beside m568's `disabled_models` and `known_models`.
- **The read rule (the rollout guard).** The provider filter is in force only while `enabled_providers` and `known_models` are not NULL and `disabled_models` is exactly the set a provider write derives from those two. Any other row reads by m568's rule and ignores the column. Under the filter the workspace offers the catalog default plus every catalog model of a listed provider, in catalog order, a later one included, and `newModels` is `[]`.
  - A previous-release Pod never writes `enabled_providers`.
  - Its `enabledModels` write on a provider row changes `disabled_models`, and m568's Pod also changes `known_models`. The row then reads by m568's rule, which is how that Pod reads it. An m568 all-on write stores `known_models` NULL, which allows all.
  - One exception: a write whose set is exactly the filter's keeps the filter. Under the same catalog that is the same offer.
  - A previous-release write of any other field leaves the filter in force.
  - The hourly legacy conversion (`known_models IS NULL`) never touches a provider row.
  - This beats a write stamp such as `updated_at`. A stamp would drop the filter on a previous-release write of an unrelated field, and it would need every new write to maintain it.
- **The write.**
  - `enabledProviders` replaces the filter with a by-provider one, and `enabledModels` replaces it with a by-model one.
  - Both in one write are refused. Refusals reuse `400 AGENT_ENVIRONMENT_MODELS_INVALID` (it is one setting, and the dashboard and CLI already word it), with `field: enabledProviders`:
    - both filter types at once also carry `fields: [enabledModels, enabledProviders]`;
    - a provider no catalog model carries also carries `provider` and `providers`, the catalog's providers;
    - an empty list also carries `providers`;
    - a write off the platform route or on the workspace's own key carries `field` alone.
  - `[]` is refused rather than read as "only the default". Devin's empty `allowed_model_uids` means "not this filter type", so an empty list would read two ways. `enabledModels: [<default>]` offers only the default.
  - **Back to "allow all"** is m568's: `enabledModels` naming every catalog model. A filter listing every provider stays a provider filter, and a model of a provider the catalog gains later is off, as Devin's tag rule reads (that model carries no listed tag).
  - `defaultModel` must be in the offered set after the write, as today.
  - No reset flag: an omitted or null `enabledProviders` keeps the filter, on GraphQL too.
- **The read fields** (REST `GET /v1/agent-environment`, GraphQL `AgentEnvironment`, MCP `get_agent_environment`):
  - `enabledProviders` (`[String!]`, nullable): the providers listed while the filter is by provider. It is null while the filter is by model, on the workspace's own key and off the route.
  - `providers` (`[String!]!`): the catalog's providers in catalog order, `[]` wherever `models` is.
  - `enabledModels` stays the effective offered list, and `newModels` is `[]` under a provider filter.
  - The input field is `enabledProviders` on REST, `AgentEnvironmentInput.enabledProviders` on GraphQL and MCP `update_agent_environment`.
- **Unchanged:** `offeredModels` stays the one helper. Create, message, dispatch, automation save and fire, the capabilities `models` and `retiredTemplateModel`'s "turned off in this workspace" note follow the provider filter through it. A model of an unlisted provider is refused with `AGENT_SESSION_MODEL_NOT_OFFERED` as any turned-off model.
- **The CLI and Settings.**
  - `bex agent env set --providers anthropic[,…]` writes the filter, and `--models` with `--providers` is a usage error.
  - `bex agent env` prints a "Providers" row under a provider filter, and its "Models off" says "(provider not allowed)".
  - Settings → Agents gains a "By model" / "By provider" choice above the switches. Switching to "By provider" lists the providers of the models on now. Switching back writes the models on now as `enabledModels`. Each provider switch shows that provider's models, the default stays on, and the last allowed provider is locked on.
- **ADRs amended:** ADR047 D7 (a "Filter by provider" bullet, the storage, and the `Scope.` line), ADR090 § Model pin ("A workspace narrows the catalog") and the ADR018 agent-model row, which drops the w10/037 divergence.

## Live check (t006)

**`dev-10`, 2026-11-22 (DoD 7, first half — holds).**

- **Setup.** bex-api was built from the tree with the m573 changes, on production's catalog: `BEX_AGENT_PLATFORM_MODEL_ROUTE=openrouter`, a placeholder key, and `BEX_AGENT_PLATFORM_MODELS=anthropic/claude-haiku-5.5,anthropic/claude-sonnet-5.5,anthropic/claude-opus-5.5`.
- **Stand-ins.** The agent-session leg on the shared cluster is held by `dev-9`, so this run did what m568 t006 did:
  - enforced authz on a dedicated in-memory OpenFGA, seeded by `scripts/authz-model.sh` and passed as `DEV_OPENFGA_URL`;
  - unreachable OpenSandbox, model-proxy and gateway URLs, which is enough for the capabilities read and a create's model check.
- **Fixtures.** Two Kratos users (`dev-10.local`, emails verified through the Kratos admin API) own workspace A `tea-dc1q2ahjg4r6qhojvc70` (disposable) and workspace B `tea-dc1q2ahjg4r6qhojvc7g` (disposable).

1. A wrote `enabledProviders: ["anthropic"]` (`PATCH /v1/agent-environment`). Its REST read gave:
   - `providers: ["deepseek","anthropic"]` and `enabledProviders: ["anthropic"]`;
   - all four models as `enabledModels`, and `newModels: []`.
   B turned opus off by model and read `enabledProviders: null`. A's three refused writes each answered `400 AGENT_ENVIRONMENT_MODELS_INVALID` with `field: enabledProviders`:
   - both filter types at once, with `fields` naming both;
   - `openai`, with `providers` listing the catalog's;
   - `[]`.
2. bex-api was restarted (`bash scripts/dev-env.sh 10 up`) with `anthropic/claude-fable-5.5` and `moonshotai/kimi-k3` appended to the catalog.
   - **A** read six `models` and `providers: ["deepseek","anthropic","moonshotai"]`. Its `enabledModels` and the capabilities read's claude `models` were the default plus the four anthropic models, fable included, without kimi, and `newModels: []`.
   - `POST /v1/agent-sessions` naming kimi answered `400 AGENT_SESSION_MODEL_NOT_OFFERED`: "moonshotai/kimi-k3 is turned off in this workspace, whose members may choose … anthropic/claude-fable-5.5; a workspace admin can turn it on in Settings → Agents".
   - A create naming fable was accepted (`201`, model `anthropic/claude-fable-5.5`), and the session was deleted at once (`204`).
   - **B**, the allowlist by model, read fable and kimi as `newModels` and offered neither.
3. **The CLI.** `bex agent env` (the CLI built from the tree, `BEX_HOST` + `BEX_API_KEY` from a key minted in A) printed:
   - `Providers  anthropic (members may choose every model of these providers, a later one included)`;
   - `Models off  moonshotai/kimi-k3 (provider not allowed)`.
   `bex agent env set --providers anthropic --models …` was a usage error. `--providers anthropic,moonshotai` offered kimi too, and `--providers anthropic` restored the filter.
4. **Settings → Agents.** The dashboard from the tree, signed in as A, ran at 1280 px and 390 px:
   - "By provider" was selected, with one switch per provider listing its models. The default read "(the default, always on)", and anthropic, the one provider allowed, was locked with its note.
   - Turning moonshotai on saved it. "By model" then saved the six models as the list, which allows all.
   - With the environment query held, the skeleton's heading, filter choice, help line and first row matched the ready state's sizes at both widths. That run moved the skeleton's help line to the by-model one, because the longer by-provider help ran a line taller.
   - The skeleton's four rows hold the longest model id. At 390 px they wrap to two lines where shorter ids take one. That is m568's placeholder, unchanged here.
5. **Torn down** on 2026-11-22: `bash scripts/dev-env.sh 10 down`, then the OpenFGA container was removed.

**Production before the deploy, 2026-11-22 (read-only, `hetzner-prod`, schema at migration 0326, so m568 is not deployed yet either).**

- `agent_environments` has 3 rows. None turns a model off and none is on its own key, so every workspace offers the whole catalog of four.
- Reproduce with `kubectl --context hetzner-prod -n bex-system exec bex-db-2 -c postgres -- psql -U postgres -d bex -At -c "SET default_transaction_read_only = on; SELECT count(*), count(*) FILTER (WHERE cardinality(disabled_models) > 0), count(*) FILTER (WHERE model_key_set_at IS NOT NULL) FROM agent_environments"`, which answered `3|0|0`. Use the primary: `bex-db-2` was the primary that day, so check `pg_is_in_recovery()` first.

**Production after the deploy:** pending.

- Once the pin carries the milestone, run the same read plus `count(*) FILTER (WHERE enabled_providers IS NOT NULL)`.
- Every row must offer what it did, and the filter count is 0 unless an admin set one since.
- `tian-personal`'s `GET /v1/agent-environment` must read the four catalog models as `enabledModels`, `providers: ["deepseek","anthropic"]` and `enabledProviders: null`.

## Render parity (t007, Devin, 2026-11-22)

Devin's pages were re-read on 2026-11-22:

- [Devin Desktop admin controls](https://docs.devin.ai/desktop/controls): "Filter by specific models or by provider (one filter type at a time)", and a model outside the allowlist is hidden "from the relevant picker or settings page".
- [List Models](https://docs.devin.ai/desktop/accounts/api-reference/list-models): `allowed_model_tags` "including provider tags such as `provider-anthropic`"; `allowed_model_uids` "Empty when the team allows models by tag only"; "A model is allowed when its UID is listed or it carries a listed tag".

| Surface | bex after m573 | Devin |
| --- | --- | --- |
| REST `GET /v1/agent-environment`, GraphQL `agentEnvironment`, MCP `get_agent_environment` | `providers` (the catalog's), `enabledProviders` (the filter in force, null by model), `enabledModels` the effective list, `newModels: []` under the filter (`TestAProviderFilterOffersEveryModelOfAListedProvider`, all three surfaces; `dev-10` REST) | `allowed_model_tags` with provider tags beside `allowed_model_uids` ✅ |
| REST `PATCH`, GraphQL `updateAgentEnvironment`, MCP `update_agent_environment`, `bex agent env set --providers` | `enabledProviders` replaces a by-model list, `enabledModels` replaces the filter; both at once, an unknown provider and `[]` are `AGENT_ENVIRONMENT_MODELS_INVALID` (`TestEnabledProvidersRefusalsOnEverySurface`, `TestSwitchingBetweenTheFilterTypes`; `dev-10` REST and CLI) | "one filter type at a time" ✅; an empty list is refused, where Devin's empty list means "not this filter type" (no observable divergence: bex has no other reading of it) |
| Capabilities `models` (REST, GraphQL, MCP), `bex agent models`, the composer, steering composer and automation form pickers | every model of a listed provider, a later one included; another provider's model is left out (the pickers read the capabilities `models`) | "A model is allowed when … it carries a listed tag"; outside the allowlist is hidden from pickers ✅ |
| Create, message, automation create and update, `defaultModel` | a model of an unlisted provider is `AGENT_SESSION_MODEL_NOT_OFFERED` / `AGENT_ENVIRONMENT_MODEL_NOT_OFFERED`; a later model of a listed one is accepted (`dev-10`: kimi refused, fable `201`) | not allowed ✅ |
| `bex agent env`, Settings → Agents | "Providers" row and "(provider not allowed)"; "By model" / "By provider" with a switch per provider listing its models, the default always on | Devin's admin page filters by model or provider ✅ |

No drift beyond w10/046 (an explicit allowlist that lists every current model), filed by m568. A filter listing every provider leaves a new provider's model off, as Devin's tag rule reads.

## Simplify (t008, 2026-11-22)

`/simplify` over the backend, store, CLI and dashboard changes.

- **Taken:** gofmt's alignment, and one refusal off the platform route shared by both list fields instead of a second copy. On the dashboard, the filter choices moved to `lib/environment.ts` (`MODEL_FILTERS`, beside `TIERS`), so the skeleton file exports components only.
- **Not taken:** `providerFilter` runs twice per environment read (once in `leftOn`, once for the read field), over a catalog of at most 64 models, which costs nothing. `modelProvider` exists in Go and TypeScript, which is inherent to the two clients.

## Test coverage (t009, 2026-11-22)

- **DoD 6, the read under a provider filter on all three surfaces:** `TestAProviderFilterOffersEveryModelOfAListedProvider` in `lego/backend/internal/agentsessions/model_providers_test.go`. It covers:
  - writes on REST, GraphQL and MCP, read back on all three;
  - the capabilities `models`;
  - a provider that leaves the catalog dropping out of `enabledProviders`, so a write built from the read is accepted.
- **The write's refusals:** `TestEnabledProvidersRefusalsOnEverySurface` covers an unknown provider, `[]`, both filter types, the workspace's own key and no route. Each case runs on the service, REST, GraphQL and MCP, and nothing is written.
- **A catalog model added later:** `TestAProviderFilterOffersEveryModelOfAListedProvider` again.
  - A model of a listed provider is created on.
  - A model of an unlisted provider is refused at create, on a message, at automation create and as `defaultModel`.
- **Switching between the filter types:** `TestSwitchingBetweenTheFilterTypes`. It covers:
  - by model → by provider → by model;
  - `defaultModel` checked against the filter in the same write;
  - every provider listed leaving a new provider's model off;
  - every model on allowing all again.
- **The rollout read rule:** `TestTheProviderFilterRolloutReadRule` checks three things:
  - a provider write reads the same under the new rule, m568's and the opt-out;
  - six previous-release writes on a provider row read as that release reads them;
  - against a grown catalog, the filter offers the next anthropic model while m568's narrowing leaves it new.
- **The store:** `TestAgentEnvironmentEnabledProviders` (real Postgres) covers the round trip, NULL by default and after a by-model write, a write naming no model field keeping it, a previous-release write leaving it in place, and the cap of 64.
- **CLI:** `TestEnvSet` (`--providers`, the two usage errors), `TestEnvShowsTheEnabledModels` (the "Providers" row, "(provider not allowed)", no row by model) and the contract fixture's `enabledProviders`.
- **Dashboard:**
  - `lib/__tests__/environment.test.ts`: `modelProvider`, `modelFilterOf`, `withProviderToggled`, `providersOfEnabledModels`, and the refusal key for `field: enabledProviders`.
  - `agent-environment-card.test.tsx` "by provider": switching each way, the provider switches and their notes, the last provider locked, the refusal message and a member's read-only view.
  - The skeleton test also compares the filter choice and help line.
- **GraphQL clients:** the dashboard's `schema-types.ts` and `definitions.ts` were regenerated. `mobile/src/generated-graphql/index.ts` was regenerated with no change, because mobile selects no agent-environment field.
- **Proved real by reverting:**
  - With `leftOn` ignoring the filter (`byProvider = false`), `TestAProviderFilterOffersEveryModelOfAListedProvider`, `TestSwitchingBetweenTheFilterTypes` and `TestTheProviderFilterRolloutReadRule` fail.
  - With `providerFilter` skipping the turned-off-list check (the rollout guard), `TestTheProviderFilterRolloutReadRule` fails on the three previous-release writes that change the list.

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
