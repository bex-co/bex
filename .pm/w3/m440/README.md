# w3 · m440 — Every extended bex command reports exactly once, and a test keeps it that way (ADR113 D7 contract)

**Worker:** worker3 **Goal:** a test guards the process-replacing call sites, and contract tests pin the events of the native launches and of agent outcomes. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | A test fails on any process-replacing call site that does not report | 30m | — |
| [t002](t002.md) | Contract tests for bex edit, bex agent open and bex agent ssh | 60m | t001 |
| [t003](t003.md) | Outcome contract: bex agent exit codes arrive as explicit_exit with their code | 30m | — |
| [t004](t004.md) | Docs: CLI guide note and ADR113 status | 15m | t002, t003 |
| [t005](t005.md) | Simplify | 15m | t004 |
| [t006](t006.md) | Test coverage | 30m | t005 |
| [t007](t007.md) | Closeout | 10m | t006 |

## Definition of done

- `cd lego/cli && go test ./...` fails when a new `syscall.Exec` or `unix.Exec` call site appears without being listed and covered.
- Contract tests show `bex edit`, `bex agent open` and `bex agent ssh` each store exactly one event whose command path carries no argument.
- Contract tests show `bex agent wait` storing `explicit_exit` with exit codes 3 and 4, and an automation run that started no session storing 6.

## Source + Goal linkage

- **Source:** [ADR113](../../../docs/ADR113-cli-usage-telemetry.md) D7 (coverage contract; gap G5). `/pm-brainstorm` for ADR113's implementation, 2026-11-16, item 4; filed by `/pmx adr113's implementation to w3`. Re-checked 2026-11-16 against `origin/main` `9a66e98b5`.
- **bex today:** five process-replacing launches report through `EmitLaunch`, and only `bex glm` and `bex --version` have binary-level tests (`lego/cli/main_test.go:869-950`). Nothing stops a new native command from replacing the process without reporting, and nothing proves how agent exit codes are stored.
- **Goal linkage:** ADR113's contract that every Bex-native command reports exactly once, the basis of every extended-CLI panel.
- **Expected outcome:** extended-CLI telemetry stays complete as commands are added, and m439's outcome panels rest on tested data.
- **Why now:** the native command tree grew by about thirty commands in two months (ADR106), and m439 starts reading exit codes.
- **Render parity omitted:** tests and docs only; no surface changes.
