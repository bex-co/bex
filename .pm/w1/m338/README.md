# w1 · m338 — `bex agent automations`: manage Agent Automations from the terminal and from files in a repo

**Worker:** worker1 **Goal:** every ADR101 automation operation is a `bex agent automations` command, automations can be kept as YAML in a repository and applied declaratively, and CI can test one with `run --wait` and branch on the exit code. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Automations group, `aut-`/name references, `list`/`get`/`runs` with the schedule in words | 60m | — |
| t002 | Schedule flags build the one `schedule` trigger | 60m | t001 |
| t003 | `create` and `update`: template flags, refused flags, read-modify-write with `ifVersion` | 75m | t002 |
| t004 | `delete`/`pause`/`resume`/`take-over`, run-now with `--wait`/`--follow`, exit code 6 | 60m | t003 |
| t005 | `apply -f` (declarative, never deletes) and `export` | 90m | t003 |
| t006 | Automation contract fixture and its backend twin | 45m | t001 |
| t007 | Document `bex agent automations` and exit code 6 | 30m | t004, t005 |
| t008 | Live check on production with a locally built binary | 45m | t007 |
| t009 | Render parity (Devin reference) | 15m | t006, t008 |
| t010 | Simplify | 15m | t009 |
| t011 | Test coverage | 30m | t009 |
| t012 | Closeout | 10m | t011 |

## Definition of done

1. A locally built `bex`, authenticated against production with the QA workspace's credential, creates an automation from flags (`create --every day --at … --max-spend …`) and another from a YAML file (`apply -f`), lists and gets both with the schedule rendered in words, the next fire and the last run.
2. `automations run <name> --wait` exits by the started session's state; a run that started nothing (`skipped_overlap`, `denied`, `failed_create`) exits 6 and names the outcome on stderr.
3. `pause`, `resume`, `take-over` (refused for a non-admin with the API's own error) and `delete` work; destructive verbs need a TTY confirmation or `--yes`.
4. `update` sends `ifVersion` and a concurrent edit surfaces `AGENT_AUTOMATION_CONFLICT` with a re-run hint; `export <name> | apply -f - --dry-run` reports no change; `apply` never deletes.
5. `lego/cli/testdata/agent-automation-cli-contract.json` is decoded by the CLI tests and checked against the real backend types by a backend twin test.
6. `docs/bex-cli.md` documents the group and exit code 6.

## Source + Goal linkage

- **Source:** ADR106 D7 and § CLI specification `bex agent automations`; ADR101 D8 (the ten operations). Approved `/pm-brainstorm ADR106 for w1` item 1 via `/pmx`, 2026-11-11. Design: [ADR106](../../../docs/ADR106-cli-cloud-agents-automations.md).
- **Goal linkage:** ADR008 AI-native surfaces; Devin parity for Automations, extended (Devin has no automations CLI, only the v3 API and Terraform).
- **Expected outcome:** recurring agent work is scriptable and reviewable as code; CI can validate an automation before merging it.
- **Why now:** the automations API has been live since w21/m278 and the only clients are the dashboard, GraphQL and MCP. It needs no server change, so it is the cheapest, highest-value phase of ADR106. Render parity is included with Devin as the reference product (an agent surface); REST/GraphQL/MCP are unchanged. Publishing a CLI release stays a separate, user-authorized `/release cli`.
