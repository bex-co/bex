# `bex` CLI

`bex` is a Bex-configured executable of the upstream [Render CLI](https://github.com/render-oss/cli). It imports the pinned upstream command package: commands, flags, parsing, request construction, and most help text are upstream behavior. The small Bex bridge changes only the default API origin and local configuration location.

The current pin and its update procedure are recorded in [`cli/UPSTREAM_RENDER_CLI.md`](../cli/UPSTREAM_RENDER_CLI.md).

## Install

When a `bex-cli/v*` tag is published, CI creates Linux and macOS GitHub-release archives plus `checksums.txt`. Download the archive matching the host, verify it, extract it, and put `bex` on `PATH`.

For a checkout, build the same binary locally:

```bash
cd cli
go build -o ../bin/bex .
../bin/bex --help
```

The importer requires the Go version declared in [`cli/go.mod`](../cli/go.mod).

## Interactive login

```bash
bex login
bex workspaces -o json
bex services -o json
bex logout
```

`bex login` starts the existing Bex-compatible device flow and opens the dashboard verification page. By default, the imported CLI writes its upstream YAML schema to `~/.bex/cli.yaml`, with the upstream CLI's restrictive `0600` file mode. It does not read or write `~/.render/cli.yaml` unless you explicitly override it with a `RENDER_*` environment variable.

The browser login stores a short-lived access token and a refresh token. The CLI refreshes automatically and `bex logout` revokes the stored OAuth credential then removes the local Bex config. Do not copy either token into source code, shell history, or logs.

## Configuration

These are Bex-owned inputs. An explicitly set corresponding `RENDER_*` variable wins, as an intentional escape hatch for upstream CLI developers; do not set `RENDER_*` in normal Bex use.

| Bex variable | Effect |
| --- | --- |
| `BEX_CLI_CONFIG_DIR` | Directory containing `cli.yaml`; default is `~/.bex`. |
| `BEX_CLI_CONFIG_PATH` | Exact local YAML path; takes precedence over the directory. |
| `BEX_HOST` | REST base URL; default `https://api.bex.co/v1/`. Use e.g. `http://localhost:8090/v1/` for a local API. |
| `BEX_WORKSPACE` | Active workspace id or name. |
| `BEX_OUTPUT` | Default output mode accepted by the upstream CLI. |
| `BEX_ACCESS_TOKEN` | Already-issued, short-lived OAuth bearer token for an unattended invocation. It is not persisted by the bridge. `bex logout` neither revokes nor unsets this environment credential. |

For example, a local run never needs a `RENDER_*` setting:

```bash
BEX_HOST=http://localhost:8090/v1/ bex workspaces -o json
```

## CI and automation

A Bex API key is an OAuth client-credentials pair. Its **secret is not a bearer token** and must never be assigned to `BEX_ACCESS_TOKEN` or written to `cli.yaml`. Exchange the pair at Bex's OAuth token endpoint, retain the returned `access_token` only in the running job, and pass that value to the launcher:

```bash
# Keep BEX_CLIENT_ID/BEX_CLIENT_SECRET in the CI secret store.
access_token="$({
  curl --fail --silent --show-error \
    --user "$BEX_CLIENT_ID:$BEX_CLIENT_SECRET" \
    --data grant_type=client_credentials \
    "$BEX_OAUTH_TOKEN_URL"
} | jq -er '.access_token')"

BEX_ACCESS_TOKEN="$access_token" bex services -o json
unset access_token
```

Set `BEX_OAUTH_TOKEN_URL` to the platform's Hydra public token endpoint. Do not enable shell tracing around this exchange, print the response, or make the token a job artifact. Prefer device login for a human terminal.

`bex logout` manages the stored interactive OAuth session only. For a job that uses `BEX_ACCESS_TOKEN`, unset the variable when the job ends and revoke the issued credential through the authority that minted it when needed.

## Compatibility and branding limits

This is intentionally not a fork. Until Bex maintains a separate command implementation, upstream help text, interactive labels, User-Agent behavior, and update messaging can say “Render.” The server-side compatibility ledger, including known Bex non-goals such as workflows, ephemeral SSH, and `ea` objects, is [`docs/cli-compatibility-checklist.md`](cli-compatibility-checklist.md). The imported command can only work where Bex implements the corresponding API operation; it does not turn an unimplemented Bex feature into a supported one.

If complete Bex branding or a dedicated OAuth public-client identity becomes a product requirement, make that explicit as a future fork/extension decision rather than silently changing the imported command surface.
