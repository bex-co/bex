# w9 · m177 — Cloud Agents from the terminal and the editor: `bex agent` and bex desktop MCP preconfig

**Worker:** worker9 **Goal:** `bex agent run/wait/get/list/send/logs/files/cancel` drives cloud agents from CI and scripts, and bex desktop ships with `/mcp/cloud-agents` preconfigured for its signed-in user. **Status:** in progress — t001, t004–t006 done; t002 shipped in bex-desktop and awaits its live check; t003 docs written, release pending

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
