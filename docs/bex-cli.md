# `bex` CLI

`bex` is a Bex-configured executable of the upstream [Render CLI](https://github.com/render-oss/cli). It imports the pinned upstream command package: commands, flags, parsing, request construction, and most runtime behavior are upstream. The Bex bridge changes the default API origin and local configuration location, registers Bex-native commands (`bex code`, `bex edit`, `bex upgrade`) alongside the imported command tree, and applies a help-chrome branding overlay, a fixed-prefix explicit-login prompt rewrite, and a run-time `render <command>` → `bex <command>` rewrite (`internal/branding`) without modifying upstream implementations.

The current pin and its update procedure are recorded in [`lego/cli/UPSTREAM_RENDER_CLI.md`](../lego/cli/UPSTREAM_RENDER_CLI.md).

## Install

One line — detects OS/arch, verifies the Sigstore-signed `checksums.txt`, installs to `~/.local/bin` (`BEX_VERSION` pins, `BEX_INSTALL_DIR` retargets). Requires [`cosign`](https://docs.sigstore.dev/cosign/system_config/installation/) **v2.4.2 or newer** on PATH so the installer shares the same release-workflow identity policy as `bex upgrade` (releases are signed with cosign v3 as a Sigstore protobuf bundle, which older cosign cannot read; the installer checks the version and names the requirement):

```bash
curl -fsSL https://raw.githubusercontent.com/bex-co/bex/main/scripts/install-bex.sh | sh
```

Or via Homebrew: `brew install bex-co/tap/bex` — the formula in [bex-co/homebrew-tap](https://github.com/bex-co/homebrew-tap) is rendered by `scripts/bex-cli-formula.sh` and pushed automatically by the release workflow (gated on the `BEX_TAP_PUSH_KEY` secret: the private half of a write **deploy key** scoped to only the tap repo, custodied via `.env`'s `BEX_TAP_PUSH_KEY_FILE` + `scripts/gh-secrets.sh`).

Both end with the CLI's one-time [usage-telemetry notice](#cli-usage-telemetry): the installer prints it and records it as shown, and Homebrew shows it as caveats.

Both channels consume the `bex-cli/v*` GitHub releases: when such a tag is published, CI creates Linux and macOS archives, `checksums.txt`, and a keyless cosign signature bundle over the checksums (GitHub-OIDC signed; no key custody). Each release's notes list only the CLI's own user-facing changes since the previous `bex-cli/v*` release — the monorepo's other churn is never a CLI change — with install, upgrade, and verify guidance; the Homebrew tap commit carries the same highlights. To verify a download's provenance manually (cosign v2.4.2 or newer):

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
| `BEX_API_KEY` | A bex API key for non-interactive use, as `RENDER_API_KEY` is for Render's CLI: the launcher sends it as the bearer, with no login and no exchange ([CI and automation](#ci-and-automation)). It outranks `BEX_ACCESS_TOKEN` and a stored login. It is not persisted by the bridge, and `bex logout` neither revokes nor unsets it. Works since `bex` v0.8.0; v0.7.0 and older ignore it, and take the key in `BEX_ACCESS_TOKEN` instead. |
| `BEX_ACCESS_TOKEN` | An already-issued OAuth access token for an unattended invocation, used when `BEX_API_KEY` is unset. An API key works here too, because both variables are sent as the bearer. It is not persisted by the bridge. `bex logout` neither revokes nor unsets this environment credential. |
| `BEX_NO_UPDATE_NOTIFIER` | Any non-empty value disables the update check entirely. By default `bex -v` reports bex's own release identity (`bex vX.Y.Z` plus a `compatible with Render CLI v2.36.0` line) and checks this repo's `bex-cli/v*` releases for something newer; every hint names the upgrade command for the install channel (`bex upgrade`, or `brew upgrade bex` for a Homebrew-owned binary); a successful `bex login` ends with the same hint (TTY or not — it replaces upstream's banner, which advertised Render CLI releases); after normal commands a gh-style passive notice appears at most once per 24h (cached under `~/.bex/cache/`), only on a TTY, and never when `CI` is set. Check failures are always silent. |
| `BEX_CLI_DISABLE_ANALYTICS` | Any non-empty value disables CLI usage telemetry (see below). Telemetry is otherwise on once the one-time notice has been shown: one event per invocation to bex-api, never to Render. |

The launcher sends the active workspace from `bex workspace set` or `BEX_WORKSPACE` to the configured API in `X-Bex-Workspace`. Service names on `/v1/services/{name}/…` resolve only in that workspace, including renamed services, deploys, restart, instances and jobs. A missing name returns 404; it never falls through to another workspace. Without a selection, a name must be unique across visible workspaces or the API returns `409 SERVICE_NAME_AMBIGUOUS` with the candidate IDs. Service IDs remain global addresses and authorize against the resource's workspace. This header is a bex extension; Render's corresponding paths accept IDs.

For example, a local run never needs a `RENDER_*` setting:

```bash
BEX_HOST=http://localhost:8090/v1/ bex workspaces -o json
```

## CLI usage telemetry

Eligible `bex` invocations send a usage event to bex-api (`POST /v1/cli-telemetry-events`, w5/m92): the command path, duration, exit code, OS/arch, output format, TTY/CI/agent signals (environment-variable _names_ only, never values), the active workspace id, and a stable per-machine install id. Each request also carries an `X-Bex-CLI-Version` header naming the launcher's own release, stored beside the pinned upstream version so release adoption is measurable (w5/m94); the two are separate fields, and a request without the header is accepted and recorded as unknown. Command arguments, prompts, and secret values are never collected. Sending is best-effort and cannot fail your command; the upstream sender chooses detached or bounded synchronous delivery according to the execution environment. Events are attributed to your authenticated identity server-side, retained under the platform audit-retention window, and never shared with Render.

Every Bex-native command reports exactly once, with its command names only (ADR113 D7). Most return through the imported CLI's post-run hook like any Render command. The ones that hand the terminal to another program can never reach that hook, so they report themselves immediately before they replace the process: the provider launchers (`bex glm`, `bex muse`, `bex kimi`, `bex deepseek`), `bex edit` and `bex <path>`, `bex agent open` when it opens bex desktop, and `bex agent ssh`. `bex s3` hands the terminal to no program: it is bex's own client, returns through the same hook as any Render command, and reports once per command (`bex s3 cp`, `bex s3 sync`, and so on). A root `--version` answer emits one event with completion kind `version`, a deliberate divergence from upstream, which treats version as a non-event. None carries the arguments handed to the launched program, and all honor exactly the opt-outs and the one-time notice below. The outcomes `bex agent` reports by exit code (3 a failed turn, 4 canceled, 6 an automation run that started no session) are stored as completion kind `explicit_exit` with that code, not as CLI failures, and so is the `2` of a Bex-native command's wrong command line. `lego/cli/telemetry_coverage_test.go` fails when a new process-replacing call site lands without a self-report and a contract test.

The ops-only [CLI usage board](https://obs.bex.co/d/bex-cli-usage/cli-usage) visualizes observed adoption, repeat usage, command outcomes/duration, and agent/CI signals, sliced by either version axis. It does not measure all installed CLIs: opt-outs, unavailable authentication, and early initialization failures are absent — an unauthenticated run in particular has no identity to attribute an event to, so it is silent by construction. See [metric definitions](runbooks/cli-analytics.md).

Opt out with `BEX_CLI_DISABLE_ANALYTICS=1` (or the cross-tool `DO_NOT_TRACK=1`, which the imported CLI honors directly). An explicit `RENDER_CLI_DISABLE_ANALYTICS` is left untouched as the upstream-developer escape hatch.

Nothing is sent until you have been told (ADR113 D5, the same gate as Render's CLI). The first `bex` command you run in a terminal prints a one-time notice on stderr after its output: what is collected, that it goes to Bex and not to Render, and how to opt out. That run sends nothing; later runs send. If you opted out, the notice says so and names the variable you set. The notice is recorded in `~/.bex/state/analytics/bex-notice-shown` (under `BEX_CLI_CONFIG_DIR` when you set it), so it shows once per machine. The rule has three edges:

- **CI.** With `CI=true` (or `1`) a run sends without a notice and records nothing, as Render's CLI does.
- **No terminal.** A run whose stderr is not a terminal cannot show the notice, so it sends nothing until a terminal run has shown it. `scripts/install-bex.sh` prints the notice at the end of an install and records it, so a headless install (an agent in a container with an API key) reports from its first run. `bex analytics notice` does the same for any other installer. The Homebrew formula shows the notice in its caveats but cannot record it, so a Homebrew install is disclosed on its first terminal run.
- **Process-replacing commands.** `bex glm` and the other provider launchers, `bex edit`, `bex agent open` and `bex agent ssh` hand the terminal to another program, so they cannot print afterwards. They send nothing until a later run has shown the notice. `bex --version` waits the same way.

The imported CLI's own notice, which names Render, never prints: the launcher records it as shown on every run.

## CI and automation

Non-interactive use is one variable, as `RENDER_API_KEY` is for Render's CLI. Set `BEX_API_KEY` to a bex API key; there is no login and no token exchange:

```bash
BEX_API_KEY=<key> bex services -o json
```

`BEX_API_KEY` works since `bex` v0.8.0: the launcher gained it in `8227312bc`, after v0.7.0 (`bex --version` names yours). v0.7.0 and older ignore it and answer ``Error: run `bex login` to authenticate``. Give them the key as `BEX_ACCESS_TOKEN=<key>` (or `RENDER_API_KEY=<key>`) instead, which they send as the bearer; in the workflow below, that is `BEX_ACCESS_TOKEN: ${{ secrets.BEX_API_KEY }}`.

Create the key in the dashboard (**Settings → API Keys**) or with `POST /v1/api-keys`. A key is one string, `bex_<id>_<secret>`, shown once when it is created. bex-api accepts it as `Authorization: Bearer`, as Render accepts a Render API key. It acts as its workspace's machine identity until it is revoked. The owner of a key created before this form existed composes the same string from the key's id and secret.

In GitHub Actions, keep the key in a repository secret and pass it to the step that runs `bex`, after the steps that install it ([Install](#install); the installer needs cosign on the runner):

```yaml
- name: Deploy
  env:
    BEX_API_KEY: ${{ secrets.BEX_API_KEY }}
  run: bex deploys create ${{ secrets.BEX_SERVICE_ID }} --wait --confirm -o json
```

- **Precedence.** The launcher sends the first of these that is set and not empty: `RENDER_API_KEY` (left untouched, as every explicit `RENDER_*` variable is), `BEX_API_KEY`, `BEX_ACCESS_TOKEN`, then the login `bex login` stored.
- **`BEX_ACCESS_TOKEN` still works.** It carries an OAuth access token. It also takes a key, because both variables are sent as the bearer.
- **Keep the key secret.** Do not print it, enable shell tracing around it, make it a job artifact or write it to `cli.yaml`.
- **Humans should use device login** in a terminal.

**Short-lived tokens.** A caller that wants a credential that expires can still exchange the key. Send `grant_type=client_credentials` with the key's id as `client_id` and its secret as `client_secret` to the OAuth token endpoint: `https://oauth.bex.co/oauth2/token` on hosted bex, as named by `https://api.bex.co/.well-known/oauth-protected-resource`. The credentials go in the form body (`client_secret_post`); HTTP Basic is refused with `401 invalid_client`. The returned `access_token` lasts 15 minutes and goes in `BEX_ACCESS_TOKEN` (verified on production, 2026-09-15).

`bex logout` manages the stored interactive OAuth session only. It neither revokes nor unsets `BEX_API_KEY` or `BEX_ACCESS_TOKEN`: unset the variable when the job ends, and revoke a key in the dashboard or with `DELETE /v1/api-keys/{id}`.

## Reading a whole build log

A non-interactive `bex logs` (`-o json`, `yaml` or `text`, which is also what a pipe or a CI job gets) prints one page and exits. bex serves at most 100 entries a page, whatever `--limit` says: `--limit` takes up to 1000 for the interactive viewer's sake and is still answered with 100 (w3/m475). The response says when more follow (`hasMore` and its cursors), but the pinned Render CLI prints only the entries and drops that, so nothing tells you the page is not the whole log. A Node build easily logs more than 1,000 lines, and its first 100 stop partway through the install. Whether Render's own server fills a larger page is unverified ([ADR018](ADR018-render-parity.md), Application logs).

There are three ways to read all of it:

- **Page forward through the deploy's window.** Take the deploy's `createdAt` and `finishedAt` from `bex deploys list <service> -o json`, then read with `--direction forward` from the first. After each page, read again with `--start` set to the last entry's `timestamp`. Both bounds are inclusive, so each page starts with the entries already printed at that instant: drop them by `id`. The log is complete when a page adds no new `id`. Use `-o json`, because `-o text` prints whole seconds, which are too coarse to resume from.

  ```bash
  set -eo pipefail
  svc=srv-… dep=dep-…
  read -r start end < <(bex deploys list "$svc" -o json |
    jq -r --arg dep "$dep" '.[] | select(.id == $dep) | "\(.createdAt) \(.finishedAt)"')
  : > build.jsonl
  while :; do
    new=$(bex logs -r "$svc" --type build --direction forward \
        --start "$start" --end "$end" --limit 100 -o json |
      jq -c --slurpfile seen <(jq .id build.jsonl) 'select(.id | IN($seen[]) | not)')
    [ -n "$new" ] || break
    printf '%s\n' "$new" >> build.jsonl
    start=$(tail -n1 build.jsonl | jq -r .timestamp)
  done
  jq -r '"\(.timestamp)  \(.message)"' build.jsonl
  ```

  On a production Node build of 2026-11-14 this printed all 1,142 entries in order, from `==> Build queued` to `==> Your service is live 🎉`. A single `--limit 1000` read of the same window, in either direction, printed 100. A deploy still building has no `finishedAt` yet, so follow it instead.

- **Follow a running build with `--tail`.** `bex logs -r <service> --type build --tail` streams each line the build prints, with no page limit. A tail shows only lines printed after it opens. Start it before `bex deploys create`, since it waits for the next build, and stop it with Ctrl-C when the build ends.
- **Use the interactive viewer.** In a terminal, `bex logs -r <service> --type build --start <createdAt> --end <finishedAt>` without `-o` opens the viewer. It loads the older pages as you scroll up.

## Bex-native commands

**A wrong command line exits `2`.** `bex code`, `bex upgrade`, `bex agent` and `bex s3` exit `2`, with nothing on stdout, for a word after a group that names none of its subcommands, arguments a command refuses, and a flag it does not have. The unknown word gets cobra's suggestions: `bex agent wiat ags-…` prints `unknown command "wiat" for "bex agent"` and `Did you mean this?` with `wait`. Without a word, a group prints its help, and `bex code` its providers' status, and exits `0`. The provider launchers and `bex edit` hand every argument to another program, so they refuse none. The imported Render commands keep the pinned Render CLI's codes, which differ: a refused argument or flag exits `1` (`bex pg get` with no id, `bex services --bogus`), and a word after a group that does nothing itself prints that group's help and exits `0` (`bex workspace frob`, `bex pg frob`). `bex skills` keeps Render's `skills` commands and flags, and exits `1` the same way (w8/368).

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

`bex agent` drives Cloud Agents, bex's cloud coding agents, from a terminal or a CI job. It is a thin client of the agent-session REST contract ([ADR094](ADR094-coding-agent-mcp.md), attachments [ADR095](ADR095-agent-attachments.md)) and authenticates like every other command (`bex login`, or `BEX_API_KEY` in CI). Sessions are created in the active workspace (`bex workspace set` or `BEX_WORKSPACE`).

| Command | What it does |
| --- | --- |
| `bex agent run [flags] <task…>` | Start a session. `--repo`, `--branch`, `--agent` (default `claude`), `--model`, `--template` (the sandbox's size, one of the templates `bex agent models` lists: `agent`, 2 CPU and 4 GB, or `agent-4c-8g`, 4 CPU and 8 GB; without it the workspace's default, which `bex agent env set --default-template` changes and which starts as `agent`; the API refuses any other name and lists the ones there are, and a sandbox killed for memory names its template and the next larger one), `--title`, `--tag` (repeatable), `--attach FILE` (repeatable; uploaded first, found under `/attachments`), `--output-schema FILE` (read the result from `outcome.result`), `--max-spend USD`, `--github-cli on\|off\|inherit` (the session's own `gh`, `agentConfig.githubCli`: off turns it off for this session; on turns it on against a workspace whose `gh` is off and is refused with `AGENT_SESSION_GITHUB_CLI_LOCKED`, reason `admin_required` for a non-admin or `mandatory` while the workspace's off is mandatory; a child session can only turn it off; without the flag, or `inherit`, the field is left out and the session follows the workspace; `get` shows a binding when there is one), `--skill NAME[:ARGUMENTS]` (a workspace skill to use this turn, repeatable, up to 5, [ADR099](ADR099-agent-skills.md)), `--secret NAME` (a workspace agent secret, repeatable) and `--secret-from-env NAME[=VAR]` (sends the local env var `VAR`, default `NAME`, as an inline secret for this session only; values are never taken on the command line, [ADR098](ADR098-agent-session-secrets.md)). `-f FILE` reads the task from a file (`-` for stdin). |
| `bex agent send <ags-id> <message…>` | Start the next turn, with `--attach`, `--output-schema`, `--skill`, `--secret` and `--secret-from-env` for that turn, and `--model` to switch the session's model from that turn on (`bex agent models` lists the choices; in `chat`, `/model ID`). While a turn runs, the server queues the message instead (see Waiting below); a queued message switches the model when it runs. |
| `bex agent wait <ags-id>…` | Wait until up to 20 sessions end their turn (`--any`: the first one). |
| `bex agent get <ags-id>` | Show a session and its outcome: state, summary, PR, shared files, result, model cost. When a settled session's latest turn was stopped with `stop`, a `Stopped` row says what became of its unpushed work: `Turn N was interrupted before it finished. Your changes are kept until 2026-11-16 10:00 PST.` (or that they are being saved, were continued, or were lost and why). The text view reads the turn with one extra request that asks for no transcript part; `-o json\|yaml` is the API's session answer and reads nothing more. |
| `bex agent list` | List the workspace's sessions (`--state`, `--phase`, `--tag`, `--repo`, `--archived`, `--mine`, `--created-by` (repeatable, up to 20), `--automation` (an `aut-` id or a name), `--parent`, `--since`/`--until` (RFC 3339 or an age: `24h`, `7d`), `--limit`, `--cursor`). |
| `bex agent logs <ags-id>` | Print the transcript: the agent's messages and one line per tool call (`--turn N`, `--reasoning`; `-o json` prints one transcript row per line). `--follow` streams until the turn ends. A turn stopped with `stop` ends with `■ Turn N was interrupted before it finished.` and, when its unpushed work was kept, is being saved, was continued or was lost, the dashboard's sentence for it (`Your changes are kept until …`, `Your unpushed changes were lost when the retained sandbox expired.`); with `-o json` that turn is printed as the API sent it, after its rows. |
| `bex agent files <ags-id>` | List the files the agent shared (`--direction input` or `all` for the attached ones; `--turn N`). `--download DIR` saves them, checks each SHA-256, never overwrites without `--force`, asks for a new link when a 15-minute download link has expired, and still lists the files that landed when one fails. |
| `bex agent cancel <ags-id>` | Stop the running turn and close the session. |
| `bex agent stop <ags-id>` | End the running turn only: the session stays open and its queued messages run next. Asks for no confirmation. Prints `Stopping the current turn. The session stays open and any queued messages run next; /cancel ends it.` (`-o json` prints the API's answer instead); exit `7` when no turn is running, `8` when the turn is still starting. The turn settles within a few seconds (the server's 15 s tick is only the fallback for a request another replica served), so `bex agent wait` follows it. |
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

- **Picking and completion.** On a terminal, a command whose one argument is a session (`get`, `logs`, `files`, `cancel`, `stop`, `open`, `delete`, `tree`, `cost`, `chat`) or an automation may leave it out: it lists your 20 most recent sessions (or the workspace's automations) and asks for a number, as Devin's `-r` does. Without a terminal an omitted argument is a usage error, as before. Shell completion (`bex completion …`) offers the same ids and names, giving up silently after 2 seconds.
- **Sessions by URL.** Every `<ags-id>` may also be the session's dashboard URL (`https://dashboard.bex.co/agents/ags-…`, on any dashboard origin), as Devin's CLI takes a session URL ([ADR106](ADR106-cli-cloud-agents-automations.md) D3). `run --parent <ags-id>` starts a child session; its spend counts against the parent's ceiling.
- **Waiting.** `run` and `send` return as soon as the turn starts. `--wait` polls until it ends; `--follow` streams the transcript while it runs. `--follow` (and `logs --follow` in text) read the live stream ([ADR047](ADR047-cloud-coding-agent-sessions.md) D9): a single-use read ticket from `attach-ticket`, then the gateway's server-sent events with the ticket in `X-Bex-Agent-Ticket` (no bearer), reconnecting with a fresh ticket after a drop and printing each item once. A deployment without the stream, or `logs --follow -o json` (one transcript row per line), polls as before. `--timeout 30m` bounds either one. REST has no blocking wait (`wait_agent_sessions` is MCP-only), so the CLI polls the session with backoff (2 s rising to 15 s; `--follow` backs off the same way while the agent is quiet) and survives up to five consecutive transient failures (a 429, a gateway 5xx or a dropped connection), waiting at least as long as a `Retry-After` asks. Once the session has settled, `--follow` reads the transcript one last time with the same tolerance. If that read still fails, the command warns on stderr that the end of the transcript may be missing, then prints the session and exits by its state as usual: the result is already known (w32/010).
- **Sending to a running session.** The server queues the message (up to 10 per session) and runs it as its own turn when the running one ends. `send` prints `Queued (position N): it runs when the session's running turn ends.` on stderr, the API's answer (the session with `queuedMessage`) on stdout, and exits 0. With `--wait` or `--follow` it then waits for **that message's** turn, not the running one: it polls `GET /v1/agent-sessions/{id}/queued-messages` (2 s rising to 15 s, surviving the same transient failures) until the message is no longer pending, then reads the session and does what the flag does for a message that starts a turn at once: `--wait` waits for the session to settle and exits by its state, `--follow` streams the turn. stdout is then the settled session, as for a direct send. If the message left the queue and the session has had no new turn (the server dropped it, or someone removed it), `send` prints `the queued message was not started; see the session` on stderr and exits 1. `--timeout` covers the queue wait too: a deadline while the message is still queued exits 124 with `the message is still queued after …`, leaves it queued and prints the API's answer to the send; Ctrl-C exits 130 and also leaves it queued. A server from before the queue refuses the message with `409 AGENT_SESSION_TURN_IN_FLIGHT`, exit 1, as it always did.
- **Exit codes.** These apply to `run`/`send` with `--wait` or `--follow`, to `wait`, and to `logs --follow`; `7` and `8` are `stop`'s. With several sessions, a failure wins over a cancel, which wins over needs-input. A plain `get` always exits 0.

  | Code  | Meaning                                                          |
  | ----- | ---------------------------------------------------------------- |
  | `0`   | the turn succeeded                                               |
  | `1`   | the CLI or the API failed (network, auth, a refused request)     |
  | `2`   | usage error                                                      |
  | `3`   | the turn failed; stderr names its `failureReasonCode`            |
  | `4`   | the session was canceled                                         |
  | `5`   | reserved for `needs_input`, which sessions never enter           |
  | `6`   | `automations run` started no session (skipped, denied, failed)   |
  | `7`   | `stop`: no turn is running, so there was nothing to stop         |
  | `8`   | `stop`: the turn is still starting; try again once it is working |
  | `124` | `--timeout` elapsed while the session was still working          |
  | `130` | interrupted (Ctrl-C or SIGTERM)                                  |

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

- **Input is line mode,** not a full-screen UI, so scrollback, tmux and pipes keep working. A message typed while a turn runs is queued on the server, marked `queued — sends when this turn ends (position N)`, and runs as its own turn once the running one ends (bex takes no messages into a running turn); typing again adds another message, up to 10, and a reattached `chat` lists what is still queued. `/stop` ends only the running turn and leaves the session open (`/cancel` ends the session; `bex agent stop` is `/stop` for scripts). Between turns a message is sent at once.
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
- **Template flags** are `run`'s: `--repo`, `--branch`, `--agent`, `--model`, `--template`, `--title`, `--tag`, `--max-spend`, `--github-cli`, `--skill`, `--secret`, `--output-schema`, and the task as arguments or `-f FILE`. A template on the platform model needs `--max-spend` (`AGENT_AUTOMATION_SPEND_LIMIT_REQUIRED`). `--github-cli on\|off\|inherit` is `template.agentConfig.githubCli`, the `gh` of the sessions the automation starts (an on is refused with `AGENT_AUTOMATION_GITHUB_CLI_LOCKED`, as on `run`); without a binding they follow the workspace's automations default. On `update` no flag keeps the stored binding and `inherit` removes it; a change reaches running sessions at their next `gh` call. `get` shows it, and `export`/`apply` carry it like any template field (`githubCli: null` in a file returns to inheriting). `--attach`, `--secret-from-env`, `--idempotency-key` and `--parent` belong to a single session and are refused with the reason.
- **Definition files.** A definition is the REST create body, in YAML or JSON: `name`, `enabled`, `triggers`, `template`, `overlapPolicy` (`skip`, the only policy so far) and `notifications`. A file may hold several YAML documents; a directory contributes its `*.yaml`, `*.yml` and `*.json`. Any other key (such as `id` or `version` pasted from `get -o yaml`) is refused before anything is sent; `export` prints the right shape.
- **`apply`** matches each definition by name: a missing automation is created, a changed one updated (with the version it read), the rest left alone. It compares only the fields a file sets, because the API fills defaults such as the title, and an update keeps what the file leaves out: a `template` or `notifications` object is sent with the file's fields laid over the stored ones, so a file may name only `template.task`, while `triggers` are replaced as the file writes them. To clear a field, set it empty. It never deletes: an automation missing from the files stays. `--dry-run` prints the plan and each changed field without writing. An automation someone else created answers `AGENT_AUTOMATION_NOT_CREATOR`; a workspace admin can `take-over` it first. One failed definition does not stop the others, and the command exits 1 naming how many failed. An automation the platform paused (`pausedReason`) is not resumed by `enabled: true`; `apply` says to `resume` it.
- **`export`** prints each automation's `name`, `enabled`, `triggers`, `template`, `overlapPolicy` and `notifications` as YAML documents (`-o json`: a JSON array), so `export | apply -f -` changes nothing.
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
| `env` | The agent environment: network tier, custom hosts, whether sessions get the GitHub CLI (`gh`) and whether its off is mandatory, the automations `gh` default (`inherit`, `on` or `off`), default model (marked when it is turned off, with the platform default that runs instead), the platform catalog models members may choose and the ones an admin turned off, default sandbox template with its size, model source, monthly spend cap and month-to-date spend. |
| `env set …` | Change only what the flags name: `--network-tier`, `--custom-host` (repeatable, replaces the list) or `--no-custom-hosts`, `--default-model` (`""` returns to the provider default; on the platform model one of the models `env` and `models` list), `--models ID,ID…` (replaces the platform catalog models members may choose and turns the catalog's others off; it must keep the catalog's default, else `AGENT_ENVIRONMENT_MODELS_INVALID`; a session, message or automation naming a turned-off model is refused with `AGENT_SESSION_MODEL_NOT_OFFERED`, and one already on it runs the workspace default from its next turn), `--default-template` (the sandbox template sessions and automation runs naming none run on, one of those `models` lists; `""` returns to `agent`; sessions created afterwards use it), `--github-cli on\|off` (off turns `gh` off for the workspace's sessions: turns dispatched afterwards get no `gh` configuration and a running session's next `gh` call is refused with a message naming the setting, while Git pushes to the session branch and the `bex` MCP tools keep working; on, the default, turns it back on), `--automations-github-cli on\|off\|inherit` (`automationsGithubCliEnabled`, the `gh` of the sessions automations start; `inherit` sends an explicit `null` and follows `--github-cli`; an automation's own binding wins; on is refused with `AGENT_ENVIRONMENT_GITHUB_CLI_LOCKED` while the off is mandatory), `--github-cli-mandatory on\|off` (`githubCliMandatory`: while `gh` is off, no automations default, automation or session turns it back on, and saving one that would is refused with a `…_GITHUB_CLI_LOCKED` code, reason `mandatory`; no effect while `gh` is on), `--idle-timeout MINUTES` (`idleTimeoutMinutes`, how long the sandbox of a session a person started stays up after its turn ends or its last editor disconnects, 5 to 120; Enterprise workspaces only, any other plan is refused with `PLAN_FEATURE_REQUIRED` and keeps 5; automations' and child sessions' sandboxes close when their turn ends; `env` shows it as `Idle timeout`), `--spend-cap USD\|default`, `--model-key-stdin` (the workspace's own model key, from stdin or a hidden prompt) or `--clear-model-key`. |
| `models` | Whether the workspace is ready for sessions, the GitHub connection, and each agent with whether it can reach its model and the model a session runs when it names none; below, the models an agent can choose from (the platform model catalog models the workspace's admins left on), the default marked, and the sandbox templates a session can name with `--template`, each with its CPU and memory and the workspace default marked. A fixed-model agent (`deepseek`, `muse`) shows its one model; on the workspace's own key any model its provider serves is accepted. |

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

## Object storage: `bex ea objects`

`bex ea objects list|put|get|delete` is Render's early-access object storage, served unchanged ([ADR112](ADR112-object-storage.md)). Your objects live in your workspace's `render-<region>` bucket on Wasabi. `put` and `get` upload and download straight to a presigned URL, so the bytes never pass through bex-api.

```sh
bex ea objects put backups/users.ndjson --file ./users.ndjson --region oregon
bex ea objects list --region oregon --limit 200
bex ea objects get backups/users.ndjson --file ./out.ndjson --region oregon
bex ea objects delete backups/users.ndjson --region oregon --yes
```

bex serves two regions, `oregon` and `frankfurt`; any other region answers 400. For buckets, listing, `sync` and presigned URLs of your own, use [`bex s3`](#buckets-objects-sync-and-presigned-urls-bex-s3). The limits are 5 GiB per object, 100 GB and 1,000,000 objects per workspace per region, and keys of 1 to 1024 bytes with no `.` or `..` segment. Render has not published its own limits, so these are bex's, listed with their status in the [object storage row of ADR018](ADR018-render-parity.md). A Render API key reaches every object of its workspace, as on Render. These commands need `bex` v0.7.0 or newer; run `bex upgrade` on an older binary. The unmodified `render` CLI works against bex too. If the platform has no Wasabi credential installed, the commands say that object storage is not offered yet. `--local` stays upstream's filesystem mode under `.render/objects/` and never calls bex.

### Buckets, objects, sync and presigned URLs: `bex s3`

`bex s3` is bex's own client of its S3-compatible object storage ([ADR112](ADR112-object-storage.md) D3). It signs and sends its own requests and needs nothing else installed: no AWS CLI, no `rclone`, no config file. Sign in with `bex login`; a key is made for you.

```sh
bex s3 mb s3://photos --region frankfurt     # a bucket in Frankfurt (the default is Oregon)
bex s3 cp cat.jpg s3://photos/cat.jpg
bex s3 ls s3://photos/
bex s3 sync ./site s3://photos/site
bex s3 presign s3://photos/cat.jpg --expires-in 3600
bex s3 cors set photos --file cors.json
bex s3 public photos enable                  # publish the bucket read-only (workspace admin)
```

**The commands.**

| Command | What it does |
| --- | --- |
| `bex s3 ls [s3://bucket[/prefix]]` | No argument: the workspace's buckets, the `render-` system buckets included. With a prefix: one level, a `PRE name/` line per folder and then date, size and name per object. `--recursive` lists every key with its full key; `--human-readable` and `--summarize`. A prefix matches by its characters, as on S3. |
| `bex s3 cp <from> <to>` | A file to a bucket, an object to a file, or an object to an object inside one workspace and region (copied on the server). `-` is standard input or output. `--recursive` copies a directory or a prefix; `--include` and `--exclude` pick paths by glob, the last matching one deciding; `--content-type`, `--metadata key=value` (repeatable), `--dryrun`, `--no-progress`, `--concurrency`. |
| `bex s3 mv <from> <to>` | `cp`, then deletes each source, only after its own copy succeeded. Same flags. |
| `bex s3 rm s3://bucket/key` | Deletes an object (a missing key is not an error, as on S3) and asks first in a terminal (below). `--recursive` deletes everything under the prefix in batches of 1,000; `--include`, `--exclude`, `--dryrun`, `--yes`, `--concurrency`. |
| `bex s3 sync <from> <to>` | Makes the destination hold what the source holds: copies files or objects that are missing, differ in size or are newer in the source (times in whole seconds), between a directory and a prefix in either direction or between two prefixes. Run again it copies nothing. `--delete` also deletes what only the destination has, and asks first in a terminal (below); `--size-only`, `--include`, `--exclude`, `--dryrun`, `--yes`. |
| `bex s3 mb s3://bucket [--region oregon\|frankfurt]` | Makes a bucket. |
| `bex s3 rb s3://bucket [--force]` | Removes a bucket; `--force` deletes its objects first and asks first in a terminal (below). A bucket with unfinished multipart uploads is refused (bex aborts them seven days after they start); a `render-` system bucket cannot be removed. |
| `bex s3 presign s3://bucket/key [--expires-in 3600]` | Prints a URL that reads the object with no key, for at most 604800 seconds. It stops working when the key that signed it expires, twelve hours for the key bex makes; the command says so when `--expires-in` is longer. |
| `bex s3 cors get\|set\|delete <bucket>` | A bucket's CORS rules. `set --file rules.json` reads the S3 API's JSON document (`{"CORSRules": [...]}`) and replaces the rules; `get -o json` prints a document `set` accepts; a bucket with no rules prints `no CORS rules`. |
| `bex s3 lifecycle get\|set\|delete <bucket>` | A bucket's expiry rules, the same way (`{"Rules": [...]}`). |
| `bex s3 info <bucket>` / `s3://bucket/key` | A bucket's region, public host, objects and bytes as of the last hourly count; or an object's size, ETag, time, content type and user metadata. |
| `bex s3 env` | Prints the endpoint and the key as `export` lines for another S3 client (an AWS SDK program, boto3, rclone's `RCLONE_S3_*`, s5cmd's `S3_ENDPOINT_URL`) and the settings no variable carries. `eval "$(bex s3 env)"` loads them; `-o json` prints them as data. |
| `bex s3 keys create [--name N] [--permission read\|readwrite] [--expires-in 12h]` | Creates a key. Without `--expires-in` it never expires (for CI, rclone, Terraform). The secret is printed once, never again; `-o json` prints the API's own answer. |
| `bex s3 keys list` / `revoke <id>…` | Lists the workspace's keys (status, expiry, last use) and revokes some; a revoked key stops working within 30 seconds. |
| `bex s3 public <bucket> enable\|disable\|status` | Publishes a bucket (below). |

A name `bex s3` does not define is an `unknown command`; to run another S3 tool against bex, `bex s3 env` prints what it needs. Output is one line per object, as in `upload: ./cat.jpg to s3://photos/cat.jpg`, `download:`, `copy:`, `move:`, `delete: s3://photos/cat.jpg`, `make_bucket: photos`; `--dryrun` prefixes `(dryrun) ` and changes nothing; `-o json` and `-o yaml` print the same results as data under `results`. Progress goes to stderr, and only to a terminal.

**Asking before a delete.** `rm`, `rb --force` and `sync --delete` ask in a terminal, before any request is made, in the words of `bex ea objects delete`:

```text
$ bex s3 rm s3://photos/cat.jpg
Are you sure you want to delete object 's3://photos/cat.jpg'? This cannot be undone. [y/N]: y
$ bex s3 rm s3://photos/tmp --recursive
Are you sure you want to delete every object under 's3://photos/tmp/'? This cannot be undone. [y/N]: y
```

`ea objects delete` names the object by its key, since it has no buckets; `rm` names it by its `s3://bucket/key`. Only `y` or `yes` goes on; any other answer, or an end of input, prints `Delete canceled.` and exits `0` having deleted nothing. `--yes` (`-y`) and the global `--confirm` ("Skip all confirmation prompts") skip the question. A run that is not a terminal never asks: the root command reads it the way it does for `ea objects delete`, so a pipe on stdin, stdout or stderr, `CI`, `TERM=dumb`, `RENDER_OUTPUT`, or `-o json|yaml|text` means no question, which is how a script runs these commands unchanged. `--dryrun` deletes nothing and is never asked, and `rb` without `--force` removes an empty bucket and does not ask. The source delete of `mv` does not ask, since its bytes survive at the destination. `bex ea objects delete` itself reads `--yes` only and still asks under `--confirm`.

**How it transfers.**

- Files go in 8 MiB parts, grown so an object fits in 10,000 parts, and at most `--concurrency` requests (default 8, at most 10) are in flight at once, whole command included, so a second command in the same workspace is not refused with `SlowDown` (the gateway admits 16 streams per workspace on each replica). The SDK retries a throttled or failed request before a command fails.
- A download is written to a temporary file beside its target and renamed when it is whole, so a failed or interrupted transfer leaves no partial file and does not replace an older one; the file gets the object's modification time, which is what lets a second `sync` copy nothing.
- A recursive command reads `s3://photos/2023` as the folder `2023/`: it never touches `2023-old/`. `ls` is the exception and matches by characters.
- `sync` reads both sides in key order, one page at a time, so a tree of millions of files is not held in memory. Only regular files are copied: a symbolic link, a device or a socket is skipped with one warning each. A directory that cannot be read stops the sync before any delete.
- One file failing is reported and the rest go on, and the command exits `1`; an error every later request would meet (an expired key, a quota, a refused operation) stops it. A command line that is wrong exits `2`, before any key is made.
- A key must be 1 to 1,024 bytes of UTF-8 with no leading or doubled slash, no `.` or `..` segment, and no backslash or control character ([ADR112](ADR112-object-storage.md) D7). `bex s3` refuses an `s3://` argument that breaks the rule before it sends anything, with exit `2` and the rule (`bex s3 cp a.txt s3://photos/x/../y`). Without that check the edge would clean the path and the request would fail its signature check. In a recursive upload or a `sync`, a file whose name holds a backslash or a control character is reported as failed, and the rest go on.

**What `bex s3` sets, and what it keeps out.**

- The endpoint is `https://objects.onbex.co`, path style, signed for the region `oregon` (`--region` overrides it). A control plane that is not the production one names its endpoint in `BEX_S3_ENDPOINT`.
- **A key is made for you.** `bex s3` makes an ephemeral key for the signed-in person in the active workspace (`bex workspace set`), good for twelve hours, keeps it in `~/.bex/state/s3/` with mode 0600 and replaces it when under one hour is left. Many commands started at once on a machine with no key make one. A command that outlives its key fails with a `403` that is not retried, and says so; a URL from `bex s3 presign` dies with its key whatever `--expires-in` says. For a transfer over hours, or a URL that must live longer, use a persistent key.
- **A persistent key from the environment.** With `BEX_S3_ACCESS_KEY_ID` and `BEX_S3_SECRET_ACCESS_KEY` set (both, or neither), `bex s3` uses that key and makes none: it needs no sign-in, no workspace and no `bex-api`, which is how CI, which has only an API key, uses it. Make the key once with `bex s3 keys create`.
- **Nothing of yours is read.** The client's configuration is built in code: your `~/.aws`, `AWS_PROFILE`, `AWS_ENDPOINT_URL` and every other `AWS_` variable change nothing, so your cloud credentials are never sent to bex and bex's key never lands in a file of theirs. A proxy in `HTTPS_PROXY` and a CA bundle in `SSL_CERT_FILE` are the Go runtime's, as for every bex command.
- Creating or revoking a key is a person's act: an API key (`BEX_API_KEY`) cannot do either, so in CI make a persistent key once from a signed-in session.
- This needs `bex` v0.9.0 or newer (`bex upgrade`); an older release lacks most of these commands, and `bex s3api`, which v0.8.0 had, is gone.

**Flags, output and exit codes.** They follow the other bex-native commands (`bex agent`): `-o json|yaml|text` (`bex s3 env` and the object listing print their own structure), exit `2` for a usage error (a malformed bucket name or key id, an object key bex cannot serve, an unknown action or flag, a missing or extra argument, `--concurrency` over 10, `--expires-in` outside its range) and `1` for any other failure. The key and bucket commands use the active workspace (`bex workspace set` or `BEX_WORKSPACE`; there is no per-command `--workspace`). Render has no S3 command and `bex ea objects` is unchanged, so there is no Render convention to match beyond these and the question before a bulk delete (above).

**Keys.** A key acts as the person who made it, in one workspace, with `read` (list and get) or `readwrite` (everything else) as a ceiling, and reaches every bucket of that workspace (per-bucket keys are not offered). It never holds the admin role, so a leaked key cannot publish a bucket. A key is revoked with its person: removing a member or deleting an account deletes the keys they made, and re-adding the person revives nothing. A workspace holds at most 100 live persistent keys and a person at most 20 live ephemeral ones; revoked and expired keys disappear after 7 days.

**Using other tools.** Any S3 client works with the endpoint, a key and path-style addressing; bex serves no virtual-hosted style (`<bucket>.objects.onbex.co`), so a tool that defaults to it must be told. `bex s3 env` has the values, and a persistent key (`bex s3 keys create`) is the one to hand to a tool that runs unattended.

- **An AWS SDK program or the AWS CLI:** `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`, the endpoint (`AWS_ENDPOINT_URL_S3` or `--endpoint-url`) and path-style addressing (`addressing_style = path` under `s3` in the tool's config).
- **rclone:** `RCLONE_S3_PROVIDER=Other`, the endpoint, the key and `RCLONE_S3_REGION` (rclone uses path style by default); then `rclone sync ./site :s3:photos/site`.
- **boto3:** `boto3.client("s3", endpoint_url="https://objects.onbex.co", config=Config(s3={"addressing_style": "path"}))`.
- **Terraform's AWS provider:** `s3_use_path_style = true`. It is not tested against bex, and attributes it reads on a bucket (versioning, ACLs, tags, encryption) answer `501`.
- **Presigned URLs** made with any SDK work until their key expires (at most 7 days for a persistent key).

**What the S3 API serves** (to `bex s3` and to any other client): bucket create, delete, head, location and listing; `PutObject`, `GetObject` with `Range` and the conditional headers, `HeadObject`, `DeleteObject`, `DeleteObjects`; `CopyObject` and `UploadPartCopy` inside one workspace and region (up to 5 GiB); `ListObjectsV2` and `ListObjects`; the whole multipart family; CORS and lifecycle (below); and `GetObjectTagging`, which is always empty. Objects are never versioned and every object is private. Differences from S3 that you will meet:

- **Two regions.** `mb --region frankfurt` (also `eu-central-1`, `eu-central-2`) or `oregon` (`us-west-1`, `us-west-2`; the default). Any other value answers `InvalidLocationConstraint`; bex does not compare a request's signing region with the bucket's. A bucket name is 3 to 63 lowercase letters, digits and hyphens, unique **within your workspace** (never `BucketAlreadyExists`), cannot start with `render-` and never changes. The workspace's `render-oregon` and `render-frankfurt` buckets, where `bex ea objects` keeps its objects, show up in `ls` and cannot be deleted; an object put either way is read the other way.
- **Not implemented, answered `501`:** versioning, ACLs and `x-amz-acl` other than `private`, bucket policies (publish with `bex s3 public`), tag writes and `x-amz-tagging`, object lock (`x-amz-bucket-object-lock-enabled: false` is accepted), encryption settings, replication, notifications, website hosting, accelerate, S3 Select, storage classes other than `STANDARD`, conditional writes (`If-None-Match` and `If-Match` on a put, which Terraform's state lock uses), signed-chunk payloads and SigV2, copies across workspaces or regions, a multipart listing with a `delimiter`, and a listing delimiter that is not one ASCII character.
- **Keys** are 1 to 1024 bytes and have no `.` or `..` segment, no doubled slash, no leading slash, no backslash and no control character: Traefik cleans such paths before bex sees them, so a client that sends one fails with `SignatureDoesNotMatch`. A put replaces an object; a delete of a missing key succeeds, as on S3.
- **Downloads are served as attachments** (`Content-Disposition: attachment` unless the object stored one, `nosniff`, no script), so a browser that follows a link never runs your HTML on bex's origin.

**Limits.** 5 GiB per object and part-count 10,000 (parts of 5 MiB or more, as Wasabi enforces; `bex s3` refuses a file past 5 GiB before sending anything, and an upload from a stream fails at that mark), 100 open multipart uploads per workspace (abandoned for 7 days: aborted), 1,000 buckets, 100 GB and 1,000,000 objects per workspace per region (a put over a cap is `403 QuotaExceeded`), 16 uploads or downloads at once per workspace on each of the two gateway replicas (`503 SlowDown`, which clients retry), reads paced at 8 MiB/s and uploads at 16 MiB/s per workspace per replica (one upload runs at that pace to `oregon` as to `frankfurt`: a single 2 GiB `PutObject` to `oregon` took 134 s, and 5 GiB takes about 5.5 minutes), 100 requests a second per workspace and 50 per key, and 5 listings a second. Every upload or download is cut off after 30 minutes, so 5 GiB in one request needs your own link to carry about 3 MiB/s; on a slower link, or with several large uploads at once in one workspace (they share the pace), use multipart, as `bex s3 cp`, rclone and boto3 do above 8 MiB, because its parts retry on their own. Reads are also limited to the larger of the bytes you store and 5 GiB a month, counted over both regions together: past it a read answers `403 QuotaExceeded` in every region until the month turns in UTC, and the message names the date reads resume. **A listing counts as egress** (Wasabi counts what a LIST reads), so a loop that lists a bucket of millions of objects spends the shared allowance. Deletes and overwrites free quota at the next hourly count, up to 72 hours later for a large region, so a `sync --delete` that frees space and refills it can be refused meanwhile. Each number is bex's own (Render and Wasabi publish none to match), recorded with its status in [ADR018](ADR018-render-parity.md). Egress is metered and not billed yet. Objects live on one provider in one region with no replica.

**CORS and lifecycle.** `bex s3 cors set` (S3's `PutBucketCors`) keeps up to 100 rules and 64 KiB per bucket; bex applies them on a public bucket's host and to presigned URLs on `objects.onbex.co` (the preflight names no credential, so the rule is found from the key id inside the presigned URL; a browser SDK that signs with headers cannot be served). `bex s3 lifecycle set` (`PutBucketLifecycleConfiguration`) takes `Filter.Prefix`, `Status`, `Expiration` (`Days` from 1 to 36,500, or a `Date` at midnight UTC, written `2027-01-01` or `2027-01-01T00:00:00Z`) and `AbortIncompleteMultipartUpload`; `Transition`, noncurrent-version and tag or size filters answer `501`. A prefix with a control character, or a file that is not UTF-8, is refused before anything is sent, because the rule stored would not be the one in the file. Rules run in an hourly sweep, at most 10,000 deletions per rule per run (a region with many objects is listed less often, so a rule can run up to 72 hours late). A short expiry saves nothing on storage: Wasabi bills a 90-day minimum per object, and whether it bills the rest for an object deleted sooner is unverified.

#### Public buckets: `bex s3 public`

```sh
bex s3 public photos enable      # prints https://pub-<id>.<hosting domain>/<key>
bex s3 public photos status
bex s3 public photos disable
```

With a bucket public, anyone can read its objects over the web at `https://pub-<id>.<hosting domain>/<key>` with no credential. Publishing is not an S3 call and not something an access key can do: it needs the workspace's admin role and your own login (`status` needs only read access), and `put-bucket-policy` answers `501`. It follows Cloudflare's `r2.dev` public development URL ([ADR112](ADR112-object-storage.md) D4, [ADR018](ADR018-render-parity.md)); Render has no public buckets. A deployment with no hosting domain refuses to publish.

- **What is served.** `GET` and `HEAD` of one object at a time, with `Range` (partial content) and conditional requests (`ETag`, `Last-Modified`), the `Content-Type` and `Cache-Control` the object was stored with, and `X-Content-Type-Options: nosniff`. The root and any path that is not a valid key answer `404`; there is no listing and no directory index. Anything but `GET`, `HEAD` and a CORS preflight answers `405`. A private, deleted or unknown bucket answers `404`, the same answer.
- **CORS.** The rules you stored apply to the public host, including the preflight. A bucket with no rules sends no CORS headers; credentials are never allowed.
- **Turning it off takes up to 30 seconds** to reach every visitor (24.5 seconds measured on production), and a changed object up to 60 seconds, because the host serves from a short cache.
- **Not for production traffic.** Like `r2.dev`, the host is rate-limited and is not a CDN: it caps requests and bandwidth per bucket and per workspace (2 MiB a second per bucket and 4 MiB a second per workspace on each gateway replica, so about twice that in all; a refusal is `429` with `Retry-After`), serves one provider from one region, and offers no custom domain, caching rules or signed URLs.
- **A shared domain.** The host is under the platform's shared hosting domain, as tenant apps and static sites are, so a page served from a public bucket shares a registrable domain with them. Serve only content you would put on a static site, and do not host untrusted uploads from a public bucket.
- **Egress is the exposure, and it is shared.** Bytes read from a public bucket count toward the workspace's `object_storage_egress_bytes` (shown at $0 until a price is set), and bex's storage provider asks that egress stay at or below the bytes stored, across all customers, so bex can switch the public hosts off to protect it. Do not rely on a public bucket for anything that must stay up.

bex's S3 surface is tested against the real clients: the AWS SDK for Go v2 suite runs with every build, and every week [`scripts/s3-contract.sh`](../scripts/s3-contract.sh) runs the pinned aws-cli v2, rclone and boto3, and a `bex` built from the tree, with their own defaults against the real gateway (their defaults are recorded in ADR112 § Not verified). `bex ea objects` and the unmodified `render` CLI keep their own routes and are unaffected.

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

**Run-time copy about a variable names the one the user set (w32/009).** When the upstream variable is unset, the launcher fills `RENDER_API_KEY` from `BEX_API_KEY`, or from `BEX_ACCESS_TOKEN` when that is unset too (w7/m426), and `RENDER_OUTPUT` from `BEX_OUTPUT`. Upstream's messages then name its own variable, which is not in the user's environment. `bridge.FilledFromBex` records what the launcher filled in this process, and `branding.NameBexInputs` renames those variables in upstream's exact phrases:

- **`logout` stdout** (`cmd/logout.go:34,73,79`): "via the RENDER_API_KEY environment variable." and both "Note: RENDER_API_KEY is still set in your environment." notes name the variable the launcher used, `BEX_API_KEY` or `BEX_ACCESS_TOKEN`.
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

**`sandbox-groups` (v2.36.0)**: `pkg/sandboxgroup/repo.go` returns "make another group the default with `render sandbox-groups default set <sandboxGroupId>` first" (a returned error, rewritten); the other new hits (`cmd/sandboxgroups.go`, `sandboxgroupsdefault.go`, `sandboxgroupsdelete.go`, the `sandboxcredentialscreate.go` example) are help, rewritten by the overlay. bex serves `delete` and `default set` since w2/m359 (the repo error is reachable: a `409 default_sandbox_group_not_deletable` prints it, rewritten) and answers `credentials` with `SANDBOX_CREDENTIALS_UNSUPPORTED` (w3/m303); groups themselves are served since w22/m311.

Strings inside commands Bex does not serve (`render workflows …` deprecation notices and the `render workflows flows list` / `tasks list` slug hints in `cmd/flow.go` and `pkg/tui/views/workflows/loader.go`, the `render sandbox-groups images …` deprecation notices on the hidden `sandbox-groups builds` alias, `render sandboxes snapshots …` help) are in the grep output but are not user-reachable residuals here — the underlying operations are non-goals in [`docs/cli-compatibility-checklist.md`](cli-compatibility-checklist.md).

The server-side compatibility ledger, including known Bex non-goals such as workflows, ephemeral SSH, and `ea` objects, is [`docs/cli-compatibility-checklist.md`](cli-compatibility-checklist.md). The imported command can only work where Bex implements the corresponding API operation; it does not turn an unimplemented Bex feature into a supported one.

If deeper runtime branding or a dedicated OAuth public-client identity becomes a product requirement, make that an explicit fork/extension decision rather than silently changing the imported command surface.
