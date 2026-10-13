# w8/082 upstream preparation audit

Date: 2026-10-12

## Premise

Holds at pin `a764810a768202704e7206eb7b87a47211fcd98e`: `pkg/command/time.go:40` formats absolute `TimeOrRelative.String()` with `time.RFC3339`, which drops fractional seconds. `pkg/command/form.go:72-75` seeds the form from that string, and `:171-184` parses it again on apply (`pkg/tui/views/logview.go:175-183`, Enter at `:246-249`). So applying the log filter, with or without edits, truncates both bounds to whole seconds.

## Patch prepared

`082-upstream.patch` (git diff against the pin, 194 lines, 4 files):

- `pkg/command/time.go`: the absolute branch of `String()` now uses `time.RFC3339Nano`. Relative strings are still returned as typed. Whole-second times are spelled the same as before because RFC3339Nano trims trailing zero fractions. `ParseTime` already accepts fractional input, since Go's `time.Parse(time.RFC3339, …)` takes fractional seconds. No exported API changes.
- `pkg/command/time_test.go`: `TestTimeOrRelativeStringRoundTrip` covers UTC ns, UTC ms, +05:30 ns, whole-second UTC, whole-second offset and relative `5m`. `TestCobraTimeFractionalSeconds` covers flag display.
- `pkg/command/form_test.go`: `TestFormValuesPreserveTimeOrRelative` runs `FormValuesFromStruct` → `StructFromFormValues` for the same six inputs, both unchanged and after an unrelated `text` edit. It also checks that unset times stay unset.
- `pkg/tui/views/logview_test.go`: a new `TestLogView` subtest drives the real `NewLogsView`: open the filter with `/`, press Enter without editing, then check that both the ns UTC start and the ns +05:30 end are unchanged.

## Shared-caller review

- `TimeOrRelative.String()` has 2 production callers. One is `form.go:74` (form seed). The other is `CobraTime.String()` (`time.go:98`), used for pflag display and by `form.go:518` (time-flag form value). Time flags are created only at `cmd/logs.go:71-72` and have no default, so help text is unchanged (the default is an empty string).
- `HuhFormFields` has 4 production callers: `logview.go:175`, `jobcreate.go:59`, `workflows/workflowcreate.go:138` and `workflows/taskrun.go:29`. Only `LogInput` declares `TimeOrRelative` fields (2: `start`, `end`). The other three callers can't be affected.
- `NewLogsView` has 2 production call sites, `pkg/tui/flows/log.go:68` and `:87`. Several places seed absolute times that the deploy, job and workflow log viewers inherit: `cmd/deploylist.go:67-74`, `cmd/joblist.go:70-77`, `pkg/tui/flows/workflow.go:96,173,329-337` and `flows/log.go:57`. Those API timestamps can carry fractional seconds, so the same fix now preserves them through a filter re-apply. Pagination (`logview.go:196-199`) builds `time.Time` directly and does not go through `String()`.
- The noninteractive path (`inputs.go:37`, `getTimeValue`) reads the parsed value, not the string, so it is unchanged. Resource-filter semantics (service, Postgres, Key Value, workflow) are untouched.

## Tests run

- At the pin, with the patch: `go test ./pkg/command ./pkg/tui/... ./cmd/...` all pass. `go vet ./...` is clean, and `gofmt -l pkg` reports nothing.
- On upstream main `668af8a` with the patch applied: `git apply --check` succeeds, and `go test ./pkg/command ./pkg/tui/views` passes.

## Fail-before evidence

I temporarily reverted only `time.go` and kept the new tests. These fail:

- `TestFormValuesPreserveTimeOrRelative`: the ns UTC, ms UTC and ns +05:30 cases, in both the unchanged-apply and text-edit variants.
- `TestTimeOrRelativeStringRoundTrip`: utc_nanoseconds, utc_milliseconds, offset_nanoseconds.
- `TestCobraTimeFractionalSeconds`.
- `TestLogView/applying_unchanged_filters_preserves_fractional_time_bounds`.

The whole-second UTC, whole-second offset, relative and unset controls pass both before and after.

## Upstream dedupe

- `git fetch origin main` gives `668af8a87195f4de4dead5f41722c46f840fd94e`. `pkg/command/time.go` and `pkg/command/form.go` are byte-identical to the pin, and line 40 still uses `time.RFC3339`. The defect is not fixed upstream.
- I ran read-only `gh search prs` and `gh search issues` on `render-oss/cli` for "RFC3339Nano", "fractional seconds", "TimeOrRelative", "log filter time precision" and "nanoseconds". All returned no results.

## What remains

1. External submission (upstream issue/PR) needs explicit user authorization. Nothing was sent.
2. Upstream acceptance and a Render CLI release.
3. A Bex refresh of the pinned `render-oss/cli` and a Bex CLI release (`bex-cli/v*`). There is no Bex-side launcher or server change: the backend already preserves fractional bounds.
4. Native receipts after adoption: the one-row fractional point journey in `-o interactive` with `/` + Enter in both the Bex and Render binaries, a text-only edit that keeps both bounds, and the inherited deploy/job/workflow log viewers. Then update the compatibility ledger.
