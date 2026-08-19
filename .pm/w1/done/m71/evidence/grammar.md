# w1/m71 t001 — the target grammar

Input: [`w1/done/m70/evidence/inventory.md`](../../done/m70/evidence/inventory.md), measured against `render-oss/render-mcp-server` `main@89c1f01b4527`. Nothing here is re-derived from source comments.

## Confirmed against the pin

All **34** `set_*` tools classify `Extension` — no upstream counterpart, no parity obligation. Verified by enumerating the live registry (`tools/list`) and classifying each name against the pin, the same mechanism `TestMCPParityInventory` runs in CI. No `Parity1to1` (10) or `Superset` (1) tool is a `set_*`, so the fold cannot touch the contract set. The 8 `Divergent` tools are all `create_*`/`get_metrics`/`trigger_deploy` — none is folded here, so their filed repairs stay separable.

## Why `update_*` is still the right destination

m70 found upstream **removed** its three `update_*` tools (#89, 2026-07-23) because they were placeholders returning a dashboard link. That is an argument against shipping fake update tools, not against real ones:

- bex's `update_*` tools do the work — `update_env_vars`, `update_environment`, `update_cron_job` ("Render ships a non-functional stub for this tool; bex makes it real") all reach the same Service methods REST and GraphQL call.
- The alternative destinations are worse. `create_*`-shaped patch verbs would overload create with update semantics. Keeping the setter grammar and merely renaming it changes nothing an agent notices.
- **REST already has the shape.** `PATCH /v1/services/{id}` (`internal/apps/rest.go:patchService`) coalesces the body into an ordered op table where a field present is applied and a field absent is left alone. `PostgresPatch` / `KeyValuePatch` / `EnvironmentPatch` are the same idea at the Service layer, each documenting `nil = unchanged`. Folding MCP onto that shape makes the third surface agree with the two that already agree, rather than inventing a fourth grammar.

So: fold into a **resource-scoped patch tool** named `update_<resource>`, mirroring that resource's REST `PATCH` semantics field for field.

## Omitted-argument semantics (the rule every folded tool obeys)

- **Argument absent ⇒ that setting is not touched.** No write, no build, no roll. Matching REST `PATCH`, where an absent JSON field never enters the op table.
- **Argument present ⇒ that setting is written to exactly the value given**, including the empty value: `""` clears a command, `[]` clears a list. This is what the old setter did — the setters were all "present and required", so an empty string was already the clear verb.
- **No field is cleared as a side effect of another field being set.**
- A call carrying **no** settable field is a read-only no-op returning current state (again mirroring `patchService`).
- Multi-field calls apply in the **same order REST applies them**, so a combination behaves identically on both surfaces.

## The mapping

### Fold — 30 setters into 5 patch tools (4 new, 1 existing)

| fold target | new? | absorbs | note |
| --- | --- | --- | --- |
| `update_service` | new | `set_display_name`, `set_branch`, `set_registry_credential`, `set_root_directory`, `set_build_command`, `set_start_command`, `set_dockerfile_path`, `set_health_check_path`, `set_pre_deploy_command`, `set_max_shutdown_delay`, `set_auto_deploy`, `set_build_filter`, `set_notify_on_fail`, `set_notifications_to_send`, `set_maintenance_mode`, `set_subdomain_policy`, `set_service_ip_allow_list`, `set_autoscaling` (18) | argument names and apply order mirror `PATCH /v1/services/{id}` |
| `update_postgres` | new | `set_postgres_ip_allow_list`, `set_postgres_parameter_overrides` (2) | one `UpdatePostgres(PostgresPatch)` call — the method REST `PATCH` already uses |
| `update_key_value` | new | `set_key_value_ip_allow_list`, `set_key_value_maxmemory_policy` (2) | one `UpdateKeyValue(KeyValuePatch)` call; keeps `dryRun` from the folded policy setter |
| `update_environment` | exists | `set_environment_acl`, `set_environment_services`, `set_environment_databases`, `set_environment_keyvalues`, `set_environment_env_groups` (5) | already the patch tool; gains membership lists + the entry-form allowlist |
| `update_project` | new | `set_project_services`, `set_project_databases`, `set_project_keyvalues` (3) | symmetric with `update_environment`, including the optional `name` |

**`set_environment_acl` is fully subsumed, not merely folded.** `update_environment` already carried `protectedStatus`, `networkIsolationEnabled`, and an entry-shaped `ipAllowList`; the ACL setter's only unique reach was the plain-CIDR-string spelling, which survives as `ipAllowListCidrs`. Its "full-replace, pass the current value of anything you don't mean to change" contract was the trap the patch tool exists to remove.

### Stay standalone — 4 setters, with the reason

| tool | why it does not fold |
| --- | --- |
| `set_env_var` | **Merge-one-key upsert**, paired with `delete_env_var`. Its resource-level partner `update_env_vars` **replaces the whole set**. Folding would make one tool mean both "replace everything" and "merge one key" — precisely the replace-vs-merge blur t003 says to refuse. |
| `set_secret_file` | Same shape for secret files (`update_env_vars`' file counterpart is the replace path; `delete_secret_file` is the removal verb). |
| `set_env_group_var` | Same, against `update_env_group_vars`. |
| `set_env_group_secret_file` | Same. |

These four are keyed writes into a map, not fields of the resource, so the patch grammar has nothing to express them with. They keep their names and their `delete_*` partners.

### Deliberately NOT touched (out of this milestone's scope)

The per-field tools already spelled `update_*` / `scale_*` / `rename_*` — `update_service_plan`, `scale_service`, `update_idle_timeout`, `update_publish_path`, `update_static_routes`, `update_static_headers`, `update_cron_job`, `update_postgres_plan`, `update_postgres_version`, `update_postgres_disk_autoscaling`, `update_key_value_plan`, `rename_project`, `rename_environment`, `disable_autoscaling` — are the *same* per-field grammar under a different prefix, and folding them too would roughly double the reduction. They are out of scope here (t008: "further surface reduction beyond the setter grammar"), so the new patch tools expose **exactly the folded fields** and cross-reference the survivors in their descriptions. A second fold is worth filing as its own note rather than smuggling in here.

`disable_autoscaling` stays because it is a delete verb, not a setter: `update_service`'s `autoscaling` argument enables/updates, `disable_autoscaling` turns it off — the same PUT/DELETE split REST has.

## One allowlist spelling across the folded tools

The setters spelled the allowlist three different ways (`cidrs`+`entries` on services and datastores, `ipAllowList`(strings)+`ipAllowListEntries` on the environment ACL, `ipAllowList`(entries) on `update_environment`). The folded grammar uses one pair everywhere:

- `ipAllowList` — `{cidrBlock, description}` entries; `[]` clears.
- `ipAllowListCidrs` — the plain-string convenience form.
- Both present and disagreeing ⇒ `core.ResolveAllowListInputs` rejects the call (`ErrBadRequest`) rather than silently dropping descriptions. The datastore setters previously let entries win silently; the stricter rule is now uniform.

## Projected delta

- removed: **30** tools
- added: **4** (`update_service`, `update_postgres`, `update_key_value`, `update_project`)
- **213 → 187, a net −26**, with 4 `set_*` deliberately surviving.

The DoD's "roughly 28" assumed every setter folds into an existing `update_*`; four setters legitimately stay and four fold targets had to be created. Measured, not estimated — t008 records the number the registry actually reports.
