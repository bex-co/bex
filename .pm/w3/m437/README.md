# w3 · m437 — The bex CLI shows its own telemetry notice and sends nothing before it does (ADR113 D5)

**Worker:** worker3 **Goal:** a `bex` user is told that Bex, not Render, receives CLI usage data before any event is stored; Render's notice never prints; the launcher's self-reports pass the same gate; installers show the notice. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Launcher gate: a Bex marker, upstream's marker pre-written, and an undisclosed run sends nothing | 75m | — |
| [t002](t002.md) | Print the Bex notice after an eligible terminal run and record that it was shown | 45m | t001 |
| [t003](t003.md) | Launcher self-reports wait for the same gate | 30m | t001 |
| [t004](t004.md) | The installer and the Homebrew formula show the same notice | 45m | t002 |
| [t005](t005.md) | Docs: CLI guide, analytics runbook, pin-bump checklist, compatibility checklist, ADR113 status | 30m | t002, t003, t004 |
| [t006](t006.md) | Live check: a tree-built bex against api.bex.co | 30m | t005 |
| [t007](t007.md) | Render parity | 30m | t006 |
| [t008](t008.md) | Simplify | 15m | t007 |
| [t009](t009.md) | Test coverage | 30m | t008 |
| [t010](t010.md) | Closeout | 10m | t009 |

## Definition of done

- On a clean `BEX_CLI_CONFIG_DIR`, a tree-built `bex` run in a terminal against `api.bex.co` prints the Bex notice once and stores no event; the next run stores exactly one. Render's notice text never appears.
- `CI=true` sends without a notice. A run with no terminal and no `CI` sends nothing and writes no Bex marker.
- `bex --version`, `bex glm` and the other self-reports store nothing until the Bex marker exists, then one event each.
- `scripts/install-bex.sh` prints the notice and writes the marker; the Homebrew formula carries the same text as caveats.
- `cd lego/cli && go test ./...` and `bash scripts/install-bex.test.sh` pass.

## Source + Goal linkage

- **Source:** [ADR113](../../../docs/ADR113-cli-usage-telemetry.md) D5 (gaps G2, G3, G4). `/pm-brainstorm` for ADR113's implementation, 2026-11-16, item 1; filed by `/pmx adr113's implementation to w3`. Re-checked 2026-11-16 against `origin/main` `9a66e98b5`.
- **bex today:** every new install sees upstream's notice, "The Render CLI collects usage data … No data has been sent to Render yet … render.com/docs/cli#usage-telemetry" (`internal/analyticsnotice/notice.go`), while the events go to Bex (w5/m92 flipped consent on 2026-09-10). `docs/bex-cli.md` tells users not to act on it. `lego/cli/internal/telemetry/telemetry.go` documents that the self-reports skip the gate. A headless install with no `CI` marker never sends, because the notice cannot be shown without a terminal.
- **Render:** CLI v2.36.0 shows a one-time notice before the first send, the run that shows it sends nothing, `CI` bypasses, a run without a terminal never sends (`cmd/execution.go:28-69`).
- **Goal linkage:** ADR008 Render parity first (the same gate and opt-outs) and ADR113's privacy line: the user is told the truth before anything is sent.
- **Expected outcome:** every `bex` user is told that Bex, not Render, receives CLI usage data before any event is stored, and installer users are told at install time.
- **Why now:** the wrong disclosure has been live for two months and reaches every new install. It is the only D5–D9 item that is a truthfulness fix rather than an improvement.
- **Recommendations recorded (not user decisions):** agent environment markers do not bypass the gate (ADR113 open question 1; nothing is sent before a disclosure a person could read). Installers use `bex analytics notice`; Homebrew shows caveats only (open question 2).
- **Render parity included:** the CLI is a customer surface and the gate must behave as Render's. There is no REST, GraphQL, MCP or dashboard change.
