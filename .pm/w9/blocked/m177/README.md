# w9 · m177 — Cloud Agents from the terminal and the editor: `bex agent` and bex desktop MCP preconfig

**Worker:** worker9 **Goal:** `bex agent run/wait/get/list/send/logs/files/cancel` drives cloud agents from CI and scripts, and bex desktop ships with `/mcp/cloud-agents` preconfigured for its signed-in user. **Status:** blocked — implementation done; waiting on the desktop live check and the CLI release authorization (user)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | `bex agent` commands in the CLI launcher — **DONE** | 120m | — |
| t002 | bex desktop preconfigures /mcp/cloud-agents | 60m | — |
| t003 | Docs and release | 30m | t001, t002 |
| t004 | Render parity (Devin reference for agent surfaces) — **DONE** | 25m | t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage — **DONE** | 45m | t004, t005 |
| t007 | Closeout | 10m | t006 |

## Definition of done

- From a CI job, `bex agent run --attach f --wait -o json` returns the outcome (PR, files, result) with correct exit codes; bex desktop shows cloud-agent tools for a signed-in user.

## Source + Goal linkage

- **Source:** user request 2026-11-05 ("把 w21 以及需要跟进这些问题移动到 w9 然后你统一跟进解决", `/pm them all`), from the ADR094/ADR095 recap next-steps list.
- **Goal linkage:** ADR094 (Cloud Agents MCP) extended to CLI and desktop clients; ADR008 AI-native surfaces.
- **Expected outcome:** customers delegate agent work from pipelines and from the editor, not only the dashboard.
- **Why now:** ADR094/095 contracts are complete; clients are the remaining reach gap.
- **Parity:** included — Devin reference (Devin has API + CLI + IDE).

## Parked (2026-11-06)

Implementation is done:

- t001 `bex agent`, and t004–t006 parity, simplify and tests, all shipped. The CLI flags include `--skill`, `--approval-mode`, `--bypass-approval` and `answer`.
- t002 bex-desktop `bex-cloud-agents` context server: bex-desktop `8058754ec2`, plus the platform-client registry fix in bex `09bd35564`.
- t003 docs.

Two gates remain, both the user's:

1. **Desktop live check (t002):** on a machine with a current bex-desktop build signed in to bex, confirm the `bex-cloud-agents` context server lists the `/mcp/cloud-agents` tools. This needs a GUI sign-in the worker cannot do.
2. **CLI release (t003):** authorize `/release cli`. Every backend the new commands call is live on production (pins 24a9203ee→669e6115c). The previous release authorization covered v0.4.0 only.

After both, the worker runs the release and closes t007.
