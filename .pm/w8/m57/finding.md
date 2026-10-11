# Applying unchanged log filters splits one literal comma term into two

Why: a precise hosting log filter silently broadens after Apply and shows lines that do not contain the requested literal text.

**Severity:** minor. **Attribution:** upstream Render CLI, independently reproduced by both native clients. **Date:** 2026-10-11 UTC. **Context:** production `https://api.bex.co/v1/`, isolated human OAuth grant, workspace `bex-canary` / `tea-daif693dqjvc73e7as3g`. Bex v0.3.2; Render v2.27.0, commit `a764810a768202704e7206eb7b87a47211fcd98e`. Source HEAD `2916c1a57300e14c1e063fc7153a4ed2f791d34b`; deployed revision not observed.

## Native reproduction

The fresh Free Node fixture `srv-db5jv0gmmd8c73drribg` reached Live and served its unique public marker. It printed three harmless app logs in the same whole-second window:

```text
QA89_LEFT_4b444e,QA89_RIGHT_4b444e
QA89_LEFT_4b444e only
QA89_RIGHT_4b444e only
```

```sh
bex logs --resources srv-db5jv0gmmd8c73drribg --type app \
  --text '"QA89_LEFT_4b444e,QA89_RIGHT_4b444e"' \
  --start 2026-10-11T07:51:22Z --end 2026-10-11T07:51:25Z \
  --limit 10 --confirm -o interactive
```

Wait for the combined row, press `/`, then Enter without editing any field. Repeat with the actual same-pin `render` binary against Bex in an equivalent separate private config. The extra quotes inside the text argument are intentional: the pinned StringSlice flag accepts one CSV-quoted literal term.

**Expected:** the single literal comma-containing term and its one matching row remain. **Actual in both clients:** initial GET has one decoded `text` value containing the comma and returns HTTP200 with one row. Apply sends two repeated `text` values, returns HTTP200 with three rows, and displays the additional separate lines. Start/end, resource, type, direction and limit remain unchanged. Whole-second bounds exclude w8/082's fractional timestamp defect as a cause.

[native-ui.json](evidence/native-ui.json) retains exact methods, raw paths, parsed queries, safe headers and complete responses. A GET-only loopback forwarder sent unchanged paths/queries to the fixed HTTPS production origin; only authority changed in a private copied config. Authorization was not journaled. Native Bex exited zero after 2.561s and Render after 4.160s following the planned Ctrl+C; both captures submitted the form before the deadline and reaped their processes. PTY output combines stdout/stderr; the direct controls below record separate streams.

[Four direct native controls](evidence/native-controls.json) all exit zero with empty stderr: quoted literal returns one row (Bex1.065s, Render0.427s); explicit two OR terms return three (Bex0.325s, Render0.429s). This confirms the flag parser and server distinguish those requests. Log JSON output is a stream of objects, not a response envelope.

The fixture is deleted. [fixture-cleanup.json](evidence/fixture-cleanup.json) includes its exact create argument vector, startup marker, readiness and cleanup observations. Reproduce with a fresh name/nonce, obtain its three log timestamps and use a whole-second window that contains them; substitute the fresh ID/terms/time bounds above. Native create/delete wire was not captured.

## Producer → form → consumer

In the actual imported pin:

- [`pkg/command/form.go:90–93`](https://github.com/render-oss/cli/blob/a764810a768202704e7206eb7b87a47211fcd98e/pkg/command/form.go#L90) serializes `[]string` into one string with `strings.Join(val, ",")`. It does not escape a comma within an element.
- `arrayFromString` at `:144–150` uses `strings.Split`, and `StructFromFormValues:226–233` reconstructs string slices with it. The original element boundary is unrecoverable.
- `pkg/tui/views/logview.go:175–183` constructs the form and reconstructs every field on Apply; `:246–249` invokes the callback on Enter. `pkg/tui/views/logloader.go:161` forwards the altered `Text` array. The generated `NewListLogsRequest`, `pkg/client/client_gen.go:12743–12749`, emits exploded text parameters from that array.
- Bex `lego/backend/internal/logs/rest.go:506` reads repeated query values directly into `LogQuery.Search`. OR-within-text is the established ADR010 behavior. It correctly returns the broadened set it was sent; this is not a server search bug.

[The offline actual-pin probe](evidence/array-form-probe.go.txt), [module](evidence/array-form-go.mod.txt), [checksums](evidence/array-form-go.sum.txt) and [eight results](evidence/array-form-results.json) call the real form conversion and `NewLogsView` Enter callback with a loader that must never run. Single comma, comma plus another element, embedded quotes, Unicode comma and a one-empty-element list lose their original boundaries. Ordinary multi-term, empty-list and CRLF controls pass. All whole-second time bounds remain exact; network calls are zero. Empty-element behavior is model evidence only, not a separate live finding.

## Concrete fix and shared callers

Correct the shared string-slice producer and decoder together, with an unambiguous form representation that preserves literal commas, quotes, accepted empty values and CRLF. Keep the advertised comma-separated editing behavior and existing plain values usable; verify malformed editing returns a useful error. Merely quoting the producer while retaining `strings.Split`, or replacing the reader with a CRLF-normalizing CSV reader, leaves a regression. Prepare the correction in an upstream checkout; do not fork Bex or coerce its backend filters.

Four production `HuhFormFields` consumers exist: LogsView, JobCreateView, WorkflowCreateView and TaskRunView. Two input structs carry string slices: `LogInput` has11 (resources, instance, text, level, type, host, status-code, method, path, task-id, task-run-id), and `WorkflowCreateInput` has one `EnvVars`. JobCreateInput and TaskRunInput have scalar strings/pointers. Log text and an owned comma display-name resource selector are live confirmed. Shared verification needs its own task: preserve all12 string-slice field contracts, enum multi-select's `NewStringSliceFormValue` construction (`form.go:499`), and the integer/float/bool branches that also call `arrayFromString` (four decoder branches total). `NewLogsView` has two production calls in `pkg/tui/flows/log.go`; deploy/job/workflow log viewers inherit that flow. Do not create paid jobs or exercise unsupported workflows merely to certify this filing.

## Sibling resource-selector reproduction (sweeps90–91)

Creating a name containing a comma is correctly refused with `RESOURCE_NAME_INVALID`; the failed nonce was absent from workspace inventory and no resource was returned. The supported route is to create a DNS-compatible resource name and rename its display name. Fresh Free Node fixture `srv-db5k4h0mmd8c73drrii0` was created as `qa-20261011-2f057a-timeform`, reached Live, served its exact marker, and was renamed by native ID to `qa-20261011-2f057a-timeform,2f057a` (exit0,1.818s). A read-only full-name lookup uniquely resolved that ledger-owned ID.

Both native clients ran `logs --resources '"qa-20261011-2f057a-timeform,2f057a"' --type app --text '"QA91_FORM_2f057a"' --start 2026-10-11T08:03:07Z --end 2026-10-11T08:03:10Z --limit 10 --confirm -o interactive`. Initially the native services query preserves `%2C` within one name and returns the owned service; the following log query returns its exact one marker. After `/` then unchanged Enter, discovery queries use the original-name prefix and separate suffix. The prefix resolves the same owned service under the existing immutable-name filter; the suffix returns empty service, Key Value, Postgres and workflow lists. The UI reports `error processing arguments: no resource found with ID or name '2f057a'`. No second log request is sent. Bex exits0 in3.878s; Render exits0 in3.957s after the planned Ctrl+C. Both observations submit before the deadline and reap their processes.

[name-native-ui.json](evidence/name-native-ui.json) preserves the native command, key events, initial log request/complete response, and all six discovery requests/complete responses per client. Owner email values are explicitly redacted while retaining string types; other nonsecret response fields are preserved. Capture uses the same authority-only GET forwarder. This is the same form `Join`/`Split` boundary, not a regression of m55: the initial complete name already resolves correctly against production. Whole-second bounds avoid082, and only the exact owned service was returned/read for logs.

[name-fixture-cleanup.json](evidence/name-fixture-cleanup.json) retains creation/rename/ownership/runtime proof and deletion. Native deletion exits0 in1.685s; detail and former runtime return404; inventory is six baseline services and quota six used / zero terminating with no datastores. Both temporary capture configs and copied comparison credentials were removed without revoking the ongoing grant. Create/rename/delete wire was not intercepted; direct Kubernetes descendant deletion remains unobserved. This extends m57's acceptance and shared-caller evidence, with no additional implementation bug filed.

## Dedupe, limits and cleanup

Open/blocked/done searches for arrayFromString, literal comma terms, interactive comma/CSV and shared form symbols found no matching issue. w8/m55 fixes server name-filter splitting; w2/done/m167 fixes server multi-term log search; w8/077 concerns the noninteractive StringArray getter's CRLF loss; w8/082 concerns time formatting in the same form. Each has a different producer/fix. The anti-goals and compatibility ledger were checked, and targeted history contains no Bex implementation for this upstream conversion. Do not claim exhaustive upstream dedupe; repeat it before any separately authorized external submission. No external issue, PR or message was sent.

Deletion exited zero in1.476s. API detail and former public runtime return404; the owned ID is absent, all six baseline IDs remain, quota is six used / zero terminating, no datastores. Capture proxies closed, native children were reaped, temporary capture configs and the copied comparison config were removed without logging out the ongoing human grant. Direct Kubernetes descendant deletion was not observed. No product code, commit or push is part of this finding.
