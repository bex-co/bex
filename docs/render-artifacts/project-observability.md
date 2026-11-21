# Render's project Observability views — captured layout, data model and queries

Captured 2026-11-19 in a headed browser signed in to the user's own Render workspace, on a project with eight resources in one environment: two Node web services, three cron jobs, a suspended web service, a static site and a Postgres 17. It is read-only: every dialog was closed without saving, and no view was created, duplicated or deleted. Log text, client addresses, request ids and every resource id below are replaced with shapes (`prj-…`, `svs-…`). Filed with `w3/m505`–`w3/m509`.

This is the project's own **Observability** group. It is not the workspace page `dashboard.render.com/observability`, which holds log and metric streams, a non-goal in [`.pm/DO_NOT_DO.md`](../../.pm/DO_NOT_DO.md) and in [dashboard-routes.md](dashboard-routes.md).

## Routes

From the dashboard bundle's route table:

| page | path |
| --- | --- |
| Project overview | `/project/{prj}` |
| Observability (any) | `/project/{prj}/observability/*` |
| Application Logs | `/project/{prj}/observability/logs/application?savedSearchId=svs-…&r=1h` |
| Request Logs | `/project/{prj}/observability/logs/request?savedSearchId=svs-…&r=1h` |
| Service Metrics | `/project/{prj}/observability/metrics` |
| Workflow Runs | `/project/{prj}/observability/workflow-runs` (Render Workflows, a non-goal) |
| Project settings | `/project/{prj}/settings` |
| Environment | `/project/{prj}/environment/{evm}` (+ `/settings`, `/new`) |

`r` is the time range (`1h`); `savedSearchId` names the view. The bundle keeps `savedSearchId` when it rewrites a URL under `/project/{prj}/observability/`. There are no separate environment observability pages: a view is narrowed to an environment by its `Environment is …` filter.

## Sidebar

The project sidebar reads, top to bottom: a back link and the project's name, **Overview**, then the group **OBSERVABILITY** with one link per saved view (**Application Logs**, **Request Logs**) and a **+ New View** button, then the group **MANAGE** with **Settings**.

**New View** opens a menu headed **TYPE**: Application Logs, Request Logs, Workflow Runs.

## Every view is a saved search

The default views are saved searches too. The page loads them with Render's private GraphQL:

```graphql
query savedSearches($projectId: String!, $type: SavedSearchType) {
  savedSearches(projectId: $projectId, type: $type) {
    id
    name
    type
    filters
  }
}
query savedSearch($id: String!) {
  savedSearch(id: $id) {
    id
    name
    type
    filters
  }
}
```

The project's answer held three, created for it by Render:

| name             | type               | filters                         |
| ---------------- | ------------------ | ------------------------------- |
| Application Logs | `APPLICATION_LOGS` | `{"filters": [], "version": 1}` |
| Request Logs     | `REQUEST_LOGS`     | `{"filters": [], "version": 1}` |
| Service Metrics  | `METRICS`          | `{"filters": [], "version": 1}` |

- `SavedSearchType` is `APPLICATION_LOGS | METRICS | REQUEST_LOGS | WORKFLOW_RUNS`; the filter operators are `CONTAINS | IS | IS_NOT`; a metrics group-by is `NONE | RESOURCE | STATUS_CODE` (all from the bundle; the operators re-read in [§ Edit view](#edit-view)).
- Render's public OpenAPI (the pinned `render-public-api-1.json`) has no saved-search operation, but its audit log records `CreateSavedSearchEvent` and `DeleteSavedSearchEvent`, and its error codes include `duplicate_saved_search_name`.
- **Service Metrics is not shown in this workspace.** The `METRICS` saved search exists, but the sidebar does not list it, New View does not offer it, and `/project/{prj}/observability/metrics?savedSearchId=…` redirects to `/`. [render.com/docs/projects](https://render.com/docs/projects) and [render.com/docs/service-metrics](https://render.com/docs/service-metrics) describe it ("Projects and environments display metrics for _all_ of their services on the _Service Metrics_ page"), so it looks gated rather than removed.

## The view page

- **Header:** the eyebrow `OBSERVABILITY VIEW`, the view's name, and **Edit view**, **Duplicate** and a trash **Delete view** button. The default views have all three.
- **Toolbar:** a **Search logs** box, a time-range button, a **Maximize** button, an **Options** (…) menu and a **Region** picker.
  - Time range: Live tail, Last hour, Last 4 hours, Last 24 hours, Last 2 days, Last 7 days, Last 14 days, Last 30 days (disabled past the plan's retention), Custom.
  - Options: **Copy logs** (`⌘ ⇧ C`), **Order by** Ascending / Descending, **Display: Expand everything**, and **Theme Settings** (below). In Live tail, while a clear is offered (`onClearLogs`), **Clear logs** (a `Reset` icon, the `Shift L` hint) sits between Order by and Display (read from `Logs-DOHYAXbe.js` on 2026-11-20, w3/m528; bex w3/092).
  - **Display** (read from the bundle on 2026-11-20, `Logs-kRP_JevQ.js`, w3/088): `Display: <choice>` opens a submenu headed **Display settings** with **Expand everything** (`expand-all`, the default: every line wrapped whole, `whitespace-pre-wrap break-words`), **Expand errors** (`expand-errors`: lines whose level is `error`, `critical`, `alert` or `emergency` wrapped whole, every other line on one) and **Collapse everything** (`collapse-all`: every line on one line, `overflow-hidden text-ellipsis whitespace-nowrap`). There is no per-row Show more. The choice is kept in `localStorage` under `render-log-truncation-mode`, one for every explorer, and an unknown value reads as Expand everything; Order by is kept the same way under `render-log-direction-setting`.
  - Region: the workspace's active regions (`Oregon (US West)`, `Global`), from `activeRegions(ownerId)`.
  - Search: focusing the box lists **Add a filter** suggestions, `level:` (`info, debug, warning, error, …`) and `instance:` (the instances seen), with a **Syntax tips** link. Typing narrows them. There is no service filter in the box; a view narrows services through its own filters.
- **Lines:** a date separator row (`Nov 19`), then per line a level icon gutter (the info icon appears only on hover; a warning line shows a warning icon and a tinted row; see § Levels), the time (`12:15:03 AM`), a colored dot and `[service name]` (one color per service), the short instance (`[rstgn]`) when there is one, then the message. Lines from web services, cron jobs and the Postgres database interleave in time order.
- **Request lines:** `[GET]`, the status as a colored chip (`200` green, `404` orange), the host and path, then the message fields (`clientIP`, `requestID`, `responseTimeMS`, `responseBytes`, `userAgent`).
- **Row menu, Return button, new-log buttons and Theme Settings** (read from Render's dashboard bundle on 2026-11-20, chunks `Logs-DOHYAXbe.js` and `UserSettings-3jrn6Tc2.js`, w3/m528; the hashes change with each Render deploy, so find them by the component names):
  - **Row menu** (`LogLineOverflowMenu`): a 24 px **…** button at the row's top right, rendered while the row is hovered or its menu is open, and the row is highlighted while it is open. It is not rendered when the line is blank and neither of the last two items applies. Its items:
    - **Copy log line**, disabled on a blank line. It copies `[label] METHOD STATUS host+path` and then the text with ANSI stripped, with the parts the line has, and no time (`En`). `label` is the resource's name in a project view and absent on one service's log; the instance is not copied. The item reads **Copied!** after it copies.
    - **View in context**, shown while the explorer is narrowed (`hasActiveSearchFilters`: the log type is not `all`, a search, or a label filter). A `target="_blank"` link to the same explorer with its default filters (no search, no labels), `contextLogId` the line, the range from 10 minutes before the line to 10 minutes after it, and `centeredTimestamp` its time. The opened explorer highlights that line and scrolls it into view.
    - **SSH into instance**, shown when the explorer has `serverInstances` and a `servicePath` and the line has an instance. It links (`target="_blank"`) to `${servicePath}/shell/${instance}` when the instance is one of the running server instances, and is otherwise disabled with the tooltip _"You can only SSH into running instances. This one is not available."_ A project view passes `servicePath: null`, so its lines offer no SSH.
  - **Return button** (`LogViewerReturnButton`): a small icon-only button at `right-6` (`bottom-4` ascending), named **Return to Bottom** (`MoveDown`) ascending and **Return to Top** (`MoveUp`) descending. It shows while the reader is away from the newest line and returns them to it.
  - **Held lines** (`BufferedLogsButton`): in Live tail, for runtime logs (`isRuntimeServiceLogs`, every log type `app` or `request`), while the reader is scrolled away, new lines are held instead of joining the list. A centred button with an arrow reads **New logs** below 10 held lines, else `${Math.ceil(Math.min(n, 100) / 5) * 5}+ new logs` (**10+**, **25+**, … **100+**). Clicking it adds them and scrolls to the newest line; reaching the newest edge adds them too.
  - **New logs in Live Tail** (`NewLogsAvailableButton`): on a relative range (`1h` … `30d`) the page keeps a `logAdded` subscription open, and its first line sets `hasNewLogsAvailable`. A centred split button shows a `Refresh` icon and **New logs in Live Tail**, with the tooltip **Switch to Live Tail** and a secondary `Close` icon named **Dismiss**. Clicking it sets the range to Live tail; Dismiss hides it. The flag resets when the range or the filters change.
  - **Theme Settings**: Options' last item, `{title: "Theme Settings", IconLeft: Settings, href: "/u/settings#appearance"}`. Render's account settings list Profile, **Appearance**, Account Security, Stripe, CLI Tokens, API Keys, Connections, SSH Public Keys, PR Requests and Delete Account. Appearance holds **Dashboard Theme** (System, Light, Dark, the default marked `(Default)`, so **System (Default)**), **Log Explorer Theme** (`Match Dashboard (Default)`, `Light`, `Dark`) and **High Contrast Mode** (_"Increase the visibility of interactive elements."_). The Appearance section itself, its two enums, the mutations, High Contrast's twins and the log theme mapping are recorded in [account-appearance.md](account-appearance.md) (w3/m531).
- **Live tail is a range** (read from Render's dashboard bundle on 2026-11-20, chunks `Logs-CxB-g50N.js`, `log-filters-1FeMUC1z.js`, `ServerLogs-C_kS751Q.js`, `ProjectLogs-WVVPlTWd.js` and `index-BR2q8yex.js`, w3/m537; and [render.com/docs/logging](https://render.com/docs/logging): _"select Live tail to view a live feed of recent logs. The default displayed range is Last hour"_):
  - **Spelling.** The explorer keeps one `timeRange`: `live`, a relative range (`1h`, `4h`, `24h`, `2d`, `7d`, `14d`, `30d`) or a custom `{start, end}`. `logTimeRangeIntervals` lists `live` first. The URL key is `r`: `r=live`, `r=1h`, or a custom `r=YYYY-MM-DD@HH:mm:ss~YYYY-MM-DD@HH:mm:ss` in UTC. Runtime logs (every log type `app` or `request`: a service's Logs, a Postgres or Key Value Logs page, a project view; `isRuntimeServiceLogs`) always write `r`, `r=live` included; a deploy's or build's log, where Live tail is the default, writes no `r` for it. The explorer tails exactly while `timeRange === "live"` (`liveMode`).
  - **Defaults.** With no `r`, runtime logs open on `1h` (Last hour), and the other explorers on `live`. A service's Logs (`ServerLogs`), a Postgres and a Key Value Logs page pass no initial range, so each opens on Last hour. A project view opens on its saved search's `timeRange.relative` when the URL names no `r` (it writes it into the URL as it opens), else on Last hour. A running job or build opens on Live tail, a finished job on its run's window.
  - **What Live tail reads.** Before new lines arrive, the last 4 hours (`live` maps to 4 in the same range-to-hours table that maps `1h` to 1), backward, 50 lines a page, older pages loading on scroll from the result's `nextStartTime`/`nextEndTime`; new lines then arrive over the `logAdded` subscription.
  - **The picker.** The time range dropdown lists Live tail, the relative ranges (those past the plan's retention disabled) and Custom. Picking Live tail records `Live Tail Switched` with `source: "time range dropdown"`, and New logs in Live Tail does the same with `source: "new logs button"`, setting `timeRange` to `live` with the filters kept. A custom range is never live, ended or not: nothing tails it, and New logs in Live Tail is offered on a relative range only. Clear logs (`Shift L`) is offered while `timeRange === "live"`.
  - **Edit view.** The Time Range field lists the relative ranges only (`value !== "live" && value !== "custom"`), so a saved view never names Live tail.

## Edit view

A modal titled **Edit view**: a **Name** field, a **Filters** list with an add (+) button, and one row per condition with **Field**, **Operator** and **Values** dropdowns and a trash button. Below the rows: _"Editing filters will update the view for everyone on the project."_ Then **Close** and **Done**. A view is shared by the whole project, not kept per person.

| view type | fields |
| --- | --- |
| Application Logs | Environment, Resource, Resource Type, Level, Instance, Text Search, Time Range |
| Request Logs | Environment, Resource, Resource Type, Level, Instance, Method, Status Code, Host, Path, Time Range |

Key Value and Postgres logs belong to the project's Application Logs.

### Operators per field (read from the bundle, 2026-11-19, w3/m523)

Read without sign-in from Render's public dashboard bundle: `https://dashboard.render.com/` loads `assets/index-*.js`, and following its `./*.js` imports to a fixpoint fetches about 450 chunks. `ProjectLogs-*.js` builds the fields (`ProjectLogs-DdJvXDu8.js` that day), `SavedViewFiltersModal-*.js` draws the rows and builds the saved document (`SavedViewFiltersModal-C9MBDKkv.js`), and `git-*.js` holds the generated GraphQL enums (`git-DtnyV415.js`). The hashes change with every Render deploy, so find the chunks by name.

| field | operators | values | view types |
| --- | --- | --- | --- |
| Environment | `IS` | one of the project's environments | both |
| Resource | `IS`, `IS_NOT` | the project's resources | both |
| Resource Type | `IS`, `IS_NOT` | Services, Cron jobs, Postgres, Key Value | both |
| Level | `IS`, `IS_NOT` | the label's values in the project's logs | both |
| Instance | `IS`, `IS_NOT` | the label's values | both |
| Text Search | `CONTAINS` | free text | Application Logs |
| Method | `IS`, `IS_NOT` | the label's values | Request Logs |
| Status Code | `IS`, `IS_NOT` | the label's values | Request Logs |
| Host | `IS`, `IS_NOT` | the label's values | Request Logs |
| Path | `IS`, `IS_NOT` | the label's values | Request Logs |
| Time Range | `IS` | one preset, Last hour … Last 30 days | both |

- **Labels.** A field that takes many values labels its operators `is one of` (`IS`) and `is not one of` (`IS_NOT`). A field that takes one labels them `is`, `is not` and `contains`. Environment and Time Range declare no `operators`, so the modal draws them as `is` with one value.
- **Values.** Level, Instance, Method, Status Code, Host and Path list the values Render's `logValues(input: {label, logQuery: {project, ownerId, region, …}})` answers for the project, many at a time.
- **The saved document** is `{"version": 1, "filters": [{"field", "operator", "values"}], "timeRange": {"relative": "<preset>"}, "display": {…}}`. The time range is not a filter row: it is the top-level `timeRange.relative`. `display` holds a metrics view's `aggregationMethod`, `requestGroupBy`, `groupBy` and `rootTasksOnly`.
- **The enums.** The saved-search filter field is `ENVIRONMENT | HOST | INSTANCE | LEVEL | METHOD | PATH | RESOURCE | RESOURCE_TYPE | RUN_ID | STATUS | STATUS_CODE | TASK | TASK_TYPE | TEXT_SEARCH | VERSION` (`RUN_ID`, `STATUS`, `TASK`, `TASK_TYPE` and `VERSION` serve Render Workflows, a non-goal: the Workflow Runs view, `ProjectWorkflowRuns-*.js`, filters on `RUN_ID`, `STATUS` and `VERSION`). Its operator is `CONTAINS | IS | IS_NOT`. The log query's own filter operator is `INCLUDES | EXCLUDES`.
- **bex's document.** Until [w3/083](../../.pm/w3/done/083.md) (2026-11-20) bex spelled every field Render's way but two and wrote no `display`: Text Search was `TEXT` with `IS`, not `TEXT_SEARCH` with `CONTAINS`, and the time range a `TIME_RANGE` filter row, not `timeRange.relative`. bex now stores and answers Render's shape, keeps `display` as written, and still reads its earlier spellings ([ADR010 § Project views](../ADR010-observability.md#project-views-one-stream-for-a-whole-project-w3m505-2026-11-19)).

## Service Metrics (re-checked 2026-11-19, w3/m508)

Render's own page could not be looked at again: the capture above is the last read of the reference workspace, and it hid the view (no sidebar entry, no New View option, `/project/{prj}/observability/metrics?savedSearchId=…` redirecting to `/`). Everything below comes from Render's docs and its dashboard bundle, read 2026-11-19.

- **Docs.** [render.com/docs/projects](https://render.com/docs/projects) § View logs, metrics, and task runs lists **Service Metrics** among the default views, under **Observability** in the left pane of a project or environment: "View and aggregate service metrics". [render.com/docs/service-metrics](https://render.com/docs/service-metrics) § Metrics across a project or environment: "Projects and environments display metrics for all of their services on the Service Metrics page." Services must be in the same region to appear. Neither page names the page's charts, its group-by or a plan gate for the page itself.
- **Charts.** The docs name the service metrics only per service: CPU and memory (every service but static sites), HTTP requests and response times (web services; response times on Pro and higher), outbound bandwidth (every service type), and database activity for Postgres and Key Value. Render's per-service Metrics page draws CPU, Memory, Total Instances, Total Requests, Response Times and Outbound Bandwidth ([metrics-page.md](metrics-page.md)).
- **Group-by.** The bundle's metrics-view group-by is `NONE | RESOURCE | STATUS_CODE`. A view's filter document is the same `{filters, version}` as a log view's, and the default `METRICS` view's is empty.
- **What bex builds from it (w3/m508).** One page per `METRICS` view at Render's route, listed in the sidebar and offered by New View: CPU, Memory and Total Instances (one line per resource, its instances added up), Total Requests (group by resource, status code or none), Response Times (one line per resource, Pro and higher) and Outbound Bandwidth, every resource the view's filters keep in one read. Which chart reads which resource type, and every choice the docs leave open, is recorded in [ADR010 § Project views](../ADR010-observability.md) and the [ADR018](../ADR018-render-parity.md) Projects row. Unverified until Render shows the page: its chart set, where its group-by sits, and whether Postgres and Key Value appear on it.

## Levels (re-read 2026-11-20, w3/086)

Read-only, from the same signed-in workspace's private GraphQL and dashboard bundle (`Logs-*.js`, `log-filters-*.js`), for w3/086.

- **Values.** `logValues(input: {label: LEVEL, logQuery: {project, ownerId, region, filters: [LOG_TYPE], filterDocument, …}})` answers `["debug","info","notice","warning","error","critical","alert","emergency"]` for `app` and for `request` logs, in two projects, whatever they logged: the eight [render.com/docs/logging](https://render.com/docs/logging) lists for the level mark and the `level` filter. The search box's `level:` examples are the same eight (`info, debug, warning, error, notice, critical, alert, emergency`).
- **Lines.** A JSON line with `"level":"warn"` carries `LEVEL` `warning`; one with `"level":"info"` and the word `fatal:` in its message carries `info`. No line in the 30 days read carried `notice`, `critical`, `alert` or `emergency`. A line whose `LEVEL` is not one of the eight is drawn as `info`.
- **Marks.** The level mark is the info icon for `info` and `notice`, a debug icon for `debug`, the warning icon for `warning` and the error icon for `error`, `critical`, `alert` and `emergency`. Its colour is info's purple for `info`, debug's grey (`--logs-v2-log-line-icon-debug`, `#8f8f8f` light, `#c7c7c7` dark) for `debug` and `notice`, orange for `warning` and red for the error four. The `debug`, `info` and `notice` marks show only while the row is hovered or highlighted. Rows take debug's, info's, warning's or error's tint by the same grouping, `notice` with `debug`.

## The log query

The page asks Render's private GraphQL for the whole project in one query, not one per service:

```graphql
query logs($query: LogQueryInput!) {
  logs(query: $query) {
    logs {
      id
      labels {
        label
        value
      }
      timestamp
      text
    }
    nextEndTime
    nextStartTime
    hasMore
  }
}
```

```json
{
  "query": {
    "start": "2026-11-19T07:15:26.843Z",
    "end": "2026-11-19T08:15:26.843Z",
    "filters": [
      { "field": "LOG_TYPE", "operator": "INCLUDES", "values": ["app"] }
    ],
    "ownerId": "tea-…",
    "pageSize": 50,
    "region": "oregon",
    "project": "prj-…",
    "filterDocument": { "filters": [], "version": 1 },
    "direction": "BACKWARD"
  }
}
```

- The view's filter document travels as `filterDocument`; the server resolves the project's resources.
- Each line's labels are a subset of `SERVICE`, `DEPLOY`, `INSTANCE`, `LEVEL` and `LOG_TYPE`, in that order. A cron run's own `==> Cron job run started` line has only `SERVICE` and `LOG_TYPE`.
- 50 lines a page; `hasMore` with `nextStartTime`/`nextEndTime` for the next page.

## What bex had before w3/m505 (2026-11-19)

Per-service Logs and Metrics pages, and datastore Logs and Metrics tabs. The project sidebar (`dashboard/src/common/components/dashboard-layout/project-sidebar.tsx`) has only Overview and Settings, and the project routes are only `project.$projectId.index.tsx` and `project.$projectId.settings.tsx`. REST `GET /v1/logs` and `/v1/logs/subscribe` already take up to 20 `resource` values (`maxLogResources`); GraphQL `logs` takes one `resource`; GraphQL `metrics` already fans out over several `RESOURCE` values (`checkFanOut`).
