# w8/085 upstream preparation audit

Date: 2026-10-12

## Patch prepared

`085-upstream.patch`, a diff against the pin `a764810a768202704e7206eb7b87a47211fcd98e` (clone `a temporary upstream clone`).

- `pkg/tui/error.go`: `ErrorModel.View` falls back to `UserFacingError.Err.Error()` when it matches a `UserFacingError` whose `Message` is empty and whose `Err` is non-nil. That is 3 added lines. An explicit `Message` still wins, and `Title` handling is unchanged. Title-only errors still render only the title, and unwrapped errors still render `err.Error()`. No exported API changes. Child stderr is not recovered, and exit/dismissal behavior is unchanged.
- `pkg/tui/error_test.go` (new, `tui_test`): renders the real `ErrorModel` view for:
  - Title+Err
  - a real `*exec.ExitError` from `false`
  - a `UserFacingError` wrapped with `%w`
  - an explicit Message plus a different Err (the Err stays hidden)
  - title-only (no "unknown error")
  - message-only
  - an ordinary error
- `pkg/tui/views/exec_error_test.go` (new, `views`): passes the actual `handleRedisCLIError`, `handlePSQLError(PSQL)` and `handlePSQLError(PGCLI)` wrappers through `NewErrorModel` and asserts that the title and `exit status 1` both appear.

## Shared-caller review

- **`ErrorModel` consumers (3):** `pkg/tui/stack.go:110`, `pkg/tui/stack.go:193` (`ErrorMsg`) and `pkg/command/wrapper.go:154`. All of them go through the same `View`.
- **`UserFacingError{` literals in non-test source:** 37 in total, not counting the `errors.As` targets at `error.go:56` and `exec.go:39`.
  - Only **2** set `Err` without `Message`: `pkg/tui/views/rediscli.go:79` (kv-cli) and `pkg/tui/views/psql.go:140` (psql and pgcli). These are the only ones whose output changes.
  - The other 35 all set `Message`, so their rendering is unchanged. They include:
    - SSH: `ssh.go:42` embeds `err` in `Message`, plus the other SSH guards.
    - Missing executable: `exec.go:47`.
    - The resolve helpers in `pkg/postgres`, `pkg/keyvalue` and `pkg/service`.
- **Unaffected:** the noninteractive psql result path does not use `ErrorModel`.

## Tests run

```sh
cd a temporary upstream clone
go test ./pkg/tui/... ./pkg/command/   # all ok
go vet ./pkg/tui/...                   # clean
gofmt -l pkg/tui                       # clean
```

## Fail-before evidence

I restored the pinned `pkg/tui/error.go` and kept the new tests.

- **Failed:**
  - `TestErrorModelView`: 3 subtests (Title+Err, the ExitError case, the nested case).
  - `TestDatastoreExecErrorsShowUnderlyingError`: all 3 subtests (kv-cli, psql, pgcli).
- **Still passed:** the precedence, title-only, message-only and ordinary-error subtests.

With the fix restored, every test passes.

## Upstream dedupe

- `git fetch origin main` gives `668af8a87195f4de4dead5f41722c46f840fd94e`.
- `pkg/tui/error.go` and the `UserFacingError` type in `pkg/tui/stack.go` are byte-identical to the pin, so the defect is still present.
- The patch applies cleanly to `origin/main` (`git apply --check`).
- Read-only `gh search prs` and `gh search issues` on `render-oss/cli` for "UserFacingError", "ErrorModel", "error panel" and "error occurred while running" found nothing.

## What remains

1. Submitting upstream (issue or PR) needs explicit user authorization. Nothing was sent.
2. Upstream has to accept and release the fix.
3. Then a Bex pin refresh and a `bex-cli/v*` release.
4. Native receipts after adoption, for both clients:

   - kv-cli failing `INCR` with the GET/PING controls
   - psql `SELECT 1/0`
   - pgcli `--csv -q` and `--version`

   The panel should show `exit status N`.

There is no Bex-side code change: no launcher default, server or dashboard change.
