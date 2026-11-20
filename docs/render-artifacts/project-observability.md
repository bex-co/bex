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
  - Options: **Copy logs** (`⌘ ⇧ C`), **Order by** Ascending / Descending, **Display: Expand everything**, and a link to Theme Settings.
  - Region: the workspace's active regions (`Oregon (US West)`, `Global`), from `activeRegions(ownerId)`.
  - Search: focusing the box lists **Add a filter** suggestions, `level:` (`info, debug, warning, error, …`) and `instance:` (the instances seen), with a **Syntax tips** link. Typing narrows them. There is no service filter in the box; a view narrows services through its own filters.
- **Lines:** a date separator row (`Nov 19`), then per line a level icon gutter (the info icon appears only on hover; a warning line shows a warning icon and a tinted row), the time (`12:15:03 AM`), a colored dot and `[service name]` (one color per service), the short instance (`[rstgn]`) when there is one, then the message. Lines from web services, cron jobs and the Postgres database interleave in time order.
- **Request lines:** `[GET]`, the status as a colored chip (`200` green, `404` orange), the host and path, then the message fields (`clientIP`, `requestID`, `responseTimeMS`, `responseBytes`, `userAgent`).
- **Live tail:** a **New logs in Live Tail** toast, and a **Return to Bottom** button. Hovering a row highlights it and shows a row menu (…).

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
- **Where bex's document differs.** bex spells every field Render's way but two, and writes no `display`: Text Search is `TEXT` with `IS`, not `TEXT_SEARCH` with `CONTAINS`, and the time range is a `TIME_RANGE` filter row, not `timeRange.relative`. [w3/083](../../.pm/w3/083.md) moves bex to Render's shape.

## Service Metrics (re-checked 2026-11-19, w3/m508)

Render's own page could not be looked at again: the capture above is the last read of the reference workspace, and it hid the view (no sidebar entry, no New View option, `/project/{prj}/observability/metrics?savedSearchId=…` redirecting to `/`). Everything below comes from Render's docs and its dashboard bundle, read 2026-11-19.

- **Docs.** [render.com/docs/projects](https://render.com/docs/projects) § View logs, metrics, and task runs lists **Service Metrics** among the default views, under **Observability** in the left pane of a project or environment: "View and aggregate service metrics". [render.com/docs/service-metrics](https://render.com/docs/service-metrics) § Metrics across a project or environment: "Projects and environments display metrics for all of their services on the Service Metrics page." Services must be in the same region to appear. Neither page names the page's charts, its group-by or a plan gate for the page itself.
- **Charts.** The docs name the service metrics only per service: CPU and memory (every service but static sites), HTTP requests and response times (web services; response times on Pro and higher), outbound bandwidth (every service type), and database activity for Postgres and Key Value. Render's per-service Metrics page draws CPU, Memory, Total Instances, Total Requests, Response Times and Outbound Bandwidth ([metrics-page.md](metrics-page.md)).
- **Group-by.** The bundle's metrics-view group-by is `NONE | RESOURCE | STATUS_CODE`. A view's filter document is the same `{filters, version}` as a log view's, and the default `METRICS` view's is empty.
- **What bex builds from it (w3/m508).** One page per `METRICS` view at Render's route, listed in the sidebar and offered by New View: CPU, Memory and Total Instances (one line per resource, its instances added up), Total Requests (group by resource, status code or none), Response Times (one line per resource, Pro and higher) and Outbound Bandwidth, every resource the view's filters keep in one read. Which chart reads which resource type, and every choice the docs leave open, is recorded in [ADR010 § Project views](../ADR010-observability.md) and the [ADR018](../ADR018-render-parity.md) Projects row. Unverified until Render shows the page: its chart set, where its group-by sits, and whether Postgres and Key Value appear on it.

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
