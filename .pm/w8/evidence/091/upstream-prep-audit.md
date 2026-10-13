# w8/091 upstream preparation audit

Date: 2026-10-12

## Patch prepared

`091-upstream.patch` is a diff against the Bex pin `a764810a768202704e7206eb7b87a47211fcd98e` of `render-oss/cli`. The work was done in the local clone `a temporary upstream clone`.

- `cmd/ssh.go` (1 line changed, plus a comment): the SSH chooser's project-filter callback in `getSSHTableOptions` now calls the lowercase chooser dispatcher `interactiveSSHView(ctx, &input, breadcrumb)`. Before, it called the uppercase execution-view constructor `InteractiveSSHView`. The callback still builds a fresh `SSHInput`. For a selected or saved project it sets `Project`/`EnvironmentIDs`. For Clear it passes nil, which leaves the input empty. Because `ServiceIDOrName` is empty, the dispatcher takes the `NewServiceList` branch through `getServiceListInput`. That keeps the environment scope and the web/private/worker type restriction. The callback no longer enters `NewSSHView` with an empty selector, so the CLI no longer sends `GET /services?name=` and gets a 400.
- `cmd/ssh_test.go` (new, 240 lines): teatest integration tests that run the real `interactiveSSHView` → stack → `WithProjectFilter` → `NewProjectFilterView` path against `internal/fakes/renderapi`. The `InteractiveSSHView` package var is stubbed as a recorder, so no SSH child process starts.
  - Selecting a project with one empty environment shows the scoped chooser with **No Results**. The last list request carries that `environmentId`, no `/services?` request has `name=`, and the execution view is never opened.
  - Selecting a project with two eligible services shows exactly those two services, without the baseline service.
  - Clear (`x`) returns the unfiltered rows, and the last list request has no `environmentId`.
  - Save as default (`s`) shows the scoped chooser and persists the project ID and name. A fresh chooser honours the saved default. Clearing from that chooser removes the config and returns the unfiltered rows. A fresh chooser after that stays unfiltered.
  - Direct controls: an instance ID, an ephemeral service ID and a service name still go to the execution view with the expected `ServiceIDOrName`, `InstanceID` and `Ephemeral` values.

No exported API changed.

## Shared-caller review (pinned source)

- `flows.WithProjectFilter`: 1 definition (`pkg/tui/flows/options.go:58`) and 6 call sites: `cmd/ssh.go:119`, `cmd/service.go:247`, `cmd/psql.go:59`, `cmd/pgcli.go:48`, `cmd/kvcli.go:47` and `pkg/tui/flows/log.go:94`. Only the SSH callback is changed. The services, psql, pgcli, kv-cli and logs callbacks already return their own list views and are left alone. The generic flow and `projectfilter.go` are not modified. Their Select, Save (`:27–32`) and Clear (`:35–41`) actions all use the same handler, so all three are fixed by this one callback change.
- `InteractiveSSHView`: 1 definition (`cmd/ssh.go:42`) and 8 calls before the patch; 7 after. Six are inside `interactiveSSHView`, at lines 51, 63, 73, 80, 90 and 95. These are the instance-ID, ephemeral-selection, instance-selection, ephemeral-service-ID, service-ID-instance and name paths, and none is changed. The seventh, `cmd/service.go:174`, is the services palette's intentional direct SSH action on a concrete service ID and is not changed. The eighth, the project callback at `cmd/ssh.go:125`, was the defect and is the one this patch moves.
- `interactiveSSHView`: 1 definition and 1 root `RunE` call before the patch; the patched callback is now a second call. No recursion hazard: the callback runs only when the user acts, and the package still builds and passes vet.
- Out of scope, intentionally not changed:
  - 086: a bare project with empty `EnvironmentIds` falls back to `DefaultListResourceInput`.
  - 087: datastore stale-input fixes.
  - The known exact-instance picker residual: lines 72–73 set `input.InstanceID = input.ServiceIDOrName`.
  - The fresh `SSHInput` in the callback drops the original `--ephemeral`/`Args`. This behaviour was already present at the pin and is noted here only as an observation.

## Tests run

All runs were in `a temporary upstream clone` with Go 1.27.0, darwin/arm64:

- `go test ./cmd -run 'TestSSHProjectFilter|TestInteractiveSSHView' -count=1 -v`: all 5 PASS. Repeated 3 more times, all ok.
- `go test ./cmd ./pkg/tui/... -count=1`: ok (cmd, tui, layouts, testhelper, views, views/workflows).
- `go vet ./cmd ./pkg/tui/...`: clean. `gofmt -l cmd/`: clean.

## Fail-before evidence

With pinned `cmd/ssh.go` restored and the new test kept, the 4 `TestSSHProjectFilter_*` tests FAIL:

- `ssh_test.go:124`, `:149`, `:172` and `:195`: `WaitFor: condition not met after 3s`.
- In each case the filter action was dispatched to the recorded `InteractiveSSHView` with an empty `ServiceIDOrName`, and the scoped or unfiltered chooser never appeared.
- The direct-argument control passes both before and after the patch.

With the fix restored, all 5 pass.

## Upstream-main dedupe

- `git fetch origin main` gave `668af8a87195f4de4dead5f41722c46f840fd94e`. Compared with the pin, `cmd/ssh.go`, `pkg/tui/flows/options.go`, `pkg/tui/views/projectfilter.go` and `pkg/tui/views/ssh.go` are byte-identical, and there are no commits touching `cmd/ssh.go`. `cmd/ssh_test.go` does not exist on main.
- The patch applies cleanly to that main in a temporary worktree, and the new tests pass there (ok, 7.6s). The worktree was removed afterwards.
- Read-only `gh search prs` and `gh search issues` on `render-oss/cli` returned 0 results for each of: "ssh project filter", "ssh invalid query parameter name", "InteractiveSSHView" and "ssh filter by project". This is not an exhaustive census.
- Nothing was sent upstream.

## What remains

1. Submitting upstream (issue/PR) needs explicit user authorization. None was given.
2. Upstream review, acceptance and a release containing the fix.
3. On the Bex side, after separate authorization: refresh the `lego/cli` pin to the accepted upstream commit or release, then cut a Bex CLI release. No Bex launcher default, server filter, request rewrite or fork is needed, and none was changed.
4. Native receipts: rerun the Bex and Render clients against an owned disposable project for these cases: select, Clear, Save as default, a fresh process with the saved default, and Clear from there. Then verify cleanup. A populated-project native run and ordinary service-row → instance selection were not covered here; the fake API has no instances route.
