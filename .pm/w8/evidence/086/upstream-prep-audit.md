# w8/086 upstream preparation audit

Date: 2026-10-12

## Patch prepared

`086-upstream.patch` is a diff against the pinned `render-oss/cli` commit `a764810a768202704e7206eb7b87a47211fcd98e`, prepared in `a temporary upstream clone`. The premise holds at the pin:

- `pkg/tui/views/resourceloader.go:16–32`: `ToParams` drops `Project`, and `pkg/resource/service.go:93–143` leaves out `environmentId` when the slice is empty. A selected project with no environments therefore makes the same requests as All Projects.
- `pkg/tui/views/resourcelist.go:45–51`: the saved default replaces the input whenever `EnvironmentIDs` is empty, even when the caller passed an explicit `Project`.

What the patch changes:

1. `ResourceLoader.LoadResourceData` returns an empty list and makes no requests when `in.Project != nil` and it has no environment IDs. This follows the existing scope-presence guard in `pkg/postgres/service.go:68–82`. When no Project is selected, an empty or nil `EnvironmentIDs` still lists everything.
2. `NewResourceView` applies the saved default only when `input.Project == nil && len(input.EnvironmentIDs) == 0`.
3. A new test file, `pkg/tui/views/resourceloader_test.go`, runs against the existing `internal/fakes/renderapi` fake with an isolated config.

There are no exported API changes and no placeholder environment IDs. `ResourceParams` and the four family adapters are unchanged.

## Shared-caller review

- `LoadResourceData` has 3 callers:
  - `cmd/service.go:234`: interactive services.
  - `cmd/service.go:276`: non-interactive listing. `Project` is always nil here, so it stays unfiltered.
  - `pkg/tui/flows/log.go:78`: logs resource picker.
- `NewResourceView` has 2 callers:
  - `resourcelistwithpalette.go:18`, which serves services.
  - `flows/log.go:78`, which serves logs.
- `InteractiveServices` has 3 input sources:
  - Project filter (`cmd/service.go:247–256`): the fixed path.
  - Environment drill-down (`cmd/environment.go:31`): passes `EnvironmentIDs` with no Project, so it is unchanged.
  - Root `RunE`: flags only, no Project, so it is unchanged.
- The logs project filter (`flows/log.go:94–101`) is fixed the same way.
- `DefaultListResourceInput` has 4 callers: `resourcelist.go`, `cmd/ssh.go:106`, `psql.go:107` and `rediscli.go:47`. Only the `resourcelist.go` branch changed.
  - `ssh.go:105` has the same "default replaces an explicit empty project" pattern for the SSH service list. It is a neighbor that was not reproduced and is left unchanged.
  - `psql`/`rediscli` check for `EnvironmentIDs == nil`, so an explicit project with an empty slice already skips the default there.
- Existing behavior left unchanged: choosing "All Projects" passes `ListResourceInput{}`, so a saved default is still re-applied. With no saved default, All Projects stays unfiltered, as the item requires.

## Tests

The new tests cover 10 cases:

- **Loader, 6 cases:** All Projects; empty env IDs without a Project; a selected project with empty env IDs, where the test asserts no `/services`, `/postgres`, `/key-value` or `/workflows` requests; a selected project with nil env IDs; a selected project with one empty environment, where the test asserts `environmentId=` is in the request; and a populated environment.
- **View, 4 cases:** explicit empty project plus a different saved default, where the screen shows the chosen header and "No Results", the saved project is never fetched, and there is no `/services` request; explicit empty project with no default; no explicit scope, where the saved default applies; and no scope and no default, where everything is listed.

Results:

- `go test ./pkg/tui/... ./pkg/resource/... ./cmd/...`: all ok (`pkg/tui`, `layouts`, `testhelper`, `views`, `views/workflows`, `cmd`).
- `go vet ./pkg/tui/views/ ./pkg/tui/flows/ ./cmd/`: clean.
- `gofmt`: clean.

Fail-before evidence:

- With both source changes reverted, 4 subtests fail: loader no-env and nil-env, and view explicit-empty with and without a saved default.
- With only `resourcelist.go` reverted, view explicit-empty-with-saved-default fails.
- With only `resourceloader.go` reverted, both loader subtests and both view explicit-empty subtests fail.

So each half is independently guarded. All subtests pass with the patch.

## Upstream-main dedupe

- `git fetch origin main` reached `668af8a87195f4de4dead5f41722c46f840fd94e`. These files have no diff from the pin: `resourceloader.go`, `resourcelist.go`, `pkg/resource/service.go`, `cmd/service.go` and `pkg/tui/flows/log.go`. The defect is not fixed upstream.
- Read-only `gh search prs/issues --repo render-oss/cli` for "project filter empty", "empty project", "project filter", "environmentIds empty", "saved default project filter" and "ResourceLoader" returned no matches. A broad "project" query returned only unrelated items: #22, #11, #32, #15, #41, #26 and PR #47.

## What remains

- External submission (issue/PR to `render-oss/cli`) needs user authorization. Nothing was sent.
- Upstream review, acceptance and a release.
- A Bex pin refresh in `lego/cli` and a Bex CLI release (`bex-cli/v*`).
- Native receipts: repeat the empty-project and empty-environment controls, plus the saved-default services/logs pickers, with disposable metadata and exact-ID cleanup.
- Bex side: nothing to change. The backend correctly allows bare projects, and no launcher rewrite is warranted.
