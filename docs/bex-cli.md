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
| `BEX_WORKSPACE` | Active workspace id or name. A name is resolved to its id once per command (one `GET /v1/owners?name=` request; an id costs none). Only a value shaped like an id (`tea-` and 20 characters) is taken for one, so a workspace named `tea-house` is resolved as a name. Only a command that reads the active workspace resolves it. For such a command, an unknown name exits 1 with `no workspace named "<x>"`, and a name shared by several workspaces asks for the id. Help, `--version`, `workspaces`, `whoami`, `workspace set`, `login`/`logout`, completion and an explicit `--workspace` never look the name up. A command whose argument is a typed resource ID (`pg get dpg-…`) tries the name but is not blocked by it, so a stale name never prevents recovery (w8/064). Without a usable login the value passes through and the login error is reported as usual. An explicit `RENDER_WORKSPACE` is never resolved. |
| `BEX_OUTPUT` | Default output mode accepted by the upstream CLI. |
| `BEX_ACCESS_TOKEN` | Already-issued, short-lived OAuth bearer token for an unattended invocation. It is not persisted by the bridge. `bex logout` neither revokes nor unsets this environment credential. |
| `BEX_NO_UPDATE_NOTIFIER` | Any non-empty value disables the update check entirely. By default `bex -v` reports bex's own release identity (`bex vX.Y.Z` plus a `compatible with Render CLI v2.36.0` line) and checks this repo's `bex-cli/v*` releases for something newer; every hint names the upgrade command for the install channel (`bex upgrade`, or `brew upgrade bex` for a Homebrew-owned binary); a successful `bex login` ends with the same hint (TTY or not — it replaces upstream's banner, which advertised Render CLI releases); after normal commands a gh-style passive notice appears at most once per 24h (cached under `~/.bex/cache/`), only on a TTY, and never when `CI` is set. Check failures are always silent. |
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
| `bex agent list` | List the workspace's sessions (`--state`, `--phase`, `--tag`, `--repo`, `--archived`, `--mine`, `--created-by` (repeatable, up to 20), `--automation` (an `aut-` id or a name), `--parent`, `--since`/`--until` (RFC 3339 or an age: `24h`, `7d`), `--limit`, `--cursor`). |
| `bex agent logs <ags-id>` | Print the transcript: the agent's messages and one line per tool call (`--turn N`, `--reasoning`; `-o json` prints one transcript row per line). `--follow` streams until the turn ends. |
| `bex agent files <ags-id>` | List the files the agent shared (`--direction input` or `all` for the attached ones; `--turn N`). `--download DIR` saves them, checks each SHA-256, never overwrites without `--force`, asks for a new link when a 15-minute download link has expired, and still lists the files that landed when one fails. |
| `bex agent cancel <ags-id>` | Stop the running turn and close the session. |
| `bex agent archive` / `unarchive <ags-id>…` | Archive (hide from the default list, read-only) or unarchive one or more sessions. A failure does not stop the others; the command then exits 1 naming each. |
| `bex agent delete <ags-id>` | Delete the session, its transcript and its files (its pull request stays). Asks on a terminal; elsewhere needs `--confirm`. |
| `bex agent tags <ags-id> (<tag>… \| --clear)` | Replace the session's tags, as the API does. |
| `bex agent tree <ags-id>` | The tree of child sessions the session belongs to, indented, with state and spend ([ADR102](ADR102-agent-child-sessions.md)). |
| `bex agent cost <ags-id>` | The session's model cost in total and per turn: requests, tokens, models. |
| `bex agent chat [<ags-id>]` | Watch and steer a session live in the terminal (alias `attach`); see [below](#live-sessions-bex-agent-chat). |
| `bex agent handoff [<task…>]` | Hand this checkout's work, uncommitted changes included, to a new session; see [below](#move-work-between-your-machine-and-a-cloud-agent-bex-agent-handoff-pickup-ssh). |
| `bex agent pickup <ags-id>` | Check out a session's branch here to continue its work locally. |
| `bex agent ssh <ags-id> [-- <ssh args…>]` | Open a shell on the session's running sandbox with your ssh. |
| `bex agent open <ags-id>` | Open the running sandbox in bex desktop (like `bex edit ags-…`), or else print and open the session's dashboard page (`--dashboard` forces it). |

- **Picking and completion.** On a terminal, a command whose one argument is a session (`get`, `logs`, `files`, `cancel`, `open`, `delete`, `tree`, `cost`, `chat`) or an automation may leave it out: it lists your 20 most recent sessions (or the workspace's automations) and asks for a number, as Devin's `-r` does. Without a terminal an omitted argument is a usage error, as before. Shell completion (`bex completion …`) offers the same ids and names, giving up silently after 2 seconds.
- **Sessions by URL.** Every `<ags-id>` may also be the session's dashboard URL (`https://dashboard.bex.co/agents/ags-…`, on any dashboard origin), as Devin's CLI takes a session URL ([ADR106](ADR106-cli-cloud-agents-automations.md) D3). `run --parent <ags-id>` starts a child session; its spend counts against the parent's ceiling.
- **Waiting.** `run` and `send` return as soon as the turn starts. `--wait` polls until it ends; `--follow` streams the transcript while it runs. `--follow` (and `logs --follow` in text) read the live stream ([ADR047](ADR047-cloud-coding-agent-sessions.md) D9): a single-use read ticket from `attach-ticket`, then the gateway's server-sent events with the ticket in `X-Bex-Agent-Ticket` (no bearer), reconnecting with a fresh ticket after a drop and printing each item once. A deployment without the stream, or `logs --follow -o json` (one transcript row per line), polls as before. `--timeout 30m` bounds either one. REST has no blocking wait (`wait_agent_sessions` is MCP-only), so the CLI polls the session with backoff (2 s rising to 15 s; `--follow` backs off the same way while the agent is quiet) and survives up to five consecutive transient failures (a 429, a gateway 5xx or a dropped connection), waiting at least as long as a `Retry-After` asks. Once the session has settled, `--follow` reads the transcript one last time with the same tolerance. If that read still fails, the command warns on stderr that the end of the transcript may be missing, then prints the session and exits by its state as usual: the result is already known (w32/010).
- **Exit codes.** These apply to `run`/`send` with `--wait` or `--follow`, to `wait`, and to `logs --follow`. With several sessions, a failure wins over a cancel, which wins over needs-input. A plain `get` always exits 0.

  | Code  | Meaning                                                        |
  | ----- | -------------------------------------------------------------- |
  | `0`   | the turn succeeded                                             |
  | `1`   | the CLI or the API failed (network, auth, a refused request)   |
  | `2`   | usage error                                                    |
  | `3`   | the turn failed; stderr names its `failureReasonCode`          |
  | `4`   | the session was canceled                                       |
  | `5`   | reserved for `needs_input`, which sessions never enter         |
  | `6`   | `automations run` started no session (skipped, denied, failed) |
  | `124` | `--timeout` elapsed while the session was still working        |
  | `130` | interrupted (Ctrl-C or SIGTERM)                                |

- **Output.** `-o json` (or `yaml`) prints the API's own answer on stdout: the session view, a JSON array for several sessions or for `files`. YAML keeps every integer digit for digit, whatever its size (a file's `size`, an id in the session's own `outcome.result`). While `--follow` streams, the transcript goes to stderr so stdout stays parseable.
- **Idempotent create.** Each `run` sends one `Idempotency-Key` and repeats the create, with the same key, after a dropped connection, a 429 or a gateway 5xx. A retry can therefore never start a second session. Pass `--idempotency-key` to make re-running the whole command safe as well, for example keyed on the CI run id.

```bash
# In CI: fix a failure, wait for the outcome, and keep the agent's report.
bex agent run --repo acme/api --attach test-output.txt --output-schema schema.json \
  --max-spend 5 --timeout 45m --wait -o json \
  "Make the failing test in test-output.txt pass" > outcome.json
bex agent files "$(jq -r .id outcome.json)" --download ./agent-files
```

### Live sessions: `bex agent chat`

`bex agent chat <ags-id>` (alias `attach`; or `run … --chat`) is a session live in the terminal, Devin's `devin --cloud` ([ADR106](ADR106-cli-cloud-agents-automations.md) D2). It prints a header (`ags-… · repo@branch · model · state · $spent`), the transcript so far, then the agent's work as it happens: its text, one line per tool call (`▸ Bash go test ./...`), a summary at each turn's end (state, PR, cost). `--reasoning` also shows reasoning. With no session and the run flags or a task (`chat --repo acme/api "…"`), it starts one first; with neither, it picks one of your recent sessions.

- **Input is line mode,** not a full-screen UI, so scrollback, tmux and pipes keep working. A message typed while a turn runs waits, marked `queued — sends when this turn ends`, and goes out as the next turn (bex takes no messages into a running turn); typing again replaces it. Between turns a message is sent at once.
- **Commands:** `/attach FILE` and `/skill NAME[:ARGS]` go with the next message; `/files`, `/open`, `/tags`, `/tree`, `/cost` and `/archive` run the `bex agent` command of that name on the session; `/cancel` asks, then cancels; `/help`; `/quit` or `/exit`.
- **Leaving is not cancelling.** Ctrl-D, Ctrl-C and `/quit` leave; the session keeps running and `bex agent chat <ags-id>` attaches again. There are no permission prompts: cloud sessions run unattended.
- **`--read-only`** watches without sending (for a session you can read but not steer), following each turn someone else starts.
- **A terminal only:** without one, `chat` exits 2 and points to `run --follow` / `logs --follow`.

### Move work between your machine and a Cloud Agent: `bex agent handoff | pickup | ssh`

These are Devin's `/handoff`, `/pickup` and `devin ssh` ([ADR106](ADR106-cli-cloud-agents-automations.md) D5/D6), built on git and SSH you already use. No new bex-api route is involved: a session started on an existing `bex-agent/*` branch begins from it.

- **`handoff [<task…>]`** commits the worktree as it is (tracked changes, plus untracked files `.gitignore` does not exclude; `--tracked-only` sends tracked changes only) through a temporary index: your index, worktree, stash and HEAD stay as they are. It shows the branch, the commit it starts from, a diffstat and the untracked files it sends, then asks (`--confirm` skips; without a terminal, `--confirm` is required). It pushes that commit with your own git to `bex-agent/handoff-<commit>` on `--remote` (default `origin`, whose `github.com` URL names the repository unless `--repo` does), then starts the session there with every `run` flag (`--model`, `--max-spend`, `--secret`, `--skill`, `--attach`, `--wait`, `--follow`) or `--chat`. `--notes FILE` uploads notes the agent finds under `/attachments`. A create the API refuses deletes the pushed branch again. The idempotency key is the commit plus the task, so re-running after a dropped connection resumes the same session. Your local agent's conversation does not travel (bex has no local agent of its own); say what matters in the task or the notes. The pull request targets the repository's default branch.
- **`pickup <ags-id>`** fetches the session's branch from `--remote` and switches to it: a new local branch tracks it; an existing one fast-forwards, and one that has diverged is refused with the commands to reconcile it. Uncommitted changes are refused unless `--stash` stashes them first (untracked files too). The checkout's remote must be the session's repository. Before the agent has pushed, it says so and exits 1. `--wait` waits for the turn to end first; otherwise a running session is noted, since it may push more. The cloud session is untouched and still takes messages.
- **`ssh <ags-id> [-- <ssh args…>]`** runs your `ssh` against the session's sandbox (`ags-…@ssh.bex.co`, as `bex edit ags-…` uses), with your registered SSH key, `~/.ssh/config` and agent. The sandbox runs only while a turn does; a session without one says so and exits 1. Port forwarding is not available ([ADR106](ADR106-cli-cloud-agents-automations.md) OD1, `w1/145`). In `chat`, `/ssh` prints this command, since `chat` reads the terminal's input itself, and `/pickup` runs `pickup` in the current directory.

### Agent Automations: `bex agent automations`

`bex agent automations` (aliases `automation`, `aut`) manages [Agent Automations](ADR101-agent-automations.md): one schedule plus the session each fire starts, run as the member who created the automation. It is a thin client of `/v1/agent-automations` ([ADR106](ADR106-cli-cloud-agents-automations.md) D7): the API checks the schedule, the template, the creator and the spend; the CLI shapes requests and prints answers. `<automation>` is an `aut-` id or a name in the active workspace.

| Command | What it does |
| --- | --- |
| `list [--mine]` | The workspace's automations with the schedule in words, next fire and status (`--limit`, `--cursor`). |
| `get <automation>` | One automation: schedule, next and last fire, failures in a row, template summary, version. |
| `create --name NAME <schedule> [template flags] <task…>` | Create one. `--disabled` creates it paused. |
| `update <automation> [flags] [<task…>]` | Change only what the flags and a new task name; everything else keeps its stored value. The write carries the version it read, so a concurrent edit is refused (`AGENT_AUTOMATION_CONFLICT`: re-run to apply on top). |
| `delete <automation>` | Delete it and its run history; its sessions stay. |
| `pause` / `resume <automation>` | Stop or restart firing; `resume` re-validates the automation. |
| `run <automation>` | Fire it once now (the schedule does not move). With `--wait`/`--follow`/`--timeout`, wait on the session and exit by its state; a run that started nothing exits **6**. |
| `take-over <automation>` | A workspace admin becomes its creator; it then runs as them. |
| `runs <automation>` | Run history: fire time, outcome, session, missed ticks, error. |
| `apply -f FILE\|DIR\|-` | Create or update automations from definition files (below). |
| `export (<automation>… \| --all)` | Print automations as definitions `apply` reads. |

`delete`, `take-over` and an `apply` that writes ask on a terminal and need `--confirm` elsewhere.

- **Schedule flags.** `create` takes exactly one of `--every hour --minute M`, `--every day --at HH:MM`, `--every week --at HH:MM --on mon,thu`, `--once TIME` or `--cron "M H DoM Mon DoW"`. `--timezone` is an IANA name and defaults to this machine's (`$TZ`, else the `/etc/localtime` link), printed on stderr; with no IANA name found the command asks for `--timezone`. `--ends TIME` stops firing (exclusive; a bare date `YYYY-MM-DD` includes that whole day) and `--no-end` removes it. `--once` and `--ends` take RFC 3339 or `YYYY-MM-DDTHH:MM` read in the time zone. On `update`, a new `--every`/`--once`/`--cron` keeps the time zone, the end and the stored fields the new frequency still uses (a day schedule switched to `--every week --on mon` keeps its `--at`), and drops the rest; other schedule flags change only their field.
- **Template flags** are `run`'s: `--repo`, `--branch`, `--agent`, `--model`, `--title`, `--tag`, `--max-spend`, `--skill`, `--secret`, `--output-schema`, and the task as arguments or `-f FILE`. A template on the platform model needs `--max-spend` (`AGENT_AUTOMATION_SPEND_LIMIT_REQUIRED`). `--attach`, `--secret-from-env`, `--idempotency-key` and `--parent` belong to a single session and are refused with the reason.
- **Definition files.** A definition is the REST create body, in YAML or JSON: `name`, `enabled`, `triggers`, `template`. A file may hold several YAML documents; a directory contributes its `*.yaml`, `*.yml` and `*.json`. Any other key (such as `id` or `version` pasted from `get -o yaml`) is refused before anything is sent; `export` prints the right shape.
- **`apply`** matches each definition by name: a missing automation is created, a changed one updated (with the version it read), the rest left alone. It compares only the fields a file sets, because the API fills defaults such as the title; to clear a field, set it empty. It never deletes: an automation missing from the files stays. `--dry-run` prints the plan and each changed field without writing. An automation someone else created answers `AGENT_AUTOMATION_NOT_CREATOR`; a workspace admin can `take-over` it first. One failed definition does not stop the others, and the command exits 1 naming how many failed. An automation the platform paused (`pausedReason`) is not resumed by `enabled: true`; `apply` says to `resume` it.
- **`export`** prints each automation's `name`, `enabled`, `triggers` and `template` as YAML documents (`-o json`: a JSON array), so `export | apply -f -` changes nothing.
- **Output.** `-o json` (or `yaml`) prints the API's own answer; `apply -o json` prints the plan, with each written automation under `automation`.

```yaml
# .bex/automations/nightly-deps.yaml
name: nightly-deps
triggers:
  - type: schedule
    schedule:
      { frequency: week, at: "03:00", weekdays: [mon], timezone: Europe/Berlin }
template:
  repo: acme/api
  task: Update dependencies, run the tests, open a draft PR.
  agentConfig: { agent: claude }
  maxSpendUSD: 5
  secretNames: [NPM_TOKEN]
```

```bash
# Preview in a pull request, apply on main.
bex agent automations apply -f .bex/automations/ --dry-run
bex agent automations apply -f .bex/automations/ --confirm
# Test one now; fail the job if it started nothing or the session failed.
bex agent automations run nightly-deps --wait --timeout 45m -o json > session.json
```

### Workspace agent configuration: `bex agent secrets | skills | env | models`

These manage what sessions and automation templates refer to by name ([ADR106](ADR106-cli-cloud-agents-automations.md) D8), each a thin client of its route. Writes to skills and the environment need a workspace admin; the API enforces it.

| Command | What it does |
| --- | --- |
| `secrets list` | The workspace's agent secret names ([ADR098](ADR098-agent-session-secrets.md)); values are never shown. |
| `secrets set NAME [--from-env VAR]` | Create or replace a secret. The value comes from `--from-env`, a hidden prompt on a terminal, or stdin (one trailing newline dropped), never from an argument. |
| `secrets delete NAME` | Delete it (asks, or `--confirm`). Sessions that name it then fail to start. |
| `skills list` | The workspace skills ([ADR099](ADR099-agent-skills.md)): name, version, enabled, triggers, description. |
| `skills get NAME [--version N]` | Print the skill's SKILL.md, or one saved version of it. |
| `skills create -f SKILL.md` / `skills update NAME -f SKILL.md` | Save a whole SKILL.md (YAML frontmatter with `name` and `description`, then the body); `-` reads stdin. Each save keeps a version. |
| `skills versions NAME` / `skills delete NAME` | The version history (`--limit`, `--cursor`); delete the skill and its history. |
| `env` | The agent environment: network tier, custom hosts, default model, model source, monthly spend cap and month-to-date spend. |
| `env set …` | Change only what the flags name: `--network-tier`, `--custom-host` (repeatable, replaces the list) or `--no-custom-hosts`, `--default-model` (`""` returns to the provider default), `--spend-cap USD\|default`, `--model-key-stdin` (the workspace's own model key, from stdin or a hidden prompt) or `--clear-model-key`. |
| `models` | Whether the workspace is ready for sessions, the GitHub connection, and each agent with whether it can reach its model. |

Workspace skills are what cloud sessions read; skills for the coding agents on your own machine are [`bex skills`](#compatibility-and-branding-limits).

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

- **Help chrome:** `Use` is `bex`, examples/Short/Long get safe string rewrites (backtick-, single-, and double-quoted `render …` command references, line-leading bare commands, and the `on/to/of/with/from/the/your Render` phrases), help prints `bex CLI v…`, and the ungrouped-commands header matches `CommandPath == "bex"`. Hostnames such as `render.com` are never rewritten globally; a help page that cites `render.com/docs` carries a narrow per-command override instead. `jobs create` drops its `render.com/docs/one-off-jobs` comment, which has no Bex page (w7/045). `ssh` links `https://bex.co/docs/platform/ssh#register-a-public-key` and `ea blueprints` links `https://bex.co/docs/platform/blueprints` (`branding.helpDocsLinks`, w32/008). A test walks every help page, so a pin bump that adds such a link to a served command fails CI.
- **`bex docs`:** opens the public Bex docs site, `https://bex.co/docs` (`branding.DocsURL`), as `render docs` opens `render.com/docs`.
- **`bex -v`:** leads with `bex vX.Y.Z`, then a separate `compatible with Render CLI v…` line; update checks still hit this repo's `bex-cli/v*` releases.

Safe rewrites preserve **`render.yaml`** (and the `bex.yml` filename alias).

**Run-time command references name `bex` (user decision 2026-11-07, w2/m263 D4=A).** Upstream output that tells the user to run `render <command>` says `bex <command>` instead. This covers text output, the JSON/YAML `meta.next` field agents follow, and backtick or quoted forms such as ``Use `render workspace set` ``. The rule covers the whole class. It is implemented in `internal/branding/runtime.go` as wrappers around the upstream `RunE`, like `brandLoginOutput`. No upstream `RunE` body is rewritten:

- **Returned errors, every command.** Upstream returns most hints as errors, which cobra prints to stderr. The wrapper rewrites the message, and `errors.Is`/`errors.As` still reach the original, so exit codes and `ErrLogin` handling are unchanged. This covers the resource-not-found, auth and workspace hints below, the `psql` usage line, and the stale manual env var file refusal.
- **Stdout, for the commands listed in `runtimeStdout`:** `ea blueprints create|plan|approve|update`, `logout` and `postgres create`. For the blueprint commands only the next step changes: JSON and YAML `meta.next`, or the line after the text `Next:` title. Blueprint names, repos, paths, notices and the rest of the API data stay byte-exact. `logout` prints only upstream copy, so every reference in it is rewritten. `postgres create` is rewritten like `logout` while its interactive wizard draws, so the success screen says "Run `bex pg get <name>`"; with `--output json|yaml|text` only the next step changes, as for the blueprint commands. The wizard's wrapper passes the terminal through, so bubbletea still sizes the screen from it (w2/073). Stdout is not wrapped for every command, for two reasons. A wrapped writer hides the terminal from upstream's `*os.File` checks (table width, color, `ssh`'s TTY). And `logs`, `sandboxes exec` and `ssh` stream user data through stdout, which must never be edited.
- **cobra `Deprecated` notices**, rewritten as command metadata with the help overlay. cobra prints a notice before the launcher's initializer runs, so this reaches only commands that exist when `branding.Apply` runs. The late-added deprecated aliases (`sandbox-groups builds`, the `workflows` shortcuts) keep upstream's notice. Both belong to commands bex does not serve (see below).

A reference is `render`, a space, and a command the root actually has (`ea`, `workspace`, `services`, …). It must sit at the start of a line (after indentation), right after a backtick, a quote or `| `, or be a bare `` `render` ``. URLs (`render.com`), file names (`render.yaml`), `RENDER_*` variables, data such as a service named `render-x`, and the product name in a sentence (for example "Render will start applying your planned changes") never match this rule. Product-name prose and variable names change only under the two exact-phrase rules below.

**Product-name prose reads Bex, in exact phrases only (user decision D2=A 2026-11-11, w2/073, w2/082).** Render's own CLI names Render, so the bex CLI names Bex. `runtimeProse` (`internal/branding/runtime.go`) is a table of whole upstream phrases, applied to the stdout of the `runtimeStdout` commands together with the command-reference rule:

- "Successfully logged out of Render." (`cmd/logout.go:83`)
- "delete the API key from your Render Dashboard." (`cmd/logout.go:36`) and "revoke your token in the Render dashboard:" (`cmd/logout.go:71`)
- "Render will start applying your planned changes." (`pkg/blueprint/output.go:149`)

Nothing else that mentions Render changes: a service named `Render-x`, a repo URL and any other API data stay byte-exact.

Regenerate the inventory on every pin bump (step 4 of the re-diff) and check that each hit reaches one of the three seams:

```bash
MOD=$(cd lego/cli && go list -m -f '{{.Dir}}' github.com/render-oss/cli)
grep -rnE '"[^"]*\brender [a-z]' "$MOD" --include='*.go' \
  | grep -v '_test.go' | grep -vE 'Example|Short:|Long:|Use:|Deprecated:'
```

At the current pin (**v2.36.0**, `4fd34231b637`) it yields the following.

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
- `pkg/tui/views/pgcreate.go:797` — "Run `render pg get <id>` to check if it's ready yet.", on the interactive `pg create` wizard's success screen (`postgres create` stdout, rewritten while the wizard draws)

**`ea blueprints` next steps (v2.35.0)** (stdout `meta.next` and text `Next:`, rewritten):

- `pkg/blueprint/output.go:120,127` — "render ea blueprints approve \<id>" after a plan, "fix the errors above, then run \"render ea blueprints plan \<id>\"" after a failed plan
- `pkg/blueprint/output.go:60,110` — the no-changes hints citing `render ea blueprints plan` / `approve`
- `pkg/blueprint/output.go:149` — after approve: "Render will start applying your planned changes … `render services` and `render deploys` commands". The command references change, and the leading sentence reads "Bex will start applying" (product-name prose, above).
- `pkg/blueprint/manualenvvars.go:71` — the stale manual env var file refusal citing `render ea blueprints approve` (a returned error)

Tests: `internal/branding/runtime_test.go` (the reference rule, JSON/YAML/text next-step rewrites, error unwrapping, the product-name phrases, the wizard screen) and `runtime_copy_test.go`. The latter drives the built binary against a stub API: `ea blueprints create` in text, JSON and YAML, `workspace current` with no workspace, and render-shaped data (`render-x`, `render.yaml`, `render.com`, `RENDER_*`) that must survive.

**Run-time copy about a variable names the one the user set (w32/009).** When the upstream variable is unset, the launcher fills `RENDER_API_KEY` from `BEX_ACCESS_TOKEN` and `RENDER_OUTPUT` from `BEX_OUTPUT`. Upstream's messages then name its own variable, which is not in the user's environment. `bridge.FilledFromBex` records what the launcher filled in this process, and `branding.NameBexInputs` renames those variables in upstream's exact phrases:

- **`logout` stdout** (`cmd/logout.go:34,73,79`): "via the RENDER_API_KEY environment variable." and both "Note: RENDER_API_KEY is still set in your environment." notes name `BEX_ACCESS_TOKEN`.
- **The output-format check** (`pkg/command/outputresolver.go:107`): "invalid RENDER_OUTPUT value: …" names `BEX_OUTPUT`. Upstream prints it to stderr from the root `PersistentPreRunE`, before any `RunE`, so neither seam above reaches it. The launcher wraps that hook and rewrites stderr for that call only. It does not validate `BEX_OUTPUT` itself: as with upstream, `--help` still answers when the value is bad.

A `RENDER_*` variable the user set keeps upstream's name, including when the Bex input is set beside it (the upstream variable is the one in force). These are the only upstream messages that name a variable the bridge fills. Check on every pin bump:

```bash
grep -rnE 'RENDER_(API_KEY|WORKSPACE|HOST|OUTPUT)' "$MOD" --include='*.go' | grep -v '_test.go'
```

Tests: `TestNameBexInputsRenamesOnlyFilledVariables` and `TestBrandPreRunOutputNamesBexOutput` (`internal/branding/runtime_test.go`), and `TestLogoutNamesTheVariableTheUserSet` and `TestBadOutputVariableIsNamedAsSet`, which drive the built binary.

**`bex skills` installs bex's own skills (w8/m143).** The launcher replaces upstream's `skills` tree. Upstream clones `render-oss/skills`, whose skills drive the `render` CLI, `mcp.render.com` and `dashboard.render.com`. The bex tree installs a pack embedded in the binary, derived from that MIT-licensed repository at a pinned commit by `lego/cli/internal/skills/packgen` and regenerated with `lego/cli/hack/sync-skills.sh`. The pack's 20 `bex-*` skills operate bex (`bex` CLI, `https://api.bex.co/mcp`, `dashboard.bex.co`, `onbex.co`). They keep Render's docs as the behavior reference and drop Render Workflows and sandboxes. The subcommands and flags are Render's (`install`, `list`, `update`, `remove`; `--tool`, `--skill`, `--scope`, `--dry-run`, `--all`, `--force`). State is `bex-skills.yaml` in the bex config directory. Selections are always explicit `bex-*` names, so a user's genuine `render-*` skills and `~/.render/skills.yaml` are never touched. In a terminal with no `--output`, `install` prompts for scope, tools and skills, and `remove` prompts for the skills to remove, as Render's commands do; each flag skips its prompt (w8/214). `list` and `update` print text there, and none of the commands panics non-interactively. The previous "accepted branding residual" (w8/m29 t004) was withdrawn because its effect was functional, not copy.

**Identity, storage and copy that branding deliberately leaves alone:**

- automatic login TUI inside other interactive commands (`views.LoginView`) saying “Render Dashboard”; explicit `bex login` rewrites only its fixed prompt prefix through Cobra stdout, preserving the device code and verification URL byte-for-byte
- User-Agent `render-cli/<upstream version>` (`cfg.Version` stays the pinned upstream release for the compatibility ledger)
- hard-coded OAuth public client id (Hydra bootstrap contract)
- one-time analytics notice copy (Render-branded disclosure + `render.com/docs` link + an invitation to re-enable Render telemetry — do not follow; see CLI usage telemetry above)

**`sandbox-groups` (v2.36.0)**: `pkg/sandboxgroup/repo.go` returns "make another group the default with `render sandbox-groups default set <sandboxGroupId>` first" (a returned error, rewritten); the other new hits (`cmd/sandboxgroups.go`, `sandboxgroupsdefault.go`, `sandboxgroupsdelete.go`, the `sandboxcredentialscreate.go` example) are help, rewritten by the overlay. bex answers `delete` and `default set` with `SANDBOX_GROUP_OPERATION_UNSUPPORTED` and `credentials` with `SANDBOX_CREDENTIALS_UNSUPPORTED` (w3/m303), so the repo error is not reachable today; groups themselves are served since w22/m311.

Strings inside commands Bex does not serve (`render workflows …` deprecation notices and the `render workflows flows list` / `tasks list` slug hints in `cmd/flow.go` and `pkg/tui/views/workflows/loader.go`, the `render sandbox-groups images …` deprecation notices on the hidden `sandbox-groups builds` alias, `render sandboxes snapshots …` help) are in the grep output but are not user-reachable residuals here — the underlying operations are non-goals in [`docs/cli-compatibility-checklist.md`](cli-compatibility-checklist.md).

The server-side compatibility ledger, including known Bex non-goals such as workflows, ephemeral SSH, and `ea` objects, is [`docs/cli-compatibility-checklist.md`](cli-compatibility-checklist.md). The imported command can only work where Bex implements the corresponding API operation; it does not turn an unimplemented Bex feature into a supported one.

If deeper runtime branding or a dedicated OAuth public-client identity becomes a product requirement, make that an explicit fork/extension decision rather than silently changing the imported command surface.
