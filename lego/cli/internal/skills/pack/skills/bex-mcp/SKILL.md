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
  version: "1.0.0"
  category: operations
---

# bex MCP Server

The bex MCP server lets AI coding tools manage bex resources directly. Its tools follow Render's MCP server, so the same tool names work. This skill covers **setup**, **authentication**, **workspace selection**, **capability discovery**, and **troubleshooting**.

Action skills (bex-deploy, bex-debug, bex-monitor) use MCP tools for their workflows. If MCP is not connected, set it up using this skill first.

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

For a headless Claude Code, pass an exchanged API-key token (see `references/bex-access.md`) instead:

```bash
claude mcp add --transport http bex https://api.bex.co/mcp --header "Authorization: Bearer ${BEX_TOKEN}"
```

### Cursor

Cursor's MCP configuration takes a credential rather than starting OAuth discovery. Add this to `~/.cursor/mcp.json`, where `BEX_TOKEN` holds an exchanged API-key token, restart Cursor, and verify with `list_services()`:

```json
{
  "mcpServers": {
    "bex": {
      "url": "https://api.bex.co/mcp",
      "headers": {
        "Authorization": "Bearer ${env:BEX_TOKEN}"
      }
    }
  }
}
```

An exchanged token lasts 15 minutes. Refresh it before it expires, by hand or with a wrapper script.

### Codex and other clients

Follow the current OAuth or token configuration fetched through `references/bex-access.md`. If the configuration reads its bearer token from an environment variable, that variable must be set where the client starts; restart the client after changing it.

### Empty results and workspace selection

MCP operations run against the active workspace. If `list_services()` connects successfully but returns no services, first call `get_selected_workspace()`, then `list_workspaces()` if the wrong workspace might be selected. Also verify that the signed-in account or API key has access to the intended workspace. Do not diagnose the server URL as the primary problem when the MCP call itself succeeds.

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
## Current documentation retrieval

bex implements Render's documented platform behavior, so this skill links Render's documentation as the reference for how a feature behaves, and bex's own documentation (`https://bex.co/docs`) for its CLI and MCP server. Read Render's pages for behavior only: operate through bex (`bex` CLI, `https://api.bex.co/mcp`, `https://dashboard.bex.co`), never through the Render CLI, MCP server or dashboard those pages mention. Where bex's behavior differs, bex's documentation wins.

Whenever this skill directs you to consult current documentation:

1. Retrieve the linked Markdown document directly with an available URL-fetching tool or HTTP client, such as `curl`. Do not substitute web-search summaries for the document.
2. Confirm that retrieval succeeded and returned the expected document, then read its contents. Saving a file or printing its path is not sufficient.
3. If the request fails or your tool cannot read the Markdown response, open and read the linked HTML version instead.
4. If neither version can be retrieved, disclose that the current reference is unavailable and follow any topic-specific fallback in the skill. Use bundled guidance only for stable constraints, and do not guess at changeable platform details.

When a task requires multiple references, apply this workflow to each one and distinguish the documents you verified from those that remain unavailable.
<!-- /shared:documentation-retrieval -->
