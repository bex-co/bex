# w8/m60 upstream preparation audit

Date: 2026-10-12

## Premise

Holds at pin `a764810a768202704e7206eb7b87a47211fcd98e`. `pkg/command/inputs.go:373–442` `InputToString` dereferences `*TimeOrRelative` before `%v` (395–401, 420), so the Go struct is printed. It joins every slice with bare commas (406–415), whatever the flag type, and joins tokens with spaces without quoting (438–441). `pkg/command/formatter.go:40–45` `CommandName` feeds that string to `wrapper.go:134` → `ModelWithCmd.Cmd` → `pkg/tui/stack.go:161` `clipboard.WriteAll`.

## Patch prepared

`upstream-copy-command-formatter.patch` (git diff against the pin; applies cleanly at the pin and at current upstream main). Files: `pkg/command/inputs.go`, `pkg/command/formatter.go`, new `pkg/command/formatter_test.go` and `cmd/logscopy_test.go`.

- `InputToString(v any) (string, error)` keeps its exported signature. It now delegates to an internal `inputArgs(v, flags *pflag.FlagSet)`.
- `CommandName` passes `cmd.Flags()`, so slice encoding follows the effective registered flag type:
  - `stringSlice` (or no metadata, matching pflag's default for `[]string`): one `--flag=` token, CSV-encoded with `encoding/csv` the same way pflag's own `writeAsCSV` does. Literal commas, quotes and leading spaces stay inside their element.
  - `stringArray`: one repeated `--flag=elem` per element.
  - Enum (`CobraEnum.Set` splits on bare commas) and int/float/bool slices: bare comma join, as before.
- `*TimeOrRelative`: emits the `Relative` text when present, otherwise `T.Format(time.RFC3339Nano)`. This is a local helper and does not call or change `TimeOrRelative.String`, so the separate 082 fix is not duplicated. `CobraTime.Set`/`time.Parse(RFC3339)` accepts the fractional seconds.
- Each positional value and each flag value is POSIX single-quoted only when it contains characters outside `[A-Za-z0-9_@%+=:,./-]`. Simple commands print exactly as before, and the existing `TestInputToString` cases are unchanged and still pass.
- These behaviours are unchanged: positional order, nil-pointer and zero-scalar skipping (form-produced `pointers.From("")` therefore stays omitted), the `cmd.Hidden` suppression and `CommandPath()`.

Examples: `--text='LEFT RIGHT'`, `--text='"LEFT,RIGHT"'`, `--start=30m`, `--start=2026-10-11T08:30:05.415654123-07:00`.

## Shared-caller review

- `CommandName` has 1 production caller (`pkg/command/wrapper.go:134` `wrappedModel`). `InputToString` has 1 production caller (`CommandName`).
- `AddToStack`/`AddToStackFunc`: 42 production call sites in 22 files. This matches the filed census. All of them reach the formatter through `wrappedModel`, and hidden commands are skipped.
- Tagged field types across the tree: `string` 78, `*string` 99, `[]string` 37, `bool` 20, `*bool` 9, `*int` 9, `int` 4, `*TimeOrRelative` 3, plus string-kind named types (pointer enums such as `*ServiceRuntime`, `Plan`). Scalars are formatted with `%v` as before; only their quoting changed.
- Flag types registered for production `[]string` tags:
  - `StringSlice`/`StringSliceP`: resources, text, instance, host, status-code, path, task-id, task-run-id, environment-ids, env-file, skill.
  - `StringArray`: env-var, ip-allow-list, read-replica, secret-file, build-filter-path, build-filter-ignored-path, status.
  - Enum `Var`: level, type, method.
  - No other slice flag types are present, so all three branches are exercised. `StringArray` copies previously collapsed into one `a,b` element when replayed; this patch fixes that as well.
- Aliases (`kv`/`keyvalue`, `pg`, `ls`, `service`, `cp`): `CommandPath()` uses canonical names, which stay valid. Flags are always emitted by long name. No change.
- No `[]T` field carries an `arg:N` tag, so moving slices into the flags branch changes no positional behaviour.

## Tests

Regression tests feed the copied string through a real POSIX shell (`sh -c 'eval "set -- $1"; printf "%s\0" "$@"'`) and then back through the real flag parser plus `command.ParseCommand`:

- `cmd/logscopy_test.go`, real `NewLogsCmd(nil)`:
  - the three native failures: phrase, literal comma, relative `30m` start;
  - both controls: plain text, and two OR terms;
  - absolute start/end bounds with nanoseconds;
  - quotes, an apostrophe, `$HOME`, `*`, Unicode and fullwidth commas, and a leading space;
  - an empty-element neighbour (`,a`);
  - multi-select enums, a single enum, `--limit` and `--tail`;
  - a repeated StringSlice flag, paths containing spaces, a lone CR, and defaults only;
  - TailLogsFlow's automatic `T`-only start time.
- `pkg/command/formatter_test.go`, synthetic command:
  - positional spaces and quotes;
  - StringSlice elements, and StringArray repeats including an empty element;
  - enum list and single enum;
  - explicit pointer scalars;
  - relative and absolute time;
  - nil/defaults;
  - exact-output checks for `CommandName` and `InputToString`.

Results with the patch: `go test ./pkg/command ./cmd` ok, `go vet ./pkg/command ./cmd` clean, `gofmt -l` clean. `go test ./...` passes every package except `e2e`, which fails identically on the unmodified pin because live Render login returns "rate limited" (environmental).

Fail-before evidence: with `inputs.go`/`formatter.go` reverted to the pin, 13 subtests fail:

- `TestCommandNameRoundTrip`: positional_spaces_and_quotes, string_slice_elements, string_array_repeated, explicit_scalars, relative_time, absolute_time.
- `TestLogsCopyCommandRoundTrip`: phrase_with_space, literal_comma, relative_start, absolute_bounds_with_nanoseconds, quotes_unicode…, repeated_slice_flag, path_with_spaces.

The control cases (plain, two OR terms, enums/scalars, defaults) pass both before and after.

CRLF note: pflag `GetStringSlice` re-encodes via CSV and the CSV reader normalizes quoted `\r\n` before the formatter sees the value, so that byte change is upstream of Copy (separate 077 scope). The patch claims round-trip only for bytes the existing parser accepts; a lone `\r` round-trips. Likewise `--text '""'` already parses to an empty slice at the pin, so it is omitted, as before.

## Upstream-main dedupe

`git fetch origin main` → `668af8a87195f4de4dead5f41722c46f840fd94e`. `pkg/command/`, `pkg/tui/stack.go` and `pkg/tui/flows/log.go` are unchanged pin→main. Main only changes `cmd/logs.go` (non-interactive tail/list split, `MarkFlagsMutuallyExclusive("tail","end")`), and that does not affect Copy. The defect is still present in main.

Read-only `gh search prs` and `gh search issues` on `render-oss/cli` returned 0 results for "InputToString", "copy command", "clipboard quote" and "copied command". The first attempt was GitHub rate-limited; the retry succeeded. Nothing external was created or sent.

## Remaining (not satisfiable by this local patch)

- External submission (issue/PR) requires explicit user authorization; do a fresh dedupe before submitting.
- Upstream review/acceptance and a release containing the fix.
- Bex side, with no change made here: refresh the reviewed `render-oss/cli` pin in `lego/cli/go.mod`/`go.sum`, `UPSTREAM_RENDER_CLI.md` and `docs/cli-compatibility-checklist.md`, then make an authorized Bex CLI release. Do not fork or vendor.
- Native receipts on fresh owned Free fixtures (t004/t006): the three journeys and two controls on both binaries, then cleanup convergence.
- Windows shells (PowerShell/cmd) are not specifically quoted; the output targets POSIX shells (macOS/Linux). Upstream maintainers should decide whether Windows needs its own quoting.
- A positional value beginning with `-` is still emitted without `--` (unchanged behaviour).
