# w8/093 upstream preparation audit

Date: 2026-10-12

## Patch prepared

`093-upstream.patch` is a `git diff` against the Bex pin `render-oss/cli@a764810a768202704e7206eb7b87a47211fcd98e` (v2.27.0). It was prepared in the clone at `a temporary upstream clone`. It changes 6 files, +461/−17, and applies cleanly to both the pin and upstream main `668af8a`.

- **`pkg/project/repo.go`, `pkg/environment/repo.go`:** add the exported sentinels `ErrProjectNotFound` and `ErrEnvironmentNotFound`. On a 404 only, `GetProject` and `GetEnvironment` return an unexported error type that unwraps to the sentinel. Its `Error()` text is unchanged (`unexpected response: 404 Not Found`). This follows the existing upstream `postgres.ErrPostgresNotFound` pattern (`pkg/postgres/repo.go:15,66`). Every other status, a 200 without a JSON body, a malformed body and transport errors behave exactly as before. `client.ErrorFromResponse` and other global error consumers are untouched.
- **`pkg/resolve/resolve.go`:** `ResolveProject` and `resolveEnvironmentAndProject` change only when the by-ID lookup returns that getter's 404 sentinel. In that case they fall through to the existing scoped exact-name path, with its workspace filter, uniqueness check and project/workspace consistency checks. If no name matches, the original ID error is returned unchanged (`project "<x>" not found: unexpected response: 404 Not Found`). When an ID resolves, it still wins over a name. No exported signatures change.

## Shared-caller review

- **The 11 getter call sites:**
  - 7 `GetProject` calls: `cmd/environment.go:65`, `pkg/keyvalue/resolve.go:69`, `pkg/postgres/resolve.go:65`, `pkg/resolve/resolve.go:234,285,338` and `pkg/tui/views/resourcelist.go:99`.
  - 4 `GetEnvironment` calls: `pkg/keyvalue/service.go:171`, `pkg/postgres/service.go:147,248` and `pkg/resolve/resolve.go:232`.
  - All of them only `return`/wrap the error, and none inspects its text. The error text is byte-identical, so only the two resolver selector sites (338 and 232) change behavior.
- **The 12 resolver consumer sites:**
  - `ResolveScopeInActiveWorkspace`: `cmd/kvlist.go:101`, `pkg/keyvalue/resolve.go:27`, `pkg/postgres/resolve.go:79` and `pkg/postgres/service.go:222`.
  - `ResolveScope`: `pkg/keyvalue/create.go:50`, `pkg/postgres/service.go:129`, `pkg/tui/views/kvcreate.go:137` and `resolve.go:143,159`.
  - `ResolveEnvironmentID` with a nil env: `keyvalue/create.go:61` and `postgres/service.go:140`.
  - `resolve.go:97` (internal `ResolveProject`).
  - Through these sites, the fallback reaches KV/PG list, create and resource resolution. Unique names now resolve, missing IDs keep their error, and ambiguous names keep the ambiguity error.

## Tests

New tests in upstream testify/fake-server style:

- **`pkg/resolve/resolve_test.go`:**
  - ID-shaped project name falls back on 404, scoped to the workspace.
  - A real ID wins over a project whose name equals that ID, and no name query is sent.
  - A missing ID with no name match keeps the exact old error and `errors.Is` sentinel.
  - An ambiguous ID-shaped project name errors.
  - An ID-shaped environment name falls back.
  - An ambiguous ID-shaped environment name errors.
  - A missing environment keeps the exact old error.
  - The environment fallback stays scoped to `projectId`.
  - A table test with 12 subtests plus a transport-error case: 401/403/429/500, a non-JSON 200 and a malformed-JSON 200 return errors that are not the sentinel, and they make **zero** list (name lookup) requests.
- **`cmd/kvlist_test.go`, `cmd/pglist_test.go`:** `kv list` / `pg list --project prj-00000000000000093206 --environment evm-00000000000000093206` returns only the datastore in the target environment. It excludes the sibling env and another project's env that has the same ID-shaped name.

Results:

- `go vet ./pkg/resolve ./pkg/project ./pkg/environment ./pkg/postgres ./pkg/keyvalue ./cmd ./pkg/tui/views` passes.
- `go test ./pkg/resolve ./pkg/postgres ./pkg/keyvalue ./pkg/tui/views ./cmd` passes.
- `go test ./...` passes except `e2e`, which needs a live Render login. It fails the same way (`rate limited` at login) on the unmodified pin.

**Fail-before evidence:** I reverted the three source files to the pin and stubbed the two sentinels so the tests compile. That run failed 9 tests:

- the 7 new resolver behavior tests
- `TestKVList_FilterByIDShapedProjectAndEnvironmentNames`
- `TestPGList_FilterByIDShapedProjectAndEnvironmentNames`

The command tests failed with the live-observed `... not found: unexpected response: 404 Not Found`. The real-ID-precedence test and the non-404 no-fallback guards pass both before and after, as intended. All tests pass with the patch.

## Upstream dedupe

- `git fetch origin main` gave `668af8a87195f4de4dead5f41722c46f840fd94e`, 32 commits past the pin.
- `git diff a764810..668af8a -- pkg/resolve pkg/project pkg/environment pkg/validate` is empty, so main still has the defect.
- Read-only `gh search prs` / `gh search issues` on render-oss/cli found no matches. Queries: "IsProjectID", "environment name not found", "project name ID", "resolve project 404", "name fallback", "ResolveProject". The first batch was rate-limited and then retried.

## Remaining

1. Submitting upstream (issue/PR) needs explicit user authorization. Nothing was sent.
2. Upstream acceptance and release.
3. Bex: refresh the `render-oss/cli` pin in `lego/cli` and cut a `bex-cli/v*` release. No Bex launcher, server or dashboard change is needed or proposed.
4. Native receipts: both native clients must resolve freshly created ID-shaped project/env names via `postgres list` / `keyvalues list`, plus the controls listed in the item's acceptance.
