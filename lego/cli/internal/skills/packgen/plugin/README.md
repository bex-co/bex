# bex plugin for Claude Code

Deploy, debug, and monitor applications on [bex](https://bex.co) from Claude Code, as Render's plugin does for Render. Installing it gives Claude Code:

- the 20 `bex-*` skills (`bex:bex-deploy`, `bex:bex-debug`, `bex:bex-monitor`, …), the same ones `bex skills install` installs;
- the `bex-assistant` agent;
- the `/deploy-to-bex` and `/check-bex-status` commands;
- a hook that, after Claude edits a `render.yaml`, suggests validating it with `bex blueprints validate` (it never runs the CLI itself);
- bex's MCP server `bex` at `https://api.bex.co/mcp`, which signs you in through Claude Code's own OAuth on first use (`/mcp`): no client ID or API key to configure.

## Install

In Claude Code:

```text
/plugin marketplace add bex-co/bex
/plugin install bex@bex-plugins
```

`bex-co/bex` is a large repository. From a shell, a sparse checkout fetches only the plugin:

```sh
claude plugin marketplace add bex-co/bex --sparse .claude-plugin plugins
claude plugin install bex@bex-plugins
```

The `bex blueprints validate` suggestion needs the bex CLI (https://bex.co/docs/cli#install-bex) and `bex login`. The hook needs `jq`; without it, it stays silent.

## Maintainers

This directory is generated: edit `lego/cli/internal/skills/packgen/plugin/` and the skills pack in `lego/cli/internal/skills/`, then run `go run ./hack/claudeplugin` from `lego/cli`.
