# bex CLI

`bex` uses the pinned upstream [Render CLI](https://github.com/render-oss/cli) command implementation, but defaults to Bex's API and stores interactive credentials separately at `~/.bex/cli.yaml`.

## Install from a checkout

Install the Go version declared in [`go.mod`](go.mod), then build:

```bash
cd cli
go build -o ../bin/bex .
../bin/bex --help
```

To make it available on your `PATH`, move or copy `../bin/bex` into a directory already on `PATH`.

## Use

```bash
bex login
bex workspaces -o json
bex services -o json
bex logout
```

The login uses Bex's device-verification flow. It persists the upstream YAML schema under `~/.bex/cli.yaml` with mode `0600`; it does not touch `~/.render/cli.yaml` in normal use.

For a local Bex API, set `BEX_HOST` rather than `RENDER_HOST`:

```bash
BEX_HOST=http://localhost:8090/v1/ bex workspaces -o json
```

For release archives, configuration, CI credentials, and known upstream-branding limitations, use the canonical [Bex CLI guide](../docs/bex-cli.md).
