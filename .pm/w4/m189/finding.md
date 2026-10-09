# Service Logs cannot find visible text across ANSI escapes

Why: copying a colored runtime message into Search logs produces a convincing empty result although that exact visible message exists and the deploy view finds it.

**Severity:** major. Live qa-find-bugs cycle31, 2026-10-09 UTC, muse profile, workspace `tea-d98210cbbpdc73dcrkvg`, main `6a84d8baf`. Filing only. The raw API's existing literal-byte match is intentional; the defect is the service dashboard exposing it as a search of the rendered message without the displayed-text behavior already implemented in deploy search.

## Reproduction and target

1. Create an owned Free web image service from `docker.io/library/busybox:1.37`, port 3000. Serve a marker using BusyBox httpd. Emit `printf 'QA_A31_ANSI_\033[31mred\033[0m_end\\n'` before `exec httpd -f -p "$PORT" -h /tmp/qa-www`. This sweep used `qa-20261009-loop-a31-logs` / `srv-db4dm134am4s73f0956g`. A Settings Docker Command edit produced config-change deploy `dep-db4dpgj93q6c73at01pg`; it reached Live. Public GET and external curl returned HTTP 200 with `QA_A31_HTTP_OK\n`.
2. Its Logs page, with Last hour and text `QA_A31_ANSI_`, shows **QA_A31_ANSI_red_end**, with red styled. Replace the text with that complete visible string: **No matching logs / No logs match these filters.** Fresh URL `/services/<id>/logs?text=QA_A31_ANSI_red_end&live=false` reproduces after query settlement. It also failed with Live enabled. Prefix `QA_A31_ANSI_` and word `red` each return the actual colored record. This is not a missing line, narrow time window, indexing delay or pagination limit.
3. Fresh `/services/<id>/deploys/<deploy-id>`: enter the same complete string in Search logs…; the colored record remains as the sole result. Both pages use the same shared rendering parser.
4. **Required target:** service dashboard text search matches case-insensitive literal substrings of the message the log row actually displays, including substrings crossing styling escapes. Preserve the raw message, timestamp, instance, log identity, colors, ordering and authorization. Existing REST/GraphQL/MCP/CLI requests that omit a new explicit display-search option retain their documented raw literal semantics. A fix must not turn the deploy control into another failing raw search or strip stored/returned log bytes.

Captured line: `2026-10-09T12:23:32.155579606Z`, instance `srv-db4dm134am4s73f0956g-4jockpuol2j9hm02r6kg`, raw message `QA_A31_ANSI_\u001b[31mred\u001b[0m_end`. The displayed message has three styled spans and textContent `QA_A31_ANSI_red_end`. The first repeat was 12:25:39Z; another 12:27:04Z. The screenshot's service header was still refreshing; its log query had settled into the recorded empty result. No header-status defect is claimed.

## Cause and concrete repair

- `dashboard/src/features/logs/lib/map.ts:38–52` retains raw `message` and computes parsed spans once. `components/log-line-list.tsx:350` renders those spans. `lib/ansi.ts:226` defines `stripAnsi` as displayed text, applying carriage-return overwrite as well as removing styling. Rendering is correct.
- `dashboard/src/features/logs/components/log-viewer.tsx:138–149` passes the ordinary text to both history and live. `hooks/use-log-history.ts:77` forwards it unchanged to the scalar `logs` argument; `hooks/use-live-logs.ts:61` puts it in the subscribe URL. No display-text matching occurs there.
- `lego/backend/internal/logs/graphql.go:67,154` declares String and makes one Search term. `loki.go:417–429` escapes that literal into a case-insensitive raw-line regex; `service.go:459–468` compares against raw Message on pod/follower paths. An escape inside the needle therefore breaks contiguity. This is the existing API contract, not a Loki corruption bug.
- `dashboard/src/features/deploys/components/deploy-log-panel.tsx:125–134` calls `stripAnsi(l.message)` before client matching. This explains the verified working control; it does not use a different renderer or repaired API.
- Opened the installed **graphql-go/graphql v0.8.1**, pinned in backend/go.mod: `values.go:41–69` walks declared argument definitions; `executor.go:627–651` supplies them to the resolver; `scalars.go:307–325` string coercion preserves the submitted string. The captured scalar input is not split, trimmed or ANSI-normalized by the framework. A new option must be declared and threaded through the schema/document/generated types; changing a resolver lookup alone cannot express it.

**Proposed repair:** add an explicit, optional `textMode=display` Bex extension to the shared log query model and its REST/GraphQL/MCP adapters; omitted/`raw` stays current. Service LogViewer opts in for history and its REST live subscription. Compare against a temporary terminal-display projection compatible with the current UI parser; keep Message untouched for serialization, hashing and rendering. Scope dashboard opt-in to the observed service viewer; do not silently change datastore callers sharing useLogHistory. Reject unsupported option values with the existing bad-request taxonomy.

For stored history, display mode cannot keep the raw literal Loki prefilter: that would discard the record before projection. Use a bounded raw-record scan with display matching, preserving structured/time constraints. Advance coverage by **raw records actually scanned**, including unmatched ones, within the existing 18 s budget. The current `loki.go:123–207` time-slice scanner assumes Loki already filters its pages; merely removing the text predicate and filtering the returned 100 lines would falsely declare older matches absent. Preserve complete timestamp groups, direction, exact next cursors and honest partial/timeout states through `render.go:138–195`. Implement this deliberately in t003, with a >100-record late-match test. A zero-match partially scanned range must expose continuation, not complete No matching logs.

The shared non-store matcher has **five** production call sites: service.go:730,1046,1237 and progress.go:398,484. Audit every one, including fallback/build-progress/followers; normalize only for opted-in searches. The display projection must swallow OSC/other escapes and apply CR overwrite consistently with the current UI, rather than a SGR-only regex that can still match overwritten text. Keep source limits, refusal semantics and ring-buffer/paging guarantees.

## Scope, neighbours and precedent

Census excluding tests: useLogHistory has **two** production callers (service LogViewer and DatastoreLogViewer); useLiveLogs has **two** (service LogViewer and deploy build follower); LogLineList has **three** (service, datastore, deploy). LogViewer has one route mount; DatastoreLogViewer has two, PG/KV. Generic service Logs covers web/private/worker/cron; static has no runtime Logs tab. Only the Free web path was reproduced. Datastore colors/search and worker/private/cron are inferred integration risks, not observed failures.

Entrypoints: `/services/$serviceId/logs`; service deploy detail and static deploy detail host the control component; datastore parent routes host their viewer. REST GET /v1/logs and /v1/logs/subscribe (SSE or WebSocket), GraphQL logs, MCP list_logs share the query model. /v1/logs/values, GraphQL logLabelValues and MCP list_log_label_values are stream-label discovery, not display-text search; their documented line-filter limitation remains. CLI imports no sibling module and its pinned Render binary needs no rewrite.

Before a new selection settles, keep the existing loading state and reject old-reader completions; include mode in reader/cache identities. Empty after complete coverage stays empty; partial coverage remains pageable; upstream error, timeout and unavailable retain their distinct states. Authentication/authorization still gates the resolved resource; foreign/forbidden/missing keep existing opaque errors, and no projection may widen namespace/pod selection. Do not add a client fallback after a denied read.

Dedupe scanned open/blocked/done entries with ANSI/stripAnsi/displayed-text terms, all 73 non-done README titles, DO_NOT_DO, latest 40 dashboard/Go commits and targeted history. No matching work or main fix found. w4/178 is stale filter discovery; w4/m174 is lost fast-exit stdout; neither covers this. The glyph/visibility work in **2826a0b30** already fixed deploy search; this is its service-view residual, not a regression of that deployed control. Complete relevant prior-commit disposition: SGR rendering and deploy displayed search passed here; raw GraphQL bytes passed; dark/light palette, CR overwrite, other escape classes, build/PG/KV viewers and CLI/raw REST/MCP were source-reviewed or untested, not claimed live. w9/m63 and m83's parse-once and virtualization/performance contracts stay intact. w2/m167's raw multi-term/OR contract remains the default; this sweep exercised scalar literal metacharacter/comma/Chinese controls, not the entire multi-term DoD.

[Render documents arbitrary-string log search and separate deploy logs](https://render.com/docs/logging#log-filters). It does not specify how ANSI bytes affect search, and no authenticated Render ANSI comparison was performed. The display option is a labeled Bex extension; Bex's own successful deploy control and existing rendered-message semantics establish the target. Regex search, drains, new transports and retention changes are outside this finding.

## Evidence and cleanup

Viewed and verified local screenshots: `.playwright-mcp/qa-log-search-a31-service.png` and `qa-log-search-a31-deploy.png`. Full safe captures, fixture mutation and baseline comparison are in `.playwright-mcp/qa-logs-a31-ledger.json`. Those files are ignored; the exact API probes below are durable, complete responses for their specified selection. They intentionally establish the existing raw API behavior and the underlying record, not a claim that existing API raw search violates its contract.

Other controls passed: JSON warn→warning, error/info, logfmt DEBUG→debug, unknown discovery, literal comma/Chinese/`.*`, request path +4xx yielding the actual GET404, wrap/timestamp/maximize controls. The presumed download action does not exist; the sweep scope was corrected and no missing-export issue filed. No paid operations.

Owned service deleted through UI at 12:28:17Z; service GET404 and public404 at12:28:58Z. The original service5/database4/KV2/project3/env-group1/Blueprint1 ID sets match exactly, including other workers' qa resources. Only this sweep's Kratos session revoked; old cookie whoami 401 at 12:30:23Z, then cookies cleared. No product changes.

## Durable UI results

These are the complete projected log regions from the fresh-reload repeat and its two controls, captured using `await page.locator('main').ariaSnapshot()` after the respective result settled. They show the actual search strings, Live state and displayed result without depending on the ignored screenshots.

```json
{
  "utc": "2026-10-09T12:25:39.916Z",
  "projection": "Complete log controls and result region from each captured main ARIA snapshot; service/deploy header omitted.",
  "serviceFullVisibleSearch": "  - combobox \"Log type\": All logs\n  - textbox \"Search logs\": QA_A31_ANSI_red_end\n  - combobox \"Log history range\": Last hour\n  - button \"Filters\"\n  - switch \"Live\"\n  - text: Live\n  - paragraph: The range limits history. Live mode appends new lines as they arrive.\n  - paragraph: No matching logs\n  - paragraph: No logs match these filters.",
  "servicePrefixControl": "  - combobox \"Log type\": All logs\n  - textbox \"Search logs\": QA_A31_ANSI_\n  - combobox \"Log history range\": Last hour\n  - button \"Filters\"\n  - switch \"Live\"\n  - text: Live\n  - paragraph: The range limits history. Live mode appends new lines as they arrive.\n  - text: 05:23:32 AM\n  - button \"Filter logs by instance srv-db4dm134am4s73f0956g-4jockpuol2j9hm02r6kg\": \"[4jock]\"\n  - text: QA_A31_ANSI_red_end\n  - paragraph: Live tail paused",
  "deployFullVisibleSearch": "  - button \"Log type\": All logs\n  - textbox \"Search logs…\": QA_A31_ANSI_red_end\n  - button \"Log options\"\n  - button \"Maximize\"\n  - text: 05:23:32 AM\n  - button \"Filter logs by instance srv-db4dm134am4s73f0956g-4jockpuol2j9hm02r6kg\": \"[4jock]\"\n  - text: QA_A31_ANSI_red_end"
}
```

## Exact requests and complete responses

POST `https://api.bex.co/graphql`, signed-in browser cookies. Query and variables are included in each request.

```json
[
  {
    "request": {
      "query": "query QaA31Ansi($resource:String!,$text:String){logs(resource:$resource,type:\"app\",text:$text,limit:100){hasMore nextStartTime nextEndTime logs{timestamp message type instance level method statusCode}}}",
      "variables": {
        "resource": "srv-db4dm134am4s73f0956g",
        "text": "QA_A31_ANSI_red_end"
      }
    },
    "status": 200,
    "response": {
      "data": {
        "logs": {
          "hasMore": false,
          "logs": [],
          "nextEndTime": "2026-10-09T12:24:45Z",
          "nextStartTime": "2026-10-09T11:24:45Z"
        }
      }
    }
  },
  {
    "request": {
      "query": "query QaA31Ansi($resource:String!,$text:String){logs(resource:$resource,type:\"app\",text:$text,limit:100){hasMore nextStartTime nextEndTime logs{timestamp message type instance level method statusCode}}}",
      "variables": {
        "resource": "srv-db4dm134am4s73f0956g",
        "text": "QA_A31_ANSI_"
      }
    },
    "status": 200,
    "response": {
      "data": {
        "logs": {
          "hasMore": false,
          "logs": [
            {
              "instance": "srv-db4dm134am4s73f0956g-4jockpuol2j9hm02r6kg",
              "level": "unknown",
              "message": "QA_A31_ANSI_\u001b[31mred\u001b[0m_end",
              "method": "",
              "statusCode": "",
              "timestamp": "2026-10-09T12:23:32.155579606Z",
              "type": "app"
            }
          ],
          "nextEndTime": "2026-10-09T12:23:32.155579605Z",
          "nextStartTime": "2026-10-09T11:24:46.725153953Z"
        }
      }
    }
  },
  {
    "request": {
      "query": "query QaA31Ansi($resource:String!,$text:String){logs(resource:$resource,type:\"app\",text:$text,limit:100){hasMore nextStartTime nextEndTime logs{timestamp message type instance level method statusCode}}}",
      "variables": {
        "resource": "srv-db4dm134am4s73f0956g",
        "text": "red"
      }
    },
    "status": 200,
    "response": {
      "data": {
        "logs": {
          "hasMore": false,
          "logs": [
            {
              "instance": "srv-db4dm134am4s73f0956g-4jockpuol2j9hm02r6kg",
              "level": "unknown",
              "message": "QA_A31_ANSI_\u001b[31mred\u001b[0m_end",
              "method": "",
              "statusCode": "",
              "timestamp": "2026-10-09T12:23:32.155579606Z",
              "type": "app"
            }
          ],
          "nextEndTime": "2026-10-09T12:23:32.155579605Z",
          "nextStartTime": "2026-10-09T11:24:48.355256178Z"
        }
      }
    }
  }
]
```
