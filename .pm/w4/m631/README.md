# w4 · m631 — `/plugin marketplace add bex-co/bex` then `/plugin install bex` gives Claude Code bex's skills, MCP server, commands, assistant and render.yaml check, as Render's plugin does

**Worker:** worker4 **Goal:** Claude Code users set bex up the way Render tells them to set Render up — with a plugin — and get the same bundle: skills, the MCP server with OAuth, two slash commands, an assistant agent and a render.yaml validation hint. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | packgen emits the bex Claude Code plugin and the repository's marketplace | 1h45m | w4/m630/t001 |
| [t002](t002.md) | The render.yaml hook only suggests validation, and is tested | 40m | t001 |
| [t003](t003.md) | CI validates the plugin and marketplace | 25m | t001 |
| [t004](t004.md) | Docs make the plugin the preferred Claude Code setup | 30m | t001 |
| [t005](t005.md) | Live: a fresh Claude Code installs the plugin and reaches production | 1h | t002, t003, t004, w4/m629/t005 |
| [t006](t006.md) | Render parity | 25m | t005 |
| [t007](t007.md) | Simplify | 15m | t006 |
| [t008](t008.md) | Test coverage | 30m | t006, t007 |
| [t009](t009.md) | Closeout | 10m | t008 |

Total: **9 tasks, about 5h40m**.

## Definition of done

In a fresh Claude Code, `/plugin marketplace add bex-co/bex` then `/plugin install bex` installs the 20 `bex:bex-*` skills, `bex-assistant`, `/deploy-to-bex`, `/check-bex-status`, the render.yaml hook and the MCP server `bex` at `https://api.bex.co/mcp`. First use signs in through Claude Code's own OAuth with no client id or API key, and `list_workspaces` answers. Editing `render.yaml` injects the `bex blueprints validate` suggestion and runs nothing. `claude plugin validate --strict` and the plugin-equals-pack test pass in CI. The docs give the sparse-clone form.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one more aspect of render parity for w4`, 2026-11-24 (the aspect: how a coding agent connects to bex, as Render's agent docs describe), each item re-verified by an independent agent on 2026-11-24/25 before filing; filed by `/pmx them all to w4` at `origin/main` `50a5ddfdfc`.
- **Evidence (verified 2026-11-24/25):** Render's preferred Claude Code setup is its plugin (`render.com/agents/claude.md`: "Preferred setup method: Claude Code plugin", `/plugin marketplace add render-oss/render-plugin-claude-code; then /plugin install render`; `render.com/docs/mcp-server.md`: `/plugin install render@claude-plugins-official`). bex has no `.claude-plugin`, `marketplace.json` or `plugin.json` anywhere, no plugin repo in `bex-co`, and no board item for one.
- **Goal linkage:** ADR008 AI-native; Render parity of the agent setup path.
- **Expected outcome:** one command gives a Claude Code user everything needed to deploy and operate bex.
- **Why now:** w4/m629 makes OAuth work for real clients and w4/m630 refreshes the pack this plugin copies; w4/m632's setup procedure names the plugin as Claude Code's preferred path.
- **Render parity:** included. Differences recorded, not signed off: bex's `.mcp.json` has no fixed client id (bex uses DCR); bex keeps 20 skills (no Workflows, no Sandboxes); no official-marketplace listing until the user submits one.
