# `bex` CLI

`bex` is a Bex-configured executable of the upstream [Render CLI](https://github.com/render-oss/cli). It imports the pinned upstream command package: commands, flags, parsing, request construction, and most help text are upstream behavior. The small Bex bridge changes only the default API origin and local configuration location, and registers Bex-native commands (`bex code`, `bex upgrade`) alongside the imported command tree without modifying it.

The current pin and its update procedure are recorded in [`lego/cli/UPSTREAM_RENDER_CLI.md`](../lego/cli/UPSTREAM_RENDER_CLI.md).

## Install

One line — detects OS/arch, verifies the Sigstore-signed `checksums.txt`, installs to `~/.local/bin` (`BEX_VERSION` pins, `BEX_INSTALL_DIR` retargets). Requires [`cosign`](https://docs.sigstore.dev/cosign/system_config/installation/) on PATH so the installer shares the same release-workflow identity policy as `bex upgrade`:

```bash
curl -fsSL https://raw.githubusercontent.com/bex-co/bex/main/scripts/install-bex.sh | sh
```

Or via Homebrew: `brew install bex-co/tap/bex` — the formula in [bex-co/homebrew-tap](https://github.com/bex-co/homebrew-tap) is rendered by `scripts/bex-cli-formula.sh` and pushed automatically by the release workflow (gated on the `BEX_TAP_PUSH_KEY` secret: the private half of a write **deploy key** scoped to only the tap repo, custodied via `.env`'s `BEX_TAP_PUSH_KEY_FILE` + `scripts/gh-secrets.sh`).

Both channels consume the `bex-cli/v*` GitHub releases: when such a tag is published, CI creates Linux and macOS archives, `checksums.txt`, and a keyless cosign signature bundle over the checksums (GitHub-OIDC signed; no key custody). To verify a download's provenance manually:

```bash
cosign verify-blob checksums.txt \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp 'github.com/bex-co/bex/.github/workflows/cli-release.yml' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

For a checkout, build the same binary locally:

```bash
cd lego/cli
go build -o ../../bin/bex .
../../bin/bex --help
```

The importer requires the Go version declared in [`lego/cli/go.mod`](../lego/cli/go.mod).

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
| `BEX_NO_UPDATE_NOTIFIER` | Any non-empty value disables the update check entirely. By default `bex -v` reports bex's own release identity (`bex vX.Y.Z (Render CLI v2.22.0 compatible)`) and checks this repo's `bex-cli/v*` releases for something newer; after normal commands a gh-style passive notice appears at most once per 24h (cached under `~/.bex/cache/`), only on a TTY, and never when `CI` is set. Check failures are always silent. |

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

## Bex-native commands

### Coding-agent launchers: `bex glm` / `bex muse` / `bex kimi` / `bex deepseek`, managed by `bex code`

Each launcher starts a [Claude Code](https://claude.com/claude-code) instance connected to that provider's Anthropic-compatible endpoint — the integration pattern every one of these providers documents officially. They are Bex-native additions beside the imported Render commands; Claude Code is an external runtime dependency, never forked — the seams are exactly its documented environment variables (`ANTHROPIC_BASE_URL`, `ANTHROPIC_AUTH_TOKEN`, `CLAUDE_CONFIG_DIR`, model-slot variables), injected at exec time.

| Command | Provider endpoint | Default model |
| --- | --- | --- |
| `bex glm` | `https://api.z.ai/api/anthropic` (Z.ai coding plan) | `glm-5.2` |
| `bex muse` | `https://api.meta.ai` (Meta Model API) | `muse-spark-1.2` |
| `bex kimi` | `https://api.moonshot.ai/anthropic` (Moonshot platform) | `kimi-k3` |
| `bex deepseek` | `https://api.deepseek.com/anthropic` | `deepseek-chat` |

- **Prerequisite:** `claude` on `PATH` (`npm install -g @anthropic-ai/claude-code`).
- **Isolation:** every provider gets its own `CLAUDE_CONFIG_DIR` under `~/.bex/code/claude-<name>` (base overridable with `BEX_CODE_HOME`) — settings, history, and permissions are independent, instances run in parallel, and a personal `~/.claude` setup is neither read nor modified. Inherited `ANTHROPIC_*`/`CLAUDE_CONFIG_DIR` values are stripped so the personal environment cannot leak in.
- **Keys are bring-your-own, captured once.** A provider's first launch opens its API-key console, takes one hidden paste, **verifies the key live** against the provider's Messages endpoint (only an explicit 401/403 fails), and stores it owner-only in `~/.bex/code/keys.toml` — the key is injected into the child process environment at launch and written into no configuration file. The environment (`ZAI_API_KEY`/`GLM_API_KEY`, `META_MODEL_API_KEY`/`META_API_KEY`, `MOONSHOT_API_KEY`/`KIMI_API_KEY`, `DEEPSEEK_API_KEY`) always overrides the store. `bex code` / `bex code keys` show status; `bex code keys set|unset <provider>` manage the store (piped stdin works for scripting).
- **Passthrough:** everything after the provider name goes to `claude` unchanged — `bex glm --continue`, `bex glm -p "one prompt"`, `bex glm --help` for Claude Code's own flags. The catalog lives in `lego/cli/internal/code/provider.go`; adding a provider is a data change. A Bex-served provider catalog and Bex-brokered keys (via `bex login`) are the forward path.

### Self-update: `bex upgrade`

`bex upgrade` updates a raw-binary install in place. It resolves the newest `bex-cli/v*` GitHub release, downloads the archive for the running OS/arch, **verifies it before installing** — the release's cosign keyless signature over `checksums.txt` (pinned to the bex-cli release workflow's Fulcio identity), then the archive's SHA-256 against that signed `checksums.txt` — and atomically replaces the running binary (a failure at any step leaves the original untouched).

- **`bex upgrade -n` / `--check`** reports whether a newer release exists without installing it.
- **Already current / dev builds:** a matching version no-ops with "already up to date"; a `dev` build (no release identity) refuses.
- **Package-manager installs are left alone.** When the binary lives under a Homebrew path (Cellar / `opt/homebrew` / Linuxbrew), `bex upgrade` prints `brew upgrade bex` instead of overwriting files the package manager owns.
- Signature verification is in-process (no `cosign` binary required) and **fail-closed**: if the sigstore trusted root can't be fetched, or the signature/identity/checksum doesn't verify, the upgrade aborts.

## Compatibility and branding limits

This is intentionally not a fork. Until Bex maintains a separate command implementation, upstream help text, interactive labels, and User-Agent behavior can say “Render.” Version and update messaging are the exception: `bex -v` is handled by the launcher itself and checks Bex's own release channel, never Render's. One remnant remains — the `bex login` TUI contains an upstream update banner comparing against render-oss/cli releases that the launcher cannot intercept; it stays dormant while the pin tracks upstream's latest release (see `lego/cli/UPSTREAM_RENDER_CLI.md`). The server-side compatibility ledger, including known Bex non-goals such as workflows, ephemeral SSH, and `ea` objects, is [`docs/cli-compatibility-checklist.md`](cli-compatibility-checklist.md). The imported command can only work where Bex implements the corresponding API operation; it does not turn an unimplemented Bex feature into a supported one.

If complete Bex branding or a dedicated OAuth public-client identity becomes a product requirement, make that explicit as a future fork/extension decision rather than silently changing the imported command surface.
