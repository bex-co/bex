# w1 · m71 — Collapse the bex-invented `set_*` MCP tool grammar

> **Gate cleared 2026-08-18** — `.pm/w1/done/m70/` landed the pin and the measured inventory. Scope below is now written from real numbers, not estimates: [m70's inventory](../done/m70/evidence/inventory.md).

**Worker:** worker1 **Goal:** cut the agent-facing tool count without losing a single ADR018 parity cell, by folding bex's own per-field setter grammar into the `update_*` tools bex already has. **Status:** done (2026-08-18)

**Measured outcome:** **213 → 187 tools (−26)**; the schema + description payload an MCP client loads before the user types anything went **157,582 → 146,018 bytes (−11,564, −7.3%)**, measured by driving both the pre- and post-fold servers through `tools/list` and summing each tool's description plus serialized input schema. The byte win is deliberately smaller than the tool-count win: `update_service` inherits the eighteen setters' documentation, and it should — clients cap on tool COUNT (most degrade at 40–60), while the prose an agent needs to use a field does not become unnecessary by moving. 30 setters folded into 5 patch tools (4 new); 4 stayed standalone with recorded reasons; zero ADR018 ✅ cells lost; REST and GraphQL unchanged.

## Tasks (in order)

| id   | title                                                        | est | depends_on |
| ---- | ------------------------------------------------------------ | --- | ---------- |
| t001 | Fix the target grammar from m70's inventory                   | 1h  | — — **DONE** |
| t002 | Fold the service-scoped setters into `update_service`         | 2h  | t001 — **DONE** |
| t003 | Fold the datastore / env-group / environment setters          | 2h  | t001 — **DONE** |
| t004 | Retire the old names through the deprecation ledger           | 45m | t002, t003 — **DONE** |
| t005 | Render parity                                                 | 45m | t004 — **DONE** |
| t006 | Simplify                                                      | 30m | t005 — **DONE** |
| t007 | Test coverage                                                 | 1h  | t005 — **DONE** |
| t008 | Closeout                                                      | 15m | t006, t007 — **DONE** |

## What m70 measured (the input to t001)

Upstream registers **22** MCP tools; bex registers **213**. Classes: **10 `Parity1to1`**, **1 `Superset`**, **8 `Divergent`**, **194 `Extension`**.

- **The 10 `Parity1to1` tools are untouchable** — `get_deploy`, `get_key_value`, `get_postgres`, `get_service`, `list_key_value`, `list_log_label_values`, `list_logs`, `list_postgres_instances`, `list_workspaces`, `query_render_postgres`. An agent written against Render's official server calls these by name.
- **All 34 `set_*` tools classify as `Extension`** — confirmed against the pin, not inferred from comments. They carry no parity obligation, so the fold is parity-neutral by construction.
- **Upstream deliberately walked away from `update_*`.** `v0.3.0` shipped `update_web_service`, `update_static_site`, and `update_cron_job` as placeholders returning a dashboard link; upstream **removed all three** in [#89](https://github.com/render-oss/render-mcp-server/pull/89) (2026-07-23, "stop exposing placeholder update tools that only return dashboard links"). Render's current answer to "how does an agent update a service over MCP" is *it does not*.

  **t001 must weigh this rather than assume `update_*` is the destination.** Folding into `update_service` is still defensible — the tools are bex extensions either way, and bex's `update_*` tools are real rather than placeholders — but "fold into `update_*`" is now a choice to argue for, not a default. Consider also whether some setters belong on `create_*`-shaped patch verbs, or whether the resource's existing REST `PATCH` shape is the better model to mirror.
- **Do not fold a `Divergent` tool.** Four of the eight have unintended argument bugs filed for repair (`create_postgres`, `create_static_site`, `get_metrics`, `trigger_deploy`). Repairing those is separate work; folding them would bury the bug.

## Definition of done

- Every tool m70 classified `Parity1to1` is untouched — same name, same arguments. That set is the parity contract and this milestone does not negotiate with it.
- The `set_*` tools m70 classified `Extension` are reachable through the resource's `update_*` tool instead, with each folded field an optional argument.
- Tool count drops by roughly 28 (34 `set_*` folding into ~6 `update_*`); the exact number comes from m70's inventory, not from this estimate.
- Zero ADR018 ✅ cells lost: every capability previously reachable over MCP is still reachable over MCP. ADR018's MCP column scores capability reachability, not tool-name identity (stated explicitly by m70/t006).
- REST and GraphQL are unchanged — this is an MCP-adapter change only, and the shared Service methods keep all three surfaces from drifting.
- The removal is recorded where the m55 deprecated-surface retirements are recorded, so an agent using an old name gets a documented answer rather than an unexplained tool-not-found.

## Source + Goal linkage

- **Source:** principal-engineer architecture review, 2026-08-18 (session hand-off, no inbox note). Measured at HEAD: 213 MCP tools totalling ~41 KB of descriptions plus ~37 KB of argument schemas — roughly 20–25k tokens of tool schema entering an MCP client's context before the user types anything. 34 are `set_*`; 17 `update_*` tools already exist.
- **Goal linkage:** ADR008's AI-native pillar. Most MCP clients degrade at 40–60 tools and several hard-cap at 128; Render's own server ships far fewer. The agent surface is the one bex differentiates on and currently the one hardest for an agent to use.
- **Expected outcome:** a materially smaller agent surface at identical capability. Every folded tool is one bex invented — `internal/apps/mcp.go:88` records "Render's official MCP has no update tools for any of them", and ADR018 repeats it in four rows ("bex extensions following the existing setter grammar").
- **Why now:** it follows immediately from m70 and is the reason m70 was worth doing. Deferring leaves the flagship AI surface at 213 tools while the pin makes the cost visible in CI but does nothing about it. It is also cheapest now — every additional feature milestone adds setters to the same grammar.
- **Render parity closing task included:** this changes a user-facing surface (MCP), so t005 checks REST/GraphQL/MCP consistency and re-walks the ADR018 rows the folded tools appear in.

## What shipped

**The grammar (t001, [`evidence/grammar.md`](evidence/grammar.md)).** All 34 `set_*` tools were confirmed `Extension` against m70's pin rather than assumed from comments, so the fold is parity-neutral by construction. The destination is REST's shape, not a new one: `PATCH /v1/services/{id}`'s ops table and the `PostgresPatch`/`KeyValuePatch`/`EnvironmentPatch` "nil = unchanged" contract already existed, so MCP joins the two surfaces that already agreed. m70's warning that upstream **removed** its own `update_*` tools (#89) was weighed and does not apply: those were placeholders returning a dashboard link; bex's do the work.

**The fold (t002/t003).**

| target | new? | absorbs |
| --- | --- | --- |
| `update_service` | new | 18 service setters |
| `update_postgres` | new | `set_postgres_ip_allow_list`, `set_postgres_parameter_overrides` |
| `update_key_value` | new | `set_key_value_ip_allow_list`, `set_key_value_maxmemory_policy` |
| `update_environment` | existing | `set_environment_acl` + the 4 membership setters |
| `update_project` | new | the 3 project membership setters |

`set_environment_acl` was **subsumed, not merely folded**: `update_environment` already carried the ACL triple, and the setter's full-replace contract ("pass the current value of anything you don't mean to change") was exactly the trap a patch tool removes.

**Four setters deliberately did not fold** — `set_env_var`, `set_secret_file`, `set_env_group_var`, `set_env_group_secret_file`. Each is a merge-one-key upsert paired with a `delete_*` verb, against a resource-level partner (`update_env_vars` / `update_env_group_vars`) that replaces the whole set. Folding them would make one tool mean both "replace everything" and "merge one key" — the blur t003 says to refuse.

**The contract, and what could have broken.** The fold's real risk is not "does a field still write" (each reaches the verb it always did) but that ONE call now carries many fields, so a bug can write a field the caller never mentioned. That is what the tests are aimed at: `TestUpdateServiceLeavesOmittedFieldsAlone` runs against a service where all eighteen settings are populated; `TestUpdateServiceBuildTriggeringFieldsStillTriggerABuild` pins which fields bump `restartedAt` (and which must not); `TestUpdateServiceMatchesRESTPatchFieldForField` requires the MCP path and the REST path to produce byte-identical CR specs.

**One deliberate behaviour change**, recorded in the ledger: the Postgres/Key Value allowlist writes now route through `UpdatePostgres`/`UpdateKeyValue` (REST `PATCH`'s method) instead of the dedicated setter, so they record a `DatabaseUpdated`/`KeyValueUpdated` effect they did not record before. Same relation (`can_operate`), same validation — an allowlist change is now audited like every other update.

**Deferred on purpose (t008's out-of-scope line), filed as [`w1/051.md`](../051.md):** the ~15 remaining per-field tools spelled `update_*` / `scale_*` / `rename_*` are the same grammar under a different prefix and would fold into the same four targets. They are not a rename-only pass — `dryRun` semantics on a multi-field patch, plan changes being billing events, and `update_cron_job`'s upstream name are three real decisions — so they belong in their own milestone, not smuggled into this one.
