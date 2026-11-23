# w10 · m592 — A workspace admin chooses "Allow all models" or an allowlist, so an allowlist listing every current catalog model still leaves the next one off, as Devin's modes do

**Worker:** worker10 **Goal:** the workspace model list gets Devin's two modes as an explicit setting, not one that follows the list. An admin who turns "Allow all models" off keeps an allowlist even while it lists every current catalog model, so a model added to the catalog later is off there and reads as new. With it on, the workspace offers the whole catalog, a later model included. REST, GraphQL, MCP, `bex agent env set --models all|ID,…` and Settings → Agents read and write it, and no workspace's offer changes when this ships. **Status:** done (2026-11-23: t001–t009 done; production pin `9fdf7e0a1171` carries `be26a7c6b2`, and t005's production read held)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Design: the mode's name and storage, what a write naming only `enabledModels` or `enabledProviders` does, its read off the route, the refusals and the rolling deploy; ADR047, ADR090 and ADR018 amended — **DONE** | 45m | — |
| [t002](done/t002.md) | Backend: the mode on REST, GraphQL and MCP, and the offered set honoring it — **DONE** | 1h30m | t001 |
| [t003](done/t003.md) | CLI: `bex agent env set --models all` and `bex agent env` show the mode — **DONE** | 45m | t002 |
| [t004](done/t004.md) | Dashboard: Settings → Agents offers "All models" beside "By model" and "By provider" — **DONE** | 1h15m | t002 |
| [t005](done/t005.md) | Live check: a catalog gaining a model on `dev-10` in an allowlist that lists every model, then production's offers unchanged by the deploy — **DONE** | 45m | t002, t003, t004 |
| [t006](done/t006.md) | Render parity (Devin) — **DONE** | 30m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 20m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 45m | t006, t007 |
| [t009](done/t009.md) | Closeout — **DONE** | 10m | t008 |

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

## Design record (t001, 2026-11-23)

Against [Devin CLI Team Settings](https://docs.devin.ai/cli/enterprise/team-settings) (two modes: "Allowlist specific models", "Restrict users to a curated list of approved models", and "Allow all models", "Give users access to all available models") and [List Models](https://docs.devin.ai/desktop/accounts/api-reference/list-models) (`allowed_model_uids` "as stored", "A model is allowed when its UID is listed or it carries a listed tag", and no allow-all field in the API), both re-read 2026-11-23. The m568 and m573 design records ([m568](../../done/m568/README.md#design-record-t001-2026-11-22), [m573](../../done/m573/README.md#design-record-t001-2026-11-22)) are the base.

- **The field.** `allowAllModels`: `Boolean!` on the read (REST `GET /v1/agent-environment`, GraphQL `AgentEnvironment.allowAllModels`, MCP `get_agent_environment`) and `Boolean` on the write (REST `PATCH`, GraphQL `AgentEnvironmentInput.allowAllModels`, MCP `update_agent_environment`). Devin's API has no mode flag, so the name is bex's wire name for the settings page's mode.
  - On the platform route it is `true` while the workspace offers the whole catalog, a model added later included, and `false` under an allowlist or a filter by provider.
  - Off the route and on the workspace's own key it reads `true`: no catalog narrows there and any model id is accepted, the way `enabledModels` reads `[]` and `enabledProviders` null. A list stored before the key was set is kept and applies again once the key is cleared, as today.
- **Storage: no migration.** The existing columns already carry the mode.
  - "Allow all" is `known_models` NULL with `disabled_models` `{}` (and so no filter by provider, which needs `known_models`).
  - An allowlist is `known_models` set, the catalog the write saw, with the filter by provider not in force. Its `disabled_models` may now be `{}`: an allowlist that lists every model it saw, which leaves a later model off.
  - `leftOn`'s pre-m568 clause (`len(env.DisabledModels) == 0` reads "allow all") is retired. It read a pre-m568 Pod's all-on write. Every production Pod has run m568 and m573 since pin `ac882b5f557f`, whose all-on write stores `known_models` NULL, and 0 production rows have `known_models` set (2026-11-23), so no row reads differently.
  - A row with `known_models` NULL and `disabled_models` not empty (narrowed under 0315, not yet converted) still reads as the opt-out and its mode reads `false`. `ConvertLegacyModelLists` (`known_models IS NULL AND cardinality(disabled_models) > 0`) never touches an "allow all" row or an allowlist.
  - A new `allow_all_models` column lost: a previous-release Pod never writes it, so its narrowing write on an "allow all" row would leave the column saying "allow all" over a narrowed list, and a read rule guarding that would come back to `known_models` NULL.
- **The writes.** `allowAllModels` changes the mode. `enabledModels` and `enabledProviders` edit the list and keep the mode, as Devin's list is edited without changing the mode. The one exception is that "Allow all models" turns no model off: a list that leaves a model off, or any filter by provider, on a workspace that allows all makes it an allowlist (m568 and m573 already did this, and clients written against them keep working).
  - `allowAllModels: true` alone allows all from an allowlist, a filter by provider or an unconverted legacy row: `known_models` NULL, `disabled_models` `{}`, `enabled_providers` NULL.
  - `allowAllModels: true` with `enabledModels` or `enabledProviders` is refused: `400 AGENT_ENVIRONMENT_MODELS_INVALID`, `field: allowAllModels`, `fields: [allowAllModels, <the list>]`. Under Devin's "Allow all models" there is no list to send.
  - `allowAllModels: false` alone on a workspace that allows all keeps the models on now as the allowlist: `known_models` = the catalog, `disabled_models` `{}`. Its offer does not change until the catalog does. On an allowlist or a filter by provider it changes nothing, and the filter is kept.
  - `allowAllModels: false` with `enabledModels` stores that allowlist even when it lists every catalog model. With `enabledProviders` it stores the filter, as m573.
  - **`enabledModels` alone keeps the mode.** On an allowlist or a filter by provider it stores that allowlist, even when it lists every catalog model, so "turning every model back on" no longer returns to "allow all" (m568's rule). On a workspace that allows all, listing every catalog model changes nothing, and leaving any off makes it an allowlist. This fails closed for an API or MCP client written against m568's description: its every-model write keeps today's offer and leaves the next model off instead of turning it on, and every description now says to send `allowAllModels: true`. A client that reads the environment and writes `enabledModels` back unchanged never flips the mode.
  - `enabledProviders` alone is a filter, as m573, and turns "allow all" off. A filter listing every provider stays a filter.
  - **Refusals.** Both `allowAllModels` values are refused off the route and on the workspace's own key, as `enabledModels` is (`AGENT_ENVIRONMENT_MODELS_INVALID`, `field: allowAllModels`). The order: both lists (m573's refusal), then `true` with a list, then off the route naming the first field the write names (`enabledModels`, `enabledProviders`, `allowAllModels`). `defaultModel` must be offered after the write, as today; under "allow all" every catalog model is.
- **The CLI.**
  - `bex agent env set --models all` sends `{"allowAllModels": true}`. `all` takes no other id (`--models all,…` is a usage error), and `--models` stays exclusive with `--providers`.
  - `--models ID,…` sends `allowAllModels: false` with `enabledModels`: an explicit allowlist, even naming every catalog model, as Devin's "Allowlist specific models" reads. A bex-api older than this milestone answers `400 json: unknown field "allowAllModels"`, as an older one answers `--providers`. The CLI release follows the server's deploy.
  - `--providers` is unchanged.
  - `bex agent env` prints a "Models allowed" row above "Models": `all (a model added to the catalog later included)`, `allowlist (a model added to the catalog later stays off until an admin turns it on)` or `by provider (…)`. A bex-api that predates the field sends none, and the row is left out.
- **Settings → Agents.**
  - "Models members can choose" offers three options: "All models", "By model" and "By provider". "All models" comes first, because it is every workspace's default.
  - Under "All models" every catalog model's switch is on and locked, the default keeps its note, and the help says a model added to the catalog later is on too.
  - "By model" chosen from "All models" sends `allowAllModels: false`: every switch stays on, as an allowlist. "By provider" chosen from it sends the providers of the models on now (every provider), as m573 does from "By model". "All models" chosen from either sends `allowAllModels: true`.
  - Under "By model", every switch on stays an allowlist, so a later model starts off with its "New in the catalog" label. The skeleton draws three options at the ready state's geometry.
- **The rollout.** During the deploy the previous-release Pod runs m573's code. It reads `allowAllModels` nowhere and refuses it in a REST `PATCH` (strict decoding) or a GraphQL input. Settings and the CLI therefore see that field refused only by a previous-release Pod, for the length of the roll.
  - **A new write read by that Pod.** "Allow all" stores `known_models` NULL, which it reads as "allow all". An allowlist listing every model (`known_models` set, `disabled_models` `{}`) it reads as "allow all" through the retired clause, which is the same offer while the catalog is unchanged. Every other row is m573's.
  - **That Pod's write read by the new code.** Its all-on write stores `known_models` NULL ("allow all", as it reads it). Its narrowing and provider writes are m573's. A write of another field leaves the model columns, so a new allowlist listing every model stays one, which is the same offer as that Pod's read while the catalog is unchanged.
  - **Hence the deploy that first ships this must add no catalog model,** so the two reads never differ. Production's catalog last changed in `1988962c01` (2026-11-19), and this milestone does not touch `BEX_AGENT_PLATFORM_MODELS`.
- **Unchanged:** `offeredModels` stays the one helper. Create, message, dispatch, automation save and fire, the capabilities `models` and `retiredTemplateModel` follow the mode through `leftOn`. A model off under an allowlist that lists every earlier model is new (`newModels`) and refused with `AGENT_SESSION_MODEL_NOT_OFFERED`.
- **ADRs amended:** ADR047 D7 (the "No explicit mode" bullet becomes "An explicit mode", and Storage and One filter type at a time are updated), ADR090 § Model pin ("A workspace narrows the catalog") and the ADR018 agent-model row, which drops the w10/046 divergence.

## Live check (t005)

**`dev-10`, 2026-11-23 (DoD 7, first half — holds).**

- **Setup.** bex-api was built from the tree on production's catalog: `BEX_AGENT_PLATFORM_MODEL_ROUTE=openrouter`, a placeholder key, and `BEX_AGENT_PLATFORM_MODELS=anthropic/claude-haiku-5.5,anthropic/claude-sonnet-5.5,anthropic/claude-opus-5.5`.
- **Stand-ins**, as m568 and m573 did, because `dev-9` holds the agent-session leg on the shared cluster:
  - enforced authz on a dedicated OpenFGA container, seeded by `scripts/authz-model.sh` and passed as `DEV_OPENFGA_URL`;
  - unreachable OpenSandbox, model-proxy and gateway URLs plus a throwaway `BEX_SHELL_TICKET_SECRET`, which turns the capabilities read on (`enabled` needs the ticket secret) and is enough for a create's model check.
- **Fixtures.** Two Kratos users (`dev-10.local`, emails verified through the Kratos admin API) own workspace A `tea-dc237lpjg4r2j2jvvuk0` (disposable) and workspace B `tea-dc237m1jg4r2j2jvvukg` (disposable).
- The shared cluster's load-balancer port had moved after a Docker restart, so this checkout's kubeconfig was repointed at it (`kubectl config set-cluster --server`) before `bash scripts/dev-env.sh 10 up`.

1. **A turned "Allow all models" off** (`PATCH /v1/agent-environment` `{"allowAllModels":false}`). Its read gave `allowAllModels: false`, all four models as `enabledModels`, `newModels: []` and `enabledProviders: null`.
   - **B** narrowed to the default and haiku, then ran `bex agent env set --models all`, which printed `Models allowed  all (a model added to the catalog later included)` and all four models.
   - `--models all,anthropic/claude-haiku-5.5` was a usage error.
   - In A, `{"allowAllModels":true,"enabledModels":[…]}` answered `400 AGENT_ENVIRONMENT_MODELS_INVALID` with `field: allowAllModels` and `fields: ["allowAllModels","enabledModels"]`.
2. **bex-api was restarted** (`bash scripts/dev-env.sh 10 up`) with `anthropic/claude-fable-5.5` appended to the catalog.
   - **A** read five `models`, the four earlier ones as `enabledModels` and `newModels: ["anthropic/claude-fable-5.5"]`, still `allowAllModels: false`. The capabilities read's claude `models` left fable out, and so did `bex agent models`.
   - `bex agent env` in A printed `Models allowed  allowlist (a model added to the catalog later stays off until an admin turns it on)` and `Models off  anthropic/claude-fable-5.5 (new in the catalog)`.
   - `POST /v1/agent-sessions` naming fable in A answered `400 AGENT_SESSION_MODEL_NOT_OFFERED`: "anthropic/claude-fable-5.5 is turned off in this workspace, whose members may choose …; a workspace admin can turn it on in Settings → Agents". `bex agent run --model anthropic/claude-fable-5.5` printed the same.
   - **B** read all five as `enabledModels` (REST and GraphQL), `newModels: []` and `allowAllModels: true`, and the capabilities `models` were all five. A create naming fable was accepted (`201`, model `anthropic/claude-fable-5.5`), and the session was deleted at once (`204`).
3. **Settings → Agents** ran from the tree, signed in as A, at 1280 px and 390 px.
   - "By model" was selected among "All models", "By model" and "By provider". Fable's switch was off with "New in the catalog" under it.
   - "All models" sent `{allowAllModels: true}`, and every switch was then on and locked under its help line. "By model" then sent `{allowAllModels: false}`, which left every switch on and A an allowlist of all five (`allowAllModels: false`, `newModels: []`).
   - With the environment query held (Playwright route), the skeleton's heading, option frame, each of the three options, help line and first four rows had the ready state's boxes exactly at both widths. At 1280 px these were 84, 30, 24/24/24, 32 and 38/20/20/20 px tall, and at 390 px 124, 30, 24/24/24, 48 and 74/20/20/20 px. The ready list's fifth row is the grown catalog's fifth model, and production's catalog has four.
4. **Torn down** on 2026-11-23: the dashboard and bex-api were stopped, `bash scripts/dev-env.sh 10 down` was run, and the OpenFGA container was removed.

**Production after the deploy, 2026-11-23 (read-only, `hetzner-prod`, pin `9fdf7e0a1171`, DoD 7 second half — holds).**

- **The image.** The newest pin on `origin/main` is `cded300699` (`9fdf7e0a1171`), and `git merge-base --is-ancestor` puts `be26a7c6b2` and `0284e0af2d` in it. Both `bex-api` Pods (started 17:38Z, after the 17:33Z pin) and both `bex-ssh-gateway` Pods run `bex-operator@sha256:df6e69bd…`, the digest that pin set.
- **No catalog model was added.** `BEX_AGENT_PLATFORM_MODELS` on the live Deployment is still `anthropic/claude-haiku-5.5,anthropic/claude-sonnet-5.5,anthropic/claude-opus-5.5` after the default `deepseek/deepseek-v4.1-flash-1017` (route `openrouter`), last changed in `1988962c01` (2026-11-19). The schema is at migration 340, not dirty.
- **Every row offers what it did.** `bex-db-2` was the primary (`pg_is_in_recovery()` false). `agent_environments` still has 3 rows: narrowed 0, own key 0, `enabled_providers` set 0, `known_models` set 0 (`3|0|0|0|0`, as before the deploy). Read per row with the ids hashed by `scripts/lib/public_log.py`, `ws#3eda4c`, `ws#351c84` and `ws#001b90` each have `disabled_models` `{}`, `known_models` and `enabled_providers` NULL, no own key and no default model. All three are bex's own fixtures (bex, tian-personal and bex-canary in `scripts/lib/first_party_workspaces.txt`). Each therefore reads as "allow all" and offers the whole catalog of four, as before.
- **`tian-personal`.** Signed in with `bash scripts/qa-login.sh` and logged out afterwards (`--logout` answered `ok logged-out`, and this run's jar and state file were deleted). `GET https://api.bex.co/v1/agent-environment?ownerId=tea-da2isimlm39c739m4ofg` answered `200` with:
  - `allowAllModels: true`;
  - `models` and `enabledModels` both the four catalog models in catalog order;
  - `newModels: []`, `providers: ["deepseek","anthropic"]` and `enabledProviders: null`.
  GraphQL `agentEnvironment(ownerId:)` read the same `allowAllModels`, `enabledModels`, `newModels` and `enabledProviders`, and `GET /v1/agent-sessions/capabilities` gave claude the same four `models`.
- **Reproduce.** `kubectl --context hetzner-prod -n bex-system exec <primary> -c postgres -- psql -U postgres -d bex -At -c "SET default_transaction_read_only = on; SELECT count(*), count(*) FILTER (WHERE cardinality(disabled_models) > 0), count(*) FILTER (WHERE model_key_set_at IS NOT NULL), count(*) FILTER (WHERE enabled_providers IS NOT NULL), count(*) FILTER (WHERE known_models IS NOT NULL) FROM agent_environments"`. Pipe any per-row read through `python3 scripts/lib/public_log.py` with `PUBLIC_LOG_KEY` set to `hetzner-prod`'s `kube-system` namespace UID: `~/.kube/config` has no current context, and without the key the hashes are random per run (m573's differ for that reason). Make the REST read with the cookie jar `qa-login.sh` prints on its `logout` line, then run `--logout` on it.

## Unblocked (2026-11-23)

- **Affected:** w10/m592 t005 step 4 and t009.
- **Source:** verified evidence. Production's pin `9fdf7e0a1171` (`cded300699`, the newest `chore(deploy): pin platform images` on `origin/main`) carries `be26a7c6b2` and `0284e0af2d` (`git merge-base --is-ancestor`), and the running `bex-api` image is that pin's digest. The gate `pin must carry be26a7c6b2` cleared by itself, with no user action.
- **Next, done the same day:** t005 step 4's read-only production read (above, it held), then t009 closed the milestone.

## Render parity (t006, Devin, 2026-11-23)

Against [Devin CLI Team Settings](https://docs.devin.ai/cli/enterprise/team-settings) (the two modes), [List Models](https://docs.devin.ai/desktop/accounts/api-reference/list-models) (the allowlist "as stored" and its matching rule) and [Devin Desktop admin controls](https://docs.devin.ai/desktop/controls) (one filter type at a time, pickers hide what the list does not allow), re-read 2026-11-23. The proof is the backend tests on all three surfaces and the `dev-10` run above.

| Surface | Devin counterpart | Verdict |
| --- | --- | --- |
| REST `GET`/`PATCH /v1/agent-environment` `allowAllModels` | "Allow all models" beside "Allowlist specific models" | ✅ explicit mode. Devin's API has no mode field, so bex's wire name is its own. |
| GraphQL `AgentEnvironment.allowAllModels`, `AgentEnvironmentInput.allowAllModels` | same | ✅ same reads, writes and refusals as REST (`TestAllowAllModelsOffersALaterModel`, `TestAllowAllModelsRefusalsOnEverySurface`) |
| MCP `get_`/`update_agent_environment` | same | ✅ same, and both descriptions state the mode |
| `allowAllModels: true` beside a list → `AGENT_ENVIRONMENT_MODELS_INVALID` | "Allow all models" has no list | ✅ |
| An allowlist listing every current model, then a catalog model added | "A model is allowed when its UID is listed" | ✅ off, in `newModels`, refused at create, message, automation save and `defaultModel` |
| "Allow all", then a catalog model added | "Give users access to all available models" | ✅ offered from bex-api's next start |
| Editing `enabledModels` keeps the mode | Devin edits the allowlist within its mode | ✅ |
| Capabilities `models`, `bex agent models`, the composer, steering composer and automation form pickers | pickers list only allowed models | ✅ they follow `offeredModels`. The dashboard now also evicts the cached capabilities after an `allowAllModels` or `enabledProviders` save; before, only an `enabledModels` save did, which left the composer stale after a provider save since m573. |
| `bex agent env` "Models allowed", `env set --models all\|ID,…` | the settings page's mode | ✅ |
| `--providers` and "By provider" | provider tags, one filter type at a time | ✅ unchanged. "By provider" chosen from "All models" lists every provider. |
| Settings → Agents "All models", "By model", "By provider" | the two modes, and the allowlist by model or by provider | ✅ |
| The read on the workspace's own key: `allowAllModels: true` | none (Devin has no workspace-key catalog) | bex-only, and consistent with `enabledModels: []` and `enabledProviders: null` there |

No drift was found, so no follow-up note is filed. Devin's enterprise-level and group restrictions stay out of scope, as ADR047 D7's `Scope.` line records.

## Simplify (t007, 2026-11-23)

- **Taken:** `modelOfferPatch`'s three writes of the model columns (by model, the mode, by provider) now go through one local setter, `setLists`, instead of three copied pairs of patch and row assignments.
- **Left:** the test's `m573LeftOn` copies the previous release's read on purpose, as the rollout rule's model.
- **Left:** the dashboard's `MODEL_FILTERS` lookup per render is three entries.
- **Unchanged:** the reuse and efficiency lenses found nothing else.
- `go test ./internal/agentsessions` and `make lint-backend` passed afterwards.

## Test coverage (t008, 2026-11-23)

| DoD 6 case | Test |
| --- | --- |
| Both modes on all three surfaces | `TestAnAllowlistListingEveryModelLeavesTheNextOneOff` (REST, GraphQL, MCP writes; reads on all three), `TestAllowAllModelsOffersALaterModel` (each surface's write, from three starting rows) |
| The writes that switch between them, from a filter by provider too | `TestTheModeWrites` (16 writes from five starting rows), `TestSwitchingBetweenTheFilterTypes` |
| A catalog model added under each mode | `TestAnAllowlistListingEveryModelLeavesTheNextOneOff`, `TestAllowAllModelsOffersALaterModel`, `TestAModelAddedAfterANarrowingWriteStaysOff`, `TestAModelAddedToAWorkspaceThatNeverNarrowedIsOffered` |
| The refusals (beside a list, off the route, the own key) | `TestAllowAllModelsRefusalsOnEverySurface` |
| The rollout read rule | `TestTheModeRolloutReadRule` (every row shape a previous release leaves reads the same offer under any catalog; every new write reads the same under the catalog both run), `TestTheModelListRolloutReadRule`, and the store's `TestAgentEnvironmentKnownModelsAndTheLegacyConversion` (an allowlist of every model round-trips and no conversion touches it) |
| CLI reads and writes | `TestEnvSet` (`--models all`, `--models ID,…`, the usage errors), `TestEnvShowsTheEnabledModels` (the three modes and an older bex-api), `TestConfigContractFixtureDecodes`, the backend's `TestShapesMatchCLIAgentContract` |
| Dashboard reads and writes | `agent-environment-card.test.tsx` › "all models" (the option order and locked rows, each switch's input, an allowlist of every model staying one with a later model "New in the catalog", the refusal), `environment.test.ts` › "reads the mode in force from the environment", `use-agent-environment.test.ts` (the cache eviction) |

**Reverts that turned the tests red (each restored):**

- Backend:
  - leftOn's pre-m568 clause restored → `TestAnAllowlistListingEveryModelLeavesTheNextOneOff`, `TestTheModeRolloutReadRule`;
  - `allowAllModels: true` storing the catalog → `TestAllowAllModelsOffersALaterModel`;
  - m568's every-model-allows-all rule restored for `enabledModels` → `TestTheModeWrites`, `TestAModelAddedAfterANarrowingWriteStaysOff`;
  - `allowsAll` ignoring turned-off models → `TestTheModeRolloutReadRule`;
  - `true` beside a list accepted → `TestAllowAllModelsRefusalsOnEverySurface`.
- CLI:
  - a list sending only `enabledModels`, or `all` beside an id accepted → `TestEnvSet`;
  - the "Models allowed" row dropped → `TestEnvShowsTheEnabledModels`.

## Source + Goal linkage

- **Source:** inbox note [w10/046](../../done/046.md), filed by w10/m568 t001 (design) on 2026-11-22 and promoted by `/loopx w10` on 2026-11-23.
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
    - It ships anyway. ADR018's agent-model row records it only as "Divergence filed", without the user's sign-off, and a Devin divergence stands only with that sign-off ([ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first); `/loopx` worker rules).
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
- **Goal linkage:** cloud coding-agent sessions ([ADR047](../../../../docs/ADR047-cloud-coding-agent-sessions.md), ADR008 pillar 5), where Devin is the reference product. It completes the m568 ([design record](../../done/m568/README.md#design-record-t001-2026-11-22)) and m573 model lists.
- **Expected outcome:** an admin who keeps an allowlist keeps exactly the models it lists when the catalog grows, even when it lists all of today's. An admin who wants every model, the next ones included, says so with "Allow all models".
- **Why now:** w10's model-list line is otherwise complete (m516, m527, m568, m573), and this is its last recorded Devin divergence. It must land before the catalog's next model: every model added before then is on in a workspace whose admins listed every model to keep the list closed. The milestone includes the Render parity task because it changes the REST, GraphQL, MCP, CLI and dashboard reads and writes of the agent environment.
