# ADR113 — CLI usage telemetry: Render parity, the bex extensions, and the boards that read it

**Status:** D1–D4 accepted and shipped (w5/m92, m93, m94, m97; 2026-09-10 onward) · D5 shipped (w3/m437, `ddf27afb8`, 2026-11-16; it reaches users with the next `bex-cli/v*` release) · D6 and D8 shipped (w3/m438, `e5bc7bb1e`, `8d438ea0d`, 2026-11-16; verified on production) · D7's read side and D9 shipped (w3/m439, `ec7d808c4`, 2026-11-16; view applied and every panel answering on production) · D7's coverage contract shipped (w3/m440, 2026-11-16; tests in `lego/cli`, no client behavior change) · D7's server facts proposed (2026-11-16) and scheduled as [w3/m441](../.pm/w3/README.md) · **Date:** 2026-11-16 · **Extends:** [ADR018](ADR018-render-parity.md) (parity ledger), [ADR088](ADR088-platform-observability-ui.md) (Grafana at `obs.bex.co`), [bex-cli.md § CLI usage telemetry](bex-cli.md#cli-usage-telemetry), [runbooks/cli-analytics.md](runbooks/cli-analytics.md) · **CLI pin:** Render CLI v2.36.0, `render-oss/cli@4fd34231b637` (`lego/cli/go.mod`)

![ADR113 architecture: on the developer's machine a command produces an execution result, passes the eligibility, disclosure, consent and backoff gates, becomes Render's 18-field event in a state file and is posted once by a subprocess; Bex-native commands that replace the process report themselves and a transport wrapper adds the bex release header. bex-api authenticates, rate-limits, attributes the event to the caller and stores it in the control-plane Postgres, counting it in Prometheus. A read-only view and reader role feed the CLI usage board on obs.bex.co. Amber marks what this ADR adds: a Bex-owned disclosure, the extended-CLI coverage contract, new view columns, a coverage panel and ingest alerts.](assets/ADR113-cli-usage-telemetry.svg)

## Context

Render instruments its own CLI. Since v2.26.0 (2026-09-01) every `render` command reports one usage event to Render's API, so Render can see which commands people run, which fail, how long they take, and whether a person, a CI job or an AI agent ran them.

`bex` is that CLI, imported and not forked ([bex-cli.md](bex-cli.md), `.pm/DO_NOT_DO.md`), so it carries the same instrumentation. Three things had to be decided: where the events go, what users are told, and what we do with the data. They were decided and shipped in w5/m92 (ingest), m93 (board), m94 (launcher self-reports) and m97 (creation surface). The decisions live in milestone notes, the CLI guide, a runbook and the compatibility checklist, and no ADR records them. This ADR is that record. It also closes the gaps found by re-reading the sender at the current pin, which has grown a disclosure gate, a backoff file and a send strategy since m92.

### How Render instruments its CLI

All references are to the pinned source.

**One event per execution.** `cmd/root.go` `execute()` runs the command and builds a `command.ExecutionResult`. `onExecutionComplete` (`cmd/execution.go:28`) then applies the gates below and calls `analytics.Sender.Send` (`pkg/analytics/analytics.go`).

| Step | Rule | Source |
| --- | --- | --- |
| Eligibility | Every command reports, except `login`, `logout`, the hidden `analytics` tree and shell completion. Root `--version` exits before dependencies exist, so it is a non-event. A setup failure before the API client exists (no credential, broken config) is also silent | `markCommandAnalyticsIneligible` (`cmd/login.go:23`, `cmd/logout.go:90`, `cmd/analytics.go:19`); `cmd/root.go:443`; `cmd/execution.go:31` |
| Disclosure gate | A one-time notice per machine is printed on stderr and recorded as the empty file `<state>/analytics/notice-shown`. Outside CI, the run that prints the notice sends nothing, and no run sends until the marker exists. With no terminal on stderr the notice cannot be shown, so nothing is sent. `CI=true` bypasses the gate: no notice, no marker, events flow | `internal/analyticsnotice` (`ShowIfNeeded`, `CanSend`) |
| Consent | Opt-out. `DO_NOT_TRACK` or `RENDER_CLI_DISABLE_ANALYTICS` set to `1` or `true` disables sending | `pkg/analytics/consent.go`, `pkg/cfg/cfg.go:45-53` |
| Backoff | A `429`, `503` or `404` answer pauses sending for 5 minutes, 30 minutes or 1 hour. A valid `Retry-After` replaces the default. The pause is clamped to 30 seconds–24 hours and persisted in `<state>/analytics/backoff.json`. No other status starts a backoff | `pkg/analytics/backoff.go` |
| Event | Render's `CliTelemetryEventPOSTInput`, 18 fields (below), written to `<state>/analytics/events/event-<uuid>.json` | `newEventPOSTBody`, `eventfile.go` |
| Delivery | A child process, `render analytics send <file>`, re-executes the same binary. It is detached by default, so the command returns at once. It runs synchronously, bounded to 3 seconds with a 2.5-second request timeout, when `CI=true`, when `RENDER_LOG_ANALYTICS=1`, or when `RENDER_CLI_ANALYTICS_STRATEGY=sync`. The child removes the event file before it posts: delivery is at most once, and a failed send drops the event. There is no retry and no queue | `strategy.go`, `subprocess.go`, `sendfile.go`, `post.go` |
| Debugging | `RENDER_LOG_ANALYTICS=1` prints the payload and the response status on stderr | `pkg/cfg/cfg.go:39` |

**The event.**

| Field(s) | Meaning |
| --- | --- |
| `command` | Space-joined names of the matched command, such as `bex services list`. Never arguments or flag values |
| `completion_kind` | One of eight: `success`, `help`, `version`, `discovery_error` (no such command), `validation_error` (bad flags or arguments), `setup_error` (failed before the command began), `execution_error` (the command ran and failed), `explicit_exit` (the command chose its exit code to report an outside outcome, as `deploys create --wait` does for a failed deploy) |
| `exit_code`, `duration_ms`, `started_at` | Process exit code, wall-clock duration, client start time |
| `cli_version`, `os`, `arch` | The upstream CLI version and the platform |
| `output_format`, `launched_full_screen_tui` | `interactive`, `json`, `yaml` or `text`, and whether the full-screen TUI opened |
| `is_stdin_tty`, `is_stdout_tty`, `is_stderr_tty`, `is_term_dumb` | Terminal attachment |
| `agent_signals` | Names of set environment variables from an allowlist of 13 AI-agent markers (`CLAUDECODE`, `CODEX_THREAD_ID`, `CURSOR_AGENT`, `GEMINI_CLI`, `COPILOT_AGENT_SESSION_ID`, …). Names only, never values (`agentsignals.go`) |
| `ci_signals` | Names from an allowlist of 14 CI markers (`CI`, `GITHUB_ACTIONS`, `GITLAB_CI`, `BUILDKITE`, …). Names only (`cisignals.go`) |
| `installation_id` | A v4 UUID stored in `<state>/installation-id.txt`. It identifies a machine, not a user (`internal/installid`) |
| `current_workspace_id` | The CLI's selected workspace |

**Never collected:** arguments, flag values, resource names or ids, environment values, command output.

### What bex ships today

| Layer | Behavior | Evidence |
| --- | --- | --- |
| Launcher: destination | The bridge points the CLI at bex-api, so the unmodified sender posts to `POST /v1/cli-telemetry-events` on Bex. Nothing goes to Render | `lego/cli/internal/bridge/bridge.go` |
| Launcher: consent | `BEX_CLI_DISABLE_ANALYTICS` is mapped onto `RENDER_CLI_DISABLE_ANALYTICS` unless the latter is already set. `DO_NOT_TRACK` is honored by upstream directly | `bridge.go:113-121` |
| Launcher: state | Installation id, notice marker, event files and backoff live under `~/.bex/state` | `bridge.go` (`RENDER_CLI_CONFIG_DIR`, w8/018) |
| Launcher: release identity | A transport wrapper adds `X-Bex-CLI-Version` to every request to the bex host, the send subprocess included. The body stays Render's, field for field | `bridge/transport.go`, `main.go:48` (w5/m94) |
| Launcher: self-reports | Paths the upstream hook cannot see report through the same `Sender`: process-replacing launches (`bex code`, `glm`, `muse`, `kimi`, `deepseek`, `bex edit`, `bex agent open`, `bex agent ssh`) emit just before `exec`, and root `--version` emits with completion kind `version` | `lego/cli/internal/telemetry/telemetry.go`; call sites `code/code.go:58`, `desktop/desktop.go:119`, `agent/agent.go:1087`, `agent/ssh.go:61`, `main.go:93` |
| Ingest: gate | Bearer auth (OAuth token or API key), write op class, identity rate limit, body cap. An unauthenticated post is refused | `lego/backend/internal/api/scope_matrix_ops.go:972`; w5/m92 t002 |
| Ingest: handler | Liberal decode (unknown fields ignored), `202 {"id"}` on store, `503` while the control-plane store is off | `lego/backend/internal/clitelemetry/rest.go:59` |
| Ingest: attribution | Subject and workspace come from the auth identity, never from the payload. The payload's workspace is kept as `reported_workspace_id`. Fields are truncated, never rejected; only a blank `command` is a 400 | `clitelemetry/service.go` `Record` |
| Store | `cli_telemetry_events` in the control-plane Postgres (migrations 0113, 0116). Rows are purged with the audit retention window, 90 days by default. Deleting an account deletes its rows | `store/clitelemetry.go`, `audit/service.go:354`, `store/accounts.go:393` |
| Counter | `bex_api_cli_telemetry_events_total{completion_kind, output_format, client}`, incremented after a successful insert, every label clamped to a closed set (D8). `bex_api_cli_requests_total{client}` counts the CLI's REST requests beside it (D6) | `api/httpmetrics.go`, `clitelemetry.MetricLabels` |
| Read path | View `cli_analytics.events` projects the table and derives `outcome`, `failed`, `duration_mode`, `upstream_version` and `bex_version`. Role `bex_cli_analytics` can read only that view: read-only transactions, 10-second timeouts, 6 connections | `scripts/cli-analytics.sql`, `scripts/cli-analytics-bootstrap.sh` |
| Board | Grafana datasource `cli-analytics` and the board "CLI usage" (`bex-cli-usage`, 24 panels), behind the ops-workspace gate | `deploy/gitops/base/values/grafana.values.yaml`; [runbook](runbooks/cli-analytics.md) |
| Second source | Every created hosting resource records its creation surface, and `cli` is set server-side from the `User-Agent: render-cli/…` prefix. No client telemetry is involved | `core/product_surface.go`, migration 0117 (w5/m97) |

### Gaps

- **G1 — no design record.** No ADR describes this pipeline, and [ADR018](ADR018-render-parity.md) has no row for it. The next pin bump or privacy question has to be answered from four scattered documents.
- **G2 — the disclosure names the wrong recipient.** The one-time notice is upstream's text: _"The Render CLI collects usage data … No data has been sent to Render yet … Learn more: render.com/docs/cli#usage-telemetry"_ (`internal/analyticsnotice/notice.go`). In `bex` the data goes to Bex. Since the notice became a gate, it is also the only thing a user sees before events start flowing, so users are told about the wrong company. A user who set `BEX_CLI_DISABLE_ANALYTICS` is told to _"remove RENDER_CLI_DISABLE_ANALYTICS from your environment"_, a variable they never set. Today the CLI guide tells users not to act on the notice; that is a workaround, not a disclosure.
- **G3 — the self-reports skip the disclosure gate.** `EmitLaunch` and `EmitVersion` run outside Cobra's post-run path, where the gate lives, and the gate's package is internal to the upstream module. A machine whose first `bex` command is `bex glm` or `bex --version` reports before any notice was shown (`telemetry.go` documents this as a deliberate divergence).
- **G4 — headless, non-CI installs never report.** Without a terminal on stderr the notice cannot be shown, so the marker is never written and nothing is ever sent, unless `CI=true`. An AI agent driving `bex` in a container with an API key and no CI marker is exactly this case. That is the population [ADR008](ADR008-vision.md) cares most about, and the board cannot see it. The size of the undercount is unknown, because nothing measures it (G6).
- **G5 — extended bex commands are measured as if they were hosting commands.** `bex agent` returns meaningful exit codes: 3 a failed turn, 4 canceled, 6 an automation that started no session, 124 a timeout. The view exposes only a `failed` boolean, so the board counts a failed agent turn as a CLI failure, as it does for a failed deploy under `deploys create --wait`. `agent run --wait`, `agent wait` and `agent chat` can last for an hour, but the view's long-running class knows only `logs`, `ssh`, `shell` and `psql`, so the waits land in the non-interactive duration percentiles. And no panel groups commands by product area.
- **G6 — "quiet" and "broken" look the same.** The runbook says so itself: _"No requests is not proof of collector health, and quiet CLI usage is not proof of an outage."_ A route change that answers 404 makes every CLI back off for an hour. Opt-outs, the gate and backoff are all invisible. Nothing compares telemetry volume with the CLI traffic the API already sees, and no alert watches the ingest route.
- **G7 — the counter has an open label and no reader.** `bex_api_cli_telemetry_events_total` takes `command` from the request body, so an authenticated client can mint series, bounded only by its rate limit. No panel or alert reads the counter: the board's Prometheus panels use the route counters.
- **G8 — our own usage counts as adoption.** The product analytics tables classify workspaces as `customer`, `internal`, `qa` or `canary` (`product_analytics_audiences`, migration 0114), and the Product adoption board filters on it. `cli_analytics.events` has no such column, so this repo's QA runs and CI jobs inflate every CLI adoption number.

### Constraints

- **Render parity first** ([ADR008](ADR008-vision.md#render-parity-first)). What the CLI collects, when it sends and how a user opts out must match Render. Only the recipient differs, and the user must be told so.
- **No fork.** The sender is imported. Changes live in the launcher around it or in bex-api.
- **The body stays Render's.** Bex-only data rides a header, so an unmodified `render` binary pointed at bex still ingests, and the body can be diffed against upstream field for field at every pin bump.
- **Privacy line.** Command names, never arguments. Environment-variable names, never values. No identity in Prometheus labels.
- **Best effort.** Telemetry never fails or visibly slows a command.

## Decision

### D1 — Events go to Bex, in Render's shape (shipped)

The sender is not patched or disabled. The launcher's host mapping makes it post to bex-api, and bex-api accepts Render's `CliTelemetryEventPOSTInput` unchanged. Unknown future fields are ignored, so a pin bump that adds a field cannot break ingest. The launcher's own release travels in `X-Bex-CLI-Version`. GraphQL and MCP have no telemetry twin, because upstream reports over REST only.

### D2 — The server decides who sent it (shipped)

Identity and workspace are taken from the authenticated request. Ingest is liberal about content (truncate, never reject) and strict about access (authenticated, write class, rate limited, size capped). The handler answers `202` and never returns an error a command could surface.

### D3 — Consent is Render's, plus one Bex name (shipped)

Opt-out stays the default, as on Render. `DO_NOT_TRACK=1` and `BEX_CLI_DISABLE_ANALYTICS=1` both disable sending. An explicit `RENDER_CLI_DISABLE_ANALYTICS` is left alone as the upstream-developer escape hatch.

### D4 — Bounded retention and a narrow read path (shipped)

Raw rows live 90 days (the audit retention window) and leave with the account. Grafana reads through one view with one restricted role. The board is internal and sits behind the ops-workspace gate. Customers never see it.

### D5 — Bex owns the disclosure (shipped)

The launcher shows a Bex notice and makes its own marker the gate. Upstream's notice is never printed.

- **Text.** It says that the bex CLI collects command names, outcomes, durations and environment signals, that the data goes to Bex and not to Render, that arguments and values are never collected, and how to opt out (`BEX_CLI_DISABLE_ANALYTICS=1` or `DO_NOT_TRACK=1`). It links to [bex-cli.md § CLI usage telemetry](bex-cli.md#cli-usage-telemetry). An opted-out user sees the opted-out variant, which names the variable they actually set.
- **Mechanism, with no fork.** The launcher keeps its own marker, `<state>/analytics/bex-notice-shown`. On every run it first makes sure upstream's marker (`notice-shown`) exists, so upstream stays silent. Then:
  - Bex marker present, or `CI=true`: nothing else happens, and upstream sends as it does today.
  - Bex marker absent, not CI: the launcher sets `RENDER_CLI_DISABLE_ANALYTICS=1` for this process only, so this run sends nothing. If stderr is a terminal and the command is notice-eligible by upstream's rule (not the `analytics` tree, not shell completion), it prints the Bex notice after the command finishes and writes the Bex marker. With no terminal it prints nothing and writes nothing, so the next run is gated the same way. A command that replaces the process (`bex glm`, `bex edit`) cannot print afterwards, so it also stays gated until a later run shows the notice.
- **Machines that already saw Render's copy** have the upstream marker but not the Bex one. They see the Bex notice once, and that run sends nothing. That corrects the earlier wrong disclosure.
- **Self-reports use the same gate.** `telemetry.send` checks the Bex marker (or `CI=true`) before it builds a client. That closes G3 and removes the documented divergence.
- **Headless installs.** `scripts/install-bex.sh` prints the same notice at the end of an install and writes the Bex marker, honoring `BEX_CLI_CONFIG_DIR`. An installer log is a disclosure a headless user can actually read, and upstream anticipates this path: its hidden `analytics notice` command exists so "installers can invoke this command after install or upgrade" (`cmd/analyticsnotice.go`). The Homebrew formula shows the notice in its caveats but cannot write to the user's home directory, so a Homebrew install is disclosed on its first terminal run. This narrows G4 without sending anything before a disclosure.
- **Pin bumps.** `lego/cli/UPSTREAM_RENDER_CLI.md` step 6 already re-checks the marker path. It gains one line: confirm that upstream still prints nothing when its marker exists.

### D6 — Measure telemetry coverage on the server

bex-api already knows when a request comes from the CLI: `core.ProductSurface` reads the `User-Agent: render-cli/…` prefix, and the launcher adds `X-Bex-CLI-Version`. Count those requests:

- `bex_api_cli_requests_total{client}`, where `client` is `bex` (the prefix and the header) or `render-cli` (the prefix alone). The telemetry route itself is excluded.
- A coverage panel plots stored telemetry events per CLI API request. The absolute value depends on how many API calls a command makes, so it is read as a trend. A drop toward zero while CLI requests continue means collection stopped: a broken route, a store outage with fleet-wide backoff, or an opt-out wave. That is the "quiet or broken?" question from G6.

The counter is server-side, so it also sees users who opted out or never passed the gate. It records only that a CLI request arrived: no command, no identity.

**As shipped (w3/m438).** A request is counted on the REST surface whatever its answer, so an unrouted CLI request still counts; the telemetry deliveries are left out by method and path, not by the pattern they resolved to, so a delivery to a missing route is left out too. Both clients are exported at 0 when the process starts (see D8).

### D7 — Extended bex CLI: one event per native command, read as what it is

The full section is [Extended bex CLI](#extended-bex-cli). In short:

- every Bex-native command reports exactly once, through the upstream hook or through `EmitLaunch`, and a contract test keeps that true as commands are added;
- the read side learns what the extended commands mean: their exit codes, their product area and their long waits;
- questions that would need flags or arguments are answered from server facts (creation surface), never by collecting more on the client.

**As shipped, read side (w3/m439).** The five columns in [What the read side learns](#what-the-read-side-learns-view-columns-appended) and their panels. Three details were settled against production's commands and data:

- **The family mapping uses the names the command path carries**, not the aliases: `postgres` and `keyvalues` (typed `pg` and `kv`), plus the session commands `pgcli` and `kv-cli` under `data`. `ea sandboxes` and `ea sandbox-groups`, which older pins exposed, are `sandbox`, and `ea blueprints` is `hosting`. `desktop` is `edit` and `agent open`. The full table is in [the runbook](runbooks/cli-analytics.md#command-families).
- **Delivery lag is measured from the command's end**, its start plus its reported duration. Measured from the start, an hour-long `agent wait` would read as an hour of lag.
- **Unmodified `render` binaries are told by the command path**, which starts with `render` rather than `bex`. A pre-header bex build also reports release `unknown`, but it names itself `bex`. On production every `render` row so far carries no release, and two `bex` rows from 2026-09-10 do.

**As shipped, coverage contract (w3/m440).** Tests and comments only; the client behaves as before.

- **The scan keys each site by file and enclosing function.** It finds every reference to `syscall.Exec` or `unix.Exec` in the module's non-test Go files, a function value as well as a call, so a second exec in a listed file still needs review. It also fails on a listed site that no longer replaces the process and on a listed contract test that does not exist.
- **The launch contract tests run the built binary** (`lego/cli/telemetry_contract_test.go`): `bex edit <path>`, `bex <path>`, `bex agent open` and `bex agent ssh` each store one `success` event named by its commands alone, and the replacing program's own record proves the process was replaced. That record matters for `agent open`, whose dashboard fallback also stores one event, through the hook. `bex glm` keeps its test in `main_test.go`.
- **The outcome data holds as the table says.** `bex agent wait` on a failed turn and on a canceled session, and an `agent automations run` that started no session, are stored as `explicit_exit` with exit codes 3, 4 and 6, so the `cli_fault` definition needed no change.

### D8 — Ingest guardrails: closed labels and three alerts

- **Closed labels.** `bex_api_cli_telemetry_events_total` drops the `command` and `cli_version` labels and gains `client` (`bex`, `render-cli`). `completion_kind` is clamped to the eight known values plus `other` and `output_format` to its four plus `unknown` (upstream itself sends `unknown` for a run that ended before setup). Per-command and per-version questions stay in Postgres, where they are answered today. No panel read the counter (G7), so nothing broke.
- **Every series exists at 0 (amended 2026-11-16, w3/m438).** This decision first kept `cli_version`, clamped to a release-tag shape. Production showed why it cannot stay: a counter series that first appears at 1 is invisible to `increase()` and `rate()`, every bex-api rollout starts both replicas empty, and 23 minutes after a rollout the counter held one series. A version label cannot be created in advance, so every new version on every new pod would lose its first event. Without it the label set is closed (9 × 5 × 2 = 90 series), each replica creates all of them, and both `bex_api_cli_requests_total` clients, at 0 when it starts. The ingest route's `202`, `429`, `500` and `503` answers on `bex_api_http_requests_total` are created at 0 on the public listener for the same reason, so a replica's first `503` counts. The release-tag rule (`sanitizeVersion`) still guards the stored `bex_version`.
- **Alerts**, all in the warning tier (daily digest, [ADR010](ADR010-observability.md) three tiers), each with a panel ([ADR088 §6](ADR088-platform-observability-ui.md)):
  - `CLITelemetryIngestFailing`: more than 5 % of deliveries answered 5xx for 15 minutes. A `503` costs every affected CLI a 30-minute backoff, so a short outage loses much more than its own duration.
  - `CLITelemetryIngestSilent`: CLI requests continue (D6) and no event has been stored for 2 hours. This is the only alert that catches a missing route, because a 404 is counted under the `unmatched` route and not under the telemetry route.
  - `CLIExecutionErrorShareHigh`: the `execution_error` share of stored events is above three times its 7-day baseline for 30 minutes. A server change that breaks a CLI command shows up here before a user reports it.
- **As shipped (w3/m438)**, in `deploy/gitops/base/rules/alerting_rules.yml` with fire and no-fire tests in `cli_telemetry_alerts_test.yml`:
  - Failing: the 5xx share over 10 minutes, held for 15, with at least two failed deliveries per window. The window is shorter than the wait, so one burst cannot fire on its own.
  - Silent: at least 30 CLI requests in 2 hours, some in each hour and in the last 30 minutes, and nothing stored in 2 hours. One command that polls for up to an hour (`deploys create --wait`, `agent wait`) reports only when it ends, so requests confined to the last hour do not count as continuing; production showed exactly that pattern on its first night. "Nothing stored" does not rest on `increase()` alone: a series that appeared within the window above 0 (an event stored before a replica's first scrape) also counts as stored.
  - Error share: `bex:cli_telemetry_execution_error_share:ratio30m` against three times `bex:cli_telemetry_execution_error_share:avg7d`, both recorded (the week from a 5-minute rate by kind and client, 18 series), so the alert and the board draw the same numbers. Floors: 10 events and 5 execution errors in the half hour. On the week to 2026-11-16 the rule would have warned twice: about 18 and 74 execution errors in half an hour, the second one workspace's `bex logs` failing in a loop.

### D9 — Audience on the read side, and the ledger row

- `cli_analytics.events` gains `audience`, joined from `product_analytics_audiences` on the server-attributed workspace and defaulting to `unclassified`. The board gains an Audience variable that defaults to `customer` and `unclassified`, so internal, QA and canary usage is excluded unless someone asks for it (G8).
- [ADR018](ADR018-render-parity.md) gains a "CLI usage telemetry" row: ✅ on REST, not applicable on GraphQL, MCP and UI, with the two recorded differences. Events go to Bex, and the disclosure is Bex's (D5).
- **As shipped (w3/m439).** The join reads the current label, so reclassifying a workspace moves its whole retained history. Audience split is the one product panel that ignores the audience filter, because it shows what that filter removes. On production every event retained on 2026-11-16 came from the `bex` workspace, which is labelled `qa`, so the default view starts empty: that is the correction this decision expected, not a regression.

## Extended bex CLI

`bex` adds commands Render's CLI does not have: Cloud Agents and their automations, coding-agent launchers, the desktop handoff, self-update and Bex's skills. Render's instrumentation knows nothing about them. This section is the contract for how they are measured.

### How each command reports

| Commands | Reports through | Exit codes that carry an outcome |
| --- | --- | --- |
| Imported Render commands (`services`, `deploys`, `logs`, `psql`, `ssh`, `kv`, `pg`, `projects`, `blueprints`, `sandboxes`, `ea objects`, `workspaces`, `whoami`, …) | Upstream post-run hook | `deploys create --wait` and `sandboxes exec` use `explicit_exit` |
| `bex agent run`, `send`, `wait`, `get`, `list`, `logs`, `files`, `cancel`, `stop`, `chat`, `handoff`, `pickup` and the rest of the session verbs | Upstream hook: they are Cobra commands on the imported root and return through `cmd.Execute()` | `2` usage, `3` the turn failed, `4` canceled, `5` needs input, `7`/`8` nothing to stop, `124` timeout, `130` interrupted (`lego/cli/internal/agent/agent.go:58-69`). All arrive as `explicit_exit` |
| `bex agent automations …`, `bex agent secrets`, `skills`, `env`, `models` | Upstream hook | `6` an automation run started no session |
| `bex agent open` (desktop handoff), `bex agent ssh` | `EmitLaunch` just before the process is replaced; the upstream hook when they return without replacing it | — |
| `bex glm`, `muse`, `kimi`, `deepseek` | `EmitLaunch` before `exec`. A launch that fails (no agent binary on `PATH`) returns through Cobra and is reported once, as `execution_error` | — |
| `bex edit <path>`, `bex <path>` | `EmitLaunch` before `exec` | — |
| `bex code` (provider status and keys), `bex upgrade`, `bex skills …` | Upstream hook | — |
| `bex --version` | `EmitVersion` (completion kind `version`). On Render this is a non-event | — |
| `bex login`, `bex logout` | Not reported, as on Render | — |

### The coverage contract

1. **Exactly one event per invocation.** A command that returns through Cobra is reported by the upstream hook. A command that replaces the process calls `EmitLaunch` first. No command does both: the hook never runs for a replaced process.
2. **Same schema, same gates.** A self-report builds the same `ExecutionResult`, goes through the same `Sender`, and after D5 passes the same disclosure gate. The command path contains command names only. Everything typed after a provider name belongs to the launched agent and is never recorded.
3. **Enforced by a test.** `lego/cli/telemetry_coverage_test.go` scans the module for `syscall.Exec` and `unix.Exec`. Today there are three call sites: `internal/code/launch.go`, `internal/agent/ssh.go` and `internal/desktop/desktop.go`. The test fails on any call site outside its reviewed list. Each listed site has a contract test, run against the built binary, that expects one stored event. A new native command that replaces the process cannot ship unreported.
4. **No new client fields.** Flags are not collected, so `agent run` cannot be told from `agent run --wait`. That is deliberate: it is Render's privacy line. It is also enforced mechanically, because the send subprocess decodes the event file into the typed upstream struct and drops anything else (`sendfile.go` `readEvent`).

### What the read side learns (view columns, appended)

`CREATE OR REPLACE VIEW` can only add columns at the end, so each of these is appended to `cli_analytics.events`:

| Column | Definition | Why |
| --- | --- | --- |
| `exit_code` | The stored exit code | Outcome panels for `explicit_exit` commands |
| `cli_fault` | True for `discovery_error`, `validation_error`, `setup_error` and `execution_error`. False for `success` and `explicit_exit`. Null for `help`, `version` and unknown | Separates "the CLI or the API failed" from "the thing the user asked about failed". The existing `failed` column and its panels stay as they are |
| `command_family` | A closed set derived from the command path: `hosting`, `data` (`postgres`, `keyvalues`, `psql`, `pgcli`, `kv-cli`, `ea objects`), `agent`, `automation`, `coding-agent` (`code` and the provider launchers), `desktop` (`edit`, `agent open`), `sandbox`, `account` (`workspaces`, `workspace`, `whoami`, `upgrade`, `skills`), `other` | Product-area panels |
| `duration_mode` (redefined) | The `session / stream` class also matches `agent chat`, `agent run`, `agent wait`, `agent send`, `agent logs` and `agent handoff` | Keeps hour-long waits out of the non-interactive percentiles |
| `started_at` | The client's start time | Delivery-lag panel (received minus the command's end) |
| `audience` | D9 | Customer-only adoption |

### Server facts, not client telemetry

Some questions need more than a command name: how many agent sessions started from a terminal, and whether CLI-created sessions end differently from dashboard-created ones. The server already has the answer for hosting resources. `product_activity_events.surface` is set from the request's `User-Agent`, and the Product adoption board charts "Creations by surface per day" and "Agent-driven share of creations".

D7 extends that recorder to the extended products: `resource_type` gains `agent_session`, `agent_automation` and `sandbox`, with the same closed `surface` set. That gives the extended CLI its business measurements without any change on the client.

## Parity

| Behavior | Render | bex after this ADR |
| --- | --- | --- |
| Event schema, eligibility, completion kinds, signals | `CliTelemetryEventPOSTInput` | Identical (imported code, D1) |
| Destination | Render's API | bex-api. Never Render |
| Consent | Opt-out: `DO_NOT_TRACK`, `RENDER_CLI_DISABLE_ANALYTICS` | The same, plus `BEX_CLI_DISABLE_ANALYTICS` (D3) |
| Disclosure before the first send | One-time notice; CI bypasses | The same gate with Bex's text and Bex's marker (D5). Also shown by the installer |
| Delivery, backoff, at-most-once | Detached child; sync in CI; 429/503/404 backoff | Identical (imported code) |
| `--version` | Non-event | Reported as `version`: a recorded Bex extension (w5/m94) |
| Commands Render does not have | — | One event each, by the coverage contract (D7) |
| Release identity | `cli_version` | `cli_version` (the pin) plus `X-Bex-CLI-Version` (the bex release) |

### The disclosure gate, case by case (D5, checked 2026-11-16)

Render's column is the pinned v2.36.0 source (`cmd/execution.go:28-69`, `internal/analyticsnotice`, `pkg/analytics/consent.go`). Bex's column is the shipped launcher (`lego/cli/internal/disclosure`), each row pinned by a test in `lego/cli` (`disclosure_test.go`, `internal/disclosure`). Only the recipient and the text differ, except root `--version` (the w5/m94 extension) and `analytics notice` (a D5 decision).

| Case | Render | bex |
| --- | --- | --- |
| `CI=true` or `CI=1` | Sends; no notice; no marker | The same |
| No terminal on stderr, notice never shown | No notice, no marker, never sends | The same, with the Bex marker |
| First terminal run | Prints the notice on stderr after the output, writes the marker, sends nothing | The same; stdout stays the command's own |
| Opted out | The opted-out variant, naming `DO_NOT_TRACK` or `RENDER_CLI_DISABLE_ANALYTICS` | The same, naming the variable the user set, `BEX_CLI_DISABLE_ANALYTICS` included |
| `login`, `logout` | Show the notice, never send | The same |
| Shell completion, the hidden `analytics` tree | Never show it | The same |
| Setup failed before the API client existed | No notice | The same |
| Root `--version` | No notice, no event | No notice; one `version` event once the notice was shown (the w5/m94 extension) |
| A command that replaces the process (`bex glm`, `bex edit`, `bex agent ssh`) | — | No notice; nothing sent until a later run shows it |
| A machine that saw Render's notice before | — | Sees the Bex notice once; that run sends nothing |
| Hidden `analytics notice` | Prints only on a terminal outside CI | **Bex decision (D5, headless installs):** prints with or without a terminal, also in CI, so an install log discloses before anything is sent |

## Phasing

1. **P1 — D5.** The disclosure is a truthfulness fix, so it goes into the next `bex-cli/v*` release. Verify on a clean `BEX_CLI_CONFIG_DIR`: the first terminal run prints the Bex notice and stores no event; the second run stores one; a run with `CI=true` on a clean directory stores one and prints nothing; a run of `bex --version` on a clean directory stores nothing.
2. **P2 — D8 and D6.** Server-only. Verify that stopping the store on a dev stack fires `CLITelemetryIngestFailing`, and that the coverage panel moves when a CLI runs with `DO_NOT_TRACK=1`.
3. **P3 — D9 and the D7 view columns and panels.** One SQL migration of the view, the board changes, and the SQL tests in `scripts/test_cli_analytics.py`.
4. **P4 — the D7 contract test and the surface extension.**

## Open questions

- **Should an agent marker count as a disclosure bypass, as `CI=true` does?** The default is no: nothing is sent before a disclosure a person could have read. If the user decides otherwise, the gate in D5 is the single place to change.
- **Does Homebrew need more than caveats?** The formula cannot write the marker. If first-run disclosure turns out to be too late for headless Homebrew installs, the alternative is a small Bex-owned hidden command that prints the notice and writes the marker, which the formula's `post_install` can call.

## Alternatives rejected

- **Keep telemetry force-disabled**, as `bex-cli/v0.2.1` did. We would know nothing about CLI usage, and the upstream notice would still invite users to re-enable a sender that phones Render.
- **Fork the CLI to change the notice text.** Ruled out by the no-fork rule. D5 gets the same result from outside.
- **Add Bex fields to the body.** The body would no longer be diffable against upstream, and the send subprocess would drop the fields anyway.
- **Collect flags or arguments for native commands.** It crosses the privacy line, and arguments to `bex agent run` are prompts.
- **Send events to a product-analytics service.** It adds a processor outside the platform for a volume that one Postgres table and Grafana handle, and it would need its own disclosure.
- **Treat agent markers as CI to count headless agents.** It sends before any disclosure. D5's installer notice reaches the same users honestly.
- **Per-command or per-workspace Prometheus labels.** They make an open label set (G7). Postgres answers those questions behind the retention window.

## Consequences

- The disclosure a `bex` user sees is true. Users who saw Render's text see Bex's once.
- Telemetry starts one run later on machines that had only the upstream marker, and the self-reports now wait for the gate. Counts dip slightly at the release that ships D5, and the board's annotations should mark it.
- The launcher owns two small things at the upstream seam: writing upstream's marker and reading its own. Both are covered by the pin-bump checklist.
- Operators can tell a quiet week from a broken collector, and a failing ingest alerts before its backoff multiplies the loss.
- CLI adoption numbers drop when internal and QA usage is filtered out. That is the correction, not a regression.

## Verification

- `cd lego/cli && go test ./...` — the launcher gate (D5), the self-reports and the coverage test (D7).
- `cd lego/backend && go test ./internal/clitelemetry/ ./internal/api/` — ingest, the closed labels and the request counter (D6, D8).
- `python3 -m unittest scripts/test_cli_analytics.py -v` — every SQL panel, including the appended columns (D7, D9).
- `bash scripts/gitops-validate.sh` — the board, the datasource and the alert rules.
- Live: `RENDER_LOG_ANALYTICS=1 bex workspaces -o json` prints the payload and `analytics response: 202 Accepted`, and the event appears on the board.

## SRE dashboard inventory

Every chart that turns this collection into a decision, written for the on-call SRE who owns the pipeline and for the people who read the numbers. All of it lives on `obs.bex.co` behind the ops-workspace gate. Customers never see it.

Rules the inventory follows:

- **Every panel answers one question for one audience.** Business panels ask whether the CLI is adopted, retained and reliable enough to invest in. Operations panels ask whether the collection is working and whether the numbers can be trusted.
- **Every alert has a panel** ([ADR088 §6](ADR088-platform-observability-ui.md)).
- **Identity never reaches Prometheus.** Per-installation, per-workspace and per-command questions are SQL over `cli_analytics.events`, bounded by the reader's 10-second timeout.
- **Status:** ✅ exists today · ➕ new panel over data already collected · 🆕 new panel that needs a D6–D9 change.

The board's filters apply to every product panel: command, upstream version, bex release, output format, audience (D9, default `customer` and `unclassified`) and command family (D7). Audience split alone ignores the audience filter.

### Business — board "CLI usage" (`bex-cli-usage`)

| Panel | Status | Question it answers | Source |
| --- | --- | --- | --- |
| Observed commands · Active installations · Active workspaces | ✅ | How large is the CLI audience in this range? | `cli_analytics.events` |
| Daily active installations, identities and workspaces | ✅ | Is daily use growing, and is it more machines or more people? | same |
| Daily / 7-day / 30-day active installations | ✅ | Is the CLI a habit (a high daily-to-monthly ratio) or an occasional tool? | same |
| First-seen and returning installations per day | ✅ | How much of today's use is new? | same |
| Next-week return — mature cohorts | ✅ | Do new installations come back the following week? | same |
| Commands per day | ✅ | Overall volume trend | same |
| Commands — reach, failures and non-interactive duration | ✅ | Which commands carry the product, and which fail or run slowly? | same |
| Command failure share · Completion outcomes | ✅ | How often does a command not do what was asked, and in which phase (unknown command, bad flags, setup, execution)? | same |
| Duration by command and mode | ✅ | How long do interactive, non-interactive and streaming commands take? | same |
| Agent and CI signals per day · Detected agent families · Detected CI providers | ✅ | How much use is automated, and by which agents and CI systems? This is the AI-native measure of [ADR008](ADR008-vision.md) | same |
| Output formats · Full-screen TUI usage | ✅ | Machine-readable or interactive use: where does UX work pay off? | same |
| Operating system / architecture | ✅ | Which platforms must release builds cover? | same |
| Upstream CLI version adoption · Bex release adoption | ✅ | How fast do releases roll out, and when can an old pin stop being supported? | same |
| **Audience split** | ✅ D9 | How much of all this is customers, and how much is our own QA and CI? | `audience` column |
| **Command families: reach and volume** | ✅ D7 | Which product areas does the CLI serve: hosting, data, Cloud Agents, coding-agent launchers, desktop? | `command_family` column |
| **Feature reach by installation** | ✅ D7 | What share of active installations used each family this week? That is adoption of the extended CLI | `command_family` column |
| **`bex agent` outcomes by exit code** | ✅ D7 | Of waited agent runs, how many succeeded, failed their turn, were canceled or timed out? | `exit_code` column |
| **CLI faults vs reported outcomes** | ✅ D7 | Is a rising failure share the CLI's fault, or are users' deploys and agent turns failing? | `cli_fault` column |
| **Agent-driven share by command family** | ✅ D7 | Do agents use the CLI for hosting, for Cloud Agents, or both? | `agent_signals` × `command_family` |
| **Activation: first week of a new installation** | ✅ | Of installations first seen in a week, how many reached a successful `deploys create`, `services create` or `agent run` within 7 days? | `cli_analytics.events` (first-seen cohort × command × outcome) |
| **Unmodified `render` binaries** | ✅ | How many installations point Render's own CLI at bex (a command path starting with `render`, and `bex_version = unknown`)? They are migration candidates | `command`, `bex_version`, `upstream_version` |

### Business — board "Product adoption" (`bex-product-adoption`)

| Panel | Status | Question it answers | Source |
| --- | --- | --- | --- |
| Creations by surface per day | ✅ | How many services and datastores are created from the CLI, the dashboard, MCP, the API or a Blueprint? | `product_activity_events.surface` |
| Agent-driven share of creations | ✅ | How much of what gets created is created by agents? | same |
| **Agent sessions, automations and sandboxes by surface** | 🆕 D7 | Does the extended CLI start real work, compared with the dashboard and MCP? | `surface` on the new resource types |

### Operations — board "CLI usage", collection health row

| Panel | Status | Question it answers | Source | Alert |
| --- | --- | --- | --- | --- |
| Last event received — global | ✅ | When did the newest event arrive, whatever the filters? | `cli_analytics.events` | — |
| Telemetry ingest rate (by status) | ✅ | Are deliveries arriving, and how are they answered? | `bex_api_http_requests_total{route="POST /v1/cli-telemetry-events"}` | `CLITelemetryIngestFailing` ✅ D8 |
| Ingest rejection share | ✅ | What share is refused (auth, scope, rate limit, store off)? | same | same |
| Ingest latency p95 | ✅ | Is the insert slow? A climbing p95 points at the control-plane database. In CI the send is synchronous, so this latency is added to every command | `bex_api_http_request_duration_seconds` | covered by `BexApiOriginLatencyHigh` ✅ |
| **Telemetry coverage** | ✅ D6 | Is the collector quiet or broken? Stored events per CLI API request, by client | `bex_api_cli_telemetry_events_total` ÷ `bex_api_cli_requests_total` | `CLITelemetryIngestSilent` ✅ D8 |
| **Stored events by completion kind** | ✅ D8 | In near real time, is the error share moving? The SQL panels refresh every 5 minutes and read retained rows; this one reads the counter | `bex_api_cli_telemetry_events_total{completion_kind}` | `CLIExecutionErrorShareHigh` ✅ D8 |
| **Backoff-causing answers** | ✅ | How many `429`, `503` answers did we give? Each one silences a CLI for 5 or 30 minutes | `bex_api_http_requests_total{route=…, status=~"429\|503"}` | — |
| **Delivery lag** | ✅ D7 | How long after the command ended was the event received (p50/p95)? It exposes clock skew and slow synchronous sends | `received_at − (started_at + duration)` | — |
| **Retained window** | ✅ | Is the purge running? The oldest retained event should stay near 90 days, and the row count should stop growing | `min(received_at)`, `count(*)` | — |
| **Clients by release, last 24 h** | ✅ | After a release, are new versions reporting at all? A release with requests and no events has a broken sender | `bex_version`, read beside the D6 counter by client (the counter has no version label, D8) | — |

### Operations — existing panels on other boards that this pipeline depends on

| Panel | Board | Status | Why it matters here | Alert |
| --- | --- | --- | --- | --- |
| Analytics datasource probe — age since success | Data plane (`bex-data-plane`) | ✅ | The CLI usage board is blank or stale when the reader cannot connect | `AnalyticsDatasourceProbeStale` ✅ |
| Origin 5xx ratio by surface + request rate · Origin latency p50/p95/p99 by surface | bex-api (`bex-api-gateways`) | ✅ | Ingest shares bex-api's fate | `BexApiOriginHighErrorRate`, `BexApiOriginLatencyHigh` ✅ |
| Admission 429s by caller (1h) | bex-api (`bex-api-gateways`) | ✅ | A rate-limited identity loses its telemetry and starts a backoff | — |

### What the pipeline promises

| SLI | Objective (28-day) | Measured on |
| --- | --- | --- |
| Ingest success: deliveries not answered 5xx | 99.5 % | Telemetry ingest rate |
| Freshness: age of the newest stored event while CLI requests are arriving | under 2 hours | Last event received + Telemetry coverage |
| Read path: analytics datasource probe succeeding | 99 % of probes | Analytics datasource probe |

Telemetry is best effort by design, so these are warning-tier objectives that feed the daily digest. Nothing here pages.

### Cardinality budget for the new series

| Series | Labels (closed sets) | Upper bound |
| --- | --- | --- |
| `bex_api_cli_requests_total` | `client` (2) | 2 per replica, exported at 0 |
| `bex_api_cli_telemetry_events_total` (relabeled) | `completion_kind` (9), `output_format` (5), `client` (2) | 90 per replica, all exported at 0 |
