# w8/m57 upstream preparation audit

Date: 2026-10-12

## Patch prepared

Prepared `upstream-lossless-string-list.patch` against the pinned upstream module `github.com/render-oss/cli@v1.1.3-0.20260909214233-a764810a7682` (`a764810a7682`). The patch changes the shared form string-list codec in `pkg/command/form.go` and adds `pkg/command/form_string_slice_test.go`.

The candidate keeps ordinary editable comma-separated input (`left,right` → two values), but serializes existing form values losslessly with CSV-style quoting only when required. The decoder is intentionally local rather than `encoding/csv` because Go's CSV reader normalizes CRLF bytes inside quoted fields; w8/m57 explicitly requires preserving CRLF.

Covered by the prepared tests:

- literal comma as one element,
- comma-containing value plus a second element,
- embedded quotes,
- Unicode comma text,
- a single empty string distinct from empty/default list,
- CRLF bytes,
- ordinary multi-term values,
- malformed quoted input errors.

Local upstream-checkout command run in a temporary copy:

```sh
cd /var/folders/q2/zn9w8byj2zv19rgy4kvwg0gm0000gn/T/tmp.IA7GNyarZz/cli
go test ./pkg/command ./pkg/tui/views
```

Result:

```text
ok  github.com/render-oss/cli/pkg/command 0.254s
ok  github.com/render-oss/cli/pkg/tui/views 8.844s
```

No Bex product code, fork, vendor replacement, backend filter, server wire schema, or dashboard code was changed.

## Shared caller / neighboring type verification

Pinned upstream call sites of `command.HuhFormFields`:

1. `pkg/tui/views/logview.go:175` — log filters. `LogInput` string-slice fields: `resources`, `instance`, `text`, `level`, `type`, `host`, `status-code`, `method`, `path`, `task-id`, `task-run-id`.
2. `pkg/tui/views/jobcreate.go:59` — job creation form. `JobCreateInput` fields are scalar strings/pointers; no string slice is present.
3. `pkg/tui/views/workflows/workflowcreate.go:138` — workflow create form. `WorkflowCreateInput` string-slice field: `env-var`.
4. `pkg/tui/views/workflows/taskrun.go:29` — task run form. `TaskRunInput` / task-run form fields are scalar strings/pointers; no string slice is present.

The prepared shared-code tests exercise the actual `FormValuesFromStruct` → `StructFromFormValues` path used by these callers for string slices. The existing `go test ./pkg/tui/views` pass keeps neighboring log-view behavior intact. The decoder branches for string, int, float and bool slices now share the same parser and surface parse errors; enum multi-select still uses `NewStringSliceFormValue` at `pkg/command/form.go:499`, so selected enum values keep using the same list representation.

`NewLogsView` production calls remain the two existing flow sites in `pkg/tui/flows/log.go:68` and `pkg/tui/flows/log.go:87`; deploy/job/workflow log viewers inherit the same log flow. No paid job or unsupported workflow live fixture was created for this preparation step.

## Remaining gate

Tasks t001 and t002 are implementable and prepared. The milestone cannot be marked done because t003–t007 require separately authorized external submission, upstream acceptance/release, then a Bex CLI pin/release and live native receipts. That is the external gate named in the milestone DoD and scope.
