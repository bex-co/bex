# bex CLI

`bex` uses the pinned upstream [Render CLI](https://github.com/render-oss/cli) command implementation, but defaults to Bex's API and stores interactive credentials separately at `~/.bex/cli.yaml`.

## Install

One line (detects OS/arch, verifies checksums, installs to `~/.local/bin`):

```bash
curl -fsSL https://raw.githubusercontent.com/bex-co/bex/main/scripts/install-bex.sh | sh
```

Or with Homebrew:

```bash
brew install bex-co/tap/bex
```

Both install from the [`bex-cli/v*` GitHub releases](https://github.com/bex-co/bex/releases); `BEX_VERSION=X.Y.Z` pins the script to a version, `BEX_INSTALL_DIR` changes the target directory.

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

## Update

`bex -v` prints bex's release identity (`bex vX.Y.Z (Render CLI v2.22.0 compatible)`) and reports when a newer [`bex-cli/v*` release](https://github.com/bex-co/bex/releases) exists; after normal commands the same hint appears passively (at most once per 24h, TTY only, never in CI, disable with `BEX_NO_UPDATE_NOTIFIER=1`).

To update, replace the binary with the newer release:

- **Install script:** re-run the one-liner above — it always installs the newest release.
- **Homebrew:** `brew update && brew upgrade bex`.
- **From a checkout:** `git pull`, then rebuild with the same `go build` above (such builds report `vdev` and never check for updates).

Maintainer knobs live elsewhere: the pinned upstream version and its bump procedure in [`UPSTREAM_RENDER_CLI.md`](UPSTREAM_RENDER_CLI.md), and release-version injection (`BEX_CLI_VERSION` → `scripts/bex-cli-build.sh`, tagged `bex-cli/vX.Y.Z`) in the [Bex CLI guide](../docs/bex-cli.md).

For release archives, configuration, CI credentials, and known upstream-branding limitations, use the canonical [Bex CLI guide](../docs/bex-cli.md).
