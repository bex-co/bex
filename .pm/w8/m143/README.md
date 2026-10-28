# w8 · m143 — `bex skills` installs bex agent skills, not Render's

**Worker:** worker8 **Goal:** `bex skills install|list|update|remove` installs and manages a bex skills pack that operates bex (`bex` CLI, `https://api.bex.co/mcp`, `dashboard.bex.co`, `onbex.co`), never Render, and never touches a user's genuine Render skills or `~/.render/skills.yaml`. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Derive the bex skills pack from render-oss/skills (MIT) with a pinned, reproducible generator and a content lint | 90m | — |
| t002 | Launcher-owned `bex skills` install/list/update/remove over the embedded pack, bex state file, explicit `bex-*` selections | 90m | t001 |
| t003 | Docs: bex-cli.md residual, cli-compatibility-checklist rows, UPSTREAM_RENDER_CLI.md re-diff note, ADR018 row | 20m | t002 |
| t004 | Render parity check: same subcommands, flags and JSON result shape as the pinned Render CLI | 15m | t002 |
| t005 | Simplify | 15m | t004 |
| t006 | Test coverage | 15m | t005 |
| t007 | Closeout: sandboxed-HOME acceptance from a local build; hosted spot-check after the next `bex-cli/v*` release | 20m | t006 |

## Definition of done

- In a sandboxed `HOME`, `bex skills install --confirm -o json` installs only `bex-*` skills. A grep of the installed directories for user-directing Render references (`render` CLI commands, `mcp.render.com`, `dashboard.render.com`, `api.render.com`, `onrender.com`, `render.com/pricing`) finds nothing; attribution/license lines are allowed. The MCP skill points at `https://api.bex.co/mcp`.
- In a `HOME` that already holds `render-*` skills and `~/.render/skills.yaml`, `bex skills install|update|remove` leaves both untouched. State lives in the bex config dir.
- `bex skills list|update|remove -o json --confirm` do not panic non-interactively.
- Skills for bex non-goals (workflows, sandboxes) are not shipped.

## Source + Goal linkage

- **Source:** promoted from [w8/212](../done/212.md) (`/qa-find-bugs-cli` loop sweep 3093, 2026-10-28).
- **Goal linkage:** ADR008 AI-native pillar and Render parity: on Render, `render skills install` gives an agent skills for the platform the user is on.
- **Expected outcome:** an agent-first bex user's coding agent learns to operate bex, not a different platform and account.
- **Why now:** major severity. It is the first command an AI-native user runs, the help text already promises bex skills, and the failure is silent.
- **Render parity:** included (t004). The command surface stays Render's, and only the content and state location are bex's. The w8/m29 t004 "branding residual" is withdrawn: it was judged as copy, but its effect is functional.
