# w1 · m339 — Session lifecycle verbs, list filters and workspace agent configuration in `bex agent`

**Worker:** worker1 **Goal:** every agent-session and workspace agent-configuration route bex-api already serves is reachable from `bex agent`, with secret values never on argv. **Status:** done (2026-11-12; verified on production, see Live check)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Dashboard URLs as `<session>`, list filters, `run --parent` — **DONE** | 60m | — |
| t002 | `archive`/`unarchive`/`delete`/`tags`/`tree`/`cost` — **DONE** | 75m | t001 |
| t003 | `secrets list/set/delete` with values never on argv — **DONE** | 45m | — |
| t004 | `skills list/get/create/update/delete/versions` — **DONE** | 60m | — |
| t005 | `env`, `env set` and `models` — **DONE** | 60m | — |
| t006 | Contract fixture additions and backend twin — **DONE** | 45m | t002, t003, t004, t005 |
| t007 | Document the new verbs — **DONE** | 30m | t006 |
| t008 | Live check on production with a locally built binary — **DONE** | 45m | t007 |
| t009 | Render parity (Devin reference) — **DONE** | 15m | t008 |
| t010 | Simplify — **DONE** | 15m | t009 |
| t011 | Test coverage — **DONE** | 30m | t009 |
| t012 | Closeout — **DONE** | 10m | t011 |

## Definition of done

1. Against production with a locally built `bex`: `list --mine/--created-by/--automation/--parent/--phase/--since/--until` filter as the API does; `run --parent <session>` starts a child session; a dashboard session URL works wherever `<session>` is accepted.
2. `archive`, `unarchive`, `delete` (TTY confirm or `--confirm`), `tags`, `tree` and `cost` work and print the API's answer with `-o json`.
3. `secrets list/set/delete`, `skills list/get/create/update/delete/versions`, `env`, `env set` and `models` work; a test pins that no secret value or model key is accepted as a flag argument (stdin or `--from-env` only).
4. The contract fixtures cover the new shapes and the backend twin test guards them.
5. `docs/bex-cli.md` documents the new verbs.

## Source + Goal linkage

- **Source:** ADR106 D3 (filters, `--parent`, URLs), D4 and D8. Approved `/pm-brainstorm ADR106 for w1` item 2 via `/pmx`, 2026-11-11. Design: [ADR106](../../../../docs/ADR106-cli-cloud-agents-automations.md).
- **Goal linkage:** ADR008 AI-native surfaces; Devin CLI parity (`/archive`, `devin list`, subagents) for the agent routes of ADR065, ADR094 w7/m296, ADR098, ADR099, ADR102.
- **Expected outcome:** a workspace can be set up for Cloud Agents and its sessions managed without the dashboard; m340's chat slash commands reuse these verbs.
- **Why now:** all of these routes are shipped and reachable only from the dashboard, GraphQL and MCP; m340 depends on the lifecycle verbs and URL references. Render parity is included with Devin as the reference product; REST/GraphQL/MCP are unchanged.

## Live check (t008, 2026-11-12)

A locally built `bex`, signed in as the QA user through the isolated device login used for m338, ran in the `bex` workspace with `qa-20261112-m339*` fixtures only.

| Check | Result |
| --- | --- |
| `models` | `Ready true`, `GitHub connected (bex-co)`. `claude` is ready; `codex` and `gemini` show "needs the workspace's own model key" |
| `env`, then `env set --default-model ""` | The env read showed the network tier, hosts, model source, `$10.00 a month, $6.34 used`, trust tier `T0`. The set was a no-op (the default model was already the provider default), and the env JSON before and after compared equal apart from `updatedAt`. `env set` with no flag exited 2 |
| Secrets | `printf … \| secrets set QA_20261112_M339` set it, and the value was never echoed. `list` showed the name. `delete` without `--confirm` exited 2; with it, deleted. `list` no longer showed it |
| Skills | `create -f` saved version 1 and `update -f` version 2. `list` showed version 2, enabled, `user,model`. `get` printed v2's body and `get --version 1` v1's. `versions` showed `2 (current)` and `1` with short ids. `delete --confirm` deleted it |
| `run` and `cost` | A parent fixture succeeded. `cost` printed 3 requests, 21967/198 tokens, model `deepseek/deepseek-v4.1-flash-1017` |
| `tree` by URL | `tree https://dashboard.bex.co/agents/ags-…` printed the one-node tree with the asked session marked |
| `tags` | `tags … qa qa:m339` set `['qa', 'qa:m339']`; `--clear` emptied them |
| Filters | `list --mine --since 1h` returned the fixture. `list --parent <URL>` returned an empty list (no child). `list --automation rt-fix-logic-bugs --since 2d` resolved the name to its `aut-` id and returned only that automation's 2 sessions |
| `archive`, `unarchive`, `delete` | `archive` of three ids archived all three, and `list --archived true` listed them. `unarchive` worked. `delete` without `--confirm` exited 2; with it, all three fixtures and m338's two fire sessions were deleted |
| `run --parent` | The API received and enforced the parent link: `409 AGENT_SESSION_PARENT_NOT_ACTIVE` once the parent had settled, and `400 AGENT_SESSION_CHILD_SPEND_EXCEEDS_PARENT` for a child ceiling above the parent's remainder. Retried at 12:27 UTC, when no other session ran: `run --parent https://dashboard.bex.co/agents/<parent>` started child `ags-dbqr1594…` under a running parent (`--max-spend 2` / `0.5`). `tree <child>` showed the parent with the child indented and marked, and `list --parent <parent>` returned exactly the child. Both succeeded; the settled tree showed both, and both were deleted |

Found and fixed during the check: a session id with a space (`"ags-a ags-b"`, which zsh passes unsplit) passed the prefix-only id check and reached the API as a 403. Ids are now `ags-` plus the lowercase letters and digits of an xid, with a test.
