# bex MCP Troubleshooting

## Connection Errors

### "MCP server not found" or tool not available

**Cause:** MCP server not configured in the AI tool.

**Fix:** Follow the current setup and verification workflow in [bex-access.md](bex-access.md).

### "Connection refused" or timeout

**Cause:** Network issue or incorrect URL.

**Fix:** Confirm the endpoint is `https://api.bex.co/mcp` (streamable HTTP) using [bex-access.md](bex-access.md), check internet connectivity, and account for corporate firewalls that might block MCP connections.

### "Transport error" or "SSE not supported"

**Cause:** Using the wrong transport or URL.

**Fix:** Reconfigure the client for streamable HTTP from the current MCP documentation fetched through [bex-access.md](bex-access.md).

## Authentication Errors

### "Unauthorized" or 401

**Cause:** OAuth sign-in not completed or expired, or a missing or expired bearer token. An exchanged API-key token expires after 15 minutes. An API key secret sent directly as a bearer token is always refused.

**Fix:** For OAuth, sign in again (`/mcp` or `claude mcp login bex` in Claude Code). For a token setup, exchange the API key for a fresh token as shown in [bex-access.md](bex-access.md), update the configured header or environment variable, restart the client, and verify with `list_services()`. Create a new API key in the bex dashboard (**Settings → API Keys**) only if the old one was deleted.

### "Forbidden" or 403

**Cause:** The signed-in account or API key doesn't have access to the requested resource, or wrong workspace.

**Fix:** Verify the intended workspace and confirm that the authenticated account has access to it, following [bex-access.md](bex-access.md).

## Tool-Specific Issues

Use the current client-specific setup instructions fetched through [bex-access.md](bex-access.md). Also check these local failure modes:

### Cursor

- Config file: `~/.cursor/mcp.json`
- Must be valid JSON (no trailing commas)
- Restart Cursor fully after config changes (not just reload window)
- If multiple MCP servers are configured, ensure the `bex` key is unique

### Claude Code

- Confirm the configured server appears in the client's MCP server list.
- Replace a stale setup with `claude mcp remove bex`, then re-add it with the setup command in `SKILL.md`.

### Codex

- If the MCP configuration reads its token from an environment variable, it must be set in the shell or environment where Codex starts.
- Restart Codex after changing its MCP configuration or credential environment.

## Workspace Issues

### "No workspace selected"

**Fix:** Follow the workspace-selection workflow in [bex-access.md](bex-access.md).

### Operations return empty results

**Cause:** Active workspace has no services, or wrong workspace is selected.

**Fix:** Call `get_selected_workspace()` to inspect the active workspace and `list_workspaces()` to find the intended one. Verify credential access before concluding that no resources exist.

## MCP vs CLI Fallback

Use the supported fallback selection in [bex-access.md](bex-access.md), then inspect the current CLI or MCP capability surface instead of relying on a fixed comparison table.
