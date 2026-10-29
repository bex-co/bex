# Selecting a relative Logs or Metrics range reuses an old clock endpoint

**Severity:** major. History loads successfully but omits newly available log lines under a newly selected “Last …” label. Metrics independently sends the same stale bounds; no incorrect Metrics value is claimed.

## Ordinary live reproduction

Fixture: Free image web `qa-20261012-a314-clock`, `srv-db6on9pg8k9s73eehlgg`, BusyBox 1.37, port 3000. Its command serves a plain root marker and emits `qa-a314-tick` every five seconds. Public root was 200 before the range tests. No clock, response or application code was replaced.

1. On desktop, load `/services/srv-db6on9pg8k9s73eehlgg/logs?type=app&range=custom&rangeStart=2026-10-12T23%3A00%3A00.000Z&rangeEnd=2026-10-13T01%3A35%3A00.000Z&live=0`. The Custom endpoint predates this fixture, so its 200/zero-row history is correct.
2. Leave the mounted page open. At **01:42:02.692Z**, select **Last 12 hours** through the actual Log history range control. Actual Logs 200 at 01:42:03.673Z asks for **13:40:30.843Z–01:40:30.843Z**, ending **91.849 seconds before selection**. It returns **20 rows, hasMore false**, last marker 01:40:26.43285096Z. The URI correctly becomes `range=12h&type=app&live=0`; old Custom keys are gone.
3. An independent diagnostic GraphQL read at 01:44:29.227Z uses the same resource/type/start/limit, changes end to **01:44:28.629Z**, and returns **68 rows, hasMore false**. **48 markers** are after the UI's stale endpoint. The diagnostic operation is renamed QaA314CurrentLogs solely to distinguish it from UI transports; it does not update Apollo's cache. At 01:44:30.257Z the unchanged UI still holds its 20-row history.
4. Fresh relative reload at **390×844**, 01:46:22.267Z, queries end **01:46:21.061Z** and returns **90 rows, hasMore false**: reload is a passing clock control. Through the ordinary picker set Custom bounds, wait, then select **Last 24 hours at 01:48:09.953Z**. Its first Logs request still ends **01:46:21.061Z**, **128.892 seconds old**, and returns those same 90 rows. Document width remains 390.
5. On that page select **Last 30 minutes at 01:52:28.174Z**. Its first request uses **01:16:21.061Z–01:46:21.061Z**, still the old anchor. Its next ordinary 15-second timer request at 01:52:44.148Z ends **01:52:43.720Z** and includes markers through 01:52:42.081Z. The timer works; the initial selection does not resample. This later response is capped at 100 with hasMore true, so it is not used to infer a total.
6. Freshly mount the same fixture's Metrics page with Custom bounds at 01:55:15.215Z. Select **Last 24 hours at 01:56:45.676Z**. Actual HTTP_REQUESTS Metrics 200 uses start **2026-10-12T01:55:15.483Z**, end **2026-10-13T01:55:15.483Z**, resolution **720**. Its endpoint is **90.193 seconds old**. There is no paused-log switch on this page.

**Expected:** a newly selected relative window ends at the current selection/render time; ordinary timer cadence applies after that first sample. Explicit Custom bounds remain fixed.

**Actual:** range identity and span change immediately, but the endpoint remains the mount/last-tick clock until the next interval. Long presets use 300 seconds (12h), 720 seconds (24h), up to 21600 seconds (30d). Log Live controls a separate stream; turning it off does not turn “Last 24 hours” into a window anchored two minutes ago.

## Durable evidence and controls

[evidence.json](evidence.json) retains exact GraphQL request documents, variables and complete response bodies for Custom history, desktop selection, current-end diagnostic, fresh reload, mobile selection, relative preset change, periodic ticks and the independently observed Metrics request. It also retains actual deletion, full inventory, two 404s and session/browser cleanup.

Supplementary ignored artifacts were checked to exist:

- `.playwright-mcp/qa-range-resume-a314-desktop.png` (130158 bytes), visually inspected: own plain app logs, Last 12 hours, Live off, 20 markers ending at the old endpoint.
- `.playwright-mcp/qa-range-resume-a314-ledger.json`: more complete local pass record.

The 20- and 90-row responses both say hasMore false, ruling out page-cap truncation for those failing requests. Custom URI bounds clear correctly. Relative reload and ordinary tick controls both work. The direct diagnostic query does not mutate the viewer. Initial deploy-start timing, normal Free hibernation and a cleanup dialog that succeeded after settling are not additional findings.

## Root cause, consumer and pinned dependency

- `dashboard/src/features/metrics/hooks/use-live-range.ts:20–40`: `useState(() => new Date())` initializes the clock once. Custom selection stops the timer without updating the clock. Returning to relative only arms an interval; its first callback waits the full resolution. Preset changes likewise reuse `now`, and a span change with equal resolution does not even restart the effect.
- `features/metrics/lib/range.ts:182–191` resolves a relative span against exactly that supplied clock. Its ISO string shape already expresses the correct endpoint; no schema widening is necessary.
- `features/logs/components/log-viewer.tsx:139–145` passes that window into `useLogHistory`, while `JSON.stringify(range)` independently marks a deliberate reader selection. `features/logs/hooks/use-log-history.ts:77–112` memoizes actual query variables from window bounds and uses cache-and-network. The first network request itself is wrong; an Apollo cache flush cannot fix it.
- `routes/services.$serviceId.metrics.tsx:82–96` passes the same window to charts, events and markers. `features/metrics/hooks/use-metrics.ts:147–193` serializes start/end/resolution directly. Its page uses pollIntervalMs 0 because the clock tick owns refresh.
- Installed React/ReactDOM **19.2.3**, read from the actual package. `dashboard/node_modules/react-dom/cjs/react-dom-client.development.js:8264–8294` invokes the state initializer at mount and retains state. `updateReducerImpl:7970–8083` uses retained state and queued updates. `mountMemo/updateMemo:8774–8807` reuse a memo when semantic dependencies match and recompute when they change. The two fresh browser reproductions agree with this mechanism; no deployed-source contradiction was found.

## Concrete proposed fix and pre-settle behavior

Fix the shared hook, preserving its existing return shape. Sample a fresh selection anchor synchronously when mode or relative span/resolution changes. One concrete implementation uses a memoized Date keyed by `custom`, relative `spanSeconds` and `resolutionSeconds`, plus tick state `{anchor, now}`. Use a stored tick only when its **anchor reference** matches the current anchor; otherwise return the newly sampled anchor for the first query. The interval records ticks against its own anchor and cleans up on selection changes/unmount.

Anchor reference identity matters: relative A → Custom → the same A is a new selection even when its scalar preset fields equal the earlier A. Comparing only a reusable preset key could revive the old A tick. Depend on semantic values, not fresh range-object identity, so unrelated rerenders do not continually reset the window. Cover a span-only change with equal resolution as well.

Custom bounds pass through unchanged and schedule no interval. Preserve resolution, one timer, ordinary relative tick cadence, reader identity and the existing distinction between a deliberate range reset and a routine tick retaining loaded history. Preserve paused stream rows, SSE subscription identity and resume behavior.

**Before network settle:** the first returned relative bounds and first emitted query already use the fresh sample. Merely adding `setNow(new Date())` in a passive effect without controlling the first query leaves an incorrect request/old-window flash and is insufficient. Existing pending/ready geometry must remain stable. Late callbacks from an old timer must not become the current selection's clock.

## Exhaustive shared callers and resource placement

Production `rg 'useLiveRange\\('` counts **3 callers**:

| Caller | Policy |
| --- | --- |
| LogViewer, `features/logs/components/log-viewer.tsx:139` | Global hook fix; service log range selection becomes current immediately. Preserve m151 paging/reader continuity and m192 pause/resume/merge. One route mount in services.$serviceId.logs.tsx; NonStaticRoute excludes static-site runtime logs. |
| ServiceMetricsPage, `routes/services.$serviceId.metrics.tsx:82` | Same hook fix for chart/event bounds. Two route mounts: services.$serviceId.metrics and static.$serviceId.metrics. Preserve static pod-metric exclusion. |
| ScalingRecentMetrics, `features/services/components/scaling-recent-metrics.tsx:59` | Fixed 48h/1440s window, no range picker. Existing mount and periodic clock must remain stable on ordinary rerenders. |

Resource families: web live-tested; cron/worker/private share service routes but were not live-probed; static shares Metrics and intentionally has no runtime Logs. Postgres and Key Value use DatastoreLogViewer (two mounts), which memoizes `rangeWindow(range)` at line 170. `features/logs/lib/datastore-log-range.ts:19–27` already resolves a changed relative selection against a new Date; it does not use this hook. That is a source-audited unaffected control, not a claimed live pass.

Aliases: service Logs accepts the existing `r`/type aliases through log-search; both Metrics routes use the canonical range/rangeStart/rangeEnd vocabulary. No new aliases are needed. API shapes stay unchanged: GraphQL Logs/Metrics, REST logs/metrics and MCP list_logs/get_metrics continue honoring explicit caller-supplied bounds. No backend resampling that would move an intentionally historical API window.

## Adjacent behavior, dedupe and limits

Authentication failures, forbidden/not-found, unavailable store, timeout, throttling and malformed range errors retain existing paths. This UI clock fix must not turn any of them into an empty success. No permission or error taxonomy change is proposed.

Whole-board open/blocked/done searches and **90 non-done README scans** found no equivalent selection-clock owner. DO_NOT_DO has no match. Last 40 dashboard/Go commits and targeted hook history show no landed fix: current hook retains the original mount clock plus m56 Custom timer suppression. w5/done/m56's full DoD also includes percentile overlays and cross-API errors, which this hunt did not replay; no full regression claim. w4/done/m151 owns automatic-tick reader continuity, m152 elapsed text, and open m175 backend boundary clipping. Preserve their guarantees without reopening unrelated work.

[Render logging docs](https://render.com/docs/logging) describe relative presets, Custom ranges and live feed separately. [Render Metrics docs](https://render.com/docs/service-metrics) describe selected-range request volume. Exact authenticated Render clock timing was not checked; the acceptance target follows Bex's existing relative-window semantics.

**Unverified:** other service/resource families live; REST/MCP; other metric values; hidden/visible resume; equal-resolution spans; timer races; first-render transport after the proposed change; all prior milestones' full DoDs. These are assigned verification work.

## Cleanup

Actual DeleteService returned 200/true at **02:04:13.295Z**. At **02:04:25.576Z**, service REST and public root both returned **404**, and services/databases/keyValues/projects/envGroups/blueprints/webhookEndpoints exactly matched the recorded pre-pass inventory. No pre-existing resource was modified.

Own session was revoked before the 02:09:12.530Z owners check (**401**); a repeat helper returned **ok already-signed-out**, exit 0. Browser is one about:blank page, 1440×1000, zero cookies, cleared permissions and detached observers. Own jar/handle were removed; shared jar was removed only after its saved stat matched this pass. Full observers were detached before cleanup, so evidence includes individually retained receipts rather than a claim that every network request was captured.
