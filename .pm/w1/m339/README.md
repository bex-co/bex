# w1 · m339 — Session lifecycle verbs, list filters and workspace agent configuration in `bex agent`

**Worker:** worker1 **Goal:** every agent-session and workspace agent-configuration route bex-api already serves is reachable from `bex agent`, with secret values never on argv. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Dashboard URLs as `<session>`, list filters, `run --parent` | 60m | — |
| t002 | `archive`/`unarchive`/`delete`/`tags`/`tree`/`cost` | 75m | t001 |
| t003 | `secrets list/set/delete` with values never on argv | 45m | — |
| t004 | `skills list/get/create/update/delete/versions` | 60m | — |
| t005 | `env`, `env set` and `models` | 60m | — |
| t006 | Contract fixture additions and backend twin | 45m | t002, t003, t004, t005 |
| t007 | Document the new verbs | 30m | t006 |
| t008 | Live check on production with a locally built binary | 45m | t007 |
| t009 | Render parity (Devin reference) | 15m | t008 |
| t010 | Simplify | 15m | t009 |
| t011 | Test coverage | 30m | t009 |
| t012 | Closeout | 10m | t011 |

## Definition of done

1. Against production with a locally built `bex`: `list --mine/--created-by/--automation/--parent/--phase/--since/--until` filter as the API does; `run --parent <session>` starts a child session; a dashboard session URL works wherever `<session>` is accepted.
2. `archive`, `unarchive`, `delete` (TTY confirm or `--confirm`), `tags`, `tree` and `cost` work and print the API's answer with `-o json`.
3. `secrets list/set/delete`, `skills list/get/create/update/delete/versions`, `env`, `env set` and `models` work; a test pins that no secret value or model key is accepted as a flag argument (stdin or `--from-env` only).
4. The contract fixtures cover the new shapes and the backend twin test guards them.
5. `docs/bex-cli.md` documents the new verbs.

## Source + Goal linkage

- **Source:** ADR106 D3 (filters, `--parent`, URLs), D4 and D8. Approved `/pm-brainstorm ADR106 for w1` item 2 via `/pmx`, 2026-11-11. Design: [ADR106](../../../docs/ADR106-cli-cloud-agents-automations.md).
- **Goal linkage:** ADR008 AI-native surfaces; Devin CLI parity (`/archive`, `devin list`, subagents) for the agent routes of ADR065, ADR094 w7/m296, ADR098, ADR099, ADR102.
- **Expected outcome:** a workspace can be set up for Cloud Agents and its sessions managed without the dashboard; m340's chat slash commands reuse these verbs.
- **Why now:** all of these routes are shipped and reachable only from the dashboard, GraphQL and MCP; m340 depends on the lifecycle verbs and URL references. Render parity is included with Devin as the reference product; REST/GraphQL/MCP are unchanged.
