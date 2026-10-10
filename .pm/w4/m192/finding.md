# Cycle34 — paused rows disappear; live replay is appended out of time order

Production functional QA on **2026-10-10 UTC** (2026-10-09 America/Los_Angeles), signed in through the private `muse.env` handoff. Workspace `tea-d98210cbbpdc73dcrkvg`; desktop 1440 × 1000. **Two minor bugs.** This schedules fixes only.

## Repeatable fixture and passing redeploy

Create a Free web service from Existing Image `docker.io/library/busybox:1.37`, port 3000, one instance, name `qa-20261010-loop-a34-tail`, no project/environment/group/disk/pre-deploy command. Docker Command:

```sh
mkdir -p /tmp/qa-www; echo QA_A34_HTTP_OK > /tmp/qa-www/index.html; (n=0; while true; do n=$((n+1)); echo "QA_A34_TICK n=$n"; sleep 2; done) & echo QA_A34_START; exec httpd -f -p "$PORT" -h /tmp/qa-www
```

Recorded service `srv-db4tlpb93q6c73at060g`. Initial deploy `dep-db4tlpb93q6c73at0610` became Live at 06:28:13.604342Z, revision 1. Public curl returned HTTP 200 at 06:29:20Z. Keep Logs open and use another tab → Manual Deploy → Deploy latest image. `dep-db4tmmvd661c73bf5gj0` became Live at 06:30:10.645697Z, revision 2. The original Live viewer received old and new instance output without navigation or a disconnected banner. This **passed**, so no tail reattachment defect is filed.

The new instance is `srv-db4tlpb93q6c73at060g-e1aqva71q9k28nc68of0`; its START timestamp is 06:29:49.234399544Z. Select Application logs, Last hour.

## 1. Turning Live off removes lines already rendered

**Severity:** minor. Inspection is interrupted; no permanent backend log loss is established. **Implementation estimate:** 45m plus shared checks.

1. Reload `/services/srv-db4tlpb93q6c73at060g/logs?type=app&live=0`.
2. Turn Live on; wait for at least two tick markers newer than the retained history head. Read the latest rows.
3. Turn Live off; scroll to the bottom again. Those already visible recent markers disappear immediately, although the label now says “Live tail paused.”

At 06:33:20.686Z the before view ended at n=106 (06:33:19Z); after pause it ended at n=95 (06:32:57Z), with n=99–106 absent after explicitly scrolling to the bottom. Fresh-page repeat at 06:34:47.788Z started with historyMax=148, displayed streamed n=149 and n=150, then removed both on pause and ended at 148. A third fresh-page repeat at 06:41:40.473Z had historyMax=354, displayed n=355/356, and removed both on pause, ending at 354.

**Expected:** pausing closes the live transport while retaining the current reader's already displayed matching rows, subject to its existing cap and selected-window expiry. **Actual:** changing enabled erases the live source from the combined view. This is not a promise to freeze automatic history refresh or persist in-memory rows through a page reload.

**Hard negative control:** pausing before any streamed frame was flushed retained the historical markers n=58–65. The issue specifically requires stream-only displayed rows; history is not universally erased.

**Source:** `dashboard/src/features/logs/hooks/use-live-logs.ts:145–155` includes enabled in subKey and clears the buffer during render when that key changes. `log-viewer.tsx:144–161` supplies `live && liveSupported` and derives rows from history plus that buffer. `use-log-history.ts:98–153` retains its same-reader head independently; it need not yet include a just streamed frame. Cleanup flushes use the old subscription key, so merely relying on cleanup cannot restore a buffer whose key has been reset.

**Target fix:** distinguish a user pause from a reader/filter/resource change. Preserve already displayed current-reader rows through pause/resume, close/reopen the transport, dedupe replay, retain the 5,000-line bound and 100ms batching, and reject a stale effect's final batch after a genuine reader change. Do not simply remove enabled from subKey: enabling a store-only level/method/status/path filter also disables this stream, and retained unfiltered lines must never be displayed as matching that new query. Keep error/refusal/unsupported-filter states explicit. Settle state retains the same reader before and after the next historical response; a new reader discards incompatible data before paint.

### Durable probe: the erased row exists

Authenticated browser GraphQL POST `https://api.bex.co/graphql`, HTTP **200**, at 2026-10-10T06:36:22.662Z. Exact request and complete selected response:

```json
{
  "operationName": "QaA34StoredMarker",
  "query": "query QaA34StoredMarker($serviceId:String!){logs(resource:$serviceId,startTime:\"2026-10-10T06:34:40Z\",endTime:\"2026-10-10T06:34:50Z\",limit:20,text:\"QA_A34_TICK n=150\",type:\"app\"){logs{timestamp message instance type level} hasMore nextStartTime nextEndTime}}",
  "variables": {
    "serviceId": "srv-db4tlpb93q6c73at060g"
  }
}
```

```json
{
  "data": {
    "logs": {
      "hasMore": false,
      "logs": [
        {
          "instance": "srv-db4tlpb93q6c73at060g-e1aqva71q9k28nc68of0",
          "level": "unknown",
          "message": "QA_A34_TICK n=150",
          "timestamp": "2026-10-10T06:34:47.490311139Z",
          "type": "app"
        }
      ],
      "nextEndTime": "2026-10-10T06:34:47.490311138Z",
      "nextStartTime": "2026-10-10T06:34:40Z"
    }
  }
}
```

Fresh-page SSE observation: request `GET https://api.bex.co/v1/logs/subscribe?resource=srv-db4tlpb93q6c73at060g&type=app&startTime=2026-10-10T05%3A34%3A45.757Z`, opened 06:34:46.816Z, zero observed errors. Frame received at 06:34:47.548Z, `id: 2026-10-10T06:34:47.490311139Z`. Complete data frame:

```json
{
  "id": "srv-db4tlpb93q6c73at060g-e1aqva71q9k28nc68of0-2026-10-10T06:34:47.490311139Z-3e989759",
  "message": "QA_A34_TICK n=150",
  "timestamp": "2026-10-10T06:34:47.490311139Z",
  "labels": [
    {
      "name": "type",
      "value": "app"
    },
    {
      "name": "resource",
      "value": "srv-db4tlpb93q6c73at060g"
    },
    {
      "name": "instance",
      "value": "srv-db4tlpb93q6c73at060g-e1aqva71q9k28nc68of0"
    },
    {
      "name": "container",
      "value": "app"
    }
  ]
}
```

A read-only EventSource observer in the QA tab captured this actual frame; it preserved native transport behavior. It did not manufacture the marker or modify product state.

REST read at 2026-10-10T06:43:58.020Z, HTTP **200**. Exact GET URL:

```text
https://api.bex.co/v1/logs?resource=srv-db4tlpb93q6c73at060g&startTime=2026-10-10T06:34:40Z&endTime=2026-10-10T06:34:50Z&limit=20&text=QA_A34_TICK%20n%3D150&type=app
```

Complete response:

```json
{
  "hasMore": false,
  "nextStartTime": "2026-10-10T06:34:40Z",
  "nextEndTime": "2026-10-10T06:34:47.490311138Z",
  "logs": [
    {
      "id": "srv-db4tlpb93q6c73at060g-e1aqva71q9k28nc68of0-2026-10-10T06:34:47.490311139Z-3e989759",
      "message": "QA_A34_TICK n=150",
      "timestamp": "2026-10-10T06:34:47.490311139Z",
      "labels": [
        {
          "name": "type",
          "value": "app"
        },
        {
          "name": "resource",
          "value": "srv-db4tlpb93q6c73at060g"
        },
        {
          "name": "instance",
          "value": "srv-db4tlpb93q6c73at060g-e1aqva71q9k28nc68of0"
        },
        {
          "name": "container",
          "value": "app"
        },
        {
          "name": "level",
          "value": "unknown"
        }
      ]
    }
  ]
}
```

MCP POST `https://api.bex.co/mcp`, Accept `application/json, text/event-stream`, HTTP **200**. Exact request and complete JSON message decoded from the returned SSE `event: message` (including content and structuredContent):

```json
{
  "jsonrpc": "2.0",
  "id": 34,
  "method": "tools/call",
  "params": {
    "name": "list_logs",
    "arguments": {
      "resource": ["srv-db4tlpb93q6c73at060g"],
      "startTime": "2026-10-10T06:34:40Z",
      "endTime": "2026-10-10T06:34:50Z",
      "limit": 20,
      "text": ["QA_A34_TICK n=150"],
      "type": ["app"]
    }
  }
}
```

```json
{
  "jsonrpc": "2.0",
  "id": 34,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "{\"hasMore\":false,\"logs\":[{\"labels\":{\"container\":\"app\",\"instance\":\"srv-db4tlpb93q6c73at060g-e1aqva71q9k28nc68of0\",\"level\":\"unknown\",\"service\":\"srv-db4tlpb93q6c73at060g\",\"type\":\"app\"},\"message\":\"QA_A34_TICK n=150\",\"timestamp\":\"2026-10-10T06:34:47.490311139Z\"}],\"nextEndTime\":\"2026-10-10T06:34:47.490311138Z\",\"nextStartTime\":\"2026-10-10T06:34:40Z\"}"
      }
    ],
    "structuredContent": {
      "hasMore": false,
      "logs": [
        {
          "labels": {
            "container": "app",
            "instance": "srv-db4tlpb93q6c73at060g-e1aqva71q9k28nc68of0",
            "level": "unknown",
            "service": "srv-db4tlpb93q6c73at060g",
            "type": "app"
          },
          "message": "QA_A34_TICK n=150",
          "timestamp": "2026-10-10T06:34:47.490311139Z"
        }
      ],
      "nextEndTime": "2026-10-10T06:34:47.490311138Z",
      "nextStartTime": "2026-10-10T06:34:40Z"
    }
  }
}
```

**Capture anomaly resolved:** SSE has no level label here; REST adds level=unknown and MCP uses labels.service instead of REST's resource. GraphQL projects instance/type/level separately. All retain the same marker and nanosecond timestamp; these shapes do not explain the UI deletion. No API schema change is proposed.

## 2. Enabling Live appends older replay below newer history

**Severity:** minor. Timestamps visibly run backwards; data remains accessible. **Implementation estimate:** 35m plus shared checks.

1. Allow this fixture to produce >100 lines, reload paused Application logs / Last hour, then enable Live.
2. Scroll up within the log viewport to the boundary after the historical head. The stream legitimately replays earlier in-window pod output; the display concatenates it after the newer head instead of merging by time.
3. Repeat from a fresh reload. At 06:40:15.588Z, DOM row 99 was n=310 / 11:40:07 PM, followed by row 100 START / 11:29:49 PM. At 06:42:31.012Z, actual visible adjacent rows 114 and 115 were **n=369 / 11:42:05 PM → START / 11:29:49 PM**, followed by n=1,2,3. This backwards jump was captured in a viewed screenshot.

**Expected:** the combined reader orders valid timestamps chronologically, newest at the bottom, while each logical record appears once. **Actual:** replay arrival order is appended after history even when it precedes that history. The two sources need not arrive in a globally sorted sequence.

**Source:** `dashboard/src/features/logs/lib/map.ts:135–136`, `mergeLogLines`, only deduplicates `[...history, ...live]`; it never sorts. Its sole production caller is `log-viewer.tsx:161`. `log-line-list.tsx:129–139,229–253` hands array index and key to the virtualizer and renders `lines[virtualRow.index]`; there is no downstream timestamp sort. The virtualizer was inspected at lockfile versions **@tanstack/react-virtual 3.14.9 / virtual-core 3.17.7**. Core `src/index.ts:1634–1651` returns indexed measurements in the selected range; it does not reorder application data. The defect is the client merge, not a library sort or the backend timestamp comparator.

**Target fix:** merge and dedupe into chronological display order without mutating either source array, changing wire timestamps/IDs or modifying backend cursor/order semantics. Define deterministic handling for equal/invalid/missing timestamps and preserve existing identity normalization. Avoid lexical RFC3339Nano ordering; test whole-second/trailing-zero precision and ordering within a millisecond where distinguishable records exist. Preserve stable viewport anchors, bounded rendered DOM, follow/jump-to-latest and loaded older pages. Do not discard valid older replay just to conceal the ordering defect.

### Durable chronological-history control

Authenticated GraphQL POST, HTTP **200**, at 2026-10-10T06:42:31.012Z. Exact request and complete selected response (the last five matching lines are already chronological):

```json
{
  "operationName": "QaA34Chronology",
  "query": "query QaA34Chronology($serviceId:String!){logs(resource:$serviceId,startTime:\"2026-10-10T06:29:40Z\",endTime:\"2026-10-10T06:42:10Z\",limit:5,type:\"app\"){logs{timestamp message instance type level} hasMore nextStartTime nextEndTime}}",
  "variables": {
    "serviceId": "srv-db4tlpb93q6c73at060g"
  }
}
```

```json
{
  "data": {
    "logs": {
      "hasMore": true,
      "logs": [
        {
          "instance": "srv-db4tlpb93q6c73at060g-e1aqva71q9k28nc68of0",
          "level": "unknown",
          "message": "QA_A34_TICK n=367",
          "timestamp": "2026-10-10T06:42:01.8591382Z",
          "type": "app"
        },
        {
          "instance": "srv-db4tlpb93q6c73at060g-e1aqva71q9k28nc68of0",
          "level": "unknown",
          "message": "QA_A34_TICK n=368",
          "timestamp": "2026-10-10T06:42:03.861865198Z",
          "type": "app"
        },
        {
          "instance": "srv-db4tlpb93q6c73at060g-e1aqva71q9k28nc68of0",
          "level": "unknown",
          "message": "QA_A34_TICK n=369",
          "timestamp": "2026-10-10T06:42:05.862437817Z",
          "type": "app"
        },
        {
          "instance": "srv-db4tlpb93q6c73at060g-e1aqva71q9k28nc68of0",
          "level": "unknown",
          "message": "QA_A34_TICK n=370",
          "timestamp": "2026-10-10T06:42:07.864176178Z",
          "type": "app"
        },
        {
          "instance": "srv-db4tlpb93q6c73at060g-e1aqva71q9k28nc68of0",
          "level": "unknown",
          "message": "QA_A34_TICK n=371",
          "timestamp": "2026-10-10T06:42:09.866093713Z",
          "type": "app"
        }
      ],
      "nextEndTime": "2026-10-10T06:42:01.859138199Z",
      "nextStartTime": "2026-10-10T06:29:40Z"
    }
  }
}
```

The UI showed n=369 immediately followed by the new instance's older START, while this direct API response orders n=367–371 correctly. The transport's lower-bound replay is compatible with Last hour; no “start only at now” server change is needed.

## Shared scope, aliases and pre-settle behavior

Exhaustive production-source grep (excluding tests): **2 useLiveLogs call sites** — LogViewer (service Logs) and useDeployLogs (live build). **1 mergeLogLines call site** — LogViewer. **3 LogLineList consumers** — service Logs, DeployLogPanel, DatastoreLogViewer. A sorting change should stay in the combined service merge unless a separately reproduced sibling gap requires more. A hook pause change must deliberately cover or allowlist both live callers.

| Family | Entry point / behavior | Evidence this run |
| --- | --- | --- |
| Web | /services/$serviceId/logs; general history + live; deploy detail also uses live build | Free image web reproduced; redeploy tail passed |
| Private | same services Logs and deploy-detail route family | Source traced; live unverified |
| Worker | same services Logs and deploy-detail route family | Source traced; live unverified |
| Cron | same services Logs; scheduled-run followers share backend tail | Source traced; scheduled runs unverified |
| Static | no runtime Logs tab; /static/$serviceId/deploys/$deployId and service deploy-detail alias use DeployDetailPage | Source traced; live build unverified |
| Postgres | /databases/$databaseId Logs mounts DatastoreLogViewer, history-only | Source traced; live unverified |
| Key Value | /keyvalue/$keyValueId Logs mounts same datastore viewer, history-only | Source traced; live unverified |

`useDeployLogs` has **one** production caller, DeployLogPanel; that panel has **one** mount, DeployDetailPage, with **two** route mounts (services and static). Its separate history/live sort is at `use-deploy-logs.ts:201,261`; it does not call mergeLogLines. Its completed-build followBuild=false transition needs a regression check if changing the common hook, but no live finished-build loss claim is made.

`LogLine` stores a raw string timestamp and client key; no type/schema invention is needed. Read `types.ts:9–29` and `map.ts:25–31` before altering equal-time handling. Preserve the current canonical-message and cross-source dedupe contract. Request logs and level/method/status/path use store-only behavior; instance and text may have a live source. On resource/filter/range changes, pending cached rows and late stream frames must respect the new reader immediately; on a user pause of the unchanged reader, those same matching rows remain.

Apollo **4.1.3**, pinned in yarn.lock and installed package, was inspected at `core/QueryManager.js:1068–1077`: cache-and-network emits the cache result and then the link result when the cache is complete. This supports pending-state checks; it is not the cause of the two defects. The retained head key excludes the Live toggle.

Adjacent normal product states: genuine no-match stays empty; missing/store-unavailable/refusal/errors keep their current presentation; disconnected/retrying can retain same-reader rows; explicit reader change rejects old rows and batches. These fixes do not change API errors or access decisions.

## Dedupe and previous guarantees

Searched open and completed board files for pause/retention/ordering/mergeLogLines, scanned **55** open/blocked milestone READMEs plus inbox notes, re-read DO_NOT_DO, reviewed the last 40 dashboard/lego commits and targeted symbol history, then pulled latest main `4c21f0097`. Incoming changes concern CLI filters/secrets and deployment pins; neither implicated dashboard implementation changed. No undeployed fix or open equivalent exists. Open w4/m189 changes visible-text matching; blocked w4/180 changes deploy predeploy-source membership. They are different causes.

These are **uncovered behavior gaps**, not a claim that a recent fix regressed. The initial live-log implementation already reset on pause and appended on merge; the existing hook test explicitly expects a disable/enable buffer reset. Replace that implementation-shaped assertion with reader behavior and stale-filter controls.

Complete relevant prior DoD disposition:

| Prior item | Every original DoD clause and disposition |
| --- | --- |
| w5/done/m6 | Historical API shape/text narrowing remains; Live delivery/redeploy passed, while retention on user pause is the newly isolated gap; Render layout/transport divergence unchanged; error/disconnect handling unaltered and requires regression; store-only filter gate must stay honest; its lint/typecheck/test/build gates and live screenshot must be rerun for the future fix |
| w4/done/m96 | One cron record per original pod/timestamp remains a required dedupe control, scheduled/manual/cancel lifecycle is unprobed here; historical/live message bytes must remain canonical, spaces/escaped newlines untouched; owned-resource removal passed. That milestone fixed message identity, not time order or pause retention |
| w4/done/m151 | Relative 100+40 reader continuity/anchor, fresh pending and explicit resets, Custom/7d paging, narrow/no-match/URL restore, GraphQL/REST/MCP cursor/cap and Live append, completed >100-line build paging, cleanup all stay required; this run specifically reproduces Live pause and replay order, not clock-driven page loss. Shared live/paging/build controls are t003/t006 work |
| w9/done/m83 | Visible-range-only DOM and bounded busy-tail performance remain; follow/scroll-up/jump, chips and wrap modes need regression; visible-row selection tradeoff unchanged; profile and 5,000 cap preserved; full dashboard gates must pass |
| w9/done/m63 | ANSI-at-ingest and smooth busy tail remain; Metrics shimmer/timeline outside this fix; Logs/Scaling pending geometry must not change; follow/selection/instance behavior stays; full suite remains required |
| w1/done/m146 | No reconnect loop/open SSE and NDJSON/next cron output guarantees are not alleged broken; web redeploy/new-instance delivery passed; idle cron and terminal deletion/refusal cases were not re-probed. Keep backend keepalive/relist/restart logic unchanged |
| w6/done/m93 | Postgres connection-string control is unrelated/unprobed; cron duplicate count is not alleged and same-record dedupe remains; REST/GraphQL parity stays. Its investigated lower-bound resume fix is not client chronological merging or retention of rows already painted |
| w8/done/044 | Parsed backend chronological comparator and whole-second/trailing-zero fixture must stay; API control above passed, wire/cursors unchanged; server tests/gates stay. This distinct client concatenation defect does not reopen that server fix |

DO_NOT_DO has no conflict: this repairs existing hosting logs, with no new service product, transport, paid fixture or API feature.

## Render and limits

[Render's current logging documentation](https://render.com/docs/logging), read 2026-10-10, documents runtime search, time ranges, Live tail, per-instance filtering and a separate live-clear shortcut. It does **not** specify this Bex toggle's pause-retention implementation. That target follows Bex's “pause” control and previously visible-reader behavior; exact authenticated Render pause behavior remains unverified. Keep the documented Bex SSE transport choice.

Unverified: mobile geometry, busy 5,000-line performance, filter/refusal/disconnect races, equal/invalid/submillisecond timestamps, finished source build, explicit range changes while paused, older-page loading with live replay, non-web families. These are tasks to verify, not observed failures.

## Evidence and cleanup

Viewed, confirmed-present screenshots:

- `.playwright-mcp/qa-logs-a34-pause-before-verified.png` — n=355/356 visible, Live on.
- `.playwright-mcp/qa-logs-a34-pause-after-verified.png` — Live off, bottom ends at n=354.
- `.playwright-mcp/qa-logs-a34-order-visible.png` — visible n=369 then earlier START.

`.playwright-mcp/qa-live-logs-a34-ledger.json` retains requests, responses, fresh DOM captures and cleanup; the screenshots are ignored local aids, while complete rerunnable API probes are preserved above. Older draft screenshots did not always show the virtualized boundary; the three paths above were visually checked against their claims. The displayed before-pause old replay rows plus newest frames are predicted by finding 2, not evidence of missing store data.

Console errors/warnings after the journey: **0**. Captured functional requests return 200. The broad MCP network log includes pre-login expired-session 401s and navigation-aborted requests from browser history; neither is filed.

Deleted only `srv-db4tlpb93q6c73at060g` through Settings/sudo confirmation at **06:44:24.457Z**, DeleteService HTTP 200/`true`; REST lookup and public host both **404**. Exact ID sets returned to the baseline across services (5), databases (4), Key Value (2), projects (3), env groups (1), blueprints (1). Pre-existing QA resources belong to other runs and were left intact. This run's isolated session logout succeeded; the old cookie got **401** at 06:44:59.448Z; cookies 7→0, owned control tab closed, remaining tab about:blank, jar/state removed.
