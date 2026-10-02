# Relative log-range refresh drops loaded history and resets the reader

**Severity:** major. **Estimate:** about 2h 45m across implementation, shared-caller review and closing checks. **Source:** continuous `$qa-find-bugs`, production sweep 12, 2026-10-02 UTC, `muse.env` credentials, filed to w4 by user request. This is a dashboard loss of loaded reading context; the retained server logs are intact.

## Reproduction and control

Own free web fixture `qa-20261002-logs-r12`, service `srv-davosamde41s73canun0`, deploy `dep-davosamde41s73canung`, workspace `tea-d98210cbbpdc73dcrkvg`. Created from public `https://github.com/bex-co/bex`, branch main, root `examples/hello-go`, Go runtime, build `go build -o app .`, start `./app`, port 3000, auto-deploy off. The first deployment reached Live at 11:00:26Z and the public URL independently returned HTTP 200 / `OK`. The source revision was `88a5245de`.

The example logs each requested path. Between 11:02:25.992Z and 11:04:00.371Z, sequential public GETs to `/qa-r12/alpha/001` through `/qa-r12/alpha/140`, with 150 ms between completed requests, all returned 200 / `OK`. Actual rate was about 1.5 requests/second. No code, secret, plan or production platform setting was changed.

1. Open `/services/<id>/logs?type=app&text=%2Fqa-r12%2Falpha%2F&live=0`. The default range is Last hour; Application logs and the distinctive text exclude unrelated/background rows. Wait for the first response.
2. The page accurately says it shows the newest 100 matching lines and offers **Load older**. Click it: the API returns the earlier 40, `hasMore:false`, and the notice disappears.
3. After layout settles, scroll within the pane to read an older marker, e.g. `alpha/009` (`scrollTop=200` at the tested desktop width). Do not change a filter, range or Live setting.
4. Cross the next 30-second range tick. The pane disappears into a skeleton, reappears with only the newest 100, returns to the bottom and offers Load older again. The missing 40 lines are still only minutes old, comfortably inside the selected hour.
5. Reload the page and repeat. Both independent page loads reproduced the loss.
6. Select **Custom…** via the range picker, Start `2026-10-02T04:00`, End `2026-10-02T04:06` in the browser's America/Los_Angeles zone (11:00–11:06Z). Wait for the new query to settle, load the same older 40, and read the same marker. All 140 lines and the exact scroll position remain across the comparable idle period.

| UTC | Observation |
| --- | --- |
| 11:04:19.724 | Fresh first page: 100 lines, alpha/041–140, hasMore true. |
| 11:04:20.341 | Cursor page: 40 lines, alpha/001–040, hasMore false. |
| 11:04:40.855 | Reading older history, Live false, no Load older, scrollTop 200, scrollHeight 3363, viewport 518. |
| 11:04:49.610 | Automatic query moves bounds forward 30 seconds and returns the same alpha/041–140 head. |
| 11:05:09.483 | No intervening interaction: scrollTop 1889, scrollHeight 2407, Load older back. |
| 11:05:39.880–11:05:41.357 | Second fresh page; load 100+40 and read older history at scrollTop 200. |
| 11:06:09.893 | MutationObserver sees no log viewport and 16 skeleton elements. |
| 11:06:10.552 | Viewport remounts with the 100-line head. |
| 11:06:10.843 | Remounted pane scrolls to 1889, the bottom. |
| 11:08:19.128–11:09:16.495 | Settled Custom control: scrollTop remains 200, scrollHeight 3363, no Load older, no new Logs request. |

The list is virtualized. DOM overscan starts at alpha/001 before and alpha/107 after; the screenshots' actually visible top rows are approximately alpha/009 and alpha/119. These are not counts of rendered DOM nodes. The 140→100 count comes from complete API pages, the reappearing cap notice and the retained-content height. The fixed-range screenshots are pixel-identical. A premature Load older click during the range-switch transition fetched the old range and was correctly discarded; it is excluded from the settled control.

**Expected:** a routine relative-range refresh preserves already loaded, still-in-window lines, backward cursor progress, and the reader's position. It must not replace a populated reader with first-load skeletons. A deliberate resource/filter/range selection may reset the query; a clock tick within the same selection must not.

## Root cause and actual dependency mechanism

- `dashboard/src/features/logs/components/log-viewer.tsx:136–137` calls `useLiveRange(range)` regardless of the Live switch. Live controls the separate SSE hook at 138–151, not this clock.
- `features/metrics/hooks/use-live-range.ts:24–40` changes `now` on the preset resolution and resolves new start/end strings. `features/metrics/lib/range.ts:17–24` makes Last hour tick every 30 seconds and Last 30 minutes every 15 seconds. Custom absolute bounds take the no-timer branch. This explains the positive control without relying on a different server.
- `features/logs/hooks/use-log-history.ts:76–101` includes both effective bounds in the memoized variables. `use-older-log-pages.ts:53–59` treats any new variables object as a new reader: it increments the generation, clears `pagedBack`, drops `older` and resets loading. It cannot distinguish a user's selection change from the preset's clock.
- The consumer compounds the drop: `use-log-history.ts:104–119` reads only current `data`, maps missing data to no first page, then merges that with the now-cleared older pages. `log-viewer.tsx:191–192` renders `LogPanelSkeleton` when history is loading with no lines. The actual 659 ms skeleton interval was observed, not inferred.
- Remounting `LogLineList` loses its unpinned state: `log-line-list.tsx:109` initializes `pinned=true`, and 138–143 / 178–180 scroll the last line into view. Retaining only the older array would still leave a first-page/viewport transition problem; both data continuity and mounted-reader continuity matter.
- **Pinned libraries were opened:** Apollo Client 4.1.3, React/react-dom 19.2.3, `@tanstack/react-virtual` 3.14.9 with virtual-core 3.17.7, all matching `dashboard/yarn.lock` and installed package metadata. Captured dashboard GraphQL requests also declare Apollo 4.1.3.
  - React-dom `cjs/react-dom-client.development.js:7601–7620,8641–8655` compares effect dependencies with object identity and marks changed dependencies for execution; the new variables object reruns the reset effect.
  - Apollo `react/hooks/useQuery.js:153–192` synchronously reobserves changed watch options and exposes previous data separately. `core/ObservableQuery.js:958–984` classifies changed variables; `1287–1290` uses a fresh initial result for a different query/variables identity. `215–253` reads the new cache key, returns undefined for incomplete data, and marks cache-and-network loading. `core/QueryManager.js:1068–1076` takes the network path for that missing entry. `common/apollo/cache.ts:6–12` has no Logs merge/key policy that would join these different windows.
  - `@tanstack/react-virtual/dist/esm/index.js:87–103` creates its Virtualizer in component state and updates its options. The app supplies `count: lines.length`, keyed rows and the unpinned state; virtualization is not deleting retained server lines. The observed component removal explains why its otherwise useful scroll-preservation behavior cannot apply.

**Type/layer check:** `LogsQueryVariables` and the nullable envelope in `graphql/definitions.ts:985–1000` can retain valid prior rows locally without a schema change. `LogFilters` (`features/logs/types.ts:76–84`) deliberately excludes the range; `RangeSelection` in `features/metrics/lib/range.ts:43–51` is the preset/custom semantic identity. A paging reset identity must include resource, actual filters and explicit selected range, while still sending the current effective time bounds to the API. Keep mapping/deduplication on the existing timestamp/instance/message key (`features/logs/lib/map.ts:25–31,79–85`).

## Fix contract and shared scope

Keep this fix local to log-reader state. Preserve loaded lines still inside the active query window, the backward continuation and a stable visible anchor during automatic head refresh, including the pre-response state. Refresh the head normally; do not freeze the shared metrics clock globally to hide the symptom. Expired rows may leave the moving range, but this fixture's three-minute-old rows cannot. Retain last good same-reader content during a transient background failure with an honest error indication; a genuine selection/resource change must discard old rows and reject its stale in-flight completions before showing another resource's data.

A minimal implementation can explicitly separate the reader's reset identity from effective transport bounds and retain the last usable same-reader head during refresh. Do not blindly use Apollo previousData across resource/filter changes, remove all reset dependencies, grow an unbounded cache, or change the 100-row backend cap. Continue to merge/dedupe live arrivals, preserve the stream's lower-bound/resume contract and keep explicit fixed windows fixed. Actual window rollover needs a bounded retention policy and cursor clamping, rather than retaining expired rows forever.

Exhaustive production-code grep, excluding tests, found:

| Shared code | Exact callers / mounts | Disposition |
| --- | --- | --- |
| `useLiveRange` | 3 call sites: LogViewer; `services.$serviceId.metrics.tsx:81`; `scaling-recent-metrics.tsx:47` | Fix the log-reader interaction; preserve both metrics clocks and custom-range behavior. |
| `useLogHistory` | 2 call sites: LogViewer:137 and DatastoreLogViewer:66 | Any history-hook policy must distinguish these callers; no global automatic reset removal. |
| `useOlderLogPages` | 2 call sites: use-log-history:110 and use-deploy-logs:90 | Shared change needs explicit reset semantics and regression controls for both. |
| `LogViewer` | 1 mount: `routes/services.$serviceId.logs.tsx:116` | Non-static service family described below. |
| `DatastoreLogViewer` | 2 mounts: `routes/databases.$databaseId.tsx:227`, `routes/keyvalue.$keyValueId.tsx:195` | Its `useMemo(rangeWindow(range), [range])` at 59 does not use the automatic clock. Current fixed-window paging must keep working. |
| Typed deploy paging | 1 helper `useTypedDeployLogs`, invoked 3 times at `use-deploy-logs.ts:150–157` for build/predeploy/app; one `useDeployLogs` consumer in DeployLogPanel:116 | Polls a stable deployment window. The finished-build positive control retained its 23 older build rows across same-window first-page refetches. Preserve isolation when the deploy/window really changes. |
| Deploy panel routes | 1 DeployLogPanel mount in DeployDetailPage:97; 2 route mounts under /services and /static | Verify both route families if changing shared paging. |

Seven resource families: **web** was reproduced live; **cron, background worker and private service** share the non-static Logs route by code (`NonStaticRoute:33–42`) and need verification, not claims of a separate live failure. **Static sites** have no runtime Logs nav (`service-nav.tsx:83–96`); their build detail remains a shared paging control. **Postgres and Key Value** use the two datastore mounts above; neither was created or reprobed in this sweep.

Entrypoints: canonical `/services/$serviceId/logs`; `/web/*`, `/worker/*`, `/pserv/*` and `/cron/*` redirect through `common/lib/render-alias.ts:12–18,60–69`, preserving query/hash. Log URL inputs also accept `t`/canonical `type`, `r`/canonical `range`, and `application`→`app` (`log-search.ts:65–127`). Preserve those aliases. Datastores use `?tab=logs` under /databases and /keyvalue (with /d and /r aliases). Static canonicalization/guards continue to exclude runtime Logs; no new static Logs page is proposed.

Backend entrypoints remain **GET /v1/logs**, GraphQL **logs**, MCP **list_logs**; their envelopes and cursor contents pass the controls below. **GET /v1/logs/subscribe** remains the separate live transport. No REST/GraphQL/MCP shape change is needed.

**Adjacent states:** first-ever load still uses its structural skeleton. A same-reader refresh keeps the reader while pending. Real no-match is the existing honest filtered empty state; complete versus partial/time-budgeted results retain their existing notice semantics. Authentication expiry, forbidden/not-found, store-unavailable, query-timeout and transport failure must not become successful empty results, nor retain data across a newly unauthorized resource. No authorization or resource-existence taxonomy change is proposed.

## Passing controls and full prior-DoD disposition

- Same fixture and text, absolute 11:00–11:06Z range: 100+40 rows, no further Logs requests, stable scrollTop 200 for 57.367 seconds after the reading position settled.
- A narrower text `/qa-r12/alpha/00` returns nine rows and no cap notice. `qa-r12-no-such-marker` shows “No matching logs / No logs match these filters.” Reload preserves Custom bounds, Application logs, text and Live off.
- Live on with `/qa-r12/live-` shows a newly generated `live-001` at 11:09:35Z; HTTP request returned 200. Its arrival beyond the custom historical upper bound is intentional under the displayed “The range limits history. Live mode appends new lines as they arrive.”
- Last 7 days, Live off, reaches alpha/001 after loading the second page; UI verified at 11:10:46Z. The fixture is new: this exercises paging with seven-day bounds, not seven days of retained traffic or the 84-minute refresh cadence.
- Finished build history has 100+23 build rows. Load older reaches `==> Build queued` at 10:57:46Z, still reachable at 11:11:06Z after two five-second first-page polls. This is the stable-variables counterexample to the relative-window reset.
- Relative-repro console: zero warnings/errors. Captured Logs responses are HTTP 200, including the replacement heads and cursor reads. The deliberate post-delete API GET's 404 is cleanup evidence, not a failed log journey.

**w4/done/m107, entire DoD:**

| Original guarantee | This sweep's disposition |
| --- | --- |
| 1. Last 7 days reaches beyond 100 | Immediate paging passes on the 140-line own fixture. Retaining that reading session at a relative clock tick is the uncovered residual; default 1h reproduces it quickly. |
| 2. Capped versus complete is explicit | Pass before/after a cursor page; the reappearing notice after the clock tick truthfully reflects the UI's newly discarded pages. |
| 3. REST nextStartTime/nextEndTime work both directions | Pass, limit 5 over alpha/001–009: backward 5+4 and forward 5+4, each chain unique and exhaustive. Full responses below. |
| 4. GraphQL envelope and cursors | Pass: 100 alpha/041–140 plus 40 alpha/001–040, false at completion. Exact request and complete response below. |
| 5. MCP same envelope | Pass on the nine-line sample, 5+4 and false at completion. Existing per-adapter row encoding is preserved. |
| 6. Server cap stays 100 | UI/API head is 100 with hasMore true; source `internal/logs/service.go:118,314–315` still caps at 100. No cap increase is proposed. |
| 7. Live appends and paging does not fight it | New live arrival passes. Simultaneous Live-on pagination at the clock boundary was not reprobed; t002/t005 must cover it. The confirmed defect uses Live off. |

**w4/done/m136, entire DoD:**

| Original guarantee | This sweep's disposition |
| --- | --- |
| 1. Postgres whole selected window | Not live-retested here. Datastore's memoized bounds do not use this clock; retain its existing paging tests and verify the shared-hook change. |
| 2. Key Value whole selected window | Not live-retested here; same caller tracing and required regression control. |
| 3. Datastore capped/complete notices | No claim of a datastore live recheck. Equivalent service capped, complete and nine-line controls pass; t002 must protect both datastore mounts. |
| 4. Long build reaches first line | Pass on own 123-line build leg, through Load older and stable-window polling. |
| 5. Server cap unchanged | Same 100-row source/live control as m107. |

m136's t006 specifically tested a refetched first page retaining an older cursor, but it held the variables fixed; the real preset clock changes them. History confirms **this is a residual gap, not a newly introduced m136 regression**: `git show 8667bab77:.../use-log-history.ts` already clears `older` on `[variables]`, and that commit's LogViewer already calls useLiveRange. `d55061dbb` extracted the pager and protected same-variable polls; neither addressed semantic range identity. The live controls match this distinction.

**Other precedent:** w6/m111's first-connect stream bound/window-relative empty copy remains separate; the Live-on control uses its existing bound. w4/m140 owns broad-query timeout/partial semantics and is not reopened. w7/m42 owns URL round trips; its control passes and browser back/forward sync remains deliberately deferred. Open w4/m147 owns service **Events** freshness, with a different hook; its “preserve paging” task is not this Logs reset.

All-board open/done term searches, every open milestone README, `.pm/DO_NOT_DO.md`, the latest 40 dashboard/lego commits and symbol/file history were checked at HEAD `88a5245def7b9450312faad32c5e98eae28e8ae1`. No open equivalent or already-landed relative-log paging fix was found. The production platform image train was still `e97ca42273a9`; the observed UI behavior agrees with the current code paths above. w9/m89 and w9/m92 supply the researched QA filing precedent, not related bugs.

[Render logging documentation](https://render.com/docs/logging) describes selected time ranges, custom bounds and live tailing. The exact idle-refresh implementation of Render's authenticated dashboard was not inspected; this finding relies on Bex's existing log-reader/paging promise, not an invented claim about Render's clock. External log drains, regex expansion and other documented non-goals remain excluded.

## Unverified and required follow-up

Mobile viewport stability, all alternate presets at their own timer intervals, window expiry, stale completion during a timer tick, errors during refresh, simultaneous live/paging, cron/worker/private aliases, both datastores, static deploy route, predeploy leg, multiple instances and durable-store-off fallback were not live exercised here. Verify these in t002/t005 with isolated fixtures where appropriate. Do not upgrade these code-inferred siblings into separately reproduced failures.

## Evidence and cleanup

Local artifacts were checked present: `.playwright-mcp/qa-logs-r12-relative-before.png`, `qa-logs-r12-relative-after.png`, `qa-logs-r12-custom-before.png`, `qa-logs-r12-custom-after.png`, `qa-r12-log-evidence-complete.json` (30 captured UI Logs responses plus observations and API controls), `qa-r12-requests.json`, and `qa-r12-inventory-final.json`. Screenshots are gitignored; the timed observations and complete API bodies below are the durable handoff.

DeleteService returned true at 11:13:09.397Z. Subsequent own service GET returned 404; name-filtered workspace list returned []; public URL returned HTTP 404. Exact App identity `tea-d98210cbbpdc73dcrkvg-qa-20261002-logs-r12`, UID `43e706e4-bf49-4c59-9b7c-3a795322c8d3`, was used for the read-only inventory. At 11:14:00.557Z there were no matching Apps, Deployments, Services, Ingresses, Certificates, Jobs, Pods, service accounts, network policies, PVCs, kpack Images/Builds or Secret metadata. A briefly terminating pod disappeared during normal grace; no manual cleanup was needed. The run's Kratos logout returned `ok logged-out`, temporary credential files were removed, and browser cookies cleared. Other resources/sessions were preserved.

## Exact GraphQL proofs

These read-only POSTs to `https://api.bex.co/graphql` use authenticated browser cookies and JSON content type. They replay the second failed UI tick's exact bounds with a deliberately small selection; every returned row is included. Recreate the fixture and substitute its id/times after the original logs expire.

### GraphQL page 1, 2026-10-02T11:08:31.554Z

Request:

```json
{
  "operationName": "QAWindow",
  "query": "query QAWindow($resource:String!,$type:String,$text:String,$startTime:String,$endTime:String,$limit:Int){logs(resource:$resource,type:$type,text:$text,startTime:$startTime,endTime:$endTime,limit:$limit){hasMore nextStartTime nextEndTime logs{message}}}",
  "variables": {
    "resource": "srv-davosamde41s73canun0",
    "type": "app",
    "text": "/qa-r12/alpha/",
    "startTime": "2026-10-02T10:06:09.889Z",
    "endTime": "2026-10-02T11:06:09.889Z",
    "limit": 100
  }
}
```

Complete HTTP 200 response:

```json
{
  "data": {
    "logs": {
      "hasMore": true,
      "logs": [
        {
          "message": "2026/10/02 11:02:53 GET /qa-r12/alpha/041"
        },
        {
          "message": "2026/10/02 11:02:54 GET /qa-r12/alpha/042"
        },
        {
          "message": "2026/10/02 11:02:55 GET /qa-r12/alpha/043"
        },
        {
          "message": "2026/10/02 11:02:55 GET /qa-r12/alpha/044"
        },
        {
          "message": "2026/10/02 11:02:56 GET /qa-r12/alpha/045"
        },
        {
          "message": "2026/10/02 11:02:57 GET /qa-r12/alpha/046"
        },
        {
          "message": "2026/10/02 11:02:57 GET /qa-r12/alpha/047"
        },
        {
          "message": "2026/10/02 11:02:58 GET /qa-r12/alpha/048"
        },
        {
          "message": "2026/10/02 11:02:59 GET /qa-r12/alpha/049"
        },
        {
          "message": "2026/10/02 11:02:59 GET /qa-r12/alpha/050"
        },
        {
          "message": "2026/10/02 11:03:00 GET /qa-r12/alpha/051"
        },
        {
          "message": "2026/10/02 11:03:01 GET /qa-r12/alpha/052"
        },
        {
          "message": "2026/10/02 11:03:01 GET /qa-r12/alpha/053"
        },
        {
          "message": "2026/10/02 11:03:02 GET /qa-r12/alpha/054"
        },
        {
          "message": "2026/10/02 11:03:03 GET /qa-r12/alpha/055"
        },
        {
          "message": "2026/10/02 11:03:03 GET /qa-r12/alpha/056"
        },
        {
          "message": "2026/10/02 11:03:04 GET /qa-r12/alpha/057"
        },
        {
          "message": "2026/10/02 11:03:05 GET /qa-r12/alpha/058"
        },
        {
          "message": "2026/10/02 11:03:05 GET /qa-r12/alpha/059"
        },
        {
          "message": "2026/10/02 11:03:06 GET /qa-r12/alpha/060"
        },
        {
          "message": "2026/10/02 11:03:07 GET /qa-r12/alpha/061"
        },
        {
          "message": "2026/10/02 11:03:07 GET /qa-r12/alpha/062"
        },
        {
          "message": "2026/10/02 11:03:08 GET /qa-r12/alpha/063"
        },
        {
          "message": "2026/10/02 11:03:09 GET /qa-r12/alpha/064"
        },
        {
          "message": "2026/10/02 11:03:09 GET /qa-r12/alpha/065"
        },
        {
          "message": "2026/10/02 11:03:10 GET /qa-r12/alpha/066"
        },
        {
          "message": "2026/10/02 11:03:11 GET /qa-r12/alpha/067"
        },
        {
          "message": "2026/10/02 11:03:11 GET /qa-r12/alpha/068"
        },
        {
          "message": "2026/10/02 11:03:12 GET /qa-r12/alpha/069"
        },
        {
          "message": "2026/10/02 11:03:13 GET /qa-r12/alpha/070"
        },
        {
          "message": "2026/10/02 11:03:13 GET /qa-r12/alpha/071"
        },
        {
          "message": "2026/10/02 11:03:14 GET /qa-r12/alpha/072"
        },
        {
          "message": "2026/10/02 11:03:15 GET /qa-r12/alpha/073"
        },
        {
          "message": "2026/10/02 11:03:15 GET /qa-r12/alpha/074"
        },
        {
          "message": "2026/10/02 11:03:16 GET /qa-r12/alpha/075"
        },
        {
          "message": "2026/10/02 11:03:17 GET /qa-r12/alpha/076"
        },
        {
          "message": "2026/10/02 11:03:17 GET /qa-r12/alpha/077"
        },
        {
          "message": "2026/10/02 11:03:18 GET /qa-r12/alpha/078"
        },
        {
          "message": "2026/10/02 11:03:19 GET /qa-r12/alpha/079"
        },
        {
          "message": "2026/10/02 11:03:19 GET /qa-r12/alpha/080"
        },
        {
          "message": "2026/10/02 11:03:20 GET /qa-r12/alpha/081"
        },
        {
          "message": "2026/10/02 11:03:21 GET /qa-r12/alpha/082"
        },
        {
          "message": "2026/10/02 11:03:22 GET /qa-r12/alpha/083"
        },
        {
          "message": "2026/10/02 11:03:22 GET /qa-r12/alpha/084"
        },
        {
          "message": "2026/10/02 11:03:23 GET /qa-r12/alpha/085"
        },
        {
          "message": "2026/10/02 11:03:24 GET /qa-r12/alpha/086"
        },
        {
          "message": "2026/10/02 11:03:24 GET /qa-r12/alpha/087"
        },
        {
          "message": "2026/10/02 11:03:25 GET /qa-r12/alpha/088"
        },
        {
          "message": "2026/10/02 11:03:26 GET /qa-r12/alpha/089"
        },
        {
          "message": "2026/10/02 11:03:26 GET /qa-r12/alpha/090"
        },
        {
          "message": "2026/10/02 11:03:27 GET /qa-r12/alpha/091"
        },
        {
          "message": "2026/10/02 11:03:28 GET /qa-r12/alpha/092"
        },
        {
          "message": "2026/10/02 11:03:28 GET /qa-r12/alpha/093"
        },
        {
          "message": "2026/10/02 11:03:29 GET /qa-r12/alpha/094"
        },
        {
          "message": "2026/10/02 11:03:30 GET /qa-r12/alpha/095"
        },
        {
          "message": "2026/10/02 11:03:30 GET /qa-r12/alpha/096"
        },
        {
          "message": "2026/10/02 11:03:31 GET /qa-r12/alpha/097"
        },
        {
          "message": "2026/10/02 11:03:32 GET /qa-r12/alpha/098"
        },
        {
          "message": "2026/10/02 11:03:32 GET /qa-r12/alpha/099"
        },
        {
          "message": "2026/10/02 11:03:33 GET /qa-r12/alpha/100"
        },
        {
          "message": "2026/10/02 11:03:34 GET /qa-r12/alpha/101"
        },
        {
          "message": "2026/10/02 11:03:35 GET /qa-r12/alpha/102"
        },
        {
          "message": "2026/10/02 11:03:35 GET /qa-r12/alpha/103"
        },
        {
          "message": "2026/10/02 11:03:36 GET /qa-r12/alpha/104"
        },
        {
          "message": "2026/10/02 11:03:37 GET /qa-r12/alpha/105"
        },
        {
          "message": "2026/10/02 11:03:38 GET /qa-r12/alpha/106"
        },
        {
          "message": "2026/10/02 11:03:38 GET /qa-r12/alpha/107"
        },
        {
          "message": "2026/10/02 11:03:39 GET /qa-r12/alpha/108"
        },
        {
          "message": "2026/10/02 11:03:40 GET /qa-r12/alpha/109"
        },
        {
          "message": "2026/10/02 11:03:40 GET /qa-r12/alpha/110"
        },
        {
          "message": "2026/10/02 11:03:41 GET /qa-r12/alpha/111"
        },
        {
          "message": "2026/10/02 11:03:42 GET /qa-r12/alpha/112"
        },
        {
          "message": "2026/10/02 11:03:42 GET /qa-r12/alpha/113"
        },
        {
          "message": "2026/10/02 11:03:43 GET /qa-r12/alpha/114"
        },
        {
          "message": "2026/10/02 11:03:44 GET /qa-r12/alpha/115"
        },
        {
          "message": "2026/10/02 11:03:44 GET /qa-r12/alpha/116"
        },
        {
          "message": "2026/10/02 11:03:45 GET /qa-r12/alpha/117"
        },
        {
          "message": "2026/10/02 11:03:46 GET /qa-r12/alpha/118"
        },
        {
          "message": "2026/10/02 11:03:46 GET /qa-r12/alpha/119"
        },
        {
          "message": "2026/10/02 11:03:47 GET /qa-r12/alpha/120"
        },
        {
          "message": "2026/10/02 11:03:48 GET /qa-r12/alpha/121"
        },
        {
          "message": "2026/10/02 11:03:48 GET /qa-r12/alpha/122"
        },
        {
          "message": "2026/10/02 11:03:49 GET /qa-r12/alpha/123"
        },
        {
          "message": "2026/10/02 11:03:50 GET /qa-r12/alpha/124"
        },
        {
          "message": "2026/10/02 11:03:50 GET /qa-r12/alpha/125"
        },
        {
          "message": "2026/10/02 11:03:51 GET /qa-r12/alpha/126"
        },
        {
          "message": "2026/10/02 11:03:52 GET /qa-r12/alpha/127"
        },
        {
          "message": "2026/10/02 11:03:52 GET /qa-r12/alpha/128"
        },
        {
          "message": "2026/10/02 11:03:53 GET /qa-r12/alpha/129"
        },
        {
          "message": "2026/10/02 11:03:54 GET /qa-r12/alpha/130"
        },
        {
          "message": "2026/10/02 11:03:54 GET /qa-r12/alpha/131"
        },
        {
          "message": "2026/10/02 11:03:55 GET /qa-r12/alpha/132"
        },
        {
          "message": "2026/10/02 11:03:56 GET /qa-r12/alpha/133"
        },
        {
          "message": "2026/10/02 11:03:56 GET /qa-r12/alpha/134"
        },
        {
          "message": "2026/10/02 11:03:57 GET /qa-r12/alpha/135"
        },
        {
          "message": "2026/10/02 11:03:58 GET /qa-r12/alpha/136"
        },
        {
          "message": "2026/10/02 11:03:58 GET /qa-r12/alpha/137"
        },
        {
          "message": "2026/10/02 11:03:59 GET /qa-r12/alpha/138"
        },
        {
          "message": "2026/10/02 11:04:00 GET /qa-r12/alpha/139"
        },
        {
          "message": "2026/10/02 11:04:00 GET /qa-r12/alpha/140"
        }
      ],
      "nextEndTime": "2026-10-02T11:02:53.717442592Z",
      "nextStartTime": "2026-10-02T10:06:09.889Z"
    }
  }
}
```

### GraphQL page 2, 2026-10-02T11:08:31.903Z

Request:

```json
{
  "operationName": "QAWindow",
  "query": "query QAWindow($resource:String!,$type:String,$text:String,$startTime:String,$endTime:String,$limit:Int){logs(resource:$resource,type:$type,text:$text,startTime:$startTime,endTime:$endTime,limit:$limit){hasMore nextStartTime nextEndTime logs{message}}}",
  "variables": {
    "resource": "srv-davosamde41s73canun0",
    "type": "app",
    "text": "/qa-r12/alpha/",
    "startTime": "2026-10-02T10:06:09.889Z",
    "endTime": "2026-10-02T11:02:53.717442592Z",
    "limit": 100
  }
}
```

Complete HTTP 200 response:

```json
{
  "data": {
    "logs": {
      "hasMore": false,
      "logs": [
        {
          "message": "2026/10/02 11:02:26 GET /qa-r12/alpha/001"
        },
        {
          "message": "2026/10/02 11:02:27 GET /qa-r12/alpha/002"
        },
        {
          "message": "2026/10/02 11:02:27 GET /qa-r12/alpha/003"
        },
        {
          "message": "2026/10/02 11:02:28 GET /qa-r12/alpha/004"
        },
        {
          "message": "2026/10/02 11:02:29 GET /qa-r12/alpha/005"
        },
        {
          "message": "2026/10/02 11:02:30 GET /qa-r12/alpha/006"
        },
        {
          "message": "2026/10/02 11:02:30 GET /qa-r12/alpha/007"
        },
        {
          "message": "2026/10/02 11:02:31 GET /qa-r12/alpha/008"
        },
        {
          "message": "2026/10/02 11:02:32 GET /qa-r12/alpha/009"
        },
        {
          "message": "2026/10/02 11:02:32 GET /qa-r12/alpha/010"
        },
        {
          "message": "2026/10/02 11:02:33 GET /qa-r12/alpha/011"
        },
        {
          "message": "2026/10/02 11:02:34 GET /qa-r12/alpha/012"
        },
        {
          "message": "2026/10/02 11:02:34 GET /qa-r12/alpha/013"
        },
        {
          "message": "2026/10/02 11:02:35 GET /qa-r12/alpha/014"
        },
        {
          "message": "2026/10/02 11:02:36 GET /qa-r12/alpha/015"
        },
        {
          "message": "2026/10/02 11:02:36 GET /qa-r12/alpha/016"
        },
        {
          "message": "2026/10/02 11:02:37 GET /qa-r12/alpha/017"
        },
        {
          "message": "2026/10/02 11:02:38 GET /qa-r12/alpha/018"
        },
        {
          "message": "2026/10/02 11:02:38 GET /qa-r12/alpha/019"
        },
        {
          "message": "2026/10/02 11:02:39 GET /qa-r12/alpha/020"
        },
        {
          "message": "2026/10/02 11:02:40 GET /qa-r12/alpha/021"
        },
        {
          "message": "2026/10/02 11:02:40 GET /qa-r12/alpha/022"
        },
        {
          "message": "2026/10/02 11:02:41 GET /qa-r12/alpha/023"
        },
        {
          "message": "2026/10/02 11:02:42 GET /qa-r12/alpha/024"
        },
        {
          "message": "2026/10/02 11:02:42 GET /qa-r12/alpha/025"
        },
        {
          "message": "2026/10/02 11:02:43 GET /qa-r12/alpha/026"
        },
        {
          "message": "2026/10/02 11:02:44 GET /qa-r12/alpha/027"
        },
        {
          "message": "2026/10/02 11:02:44 GET /qa-r12/alpha/028"
        },
        {
          "message": "2026/10/02 11:02:45 GET /qa-r12/alpha/029"
        },
        {
          "message": "2026/10/02 11:02:46 GET /qa-r12/alpha/030"
        },
        {
          "message": "2026/10/02 11:02:46 GET /qa-r12/alpha/031"
        },
        {
          "message": "2026/10/02 11:02:47 GET /qa-r12/alpha/032"
        },
        {
          "message": "2026/10/02 11:02:48 GET /qa-r12/alpha/033"
        },
        {
          "message": "2026/10/02 11:02:48 GET /qa-r12/alpha/034"
        },
        {
          "message": "2026/10/02 11:02:49 GET /qa-r12/alpha/035"
        },
        {
          "message": "2026/10/02 11:02:50 GET /qa-r12/alpha/036"
        },
        {
          "message": "2026/10/02 11:02:51 GET /qa-r12/alpha/037"
        },
        {
          "message": "2026/10/02 11:02:51 GET /qa-r12/alpha/038"
        },
        {
          "message": "2026/10/02 11:02:52 GET /qa-r12/alpha/039"
        },
        {
          "message": "2026/10/02 11:02:53 GET /qa-r12/alpha/040"
        }
      ],
      "nextEndTime": "2026-10-02T11:02:26.542302033Z",
      "nextStartTime": "2026-10-02T10:06:09.889Z"
    }
  }
}
```

## Exact REST and MCP cursor controls

REST responses use Render label arrays; MCP uses its existing labels map and duplicated text/structuredContent envelope. Those row encodings are not part of this UI finding. All requests below are read-only and authenticated; MCP sends Content-Type: application/json and Accept: application/json, text/event-stream.

### Cursor control 1, 2026-10-02T11:09:01.258Z

`GET https://api.bex.co/v1/logs?resource=srv-davosamde41s73canun0&type=app&text=%2Fqa-r12%2Falpha%2F00&startTime=2026-10-02T11%3A02%3A25Z&endTime=2026-10-02T11%3A02%3A33Z&direction=backward&limit=5`

Complete HTTP 200 response:

```json
{
  "hasMore": true,
  "nextStartTime": "2026-10-02T11:02:25Z",
  "nextEndTime": "2026-10-02T11:02:29.340131202Z",
  "logs": [
    {
      "id": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg-2026-10-02T11:02:29.340131203Z-b70e617d",
      "message": "2026/10/02 11:02:29 GET /qa-r12/alpha/005",
      "timestamp": "2026-10-02T11:02:29.340131203Z",
      "labels": [
        {
          "name": "type",
          "value": "app"
        },
        {
          "name": "resource",
          "value": "srv-davosamde41s73canun0"
        },
        {
          "name": "instance",
          "value": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg"
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
    },
    {
      "id": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg-2026-10-02T11:02:30.011554862Z-9f8ec596",
      "message": "2026/10/02 11:02:30 GET /qa-r12/alpha/006",
      "timestamp": "2026-10-02T11:02:30.011554862Z",
      "labels": [
        {
          "name": "type",
          "value": "app"
        },
        {
          "name": "resource",
          "value": "srv-davosamde41s73canun0"
        },
        {
          "name": "instance",
          "value": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg"
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
    },
    {
      "id": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg-2026-10-02T11:02:30.693325391Z-a08ec729",
      "message": "2026/10/02 11:02:30 GET /qa-r12/alpha/007",
      "timestamp": "2026-10-02T11:02:30.693325391Z",
      "labels": [
        {
          "name": "type",
          "value": "app"
        },
        {
          "name": "resource",
          "value": "srv-davosamde41s73canun0"
        },
        {
          "name": "instance",
          "value": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg"
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
    },
    {
      "id": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg-2026-10-02T11:02:31.361503379Z-741a617f",
      "message": "2026/10/02 11:02:31 GET /qa-r12/alpha/008",
      "timestamp": "2026-10-02T11:02:31.361503379Z",
      "labels": [
        {
          "name": "type",
          "value": "app"
        },
        {
          "name": "resource",
          "value": "srv-davosamde41s73canun0"
        },
        {
          "name": "instance",
          "value": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg"
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
    },
    {
      "id": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg-2026-10-02T11:02:32.068147881Z-2ad4d23d",
      "message": "2026/10/02 11:02:32 GET /qa-r12/alpha/009",
      "timestamp": "2026-10-02T11:02:32.068147881Z",
      "labels": [
        {
          "name": "type",
          "value": "app"
        },
        {
          "name": "resource",
          "value": "srv-davosamde41s73canun0"
        },
        {
          "name": "instance",
          "value": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg"
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

### Cursor control 2, 2026-10-02T11:09:01.506Z

`GET https://api.bex.co/v1/logs?resource=srv-davosamde41s73canun0&type=app&text=%2Fqa-r12%2Falpha%2F00&startTime=2026-10-02T11%3A02%3A25Z&endTime=2026-10-02T11%3A02%3A29.340131202Z&direction=backward&limit=5`

Complete HTTP 200 response:

```json
{
  "hasMore": false,
  "nextStartTime": "2026-10-02T11:02:25Z",
  "nextEndTime": "2026-10-02T11:02:26.542302033Z",
  "logs": [
    {
      "id": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg-2026-10-02T11:02:26.542302034Z-17d3d9dc",
      "message": "2026/10/02 11:02:26 GET /qa-r12/alpha/001",
      "timestamp": "2026-10-02T11:02:26.542302034Z",
      "labels": [
        {
          "name": "type",
          "value": "app"
        },
        {
          "name": "resource",
          "value": "srv-davosamde41s73canun0"
        },
        {
          "name": "instance",
          "value": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg"
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
    },
    {
      "id": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg-2026-10-02T11:02:27.222945792Z-d766093a",
      "message": "2026/10/02 11:02:27 GET /qa-r12/alpha/002",
      "timestamp": "2026-10-02T11:02:27.222945792Z",
      "labels": [
        {
          "name": "type",
          "value": "app"
        },
        {
          "name": "resource",
          "value": "srv-davosamde41s73canun0"
        },
        {
          "name": "instance",
          "value": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg"
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
    },
    {
      "id": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg-2026-10-02T11:02:27.906680182Z-d8660acd",
      "message": "2026/10/02 11:02:27 GET /qa-r12/alpha/003",
      "timestamp": "2026-10-02T11:02:27.906680182Z",
      "labels": [
        {
          "name": "type",
          "value": "app"
        },
        {
          "name": "resource",
          "value": "srv-davosamde41s73canun0"
        },
        {
          "name": "instance",
          "value": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg"
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
    },
    {
      "id": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg-2026-10-02T11:02:28.589951941Z-d889ea0d",
      "message": "2026/10/02 11:02:28 GET /qa-r12/alpha/004",
      "timestamp": "2026-10-02T11:02:28.589951941Z",
      "labels": [
        {
          "name": "type",
          "value": "app"
        },
        {
          "name": "resource",
          "value": "srv-davosamde41s73canun0"
        },
        {
          "name": "instance",
          "value": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg"
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

### Cursor control 3, 2026-10-02T11:09:01.844Z

`GET https://api.bex.co/v1/logs?resource=srv-davosamde41s73canun0&type=app&text=%2Fqa-r12%2Falpha%2F00&startTime=2026-10-02T11%3A02%3A25Z&endTime=2026-10-02T11%3A02%3A33Z&direction=forward&limit=5`

Complete HTTP 200 response:

```json
{
  "hasMore": true,
  "nextStartTime": "2026-10-02T11:02:29.340131204Z",
  "nextEndTime": "2026-10-02T11:02:33Z",
  "logs": [
    {
      "id": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg-2026-10-02T11:02:26.542302034Z-17d3d9dc",
      "message": "2026/10/02 11:02:26 GET /qa-r12/alpha/001",
      "timestamp": "2026-10-02T11:02:26.542302034Z",
      "labels": [
        {
          "name": "type",
          "value": "app"
        },
        {
          "name": "resource",
          "value": "srv-davosamde41s73canun0"
        },
        {
          "name": "instance",
          "value": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg"
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
    },
    {
      "id": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg-2026-10-02T11:02:27.222945792Z-d766093a",
      "message": "2026/10/02 11:02:27 GET /qa-r12/alpha/002",
      "timestamp": "2026-10-02T11:02:27.222945792Z",
      "labels": [
        {
          "name": "type",
          "value": "app"
        },
        {
          "name": "resource",
          "value": "srv-davosamde41s73canun0"
        },
        {
          "name": "instance",
          "value": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg"
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
    },
    {
      "id": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg-2026-10-02T11:02:27.906680182Z-d8660acd",
      "message": "2026/10/02 11:02:27 GET /qa-r12/alpha/003",
      "timestamp": "2026-10-02T11:02:27.906680182Z",
      "labels": [
        {
          "name": "type",
          "value": "app"
        },
        {
          "name": "resource",
          "value": "srv-davosamde41s73canun0"
        },
        {
          "name": "instance",
          "value": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg"
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
    },
    {
      "id": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg-2026-10-02T11:02:28.589951941Z-d889ea0d",
      "message": "2026/10/02 11:02:28 GET /qa-r12/alpha/004",
      "timestamp": "2026-10-02T11:02:28.589951941Z",
      "labels": [
        {
          "name": "type",
          "value": "app"
        },
        {
          "name": "resource",
          "value": "srv-davosamde41s73canun0"
        },
        {
          "name": "instance",
          "value": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg"
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
    },
    {
      "id": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg-2026-10-02T11:02:29.340131203Z-b70e617d",
      "message": "2026/10/02 11:02:29 GET /qa-r12/alpha/005",
      "timestamp": "2026-10-02T11:02:29.340131203Z",
      "labels": [
        {
          "name": "type",
          "value": "app"
        },
        {
          "name": "resource",
          "value": "srv-davosamde41s73canun0"
        },
        {
          "name": "instance",
          "value": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg"
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

### Cursor control 4, 2026-10-02T11:09:02.086Z

`GET https://api.bex.co/v1/logs?resource=srv-davosamde41s73canun0&type=app&text=%2Fqa-r12%2Falpha%2F00&startTime=2026-10-02T11%3A02%3A29.340131204Z&endTime=2026-10-02T11%3A02%3A33Z&direction=forward&limit=5`

Complete HTTP 200 response:

```json
{
  "hasMore": false,
  "nextStartTime": "2026-10-02T11:02:32.068147882Z",
  "nextEndTime": "2026-10-02T11:02:33Z",
  "logs": [
    {
      "id": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg-2026-10-02T11:02:30.011554862Z-9f8ec596",
      "message": "2026/10/02 11:02:30 GET /qa-r12/alpha/006",
      "timestamp": "2026-10-02T11:02:30.011554862Z",
      "labels": [
        {
          "name": "type",
          "value": "app"
        },
        {
          "name": "resource",
          "value": "srv-davosamde41s73canun0"
        },
        {
          "name": "instance",
          "value": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg"
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
    },
    {
      "id": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg-2026-10-02T11:02:30.693325391Z-a08ec729",
      "message": "2026/10/02 11:02:30 GET /qa-r12/alpha/007",
      "timestamp": "2026-10-02T11:02:30.693325391Z",
      "labels": [
        {
          "name": "type",
          "value": "app"
        },
        {
          "name": "resource",
          "value": "srv-davosamde41s73canun0"
        },
        {
          "name": "instance",
          "value": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg"
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
    },
    {
      "id": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg-2026-10-02T11:02:31.361503379Z-741a617f",
      "message": "2026/10/02 11:02:31 GET /qa-r12/alpha/008",
      "timestamp": "2026-10-02T11:02:31.361503379Z",
      "labels": [
        {
          "name": "type",
          "value": "app"
        },
        {
          "name": "resource",
          "value": "srv-davosamde41s73canun0"
        },
        {
          "name": "instance",
          "value": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg"
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
    },
    {
      "id": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg-2026-10-02T11:02:32.068147881Z-2ad4d23d",
      "message": "2026/10/02 11:02:32 GET /qa-r12/alpha/009",
      "timestamp": "2026-10-02T11:02:32.068147881Z",
      "labels": [
        {
          "name": "type",
          "value": "app"
        },
        {
          "name": "resource",
          "value": "srv-davosamde41s73canun0"
        },
        {
          "name": "instance",
          "value": "srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg"
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

### Cursor control 5, 2026-10-02T11:09:02.335Z

`POST https://api.bex.co/mcp`

Request:

```json
{
  "jsonrpc": "2.0",
  "id": 1201,
  "method": "tools/call",
  "params": {
    "name": "list_logs",
    "arguments": {
      "resource": ["srv-davosamde41s73canun0"],
      "type": ["app"],
      "text": ["/qa-r12/alpha/00"],
      "startTime": "2026-10-02T11:02:25Z",
      "endTime": "2026-10-02T11:02:33Z",
      "direction": "backward",
      "limit": 5
    }
  }
}
```

Complete HTTP 200 response:

```text
event: message
data: {"jsonrpc":"2.0","id":1201,"result":{"content":[{"type":"text","text":"{\"hasMore\":true,\"logs\":[{\"labels\":{\"container\":\"app\",\"instance\":\"srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg\",\"level\":\"unknown\",\"service\":\"srv-davosamde41s73canun0\",\"type\":\"app\"},\"message\":\"2026/10/02 11:02:29 GET /qa-r12/alpha/005\",\"timestamp\":\"2026-10-02T11:02:29.340131203Z\"},{\"labels\":{\"container\":\"app\",\"instance\":\"srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg\",\"level\":\"unknown\",\"service\":\"srv-davosamde41s73canun0\",\"type\":\"app\"},\"message\":\"2026/10/02 11:02:30 GET /qa-r12/alpha/006\",\"timestamp\":\"2026-10-02T11:02:30.011554862Z\"},{\"labels\":{\"container\":\"app\",\"instance\":\"srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg\",\"level\":\"unknown\",\"service\":\"srv-davosamde41s73canun0\",\"type\":\"app\"},\"message\":\"2026/10/02 11:02:30 GET /qa-r12/alpha/007\",\"timestamp\":\"2026-10-02T11:02:30.693325391Z\"},{\"labels\":{\"container\":\"app\",\"instance\":\"srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg\",\"level\":\"unknown\",\"service\":\"srv-davosamde41s73canun0\",\"type\":\"app\"},\"message\":\"2026/10/02 11:02:31 GET /qa-r12/alpha/008\",\"timestamp\":\"2026-10-02T11:02:31.361503379Z\"},{\"labels\":{\"container\":\"app\",\"instance\":\"srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg\",\"level\":\"unknown\",\"service\":\"srv-davosamde41s73canun0\",\"type\":\"app\"},\"message\":\"2026/10/02 11:02:32 GET /qa-r12/alpha/009\",\"timestamp\":\"2026-10-02T11:02:32.068147881Z\"}],\"nextEndTime\":\"2026-10-02T11:02:29.340131202Z\",\"nextStartTime\":\"2026-10-02T11:02:25Z\"}"}],"structuredContent":{"hasMore":true,"logs":[{"labels":{"container":"app","instance":"srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg","level":"unknown","service":"srv-davosamde41s73canun0","type":"app"},"message":"2026/10/02 11:02:29 GET /qa-r12/alpha/005","timestamp":"2026-10-02T11:02:29.340131203Z"},{"labels":{"container":"app","instance":"srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg","level":"unknown","service":"srv-davosamde41s73canun0","type":"app"},"message":"2026/10/02 11:02:30 GET /qa-r12/alpha/006","timestamp":"2026-10-02T11:02:30.011554862Z"},{"labels":{"container":"app","instance":"srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg","level":"unknown","service":"srv-davosamde41s73canun0","type":"app"},"message":"2026/10/02 11:02:30 GET /qa-r12/alpha/007","timestamp":"2026-10-02T11:02:30.693325391Z"},{"labels":{"container":"app","instance":"srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg","level":"unknown","service":"srv-davosamde41s73canun0","type":"app"},"message":"2026/10/02 11:02:31 GET /qa-r12/alpha/008","timestamp":"2026-10-02T11:02:31.361503379Z"},{"labels":{"container":"app","instance":"srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg","level":"unknown","service":"srv-davosamde41s73canun0","type":"app"},"message":"2026/10/02 11:02:32 GET /qa-r12/alpha/009","timestamp":"2026-10-02T11:02:32.068147881Z"}],"nextEndTime":"2026-10-02T11:02:29.340131202Z","nextStartTime":"2026-10-02T11:02:25Z"}}}
```

### Cursor control 6, 2026-10-02T11:09:16.754Z

`POST https://api.bex.co/mcp`

Request:

```json
{
  "jsonrpc": "2.0",
  "id": 1202,
  "method": "tools/call",
  "params": {
    "name": "list_logs",
    "arguments": {
      "resource": ["srv-davosamde41s73canun0"],
      "type": ["app"],
      "text": ["/qa-r12/alpha/00"],
      "startTime": "2026-10-02T11:02:25Z",
      "endTime": "2026-10-02T11:02:29.340131202Z",
      "direction": "backward",
      "limit": 5
    }
  }
}
```

Complete HTTP 200 response:

```text
event: message
data: {"jsonrpc":"2.0","id":1202,"result":{"content":[{"type":"text","text":"{\"hasMore\":false,\"logs\":[{\"labels\":{\"container\":\"app\",\"instance\":\"srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg\",\"level\":\"unknown\",\"service\":\"srv-davosamde41s73canun0\",\"type\":\"app\"},\"message\":\"2026/10/02 11:02:26 GET /qa-r12/alpha/001\",\"timestamp\":\"2026-10-02T11:02:26.542302034Z\"},{\"labels\":{\"container\":\"app\",\"instance\":\"srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg\",\"level\":\"unknown\",\"service\":\"srv-davosamde41s73canun0\",\"type\":\"app\"},\"message\":\"2026/10/02 11:02:27 GET /qa-r12/alpha/002\",\"timestamp\":\"2026-10-02T11:02:27.222945792Z\"},{\"labels\":{\"container\":\"app\",\"instance\":\"srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg\",\"level\":\"unknown\",\"service\":\"srv-davosamde41s73canun0\",\"type\":\"app\"},\"message\":\"2026/10/02 11:02:27 GET /qa-r12/alpha/003\",\"timestamp\":\"2026-10-02T11:02:27.906680182Z\"},{\"labels\":{\"container\":\"app\",\"instance\":\"srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg\",\"level\":\"unknown\",\"service\":\"srv-davosamde41s73canun0\",\"type\":\"app\"},\"message\":\"2026/10/02 11:02:28 GET /qa-r12/alpha/004\",\"timestamp\":\"2026-10-02T11:02:28.589951941Z\"}],\"nextEndTime\":\"2026-10-02T11:02:26.542302033Z\",\"nextStartTime\":\"2026-10-02T11:02:25Z\"}"}],"structuredContent":{"hasMore":false,"logs":[{"labels":{"container":"app","instance":"srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg","level":"unknown","service":"srv-davosamde41s73canun0","type":"app"},"message":"2026/10/02 11:02:26 GET /qa-r12/alpha/001","timestamp":"2026-10-02T11:02:26.542302034Z"},{"labels":{"container":"app","instance":"srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg","level":"unknown","service":"srv-davosamde41s73canun0","type":"app"},"message":"2026/10/02 11:02:27 GET /qa-r12/alpha/002","timestamp":"2026-10-02T11:02:27.222945792Z"},{"labels":{"container":"app","instance":"srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg","level":"unknown","service":"srv-davosamde41s73canun0","type":"app"},"message":"2026/10/02 11:02:27 GET /qa-r12/alpha/003","timestamp":"2026-10-02T11:02:27.906680182Z"},{"labels":{"container":"app","instance":"srv-davosamde41s73canun0-9ma8c2i529i6gqj5rlfg","level":"unknown","service":"srv-davosamde41s73canun0","type":"app"},"message":"2026/10/02 11:02:28 GET /qa-r12/alpha/004","timestamp":"2026-10-02T11:02:28.589951941Z"}],"nextEndTime":"2026-10-02T11:02:26.542302033Z","nextStartTime":"2026-10-02T11:02:25Z"}}}
```
