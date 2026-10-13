# w8/087 upstream preparation audit

Date: 2026-10-12

## Patch prepared

`087-upstream.patch` is a `git diff` against pin `a764810a768202704e7206eb7b87a47211fcd98e` (render-oss/cli). It touches 4 files with 176 insertions and no deletions:

- `cmd/psql.go`, `cmd/pgcli.go`, `cmd/kvcli.go`: each project-filter callback gains a nil-selection (`Clear filter`) branch that sets `input.Project = nil` and `input.EnvironmentIDs = nil` before reconstructing the picker. `NewPSQLView` / `NewRedisCLIView` then see `EnvironmentIDs == nil`, consult the now-cleared saved default and list the whole workspace. The existing non-nil branches are unchanged. That includes kv-cli not assigning `Project`. Tool, command, passthrough args and ID handling are unchanged, and no exported API changes.
- `cmd/projectfilterclear_test.go` (new): teatest-driven tests of the real `Interactive{PSQL,PGCLI,KeyValueCLI}View` → stack → `ProjectFilterView` path. They run against the existing `internal/fakes/renderapi` server: one workspace, one project with one empty environment, and one unassigned datastore.
  - `TestDatastorePickerFreshProcessShowsWorkspaceDatastore` is the fresh-process control: the row shows and the input has no scope.
  - `TestDatastorePickerClearingProjectFilterRestoresWorkspaceScope` runs these steps:
    1. Press `f` and Enter on the project. The view shows `No Results`, so the selected scope excludes the unrelated unassigned row, and the input holds that environment.
    2. Press `f`, then `x`. The unassigned row returns in the same process, `Project` and `EnvironmentIDs` are nil, and `config.GetProjectFilter()` is empty.

  Both tests run for psql, pgcli and kv-cli.

## Shared-caller review

`flows.WithProjectFilter` has **6** production callers:

- **3 are defective and fixed:** `cmd/psql.go:59`, `cmd/pgcli.go:48` and `cmd/kvcli.go:47`.
- **3 are unaffected controls and unchanged:**
  - `cmd/service.go:247`, `cmd/ssh.go:119` and `pkg/tui/flows/log.go:94` already build fresh inputs on nil.

`NewProjectFilterView` has 1 producer (`pkg/tui/flows/options.go:64`) and is unchanged. The saved-default precedence (`EnvironmentIDs == nil` → `DefaultListResourceInput`) is unchanged and is what the fix relies on. No server, REST/GraphQL/MCP or dashboard change is needed.

## Tests run

- `go test ./cmd -run TestDatastorePicker -count=1` (and `-count=5`, to check for flakiness): pass.
- `go test ./cmd ./pkg/tui/...`: all pass (`cmd`, `pkg/tui`, `layouts`, `testhelper`, `views`, `views/workflows`).
- `go vet ./cmd ./pkg/tui/...`: clean. `gofmt -l cmd`: clean.

## Fail-before evidence

With only the three source edits stashed, the clearing test fails for psql, pgcli and kv-cli. Each wait for the restored row times out after 5s, and the last frame still shows `No Results`. For psql and pgcli the stale header `Project: scoped-project` is also still there, which matches the native reproduction. The fresh-process control passes before and after. With the fix restored, all subtests pass.

## Upstream dedupe

- `git fetch origin main` → `668af8a87195f4de4dead5f41722c46f840fd94e`. `git diff` between the pin and that commit is empty for `cmd/{psql,pgcli,kvcli,service,ssh}.go`, `pkg/tui/views/{projectfilter,psql,rediscli}.go` and `pkg/tui/flows/`, so upstream main still has the defect.
- Read-only `gh search prs/issues --repo render-oss/cli` for "project filter", "clear filter", "clear project", "ClearProjectFilter", "psql project", "kv-cli filter", "project filter psql", "No Results filter" and "EnvironmentIDs" returned no results.
- Nothing was submitted externally.

## What remains

1. External submission (an issue or PR to render-oss/cli) needs explicit user authorization.
2. Upstream acceptance and release.
3. On the Bex side: refresh the `lego/cli` pin to a release that contains the fix, then cut a Bex CLI release. No Bex launcher default is involved.
4. Native receipts: repeat the psql, pgcli and kv-cli clear journeys on both clients with disposable fixtures, and verify cleanup.
