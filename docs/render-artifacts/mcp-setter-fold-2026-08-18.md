# MCP setter fold — 2026-08-18

This record is the breaking-change boundary for **w1/m71**. Thirty per-field MCP `set_*` tools were removed and their capability moved onto five patch-shaped `update_*` tools. It follows [`deprecated-surface-removal-2026-07-27.md`](deprecated-surface-removal-2026-07-27.md) in shape: what was removed, what to call instead, and what was deliberately kept.

Every removed tool was classified `Extension` by the [w1/m70 parity pin](../ADR006-bex-api.md#mcp-parity-is-pinned-w1m70) — no upstream counterpart, so no Render contract is broken by the rename. No `Parity1to1`, `Superset`, or `Divergent` tool was touched. Measured surface: **213 → 187 tools**.

## Decision: hard removal, not aliases

The old names are gone; calling one returns MCP's ordinary unknown-tool error. Aliases were considered and rejected:

- These are bex-invented names with no Render contract behind them and no known external caller — nothing in this repo (dashboard, mobile, `scripts/`, skills, docs) calls them, and the removal was verified repo-wide.
- bex is pre-1.0 and MCP clients read `tools/list` on every connection, so an agent discovers the replacement immediately rather than holding a stale name.
- An alias tool would keep the surface at the size the milestone exists to reduce, which is the whole point: an aliased fold reduces nothing an agent's context can feel.

What makes the removal _findable_ rather than silent is this table plus each replacement tool's description, which names the tools it replaced.

## Caller migration

Every row is a pure rename of reach: the same Service verb, the same authorization, the same validation.

| Removed MCP tool | Canonical replacement call |
| --- | --- |
| `set_display_name` | `update_service {serviceId, displayName}` |
| `set_branch` | `update_service {serviceId, branch}` |
| `set_registry_credential` | `update_service {serviceId, registryCredentialId}` |
| `set_root_directory` | `update_service {serviceId, rootDir}` |
| `set_build_command` | `update_service {serviceId, buildCommand}` |
| `set_start_command` | `update_service {serviceId, startCommand}` |
| `set_dockerfile_path` | `update_service {serviceId, dockerfilePath}` |
| `set_health_check_path` | `update_service {serviceId, healthCheckPath}` |
| `set_pre_deploy_command` | `update_service {serviceId, preDeployCommand}` |
| `set_max_shutdown_delay` | `update_service {serviceId, maxShutdownDelaySeconds}` (the argument was `seconds`) |
| `set_auto_deploy` | `update_service {serviceId, autoDeploy}` (the argument was `enabled`) |
| `set_build_filter` | `update_service {serviceId, buildFilter}` |
| `set_notify_on_fail` | `update_service {serviceId, notifyOnFail}` (the argument was `value`) |
| `set_notifications_to_send` | `update_service {serviceId, notificationsToSend}` (the argument was `value`) |
| `set_maintenance_mode` | `update_service {serviceId, maintenanceMode}` |
| `set_subdomain_policy` | `update_service {serviceId, renderSubdomainPolicy}` (the argument was `policy`; the new spelling is REST's) |
| `set_service_ip_allow_list` | `update_service {serviceId, ipAllowList}` or `{serviceId, ipAllowListCidrs}` (the arguments were `entries`/`cidrs`) |
| `set_autoscaling` | `update_service {serviceId, autoscaling: {minInstances, maxInstances, targetCPUPercent?, targetMemoryPercent?}}` — the four flat arguments became one object. `disable_autoscaling` is unchanged. |
| `set_postgres_ip_allow_list` | `update_postgres {postgresId, ipAllowList}` or `{postgresId, ipAllowListCidrs}` |
| `set_postgres_parameter_overrides` | `update_postgres {postgresId, parameterOverrides}` (the argument was `parameters`) |
| `set_key_value_ip_allow_list` | `update_key_value {keyValueId, ipAllowList}` or `{keyValueId, ipAllowListCidrs}` |
| `set_key_value_maxmemory_policy` | `update_key_value {keyValueId, maxmemoryPolicy}` |
| `set_environment_acl` | `update_environment {id, protectedStatus?, networkIsolationEnabled?, ipAllowList?}` — and unlike the setter, you send only the fields you are changing |
| `set_environment_services` | `update_environment {id, serviceIds}` |
| `set_environment_databases` | `update_environment {id, databaseIds}` |
| `set_environment_keyvalues` | `update_environment {id, keyValueIds}` |
| `set_environment_env_groups` | `update_environment {id, envGroupIds}` |
| `set_project_services` | `update_project {id, serviceIds}` |
| `set_project_databases` | `update_project {id, databaseIds}` |
| `set_project_keyvalues` | `update_project {id, keyValueIds}` |

### Semantics a caller must know

- **Omitted argument ⇒ unchanged.** No write, no build, no roll. The old setters had one required argument, so "not sending it" was not expressible; now it is, and it means "leave this alone".
- **Present argument ⇒ written exactly, empty included.** `""` clears a command or path, `[]` clears a list or membership — the same way a required-empty setter call cleared before.
- **A present list REPLACES.** Allowlists, parameter overrides, and environment/project membership were replace-shaped as setters and stay replace-shaped as arguments.
- **Order matches REST.** A multi-field `update_service` applies its fields in `PATCH /v1/services/{id}` order, so the two surfaces cannot disagree about a combination.
- **`ipAllowList` / `ipAllowListCidrs` are one pair everywhere.** The setters spelled this three different ways (`entries`/`cidrs`, `ipAllowList`(strings)/`ipAllowListEntries`). Sending both forms with conflicting values is now a bad request on every folded tool — the datastore setters previously let the entry form win silently.
- **Postgres/Key Value allowlist writes are now audited.** Both route through `UpdatePostgres`/`UpdateKeyValue` (REST `PATCH`'s method) instead of the dedicated setter, so the change records a `DatabaseUpdated`/`KeyValueUpdated` effect it did not record before. Same relation (`can_operate`), same validation.

## Deliberately retained

- **`set_env_var`, `set_secret_file`, `set_env_group_var`, `set_env_group_secret_file`** — merge-one-key upserts, each paired with a `delete_*` verb, against resource-level partners (`update_env_vars`, `update_env_group_vars`) that replace the whole set. Folding them would make one tool mean both "replace everything" and "merge one key", so they keep their names.
- **The narrow `update_*` / `scale_*` / `rename_*` tools** — `update_service_plan` (carries `dryRun`), `scale_service`, `update_idle_timeout`, `update_publish_path`, `update_static_routes`, `update_static_headers`, `update_cron_job`, `update_postgres_plan`, `update_postgres_version`, `update_postgres_disk_autoscaling`, `update_key_value_plan`, `rename_project`, `rename_environment`, `rename_postgres`, `rename_key_value`. These are the same per-field grammar under a different prefix; folding them too is a second, larger reduction that this milestone deliberately did not smuggle in.
- **`disable_autoscaling`** — a delete verb, not a setter. `update_service`'s `autoscaling` argument enables/updates; this turns it off, mirroring REST's PUT/DELETE split.
- **`get_autoscaling`, `get_postgres_ip_allow_list`, `list_postgres_parameter_overrides`** — reads, untouched by a write fold.

## Verification anchors

- `internal/api`'s `TestMCPParityInventory` pins the measured surface at **187** tools (10 `Parity1to1` / 1 `Superset` / 8 `Divergent` / 168 `Extension`) and fails if it moves without this ledger and ADR018 moving with it.
- `internal/apps`' `TestUpdateServiceReachesEveryFoldedField` covers every folded service field through the new tool; `TestUpdateServiceLeavesOmittedFieldsAlone` asserts the omitted-argument rule against a service where all eighteen settings are populated; `TestUpdateServiceBuildTriggeringFieldsStillTriggerABuild` pins which fields rebuild; `TestUpdateServiceMatchesRESTPatchFieldForField` asserts MCP and REST produce identical CR specs field by field.
- `internal/postgres`, `internal/keyvalue`, `internal/environments`, and `internal/projects` each cover their fold's replace semantics and the "one field mentioned, the others untouched" property.
- `scripts/platform-deprecations-validate.sh` fails closed if any retired setter name returns to an MCP registration.
