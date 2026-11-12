# w1 · m338 — `bex agent automations`: manage Agent Automations from the terminal and from files in a repo

**Worker:** worker1 **Goal:** every ADR101 automation operation is a `bex agent automations` command, automations can be kept as YAML in a repository and applied declaratively, and CI can test one with `run --wait` and branch on the exit code. **Status:** done (2026-11-12; verified on production, see Live check)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Automations group, `aut-`/name references, `list`/`get`/`runs` with the schedule in words — **DONE** | 60m | — |
| t002 | Schedule flags build the one `schedule` trigger — **DONE** | 60m | t001 |
| t003 | `create` and `update`: template flags, refused flags, read-modify-write with `ifVersion` — **DONE** | 75m | t002 |
| t004 | `delete`/`pause`/`resume`/`take-over`, run-now with `--wait`/`--follow`, exit code 6 — **DONE** | 60m | t003 |
| t005 | `apply -f` (declarative, never deletes) and `export` — **DONE** | 90m | t003 |
| t006 | Automation contract fixture and its backend twin — **DONE** | 45m | t001 |
| t007 | Document `bex agent automations` and exit code 6 — **DONE** | 30m | t004, t005 |
| t008 | Live check on production with a locally built binary — **DONE** | 45m | t007 |
| t009 | Render parity (Devin reference) — **DONE** | 15m | t006, t008 |
| t010 | Simplify — **DONE** | 15m | t009 |
| t011 | Test coverage — **DONE** | 30m | t009 |
| t012 | Closeout — **DONE** | 10m | t011 |

## Definition of done

1. A locally built `bex`, authenticated against production with the QA workspace's credential, creates an automation from flags (`create --every day --at … --max-spend …`) and another from a YAML file (`apply -f`), lists and gets both with the schedule rendered in words, the next fire and the last run.
2. `automations run <name> --wait` exits by the started session's state; a run that started nothing (`skipped_overlap`, `denied`, `failed_create`) exits 6 and names the outcome on stderr.
3. `pause`, `resume`, `take-over` (refused for a non-admin with the API's own error) and `delete` work; destructive verbs need a TTY confirmation or `--confirm`.
4. `update` sends `ifVersion` and a concurrent edit surfaces `AGENT_AUTOMATION_CONFLICT` with a re-run hint; `export <name> | apply -f - --dry-run` reports no change; `apply` never deletes.
5. `lego/cli/testdata/agent-automation-cli-contract.json` is decoded by the CLI tests and checked against the real backend types by a backend twin test.
6. `docs/bex-cli.md` documents the group and exit code 6.

## Source + Goal linkage

- **Source:** ADR106 D7 and § CLI specification `bex agent automations`; ADR101 D8 (the ten operations). Approved `/pm-brainstorm ADR106 for w1` item 1 via `/pmx`, 2026-11-11. Design: [ADR106](../../../../docs/ADR106-cli-cloud-agents-automations.md).
- **Goal linkage:** ADR008 AI-native surfaces; Devin parity for Automations, extended (Devin has no automations CLI, only the v3 API and Terraform).
- **Expected outcome:** recurring agent work is scriptable and reviewable as code; CI can validate an automation before merging it.
- **Why now:** the automations API has been live since w21/m278 and the only clients are the dashboard, GraphQL and MCP. It needs no server change, so it is the cheapest, highest-value phase of ADR106. Render parity is included with Devin as the reference product (an agent surface); REST/GraphQL/MCP are unchanged. Publishing a CLI release stays a separate, user-authorized `/release cli`.

## Live check (t008, 2026-11-12)

A locally built `bex` (this tree, `go build` in `lego/cli`) was signed in as the QA user through its own device login: an isolated `BEX_CLI_CONFIG_DIR`, approved headlessly with `scripts/qa-login.sh --login-challenge`. No code, cookie or token was recorded. `bex-canary` refused the template with the API's own `400 AGENT_SESSION_MODEL_KEY_REQUIRED` (no workspace model key). The checks therefore ran in `bex`, as w1/m171's fixtures did. They touched only `qa-20261112-m338-*` fixtures; the workspace's eight `rt-*` automations were left alone.

| Check | Command | Result |
| --- | --- | --- |
| Create from flags | `create --name qa-20261112-m338-daily --every day --at 03:17 --timezone Europe/Berlin --max-spend 1 …` | exit 0. `every day at 03:17 Europe/Berlin`, next fire `2026-11-13 03:17 CET`, version 1 |
| Apply from YAML | `apply -f qa.yaml --dry-run`, then without `--confirm` (no TTY), then with `--confirm` | `will create …` (no write), then exit 2 `needs confirmation: pass --confirm`, then `created qa-20261112-m338-yaml` |
| Get, schedule in words | `get qa-20261112-m338-yaml` | `every week on Sun at 04:05 UTC, ends before 2026-12-01 00:00` |
| Update with `ifVersion` | `update qa-20261112-m338-daily --at 03:30` | exit 0, `at 03:30`, version 2 |
| Round trip | `export <both> \| apply -f - --dry-run`; re-running `apply -f qa.yaml --dry-run` | both `unchanged`, exit 0 |
| Run now | `run qa-20261112-m338-daily` | `started ags-dbqn5paibtfs738vkq5g` |
| Exit 6 | `run qa-20261112-m338-daily` again while that session worked | `skipped_overlap (the previous run's session … is still working)`, **exit 6** |
| `run --wait` | after the first session succeeded: `run … --wait --timeout 15m -o json` | exit 0. Stdout is the settled session `ags-dbqn5uq6krac73d83760` (succeeded, summary `OK`, $0.0039) |
| Runs | `runs qa-20261112-m338-daily` | the `skipped_overlap` and `started` rows with the session id |
| Pause, resume | `pause`, `resume` | `paused`, then `active` with the next fire recomputed |
| Take-over, delete | without `--confirm`, then with `--confirm` | exit 2 each without `--confirm`. `take-over --confirm` succeeded: the QA user is an admin, so the non-admin refusal stays covered only by the API's own tests. `delete --confirm`: `Deleted automation …` for both |
| Cleanup | `list` | no `qa-20261112` row is left. The two sessions the fires started (succeeded, about $0.009 together) stay in the session list as ordinary history |

The `AGENT_AUTOMATION_CONFLICT` path (a stale `ifVersion`) cannot be provoked from one client without racing a second writer. Unit tests pin it: a single PATCH, exit 1 and the re-run hint.

**Re-check after `/simplify` (same day, rebuilt binary).** The review moved confirmation from `--yes` to upstream's global `--confirm` (the Render CLI's own "skip all confirmation prompts"). It also made `apply` plan from one list read. A second live pass on `qa-20261112-m338-r2*` showed:

- `create` without `--timezone` printed `Time zone: America/Los_Angeles (this machine's …)` and used it;
- `apply --dry-run`, then `apply --confirm`, created the YAML fixture;
- `export … | apply -f - --dry-run` reported both `unchanged`;
- `delete` without `--confirm` exited 2, then `--confirm` deleted both.

The pass also found a gap. `update --every week --on mon,fri` on a daily automation dropped the stored `at`, and the API refused it (`AGENT_AUTOMATION_SCHEDULE_INVALID: at is required for a week schedule`). A new frequency now keeps the stored fields it still uses. Re-checked live on `qa-20261112-m338-r3`: `every day at 05:45 UTC` became `every week on Mon, Fri at 05:45 UTC` (version 2), and the fixture was deleted. No `qa-20261112` automation remains.
