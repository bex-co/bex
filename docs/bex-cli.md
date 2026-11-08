# `bex` CLI

`bex` is a Bex-configured executable of the upstream [Render CLI](https://github.com/render-oss/cli). It imports the pinned upstream command package: commands, flags, parsing, request construction, and most runtime behavior are upstream. The Bex bridge changes the default API origin and local configuration location, registers Bex-native commands (`bex code`, `bex edit`, `bex upgrade`) alongside the imported command tree, and applies a help-chrome branding overlay, a fixed-prefix explicit-login prompt rewrite, and a run-time `render <command>` → `bex <command>` rewrite (`internal/branding`) without modifying upstream implementations.

The current pin and its update procedure are recorded in [`lego/cli/UPSTREAM_RENDER_CLI.md`](../lego/cli/UPSTREAM_RENDER_CLI.md).

## Install

One line — detects OS/arch, verifies the Sigstore-signed `checksums.txt`, installs to `~/.local/bin` (`BEX_VERSION` pins, `BEX_INSTALL_DIR` retargets). Requires [`cosign`](https://docs.sigstore.dev/cosign/system_config/installation/) **v2.4.2 or newer** on PATH so the installer shares the same release-workflow identity policy as `bex upgrade` (releases are signed with cosign v3 as a Sigstore protobuf bundle, which older cosign cannot read; the installer checks the version and names the requirement):

```bash
curl -fsSL https://raw.githubusercontent.com/bex-co/bex/main/scripts/install-bex.sh | sh
```

Or via Homebrew: `brew install bex-co/tap/bex` — the formula in [bex-co/homebrew-tap](https://github.com/bex-co/homebrew-tap) is rendered by `scripts/bex-cli-formula.sh` and pushed automatically by the release workflow (gated on the `BEX_TAP_PUSH_KEY` secret: the private half of a write **deploy key** scoped to only the tap repo, custodied via `.env`'s `BEX_TAP_PUSH_KEY_FILE` + `scripts/gh-secrets.sh`).

Both channels consume the `bex-cli/v*` GitHub releases: when such a tag is published, CI creates Linux and macOS archives, `checksums.txt`, and a keyless cosign signature bundle over the checksums (GitHub-OIDC signed; no key custody). To verify a download's provenance manually (cosign v2.4.2 or newer):

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

`bex login` starts the existing Bex-compatible device flow and opens the dashboard verification page. By default, the imported CLI writes its upstream YAML schema to `~/.bex/cli.yaml`, with the upstream CLI's restrictive `0600` file mode, and keeps persistent state (telemetry `installation_id`, the one-time analytics notice marker) under `~/.bex/state`. It does not read or write `~/.render/` unless you explicitly override it with a `RENDER_*` environment variable. Existing machines that previously shared `~/.render/state/installation-id.txt` with a `render` binary mint a **new** bex `installation_id` on first run after this change — that id was never bex's, so it is not copied.

The browser login stores a short-lived access token and a refresh token. The CLI refreshes automatically and `bex logout` revokes the stored OAuth credential then removes the local Bex config. Do not copy either token into source code, shell history, or logs.

## Configuration

These are Bex-owned inputs. An explicitly set corresponding `RENDER_*` variable wins, as an intentional escape hatch for upstream CLI developers; do not set `RENDER_*` in normal Bex use. A non-empty `RENDER_CLI_CONFIG_DIR` relocates both `cli.yaml` and `state/`. A non-empty `RENDER_CLI_CONFIG_PATH` selects an exact config file without writing siblings next to it; state still follows the Bex directory default unless a directory override is also set.

| Bex variable | Effect |
| --- | --- |
| `BEX_CLI_CONFIG_DIR` | Directory containing `cli.yaml` **and** `state/` (installation id, notice marker); default is `~/.bex`. |
| `BEX_CLI_CONFIG_PATH` | Exact local YAML path; takes precedence over the directory for the config file. Does not write siblings next to that file; state stays under `BEX_CLI_CONFIG_DIR` or `~/.bex/state`. |
| `BEX_HOST` | REST base URL; default `https://api.bex.co/v1/`. Use e.g. `http://localhost:8090/v1/` for a local API. |
| `BEX_WORKSPACE` | Active workspace id or name. A name is resolved to its id once per command (one `GET /v1/owners?name=` request; an id costs none). Only a command that reads the active workspace resolves it. For such a command, an unknown name exits 1 with `no workspace named "<x>"`, and a name shared by several workspaces asks for the id. Help, `--version`, `workspaces`, `whoami`, `workspace set`, `login`/`logout`, completion and an explicit `--workspace` never look the name up. A command whose argument is a typed resource ID (`pg get dpg-…`) tries the name but is not blocked by it, so a stale name never prevents recovery (w8/064). Without a usable login the value passes through and the login error is reported as usual. An explicit `RENDER_WORKSPACE` is never resolved. |
| `BEX_OUTPUT` | Default output mode accepted by the upstream CLI. |
| `BEX_ACCESS_TOKEN` | Already-issued, short-lived OAuth bearer token for an unattended invocation. It is not persisted by the bridge. `bex logout` neither revokes nor unsets this environment credential. |
| `BEX_NO_UPDATE_NOTIFIER` | Any non-empty value disables the update check entirely. By default `bex -v` reports bex's own release identity (`bex vX.Y.Z` plus a `compatible with Render CLI v2.35.0` line) and checks this repo's `bex-cli/v*` releases for something newer; every hint names the upgrade command for the install channel (`bex upgrade`, or `brew upgrade bex` for a Homebrew-owned binary); a successful `bex login` ends with the same hint (TTY or not — it replaces upstream's banner, which advertised Render CLI releases); after normal commands a gh-style passive notice appears at most once per 24h (cached under `~/.bex/cache/`), only on a TTY, and never when `CI` is set. Check failures are always silent. |
| `BEX_CLI_DISABLE_ANALYTICS` | Any non-empty value disables CLI usage telemetry (see below). Telemetry is otherwise on: one event per invocation to bex-api, never to Render. |

The launcher sends the active workspace from `bex workspace set` or `BEX_WORKSPACE` to the configured API in `X-Bex-Workspace`. Service names on `/v1/services/{name}/…` resolve only in that workspace, including renamed services, deploys, restart, instances and jobs. A missing name returns 404; it never falls through to another workspace. Without a selection, a name must be unique across visible workspaces or the API returns `409 SERVICE_NAME_AMBIGUOUS` with the candidate IDs. Service IDs remain global addresses and authorize against the resource's workspace. This header is a bex extension; Render's corresponding paths accept IDs.

For example, a local run never needs a `RENDER_*` setting:

```bash
BEX_HOST=http://localhost:8090/v1/ bex workspaces -o json
```

## CLI usage telemetry

Eligible `bex` invocations send a usage event to bex-api (`POST /v1/cli-telemetry-events`, w5/m92): the command path, duration, exit code, OS/arch, output format, TTY/CI/agent signals (environment-variable _names_ only, never values), the active workspace id, and a stable per-machine install id. Each request also carries an `X-Bex-CLI-Version` header naming the launcher's own release, stored beside the pinned upstream version so release adoption is measurable (w5/m94); the two are separate fields, and a request without the header is accepted and recorded as unknown. Command arguments, prompts, and secret values are never collected. Sending is best-effort and cannot fail your command; the upstream sender chooses detached or bounded synchronous delivery according to the execution environment. Events are attributed to your authenticated identity server-side, retained under the platform audit-retention window, and never shared with Render.

Two launcher-owned paths that the imported CLI's own post-run hook cannot observe report themselves instead (w5/m94): a provider launch (`bex code`, `bex glm`, …) emits one event immediately before it replaces the process, and a root `--version` answer emits one with completion kind `version` — a deliberate divergence from upstream, which treats version as a non-event. Both carry only the command path, never the arguments handed to the launched agent, and both honor exactly the opt-outs below. Because they run outside Cobra's post-run path, neither waits for the upstream one-time notice described at the end of this section.

The ops-only [CLI usage board](https://obs.bex.co/d/bex-cli-usage/cli-usage) visualizes observed adoption, repeat usage, command outcomes/duration, and agent/CI signals, sliced by either version axis. It does not measure all installed CLIs: opt-outs, unavailable authentication, and early initialization failures are absent — an unauthenticated run in particular has no identity to attribute an event to, so it is silent by construction. See [metric definitions](runbooks/cli-analytics.md).

Opt out with `BEX_CLI_DISABLE_ANALYTICS=1` (or the cross-tool `DO_NOT_TRACK=1`, which the imported CLI honors directly). An explicit `RENDER_CLI_DISABLE_ANALYTICS` is left untouched as the upstream-developer escape hatch.

On a fresh machine the imported CLI prints its one-time upstream analytics notice once (a Render-branded block ending in a `render.com/docs` link). That copy describes Render's telemetry, not Bex's — and its "remove `RENDER_CLI_DISABLE_ANALYTICS` to enable telemetry" instruction must not be followed: re-enabling the upstream flag would phone Render, not Bex. The notice never repeats (upstream marker file) and is otherwise unrelated to the Bex telemetry above.

## CI and automation

A Bex API key is an OAuth client-credentials pair. Its **secret is not a bearer token** and must never be assigned to `BEX_ACCESS_TOKEN` or written to `cli.yaml`. Exchange the pair at Bex's OAuth token endpoint, retain the returned `access_token` only in the running job, and pass that value to the launcher:

```bash
# Keep BEX_CLIENT_ID/BEX_CLIENT_SECRET in the CI secret store.
access_token="$({
  curl --fail --silent --show-error \
    --data grant_type=client_credentials \
    --data-urlencode "client_id=$BEX_CLIENT_ID" \
    --data-urlencode "client_secret=$BEX_CLIENT_SECRET" \
    "$BEX_OAUTH_TOKEN_URL"
} | jq -er '.access_token')"

BEX_ACCESS_TOKEN="$access_token" bex services -o json
unset access_token
```

Set `BEX_OAUTH_TOKEN_URL` to the platform's Hydra public token endpoint: `https://oauth.bex.co/oauth2/token` on hosted bex, as named by `https://api.bex.co/.well-known/oauth-protected-resource` and the issuer metadata.

- **Send the credentials in the form body.** An API key's OAuth client authenticates with `client_secret_post` (`lego/backend/internal/apikeys/service.go`). HTTP Basic (`curl --user id:secret`) is refused with `401 invalid_client`.
- **Keep the token secret.** Do not enable shell tracing around this exchange, print the response, or make the token a job artifact.
- **Humans should use device login** in a terminal.

Verified 2026-09-15 on production: a form-body exchange returned a 15-minute token, and `BEX_ACCESS_TOKEN="$access_token" bex logs --resources <cron srv-id> --tail -o text` streamed a cron job's next run live.

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

### Open code in bex desktop: `bex edit` and `bex <path>`

`bex edit` hands its arguments, unchanged, to the bex desktop editor's launcher. The launcher's names and locations are a cross-repo contract: [bex-desktop ADR002](https://github.com/bex-co/bex-desktop/blob/main/docs/adrs/ADR002-cli-command-bex-desktop.md).

- **The name `bex` belongs to this CLI.** bex desktop installs its own terminal command as `bex-desktop`. Desktop builds before ADR002 linked it as `bex`, and `scripts/install-bex.sh` says so when it replaces such a link.
- **Lookup order:**

  1. `bex-desktop` on `PATH`.
  2. On macOS, `/Applications` or `~/Applications` `Bex.app` (or `Bex Preview` / `Bex Nightly` / `Bex Dev`) `/Contents/MacOS/cli`.
  3. On Linux, `~/.local/bex.app/bin/bex`, then the other channels' `~/.local/bex-*.app/bin/{bex,cli}`.

  It never runs a `bex` from `PATH`, and it skips any candidate that resolves to itself. If no launcher is found, it fails with an install hint.

- **Passthrough:** paths, `path:line[:col]`, `-n`/`-a`/`-e`/`-w`, `-` for stdin, `--diff OLD NEW` and `ssh://` URLs. Relative paths resolve in the current directory. `bex edit --help` shows this command's help; every other flag belongs to the editor.
- **Agent sessions:** `bex edit ags-<id>` reads the session from the API and opens its running sandbox's `/workspace` over SSH. It uses the same target as the dashboard's Open in Zed ([ADR054](ADR054-open-in-zed.md)). You need `can_view_sensitive` on the session and an SSH key registered with bex. A session with no running sandbox says so; send it a message to start one. An existing file or folder named `ags-…` is opened as a path.
- **Shorthand:** `bex <path> …` is `bex edit <path> …` when the first argument exists and looks like a path: `.`, `..`, `~`, or a value that starts with `./`, `../`, `/` or `~/` or contains a `/`, optionally with `:line[:col]`.
  - A bare word is always a command, even beside a same-named folder: `bex docs` runs `docs`, while `bex ./docs` opens the folder.
  - A path-shaped argument that does not exist is an error, not a new file. `bex edit <path>` creates it on purpose.
  - Bare `bex`, flags, and URLs are never rewritten.
- **Interactive only:** with no terminal on stdout or stderr, or with `CI` set, both forms fail fast instead of launching a GUI. Scripts can call `bex-desktop` directly.
- **Telemetry:** one launch event with the command path `bex edit`, as for `bex code`. It never includes the paths or other arguments.

### Cloud Agents: `bex agent`

`bex agent` drives Cloud Agents, bex's cloud coding agents, from a terminal or a CI job. It is a thin client of the agent-session REST contract ([ADR094](ADR094-coding-agent-mcp.md), attachments [ADR095](ADR095-agent-attachments.md)) and authenticates like every other command (`bex login`, or `BEX_ACCESS_TOKEN` in CI). Sessions are created in the active workspace (`bex workspace set` or `BEX_WORKSPACE`).

| Command | What it does |
| --- | --- |
| `bex agent run [flags] <task…>` | Start a session. `--repo`, `--branch`, `--agent` (default `claude`), `--model`, `--title`, `--tag` (repeatable), `--attach FILE` (repeatable; uploaded first, found under `/attachments`), `--output-schema FILE` (read the result from `outcome.result`), `--max-spend USD`, `--skill NAME[:ARGUMENTS]` (a workspace skill to use this turn, repeatable, up to 5, [ADR099](ADR099-agent-skills.md)), `--secret NAME` (a workspace agent secret, repeatable) and `--secret-from-env NAME[=VAR]` (sends the local env var `VAR`, default `NAME`, as an inline secret for this session only; values are never taken on the command line, [ADR098](ADR098-agent-session-secrets.md)). `-f FILE` reads the task from a file (`-` for stdin). |
| `bex agent send <ags-id> <message…>` | Start the next turn, with `--attach`, `--output-schema`, `--skill`, `--secret` and `--secret-from-env` for that turn. |
| `bex agent wait <ags-id>…` | Wait until up to 20 sessions end their turn (`--any`: the first one). |
| `bex agent get <ags-id>` | Show a session and its outcome: state, summary, PR, shared files, result, model cost. |
| `bex agent list` | List the workspace's sessions (`--state`, `--tag`, `--repo`, `--archived`, `--limit`, `--cursor`). |
| `bex agent logs <ags-id>` | Print the transcript: the agent's messages and one line per tool call (`--turn N`, `--reasoning`; `-o json` prints one transcript row per line). `--follow` streams until the turn ends. |
| `bex agent files <ags-id>` | List the files the agent shared (`--direction input` or `all` for the attached ones; `--turn N`). `--download DIR` saves them, checks each SHA-256, never overwrites without `--force`, asks for a new link when a 15-minute download link has expired, and still lists the files that landed when one fails. |
| `bex agent cancel <ags-id>` | Stop the running turn and close the session. |
| `bex agent open <ags-id>` | Open the running sandbox in bex desktop (like `bex edit ags-…`), or else print and open the session's dashboard page (`--dashboard` forces it). |

- **Waiting.** `run` and `send` return as soon as the turn starts. `--wait` polls until it ends; `--follow` streams the transcript while it runs. `--timeout 30m` bounds either one. REST has no blocking wait (`wait_agent_sessions` is MCP-only), so the CLI polls the session with backoff (2 s rising to 15 s; `--follow` backs off the same way while the agent is quiet) and survives up to five consecutive transient failures (a 429, a gateway 5xx or a dropped connection), waiting at least as long as a `Retry-After` asks.
- **Exit codes.** These apply to `run`/`send` with `--wait` or `--follow`, to `wait`, and to `logs --follow`. With several sessions, a failure wins over a cancel, which wins over needs-input. A plain `get` always exits 0.

  | Code  | Meaning                                                      |
  | ----- | ------------------------------------------------------------ |
  | `0`   | the turn succeeded                                           |
  | `1`   | the CLI or the API failed (network, auth, a refused request) |
  | `2`   | usage error                                                  |
  | `3`   | the turn failed; stderr names its `failureReasonCode`        |
  | `4`   | the session was canceled                                     |
  | `5`   | reserved for `needs_input`, which sessions never enter       |
  | `124` | `--timeout` elapsed while the session was still working      |
  | `130` | interrupted (Ctrl-C or SIGTERM)                              |

- **Output.** `-o json` (or `yaml`) prints the API's own answer on stdout: the session view, a JSON array for several sessions or for `files`. While `--follow` streams, the transcript goes to stderr so stdout stays parseable.
- **Idempotent create.** Each `run` sends one `Idempotency-Key` and repeats the create, with the same key, after a dropped connection, a 429 or a gateway 5xx. A retry can therefore never start a second session. Pass `--idempotency-key` to make re-running the whole command safe as well, for example keyed on the CI run id.

```bash
# In CI: fix a failure, wait for the outcome, and keep the agent's report.
bex agent run --repo acme/api --attach test-output.txt --output-schema schema.json \
  --max-spend 5 --timeout 45m --wait -o json \
  "Make the failing test in test-output.txt pass" > outcome.json
bex agent files "$(jq -r .id outcome.json)" --download ./agent-files
```

### Self-update: `bex upgrade`

`bex upgrade` updates a raw-binary install in place. It resolves the newest `bex-cli/v*` GitHub release, downloads the archive for the running OS/arch, **verifies it before installing** — the release's cosign keyless signature over `checksums.txt` (pinned to the bex-cli release workflow's Fulcio identity), then the archive's SHA-256 against that signed `checksums.txt` — and atomically replaces the running binary (a failure at any step leaves the original untouched).

- **`bex upgrade -n` / `--check`** reports whether a newer release exists without installing it.
- **Already current / dev builds:** a matching version no-ops with "already up to date"; a `dev` build (no release identity) refuses.
- **Package-manager installs are left alone.** When the binary lives under a Homebrew path (Cellar / `opt/homebrew` / Linuxbrew), `bex upgrade` prints `brew upgrade bex` instead of overwriting files the package manager owns.
- Signature verification is in-process (no `cosign` binary required) and **fail-closed**: if the sigstore trusted root can't be fetched, or the signature/identity/checksum doesn't verify, the upgrade aborts.

## Canceling a running query over `bex psql`

Pressing Ctrl+C in `bex psql` (and in any psql connected to a `*.db.bex.co` external URL) does **not** stop the running statement: psql, including 18.x, sends PostgreSQL's legacy cancel request, a plaintext packet with no TLS server name. bex's shared external endpoint routes every connection by TLS SNI and never decrypts it (ADR009 § TLS passthrough), so that packet has nowhere to go. The query runs to completion and the session stays usable. This is a user-signed difference from Render (w8/m87, ADR018).

To cancel, use either of these, which do reach the database:

- a client built on **libpq 17 or newer that calls `PQcancelCreate`/`PQcancelBlocking`** (the TLS cancel connection carries the server name), for example a driver's cancel method backed by that API;
- `SELECT pg_cancel_backend(<pid>)` from a second session to the same database (`SELECT pg_backend_pid()` in the first session names the pid).

Internal connections from services in the same workspace are not affected.

## Secret files must be UTF-8 text

`--secret-file name:./path` (on `services create`) sends the file as a JSON string, the only shape Render's API takes on every surface. The pinned client sends `string(data)`, and JSON encoding replaces every invalid UTF-8 byte with U+FFFD, so a **binary** file (a keystore, `.p12`, DER certificate) reaches `/etc/secrets/` silently altered, with the command still exiting 0 (w8/029). The unmodified `render` binary does the same against Render. Text files of any size up to the limit round-trip byte for byte.

For a binary file, store it base64-encoded and decode it when the service starts:

```bash
base64 < keystore.p12 > keystore.b64            # locally (macOS or Linux)
bex services create … --secret-file keystore.b64:./keystore.b64
# in the start command:
base64 -d /etc/secrets/keystore.b64 > /tmp/keystore.p12
```

The dashboard's secret-file upload refuses a non-UTF-8 file with the same hint instead of saving a mangled copy.

## Compatibility and branding limits

`bex` imports the pinned upstream command package; it does not fork or vendor it. A Layer-1 overlay in `lego/cli/internal/branding` mutates the exported `cmd.RootCmd` after Bex-native commands attach:

- **Help chrome:** `Use` is `bex`, examples/Short/Long get safe string rewrites (backtick-, single-, and double-quoted `render …` command references, line-leading bare commands, and the `on/to/of/with/from/the/your Render` phrases), help prints `bex CLI v…`, and the ungrouped-commands header matches `CommandPath == "bex"`. Hostnames such as `render.com` are never rewritten globally; the one help page that cited `render.com/docs/one-off-jobs` (`jobs create`) carries a narrow per-command override instead (w7/045).
- **`bex docs`:** opens the Bex CLI guide on GitHub (`branding.DocsURL`), not `render.com/docs`.
- **`bex -v`:** leads with `bex vX.Y.Z`, then a separate `compatible with Render CLI v…` line; update checks still hit this repo's `bex-cli/v*` releases.

Safe rewrites preserve **`render.yaml`** (and the `bex.yml` filename alias).

**Run-time command references name `bex` (user decision 2026-11-07, w2/m263 D4=A).** Upstream output that tells the user to run `render <command>` says `bex <command>` instead. This covers text output, the JSON/YAML `meta.next` field agents follow, and backtick or quoted forms such as ``Use `render workspace set` ``. The rule covers the whole class. It is implemented in `internal/branding/runtime.go` as wrappers around the upstream `RunE`, like `brandLoginOutput`. No upstream `RunE` body is rewritten:

- **Returned errors, every command.** Upstream returns most hints as errors, which cobra prints to stderr. The wrapper rewrites the message, and `errors.Is`/`errors.As` still reach the original, so exit codes and `ErrLogin` handling are unchanged. This covers the resource-not-found, auth and workspace hints below, the `psql` usage line, and the stale manual env var file refusal.
- **Stdout, for the commands listed in `runtimeStdout`:** `ea blueprints create|plan|approve|update` and `logout`. For the blueprint commands only the next step changes: JSON and YAML `meta.next`, or the line after the text `Next:` title. Blueprint names, repos, paths, notices and the rest of the API data stay byte-exact. `logout` prints only upstream copy, so every reference in it is rewritten. Stdout is not wrapped for every command, for two reasons. A wrapped writer hides the terminal from upstream's `*os.File` checks (table width, color, `ssh`'s TTY). And `logs`, `sandboxes exec` and `ssh` stream user data through stdout, which must never be edited.
- **cobra `Deprecated` notices**, rewritten as command metadata with the help overlay. cobra prints a notice before the launcher's initializer runs, so this reaches only commands that exist when `branding.Apply` runs. The late-added deprecated aliases (`sandbox-groups builds`, the `workflows` shortcuts) keep upstream's notice. Both belong to commands bex does not serve (see below).

A reference is `render`, a space, and a command the root actually has (`ea`, `workspace`, `services`, …). It must sit at the start of a line (after indentation), right after a backtick, a quote or `| `, or be a bare `` `render` ``. URLs (`render.com`), file names (`render.yaml`), `RENDER_*` variables, data such as a service named `render-x`, and "Render" prose (for example "Render will start applying your planned changes") never match.

Regenerate the inventory on every pin bump (step 4 of the re-diff) and check that each hit reaches one of the three seams:

```bash
MOD=$(cd lego/cli && go list -m -f '{{.Dir}}' github.com/render-oss/cli)
grep -rnE '"[^"]*\brender [a-z]' "$MOD" --include='*.go' \
  | grep -v '_test.go' | grep -vE 'Example|Short:|Long:|Use:|Deprecated:'
```

At the current pin (**v2.35.0**, `24ac2d60fdb2`) it yields the following.

**Resource-not-found, one per resource type** (returned errors, rewritten). Each ends "run `bex workspace set <name|ID>`":

- `pkg/service/repo.go:233` — "No service named '\<x>' in workspace tea-…. To search another workspace, run `render workspace set <name|ID>`, or pass the service ID instead."
- `pkg/postgres/resolve.go:186` — same shape, "No Postgres database named …"
- `pkg/keyvalue/resolve.go:187` — same shape, "No Key Value named …"

**Auth / workspace state** (returned errors and `logout` stdout, rewritten):

- `pkg/config/config.go:27` (`ErrLogin`) — "run `render login` to authenticate", and the logout strings (`cmd/logout.go:29,84`)
- `pkg/config/config.go:26` (`ErrNoWorkspace`) — "no workspace set. Use `render workspace set` to set a workspace", on `workspace current` and every list command with no active workspace (`w8/016`)
- `pkg/command/wrapper.go:21` (`ErrTokenExpired`) — "your token is expired; run `render login` to get a new one"
- `pkg/validate/workspace.go:17` — the cross-workspace mismatch, "Run `render workspace set <id>` to change contexts"
- `cmd/blueprintvalidate.go:92` — `no workspace specified and no default workspace set. Use --workspace or run 'render workspace set'`

**Usage / follow-up hints:**

- `cmd/psql.go:96` — `Usage: render psql <postgresID> --command "SELECT ..." -o json` (a returned error, rewritten)
- `cmd/kvcreate.go:51` — the `--workspace` flag help citing `'render workspace set'` (help overlay, rewritten)
- `pkg/tui/views/pgcreate.go:797` — "Run `render pg get <id>` to check if it's ready yet." **Residual:** the interactive `pg create` wizard draws this in its bubbletea view, which writes to the terminal directly and has no Cobra writer to wrap.

**`ea blueprints` next steps (v2.35.0)** (stdout `meta.next` and text `Next:`, rewritten):

- `pkg/blueprint/output.go:120,127` — "render ea blueprints approve \<id>" after a plan, "fix the errors above, then run \"render ea blueprints plan \<id>\"" after a failed plan
- `pkg/blueprint/output.go:60,110` — the no-changes hints citing `render ea blueprints plan` / `approve`
- `pkg/blueprint/output.go:149` — after approve: "Render will start applying your planned changes … `render services` and `render deploys` commands". The command references change; the leading "Render will start applying" is prose and stays.
- `pkg/blueprint/manualenvvars.go:71` — the stale manual env var file refusal citing `render ea blueprints approve` (a returned error)

Tests: `internal/branding/runtime_test.go` (the reference rule, JSON/YAML/text next-step rewrites, error unwrapping) and `runtime_copy_test.go`. The latter drives the built binary against a stub API: `ea blueprints create` in text, JSON and YAML, `workspace current` with no workspace, and render-shaped data (`render-x`, `render.yaml`, `render.com`, `RENDER_*`) that must survive.

**`bex skills` installs bex's own skills (w8/m143).** The launcher replaces upstream's `skills` tree. Upstream clones `render-oss/skills`, whose skills drive the `render` CLI, `mcp.render.com` and `dashboard.render.com`. The bex tree installs a pack embedded in the binary, derived from that MIT-licensed repository at a pinned commit by `lego/cli/internal/skills/packgen` and regenerated with `lego/cli/hack/sync-skills.sh`. The pack's 20 `bex-*` skills operate bex (`bex` CLI, `https://api.bex.co/mcp`, `dashboard.bex.co`, `onbex.co`). They keep Render's docs as the behavior reference and drop Render Workflows and sandboxes. The subcommands and flags are Render's (`install`, `list`, `update`, `remove`; `--tool`, `--skill`, `--scope`, `--dry-run`, `--all`, `--force`). State is `bex-skills.yaml` in the bex config directory. Selections are always explicit `bex-*` names, so a user's genuine `render-*` skills and `~/.render/skills.yaml` are never touched. In a terminal with no `--output`, `install` prompts for scope, tools and skills, and `remove` prompts for the skills to remove, as Render's commands do; each flag skips its prompt (w8/214). `list` and `update` print text there, and none of the commands panics non-interactively. The previous "accepted branding residual" (w8/m29 t004) was withdrawn because its effect was functional, not copy.

**Identity, storage and copy that branding deliberately leaves alone:**

- automatic login TUI inside other interactive commands (`views.LoginView`) saying “Render Dashboard”; explicit `bex login` rewrites only its fixed prompt prefix through Cobra stdout, preserving the device code and verification URL byte-for-byte
- "Render" prose in run-time output ("Successfully logged out of Render.", "Render will start applying …"): the run-time rule covers command references only
- User-Agent `render-cli/<upstream version>` (`cfg.Version` stays the pinned upstream release for the compatibility ledger)
- hard-coded OAuth public client id (Hydra bootstrap contract)
- one-time analytics notice copy (Render-branded disclosure + `render.com/docs` link + an invitation to re-enable Render telemetry — do not follow; see CLI usage telemetry above)

Strings inside commands Bex does not serve (`render workflows …` deprecation notices and the `render workflows flows list` / `tasks list` slug hints in `cmd/flow.go` and `pkg/tui/views/workflows/loader.go`, the `render sandbox-groups images …` deprecation notices on the hidden `sandbox-groups builds` alias, `render sandboxes snapshots …` help) are in the grep output but are not user-reachable residuals here — the underlying operations are non-goals in [`docs/cli-compatibility-checklist.md`](cli-compatibility-checklist.md).

The server-side compatibility ledger, including known Bex non-goals such as workflows, ephemeral SSH, and `ea` objects, is [`docs/cli-compatibility-checklist.md`](cli-compatibility-checklist.md). The imported command can only work where Bex implements the corresponding API operation; it does not turn an unimplemented Bex feature into a supported one.

If deeper runtime branding or a dedicated OAuth public-client identity becomes a product requirement, make that an explicit fork/extension decision rather than silently changing the imported command surface.
