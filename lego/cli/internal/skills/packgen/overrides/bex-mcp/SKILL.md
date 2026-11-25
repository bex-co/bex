---
name: bex-mcp
description: >-
  Connects and configures the bex MCP server for AI coding tools—setup per
  tool (Claude Code, Cursor, Codex), authentication, workspace selection,
  capability discovery, and troubleshooting. Use when MCP is not configured, list_services()
  fails, the user asks about bex MCP setup, or an action skill needs MCP
  but it's not connected yet.
  Trigger terms: MCP, bex MCP, list_services, MCP setup, MCP server,
  OAuth, API key, Bearer token, api.bex.co/mcp, workspace selection.
license: MIT
compatibility: bex MCP server (hosted at api.bex.co/mcp)
metadata:
  author: bex
  version: "1.1.0"
  category: operations
---

# bex MCP Server

The bex MCP server lets AI coding tools manage bex resources directly. Its tools follow Render's MCP server, so the same tool names work. This skill covers **setup**, **authentication**, **workspace selection**, **capability discovery**, and **troubleshooting**.

Action skills (bex-deploy, bex-debug, bex-monitor) use MCP tools for their workflows. If MCP is not connected, use the bex CLI when it can complete the task, or set up MCP using this skill.

Before configuring, troubleshooting, or using bex MCP, read `references/bex-access.md` for the current server, authentication, workspace, capability-discovery, and fallback workflow.

## When to Use

- `list_services()` fails or MCP tools are unavailable
- First-time bex MCP setup for any AI tool
- User asks how to connect their AI tool to bex
- Switching workspaces or troubleshooting auth errors
- Discovering which MCP tools exist and what they do

## Current Setup and Capabilities

Follow `references/bex-access.md` to fetch the current MCP documentation, configure the client, verify read-only access, select the intended workspace, and inspect the tools actually available. Do not rely on a memorized tool catalog.

If the user has not identified their AI tool, ask which tool they use before giving client-specific setup instructions. Prefer the browser-based OAuth sign-in when the client supports MCP's authorization discovery: no credential is stored in its configuration.

### Claude Code

```bash
claude mcp add --transport http bex https://api.bex.co/mcp
```

Claude Code discovers bex's OAuth server from the endpoint's 401. Sign in from a session with `/mcp`, or ahead of time with `claude mcp login bex`; both open the bex dashboard login in a browser. Verify with `list_services()`. To replace a stale configuration, run `claude mcp remove bex` before adding it again.

For a headless Claude Code, pass a bex API key (see `references/bex-access.md`) as the bearer token instead:

```bash
claude mcp add --transport http bex https://api.bex.co/mcp --header "Authorization: Bearer ${BEX_API_KEY}"
```

### Cursor

Cursor's MCP configuration takes a credential rather than starting OAuth discovery. Add this to `~/.cursor/mcp.json`, where `BEX_API_KEY` holds a bex API key, restart Cursor, and verify with `list_services()`:

```json
{
  "mcpServers": {
    "bex": {
      "url": "https://api.bex.co/mcp",
      "headers": {
        "Authorization": "Bearer ${env:BEX_API_KEY}"
      }
    }
  }
}
```

### Codex and other clients

Follow the current OAuth or API-key configuration fetched through `references/bex-access.md`. If the configuration reads its bearer token from an environment variable, that variable must be set where the client starts; restart the client after changing it.

### Empty results and workspace selection

If `list_services()` connects successfully but returns no services, check the call's `workspaceId`. Use `list_workspaces()` if the intended workspace has not been confirmed, then pass that ID explicitly on related resource calls. bex has no session-based workspace selection: a call without `workspaceId` runs in the account's default workspace. Also verify that the signed-in account or API key has access to the intended workspace. If API-key auth is configured, check that `BEX_API_KEY` is available where the client starts. Do not diagnose the server URL as the primary problem when the MCP call itself succeeds.

### Blueprint capabilities

Inspect the current tools for `create_blueprint`, `plan_blueprint`, `approve_blueprint`, `update_blueprint`, and `get_blueprint`; these are public-alpha capabilities. Use **bex-deploy** for their lifecycle and approval workflow. If these tools are absent, use a capable bex CLI (`bex ea blueprints`) or the bex dashboard instead of assuming all MCP clients expose the same tool set.

## Troubleshooting

See `references/troubleshooting.md` for connection errors, auth failures, timeout issues, and tool-specific quirks.

## References

| Document | Contents |
|----------|----------|
| `references/bex-access.md` | Current MCP and CLI access, authentication, workspace, capability-discovery, and authorization workflow |
| `references/troubleshooting.md` | Connection errors, auth failures, tool-specific issues, timeout handling |

## Related Skills

- **bex-deploy** — Deploy flows using MCP tools
- **bex-debug** — Debug failures using MCP logs and metrics
- **bex-monitor** — Monitor health using MCP metrics
- **bex-cli** — CLI alternative when MCP is unavailable

<!-- shared:documentation-retrieval -->
<!-- /shared:documentation-retrieval -->
