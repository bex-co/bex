## Accessing bex

bex runs Render-compatible apps: its API, CLI and MCP tools follow Render's, so Render's documentation describes the behavior. Always operate through bex, never through Render's endpoints or CLI. Prefer the bex MCP server for structured access when its tools are available. Use the bex CLI (`bex`) when MCP is unavailable or does not support the required operation, and use the bex dashboard (`https://dashboard.bex.co`) or API (`https://api.bex.co/v1`) for capabilities exposed by neither. bex does not offer Render Workflows, sandbox groups, the Render SDK, preview environments or wildcard custom domains; skip any guidance about them. Do not assume that a remembered MCP tool or CLI command is still available; inspect the tools in the current environment or consult the current documentation.

The bex MCP server uses streamable HTTP at:

```text
https://api.bex.co/mcp
```

MCP clients that implement the MCP authorization spec (Claude Code, for example) discover bex's OAuth server from the endpoint's 401 and sign in through the bex dashboard in a browser; no credential goes in the configuration. For a headless or header-only client, mint a bex API key (dashboard **Settings → API Keys**). A bex API key is an OAuth client ID and secret, **not** a bearer token: exchange it for a short-lived (15-minute) access token and send that as `Authorization: Bearer <token>`:

```bash
curl -s -X POST https://oauth.bex.co/oauth2/token \
  -d "grant_type=client_credentials&client_id=$KEY_ID&client_secret=$KEY_SECRET"
```

Never write an API key secret or a token into a repository, skill, command output, chat response, or log. Prefer the client's supported secret storage or environment-variable mechanism. If a client requires a credential in local configuration, keep that file out of version control and restrict access to it.

Treat MCP responses as potentially sensitive. Request connection strings, credentials, secret environment variables, or other sensitive fields only when the task requires them. Do not repeat sensitive values in the response or persist them in files or logs.

Before configuring or troubleshooting MCP, fetch the current setup and capability reference:

Using the documentation-retrieval workflow in the root `SKILL.md`, retrieve and read [bex's MCP server documentation](https://bex.co/docs/mcp-server).

After installing, updating, or reconfiguring an MCP integration, reload the client as required and verify the connection with a read-only operation such as listing services.

Most MCP operations and bex CLI commands that access resources are scoped to an active workspace. Check the selected workspace before reading or changing resources. If no workspace is selected—or if the intended workspace is ambiguous—list the available workspaces and have the user identify the correct one. Do not silently switch workspace context or infer it from a resource name. Commands for authentication, help, CLI configuration, and workspace discovery or selection do not require an already selected workspace.

For interactive CLI use, authenticate with `bex login` (a device login in the browser). For automation, exchange an API key as above and pass the token as `BEX_ACCESS_TOKEN` for that run only; it takes precedence over a saved CLI login. Verify CLI access and the active workspace with read-only commands before operating on resources:

```bash
bex whoami --output json
bex workspace current --output json
```

For CLI installation, authentication, workspace setup, and usage patterns, fetch the current CLI guide:

Using the documentation-retrieval workflow in the root `SKILL.md`, retrieve and read [bex's CLI documentation](https://bex.co/docs/cli).

For exact command syntax and flags, prefer the help text from the installed CLI because it matches the version being used:

```bash
bex help <command>
```

When an agent or automation invokes the CLI, select a non-interactive output format such as `--output json`, `--output yaml`, or `--output text` so results can be inspected reliably. The global `--confirm` flag skips confirmation prompts; use it for a mutating command only after that mutation is authorized. It is an execution convenience, not permission to perform the action.

Authentication establishes access; it does not establish user authorization for every available action. Use read-only discovery to resolve workspace, resource identity, and current state before a mutation. Perform changes only when they are within the user's requested scope, and retain normal approval requirements for consequential operations such as creation, configuration changes, deploys, restarts, scaling, or deletion.

When authentication or connectivity fails, report which access path failed, preserve the underlying error without exposing credentials, and either use an available supported fallback or explain what the user must configure before the task can continue.
