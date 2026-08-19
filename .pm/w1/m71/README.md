# w1 · m71 — Collapse the bex-invented `set_*` MCP tool grammar

> **Gate cleared 2026-08-18** — `.pm/w1/done/m70/` landed the pin and the measured inventory. Scope below is now written from real numbers, not estimates: [m70's inventory](../done/m70/evidence/inventory.md).

**Worker:** worker1 **Goal:** cut the agent-facing tool count without losing a single ADR018 parity cell, by folding bex's own per-field setter grammar into the `update_*` tools bex already has. **Status:** todo (m70 gate cleared)

## Tasks (in order)

| id   | title                                                        | est | depends_on |
| ---- | ------------------------------------------------------------ | --- | ---------- |
| t001 | Fix the target grammar from m70's inventory                   | 1h  | —          |
| t002 | Fold the service-scoped setters into `update_service`         | 2h  | t001       |
| t003 | Fold the datastore / env-group / environment setters          | 2h  | t001       |
| t004 | Retire the old names through the deprecation ledger           | 45m | t002, t003 |
| t005 | Render parity                                                 | 45m | t004       |
| t006 | Simplify                                                      | 30m | t005       |
| t007 | Test coverage                                                 | 1h  | t005       |
| t008 | Closeout                                                      | 15m | t006, t007 |

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
