# w3 · m440 — Every extended bex command reports exactly once, and a test keeps it that way (ADR113 D7 contract)

**Worker:** worker3 **Goal:** a test guards the process-replacing call sites, and contract tests pin the events of the native launches and of agent outcomes. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | A test fails on any process-replacing call site that does not report — **DONE** | 30m | — |
| [t002](done/t002.md) | Contract tests for bex edit, bex agent open and bex agent ssh — **DONE** | 60m | t001 |
| [t003](done/t003.md) | Outcome contract: bex agent exit codes arrive as explicit_exit with their code — **DONE** | 30m | — |
| [t004](done/t004.md) | Docs: CLI guide note and ADR113 status — **DONE** | 15m | t002, t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 30m | t005 |
| [t007](done/t007.md) | Closeout — **DONE** | 10m | t006 |

## Definition of done

- `cd lego/cli && go test ./...` fails when a new `syscall.Exec` or `unix.Exec` call site appears without being listed and covered.
- Contract tests show `bex edit`, `bex agent open` and `bex agent ssh` each store exactly one event whose command path carries no argument.
- Contract tests show `bex agent wait` storing `explicit_exit` with exit codes 3 and 4, and an automation run that started no session storing 6.

## Source + Goal linkage

- **Source:** [ADR113](../../../../docs/ADR113-cli-usage-telemetry.md) D7 (coverage contract; gap G5). `/pm-brainstorm` for ADR113's implementation, 2026-11-16, item 4; filed by `/pmx adr113's implementation to w3`. Re-checked 2026-11-16 against `origin/main` `9a66e98b5`.
- **bex today:** five process-replacing launches report through `EmitLaunch`, and only `bex glm` and `bex --version` have binary-level tests (`lego/cli/main_test.go:869-950`). Nothing stops a new native command from replacing the process without reporting, and nothing proves how agent exit codes are stored.
- **Goal linkage:** ADR113's contract that every Bex-native command reports exactly once, the basis of every extended-CLI panel.
- **Expected outcome:** extended-CLI telemetry stays complete as commands are added, and m439's outcome panels rest on tested data.
- **Why now:** the native command tree grew by about thirty commands in two months (ADR106), and m439 starts reading exit codes.
- **Render parity omitted:** tests and docs only; no surface changes.

## Outcome (2026-11-16)

- **t001:** `lego/cli/telemetry_coverage_test.go` scans every non-test Go file for references to `syscall.Exec` and `unix.Exec`, keyed by file and enclosing function, against the reviewed `processReplacers` list. It also fails on a stale entry, an entry that names no test or a missing one, and a dot import. A `syscall.Exec` planted in a new `internal/zzplanted/run.go` failed `go test` with the file, line and what to add.
- **t002:** `lego/cli/telemetry_contract_test.go` runs the built binary for `bex edit <path>`, `bex <path>`, `bex agent open` and `bex agent ssh`: one `success` event each, named by its commands alone, with the replacing program's own record proving the process was replaced. Removing each `EmitLaunch` turned its test red (0 events).
- **t003:** `bex agent wait` on a failed and on a canceled session and an `agent automations run` that started no session store `explicit_exit` with 3, 4 and 6. No kind differed, so ADR113 and m439's `cli_fault` needed no correction.
- **t004:** `docs/bex-cli.md`, `docs/runbooks/cli-analytics.md` and ADR113 (Status, D7 as shipped) updated. `bex code` itself was listed as a process-replacing launcher; it only prints status and manages keys, so the docs and the telemetry package comment now say so.
- **t005/t006:** `/simplify` made one cleanup. The scan's shadowing and dot-import branches each have a test that fails without them.
