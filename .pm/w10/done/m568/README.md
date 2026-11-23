# w10 · m568 — A model added to the platform catalog stays off in a workspace whose admins narrowed its list, as Devin's allowlist leaves a model it does not list off

**Worker:** worker10 **Goal:** the workspace model list becomes Devin's two modes. A workspace whose admins have turned no model off allows every catalog model, including one added later. Once an admin turns any model off, the list is an allowlist: a model added to the catalog afterwards is off there until an admin turns it on. Settings → Agents and `bex agent env` show such a model as off because it is new in the catalog. No workspace's offer changes when this ships. **Status:** done (t001–t010 done; t006's production read held on pin `1a5a8e872398` on 2026-11-23)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Design: storage of the allowlist, when a workspace allows every model, the read field naming new models, the legacy-row conversion and the rolling deploy; ADR047, ADR090 and ADR018 amended — **DONE** | 45m | — |
| [t002](done/t002.md) | Backend: the migration, the store, and the one-shot conversion of rows narrowed under the opt-out — **DONE** | 1h15m | t001 |
| [t003](done/t003.md) | Backend: the offered set reads the allowlist, and the environment read names the models that are new in the catalog on REST, GraphQL and MCP — **DONE** | 1h30m | t002 |
| [t004](done/t004.md) | CLI: `bex agent env` shows a new catalog model as off, "new in the catalog" — **DONE** | 45m | t003 |
| [t005](done/t005.md) | Dashboard: Settings → Agents shows a new catalog model off with a "New in the catalog" label — **DONE** | 1h | t003 |
| [t006](done/t006.md) | Live check: a catalog gaining a model on `dev-10`, then production's offers unchanged by the deploy — **DONE** | 45m | t003, t004, t005 |
| [t007](done/t007.md) | Render parity (Devin) — **DONE** | 30m | t006 |
| [t008](done/t008.md) | Simplify — **DONE** | 20m | t007 |
| [t009](done/t009.md) | Test coverage — **DONE** | 45m | t007, t008 |
| [t010](done/t010.md) | Closeout — **DONE** | 10m | t009 |

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

## Design record (t001, 2026-11-22)

Against Devin's team settings, "Allow all models" beside "Allowlist specific models" ([Devin CLI Team Settings](https://docs.devin.ai/cli/enterprise/team-settings)), and its rule "A model is allowed when its UID is listed or it carries a listed tag" over "Model UIDs on your team's allowlist, as stored" ([List Models](https://docs.devin.ai/desktop/accounts/api-reference/list-models)), both re-read 2026-11-22.

- **Storage.** Migration 0330 adds a nullable `agent_environments.known_models text[]` (at most 64), the catalog the last narrowing write saw. `disabled_models` stays the models an admin turned off. The allowlist is `known_models − disabled_models`, and `catalog − known_models` are the new ones. This beat a nullable `enabled_models`: an allowlist column that a previous-release Pod never writes would go stale under its writes, and a write that turns a model on could not be told apart from a model new in the catalog. With `known_models`, a previous-release write (`disabled_models` alone, against the same catalog) reads exactly as that Pod reads it.
- **When a workspace allows every model.** The mode follows the list. A write leaving no catalog model off stores `known_models` NULL ("Allow all models"): no row, a row that never narrowed, and a write turning every model back on all allow a later model. Any model off stores the catalog as `known_models` ("Allowlist specific models"). No explicit mode: DoD 1 asks that "turned every model back on" allows all, and Devin's allowlist that lists every current model is a narrower variant, filed as [w10/046](../../046.md).
- **The read.** `newModels` on REST `GET /v1/agent-environment`, GraphQL `AgentEnvironment.newModels: [String!]!` and MCP `get_agent_environment`: the catalog models off only because the allowlist predates them, in catalog order. It is `[]` off the route, on the workspace's own key and in "allow all". `enabledModels` stays the effective offered list, the catalog default always first. A write records every model it leaves off as an admin's, a new one included: the admin saw it in the list.
- **Legacy rows and the rollout.**
  - The read rule: a row with `known_models` NULL or `disabled_models` empty reads as the opt-out did, `catalog − disabled_models`. That covers a row narrowed under 0315 and not yet converted, and every row a previous-release Pod writes: one narrowing an "allow all" row, one turning every model on, and one changing which models an allowlist has.
  - New writes keep `disabled_models` exact (`catalog − enabledModels`), so a previous-release Pod reads them as the new code does while the catalog is unchanged.
  - The conversion: bex-api records the catalog it starts with as `known_models` on every row with `known_models` NULL and `disabled_models` not empty (`PGStore.ConvertLegacyModelLists`, one idempotent `UPDATE` that keeps `updated_at`). It runs at start and hourly, so a row a previous-release Pod narrows during this rollout converts before a later deploy can grow the catalog. It is skipped off the platform route. It is exact because production's catalog last changed in `1988962c01` (2026-11-19), before 0315 shipped (`68e848a935`, 2026-11-20), and re-checked 2026-11-22. The deploy that first ships m568 must add no catalog model.
- **Unchanged:** the catalog default always on (`leftOn`'s `i == 0`), `AGENT_ENVIRONMENT_MODELS_INVALID`, the next-turn fallback, and `retiredTemplateModel`'s "turned off in this workspace" note, which a new model gets too because `inPlatformCatalog` holds for it.
- **ADRs amended:** ADR047 D7's w10/m527 amendment ("Two modes, as Devin's", "Storage"), ADR090 § Model pin ("A workspace narrows the catalog") and the ADR018 agent-model row. Each drops "a model added to the catalog later starts on" and the w10/036 divergence.

## Live check (t006)

**`dev-10`, 2026-11-22 (DoD 7, first half — holds).** bex-api built from the tree with the m568 changes, on production's catalog (`BEX_AGENT_PLATFORM_MODEL_ROUTE=openrouter`, a placeholder key, `BEX_AGENT_PLATFORM_MODELS=anthropic/claude-haiku-5.5,anthropic/claude-sonnet-5.5,anthropic/claude-opus-5.5`). The agent-session leg on the shared cluster is held by `dev-9`, so `dev-10` ran its agent dependencies as stand-ins: enforced authz on this checkout's `scripts/backend-test-deps.sh` OpenFGA (`DEV_OPENFGA_URL`), and unreachable OpenSandbox, model-proxy and gateway URLs. That is enough for the capabilities read and a create's model check, which come before any sandbox work. Two Kratos users (`dev-10.local`, emails verified through the Kratos admin API) own workspaces A `tea-dc1ntc9jg4r42mh7uko0` (disposable) and B `tea-dc1ntc9jg4r42mh7ukog` (disposable).

1. A turned opus off (`PATCH /v1/agent-environment`), and B wrote only `networkTier`. A's REST environment read `enabledModels` the other three models and `newModels: []`. B's read all four.
2. bex-api restarted (`bash scripts/dev-env.sh 10 up`) with `moonshotai/kimi-k3` appended to the catalog:
   - **A** read five `models`, the same three `enabledModels` and `newModels: ["moonshotai/kimi-k3"]`. The capabilities read's claude `models` left kimi out.
   - `bex agent env` (the CLI built from the tree, `BEX_HOST` + `BEX_API_KEY`) printed `Models off  anthropic/claude-opus-5.5 (turned off by an admin); moonshotai/kimi-k3 (new in the catalog)`.
   - `POST /v1/agent-sessions` naming kimi answered `400 AGENT_SESSION_MODEL_NOT_OFFERED`: "moonshotai/kimi-k3 is turned off in this workspace, whose members may choose …; a workspace admin can turn it on in Settings → Agents". `bex agent run --model moonshotai/kimi-k3` printed the same.
   - **B** read all five as `enabledModels`, `newModels: []` and the capabilities `models` all five, and a create naming kimi was accepted (`201`; deleted at once).
3. `bex agent env set --models …,moonshotai/kimi-k3` in A turned it on. A then read `newModels: []`, and its capabilities read and a create offered kimi (session deleted). Opus stayed "(turned off by an admin)".
4. Settings → Agents (the dashboard from the tree against `dev-10`, signed in with a third disposable user whose row was set so kimi is new) showed kimi switched off with "New in the catalog" under it at 1280 px and 390 px. The section's heading and description wrapped identically in the skeleton and the ready state. The skeleton holds four representative rows, so the ready list differs only by the catalog's fifth row and the data-dependent label.
5. Torn down: `bash scripts/dev-env.sh 10 down` (2026-11-22).

**Production before the deploy, 2026-11-22 (read-only, `hetzner-prod`, pin `19fdb46f1758`).** `agent_environments` has 3 rows. None is narrowed (`cardinality(disabled_models) > 0`: 0) and none is on its own key, so every workspace offers the whole catalog of four. `tian-personal`'s row turns no model off. Reproduce with `kubectl --context hetzner-prod -n bex-system exec bex-db-2 -c postgres -- psql -U postgres -d bex -At -c "SET default_transaction_read_only = on; SELECT count(*), count(*) FILTER (WHERE cardinality(disabled_models) > 0), count(*) FILTER (WHERE known_models IS NOT NULL) FROM agent_environments"` (the last column exists once 0330 runs). The conversion therefore has no row to convert there.

**Production after the deploy, 2026-11-23 (read-only, `hetzner-prod`, pin `1a5a8e872398`, DoD 7 second half — holds).**

- **The image.** Both `bex-api` Pods (started 04:30Z, 2/2 updated and ready) run `bex-operator@sha256:65ef4f0d…`, the digest pin commit `e6c65f5c62` set. Its `BEX_AGENT_PLATFORM_MODELS` is unchanged (`anthropic/claude-haiku-5.5,anthropic/claude-sonnet-5.5,anthropic/claude-opus-5.5` after the default `deepseek/deepseek-v4.1-flash-1017`), so the deploy added no catalog model. The schema is at migration 0334, so 0330's `known_models` exists.
- **Every row offers what it did.** `agent_environments` still has 3 rows, all bex's own fixtures (`bex-canary`, `bex`, `tian-personal`). Narrowed (`cardinality(disabled_models) > 0`): 0. `known_models` set: 0. On their own key: 0. Each row has `disabled_models` `{}` and `known_models` NULL, "allow all", so each offers the whole catalog of four, as before. The conversion had nothing to convert, and neither Pod logged a conversion or a failure. (`tian-personal`'s `updated_at` moved to 05:07Z with w10/m561's `gh` live check, which left its model list untouched. Its `enabled_providers` is NULL too; that column is w10/m573's check.)
- **`tian-personal`.** Signed in with `bash scripts/qa-login.sh` (logged out afterwards), `GET https://api.bex.co/v1/agent-environment?ownerId=tea-da2isimlm39c739m4ofg` answered `200` with `models` and `enabledModels` both the four catalog models in catalog order and `newModels: []`. `GET /v1/agent-sessions/capabilities` gave claude the same four `models`.
- **Reproduce.** Run the read above, adding `count(*) FILTER (WHERE model_key_set_at IS NOT NULL)` for rows on their own key. Pipe any per-row read through `python3 scripts/lib/public_log.py`. Make the REST read with the cookie jar `qa-login.sh` prints on its `logout` line, then run `--logout` on it.

## Render parity (t007, Devin, 2026-11-22)

Devin's pages re-read 2026-11-22: [Devin CLI Team Settings](https://docs.devin.ai/cli/enterprise/team-settings) ("Allowlist specific models": "Restrict users to a curated list of approved models"; "Allow all models": "Give users access to all available models"), [List Models](https://docs.devin.ai/desktop/accounts/api-reference/list-models) ("A model is allowed when its UID is listed or it carries a listed tag"), [Devin Desktop admin controls](https://docs.devin.ai/desktop/controls) (a model outside the allowlist is hidden "from the relevant picker or settings page"; a default "Must be in the allowlist to take effect").

| Surface | bex after m568 | Devin |
| --- | --- | --- |
| REST `GET /v1/agent-environment`, GraphQL `agentEnvironment`, MCP `get_agent_environment` | `enabledModels` the effective list; `newModels` the models off because the allowlist predates them (`TestAModelAddedAfterANarrowingWriteStaysOff`, all three surfaces) | an allowlist of stored UIDs; a model it does not list is not allowed ✅ |
| REST `PATCH`, GraphQL `updateAgentEnvironment`, MCP `update_agent_environment`, `bex agent env set --models` | every model on allows all; any model off is an allowlist; the descriptions and `--models` help say so | "Allow all models" / "Allowlist specific models" ✅ (the mode follows the list: w10/046) |
| Capabilities `models` (REST, GraphQL, MCP), `bex agent models`, the composer, steering composer and automation form pickers | a new model is left out, as any turned-off one (the pickers read the capabilities `models`) | a model outside the allowlist is hidden from pickers ✅ |
| Create, message, automation create and update, `defaultModel` | `AGENT_SESSION_MODEL_NOT_OFFERED` / `AGENT_ENVIRONMENT_MODEL_NOT_OFFERED`, "turned off in this workspace … an admin can turn it on" | not allowed; a default outside the list does not take effect ✅ |
| `bex agent env`, Settings → Agents | "(new in the catalog)" beside "(turned off by an admin)"; "New in the catalog" under the switch | Devin shows no such label; bex's is extra information, not a divergence |

No drift beyond w10/037 (allow by provider tag) and the new w10/046 (an explicit allowlist that lists every current model).

## Simplify (t008, 2026-11-22)

`/simplify` over the backend, CLI and dashboard changes. Taken: the conversion passes the catalog as is, so a nil catalog writes no change instead of an empty allowlist, and the model switch row computes its note once with one id per row. Nothing else was worth changing.

## Test coverage (t009, 2026-11-22)

- `lego/backend/internal/agentsessions/model_allowlist_test.go`:
  - `TestAModelAddedAfterANarrowingWriteStaysOff`: a model added after a narrowing write is left out of the capabilities `models` and listed in `newModels` on REST, GraphQL and MCP. Create, message, automation create and update are refused, as is `defaultModel` and the automation run's note. Turning it on offers it, and every model on allows all, so the next model is on.
  - `TestAModelAddedToAWorkspaceThatNeverNarrowedIsOffered`: no row, a row turning nothing off, and every model turned back on.
  - `TestTheModelListRolloutReadRule`: a previous-release write reads as the opt-out under the same catalog. A 0315 row and a previous-release all-on write allow a later model, an allowlist does not, and the old default stays on. A model leaving the catalog drops out of the reads and of the next write.
- `TestALegacyNarrowedListConvertsWithItsOfferUnchanged` (real Postgres): skipped off the route, offer unchanged after the conversion, a later model off and new, a second run idempotent.
- `TestAgentEnvironmentKnownModelsAndTheLegacyConversion` (store, real Postgres): the column round trip, NULL for allow all, the conversion keeping `updated_at`, the 64 cap.
- CLI `TestEnvShowsTheEnabledModels` (new-in-the-catalog cases) and the contract fixture's `newModels`. Dashboard: the label renders only for a new model, and turning one on saves it into `enabledModels` and drops the label while it saves.
- Proved real by reverting: with `leftOn` allowing every model, `TestAModelAddedAfterANarrowingWriteStaysOff` and `TestTheModelListRolloutReadRule` fail. Without the empty-`disabled_models` rule the rollout test fails. Without the dashboard's pending guard the label test fails.

## Unblock record (2026-11-23)

- **Affected:** w10/m568 t006 (step 3) and t010.
- **Source:** verified evidence. The newest `chore(deploy): pin platform images to <sha>` on `origin/main` is `e6c65f5c62` (pin `1a5a8e872398`, 04:25Z on 2026-11-23), and `git merge-base --is-ancestor 343ae3da8d 1a5a8e872398` holds. Both `bex-api` Pods run `bex-operator@sha256:65ef4f0d…`, the digest that pin set (read-only `kubectl`). The pin gate was the only one.
- **Next:** t006 step 3's read-only production read and `tian-personal`'s `GET /v1/agent-environment`, then t010 closes the milestone if DoD 7 holds. Done the same day: see Live check (t006), "Production after the deploy".

## Source + Goal linkage

- **Source:** inbox note [w10/036](../../done/036.md), filed by w10/m527 t007 (Devin comparison) on 2026-11-20 and promoted by `/loopx w10` on 2026-11-22.
- **Triage evidence (2026-11-22, `da137839b8`):**
  - **The premise holds.** `modelOfferPatch` (`lego/backend/internal/agentsessions/environment.go:585`) stores every catalog model a write leaves out as `disabled_models`. `leftOn` (`environment.go:853`), behind `offeredModels` (`:837`), offers every catalog model not in that column. A model appended to `BEX_AGENT_PLATFORM_MODELS` after a narrowing write is therefore on. Migration `0315_agent_environment_models.up.sql` says so in its comment.
  - **Devin's behavior is documented, so no Devin team account is needed.**
    - [List Models](https://docs.devin.ai/desktop/accounts/api-reference/list-models) (read 2026-11-22) describes the allowlist as "Model UIDs on your team's allowlist, as stored" and "Model tags on your team's allowlist, as stored, including provider tags such as `provider-anthropic`". Its rule is "A model is allowed when its UID is listed or it carries a listed tag".
    - A newly released model's UID is on no stored list, so under a by-model allowlist it stays off.
    - [Devin CLI Team Settings](https://docs.devin.ai/cli/enterprise/team-settings) (read 2026-11-22) offers that "Allowlist specific models" option ("Restrict users to a curated list of approved models") beside "Allow all models" ("Give users access to all available models").
    - Allowing by provider tag, the other half of the rule, is [w10/037](../../done/037.md), promoted to [w10/m573](../../blocked/m573/README.md).
  - **Legacy rows convert exactly today.** Production's `BEX_AGENT_PLATFORM_MODELS` (`lego/operator/config/api/deployment.yaml:652`) last changed in `1988962c01` (w10/m516, 2026-11-19), before w10/m527 shipped the list (2026-11-20). Every row narrowed so far was written against today's catalog, so converting it with the catalog bex-api starts with keeps its offer. Re-check that history before shipping.
  - **Surfaces to touch:**
    - Backend: `Environment`/`EnvironmentInput` (`environment.go:73-186`), `environmentView` (`:675`) and `retiredTemplateModel` (`automation_scheduler.go:248`).
    - CLI: the "Models off" row (`lego/cli/internal/agent/config.go:610`).
    - Dashboard: `dashboard/src/features/agent-environment/components/enabled-models-section.tsx`.
  - **It is more than one sitting.** It touches a migration, the store and a startup conversion, three API surfaces, the CLI, the dashboard, three ADRs and two live checks.
- **Goal linkage:** cloud coding-agent sessions ([ADR047](../../../../docs/ADR047-cloud-coding-agent-sessions.md), ADR008 pillar 5), where Devin is the reference product. An admin who narrows the list to control spend must not find the next expensive model on for every member.
- **Expected outcome:** a workspace that narrowed its list keeps exactly the models its admins chose when the catalog grows. A workspace that never narrowed it gets new models at once, as Devin's "Allow all models" does.
- **Why now:** the catalog is about to grow (w10/037 waits on this item), and every model added before this ships is silently on in every narrowed workspace. The milestone includes the Render parity task because it changes the REST, GraphQL, MCP, CLI and dashboard reads of the agent environment.
