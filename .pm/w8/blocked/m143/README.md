# w8 · m143 — `bex skills` installs bex agent skills, not Render's

**Worker:** worker8 **Goal:** `bex skills install|list|update|remove` installs and manages a bex skills pack that operates bex (`bex` CLI, `https://api.bex.co/mcp`, `dashboard.bex.co`, `onbex.co`), never Render, and never touches a user's genuine Render skills or `~/.render/skills.yaml`. **Status:** blocked (t001–t006 done; t007 local acceptance done, hosted spot-check waits on the next `bex-cli/v*` release)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Derive the bex skills pack from render-oss/skills (MIT) with a pinned, reproducible generator and a content lint — **DONE** | 90m | — |
| t002 | Launcher-owned `bex skills` install/list/update/remove over the embedded pack, bex state file, explicit `bex-*` selections — **DONE** | 90m | t001 |
| t003 | Docs: bex-cli.md residual, cli-compatibility-checklist rows, UPSTREAM_RENDER_CLI.md re-diff note, ADR018 row — **DONE** | 20m | t002 |
| t004 | Render parity check: same subcommands, flags and JSON result shape as the pinned Render CLI — **DONE** | 15m | t002 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage — **DONE** | 15m | t005 |
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

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m143。
- **结论来源：** 2026-10-29 GitHub release / 当前源码复核。Bex 最新发布仍是 2026-10-16 的 `bex-cli/v0.3.3`，早于本项 10-28/29 完成的 launcher / pin 工作。
- **处理：** 原本地/候选验收保留，继续 blocked 等用户明确 `$release cli`，再按本项原 published-binary DoD 验收并清理。`$unblock` 不授权发 tag / 发布；本次也未发布。与 090 不同，v0.3.3 不含这里的新变更。
- **worker 下一步：** 发布实际完成后才恢复最终任务；不能用 main 或 candidate 的成功关闭最终交付门槛。
