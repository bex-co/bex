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

- `SavedSearchType` is `APPLICATION_LOGS | METRICS | REQUEST_LOGS | WORKFLOW_RUNS`; the filter operators are `IS | IS_NOT`; a metrics group-by is `NONE | RESOURCE | STATUS_CODE` (all from the bundle).
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

- `Environment` takes the operator `is` and one environment.
- `Resource Type` takes `is one of` / `is not one of` over **Services, Cron jobs, Postgres, Key Value**. Key Value and Postgres logs belong to the project's Application Logs.

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

## What bex has (2026-11-19)

Per-service Logs and Metrics pages, and datastore Logs and Metrics tabs. The project sidebar (`dashboard/src/common/components/dashboard-layout/project-sidebar.tsx`) has only Overview and Settings, and the project routes are only `project.$projectId.index.tsx` and `project.$projectId.settings.tsx`. REST `GET /v1/logs` and `/v1/logs/subscribe` already take up to 20 `resource` values (`maxLogResources`); GraphQL `logs` takes one `resource`; GraphQL `metrics` already fans out over several `RESOURCE` values (`checkFanOut`).
