# w5 · m94 — CLI telemetry sender completeness: bex release version, launcher-native commands, exec-based provider launches

**Worker:** worker5 **Goal:** every CLI telemetry row bex-api stores carries the bex launcher release next to the upstream pin, and the launcher-owned commands the upstream sender never sees — root `--version`, `bex upgrade`, and the `bex code` / `bex glm` provider launches that `exec` away — emit one Render-shaped event each under the same consent rules, so the CLI usage board can measure release adoption and agent-launch usage instead of labeling them as gaps. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                       | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Launcher: `X-Bex-CLI-Version` header on every bex-api request via a default-transport wrapper               | 30m | —          |
| t002 | bex-api: persist a bounded `bex_version` from the header; expose it in `cli_analytics.events`               | 45m | t001       |
| t003 | Launcher-native events: root version, upgrade, and provider launches before `exec`, consent-gated           | 60m | t002       |
| t004 | CLI usage board: bex-release filter + installations-by-release panel; label provider launches; retire caveat | 30m | t003       |
| t005 | End-to-end on dev-5 with a tree-built launcher at the pinned upstream version                               | 30m | t004       |
| t006 | Render parity                                                                                               | 30m | t005       |
| t007 | Simplify                                                                                                    | 30m | t006       |
| t008 | Test coverage                                                                                               | 45m | t006       |
| t009 | Closeout                                                                                                    | 15m | t008       |

## Definition of done

- A telemetry row produced by a launcher built with `scripts/bex-cli-build.sh` carries `cli_version` = the pinned upstream version **and** `bex_version` = the bex tag; a plain `go build` stores `dev`. An unmodified upstream `render` binary pointed at bex still ingests with `bex_version = ''` (never rejected).
- `bex --version`, `bex upgrade`, and one `bex glm` / `bex code` launch each produce exactly one event; no upstream command produces a second event.
- `DO_NOT_TRACK`, `BEX_CLI_DISABLE_ANALYTICS`, and an explicit `RENDER_CLI_DISABLE_ANALYTICS` suppress the new emitters exactly as they suppress the upstream sender; no event body ever carries command arguments, prompts, or paths.
- The live CLI usage board at obs.bex.co filters by bex release and shows installations per release; provider launches appear under their own command labels; the m93 "coverage boundary" wording is removed wherever the measurement now exists.
- `docs/bex-cli.md` § CLI usage telemetry and the `docs/cli-compatibility-checklist.md` telemetry row describe the header, the stored field, and the launcher-native events.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5` 2026-09-10 proposal 2, from the `w5/m93` closeout's recorded coverage boundary: "Bex release-version instrumentation and successful exec-based provider-launch telemetry require separate sender work" and "root version requests, logged-out/early setup failures and successful exec-based provider launchers have collection gaps" ([done/m93/README.md](../done/m93/README.md)).
- **Facts verified 2026-09-10:** `cli_telemetry_events.cli_version` stores only the upstream pin (`store/migrations/0113_cli_telemetry_events.up.sql`); `main.bexVersion` is injected by `scripts/bex-cli-build.sh` (`-X main.bexVersion=`) but never reaches bex-api. The upstream generated client is built as `&http.Client{}` with a nil Transport (`render-oss/cli` `pkg/client/client.go:41`), so it rides `http.DefaultTransport`; the analytics sender re-execs the same binary through `os.Executable()` (`pkg/analytics/subprocess.go:86`), so a transport wrapper installed in the launcher's `main()` applies to the detached send as well — no fork. `bex code` / `bex glm` replace the process with `syscall.Exec` (`lego/cli/internal/code/launch.go:116`), so upstream's post-run analytics hook never fires for them; root `--version` exits before `cmd.Execute()` (`lego/cli/main.go`).
- **Goal linkage:** ADR008's AI-native thesis — the coding commands are the agent-launch path of the CLI and are invisible today; ADR088 CLI usage board (product analytics); ADR058 release train (release adoption of `bex-cli/v*` is unmeasurable without the field).
- **Expected outcome:** operators can answer "which bex releases are in use" and "how often do people launch an agent through bex" from the existing board, and the self-update channel (`w4/m37`) has a measured adoption curve.
- **Why now:** m93 shipped 2026-09-10 with this gap labeled on the board itself; the next `bex-cli/v*` train can carry the sender, and every release until then adds unattributable rows to a 90-day retention window.
- **Render parity included:** the milestone extends the REST telemetry endpoint with a header-derived field. Upstream exposes telemetry over REST only, so — as `w5/m92` decided — GraphQL/MCP twins are deliberately out of scope; the parity task confirms the exact upstream body is still accepted and records the header as a bex extension in the checklist.
- **Anti-goal check:** `.pm/DO_NOT_DO.md` forbids a first-party/forked CLI. This milestone touches only the launcher's process configuration (a transport wrapper) and its own added cobra commands; `github.com/render-oss/cli` stays an unmodified dependency.
