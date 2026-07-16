# Managed Postgres logs contract

**Captured:** 2026-07-15 · **Implemented:** w3/m28

## Source evidence

Render's current [List logs API](https://api-docs.render.com/reference/list-logs) uses one repeatable `resource` filter for every log-producing resource and explicitly includes Postgres alongside servers, cron jobs, jobs, Key Value, and workflows. Its documented controls include `startTime`, `endTime`, `direction`, `resource`, `instance`, `text`, and `limit`. Render's [logging guide](https://render.com/docs/logging) shows a default recent window, time-range selection, search, and per-instance filtering. The [managed Postgres guide](https://render.com/docs/postgresql-creating-connecting) states that database and slow-query output is available in logs; queries longer than two seconds are logged by default.

The 2026-07-15 live dashboard walk captured a dedicated Postgres **Logs** tab. The baseline screenshots are `render-walk-postgres-logs.png` and `bex-walk-postgres.png` in the walk capture set; at that point bex had no database-log source or tab. After implementation, a fresh Chrome capture against the deterministic local bex fixture is `.playwright-mcp/bex-walk-postgres-logs-current.png`: it shows the directly linked active Logs tab, Postgres-attributed lines, Last hour, All instances, and search controls. Backend adapter tests and dashboard component/route tests cover the nonvisual behavior; the shared walk cluster was not an implementation dependency.

## bex contract

- REST `GET /v1/logs?resource=dpg-…`, GraphQL `logs(resource: "dpg-…")`, and MCP `list_logs(resource: ["dpg-…"])` all call the same database-scoped core query. The dedicated Postgres REST, GraphQL, and MCP compatibility adapters remain available and delegate typed ids to that same production core.
- Supported Postgres filters are RFC3339 start/end time, direction, text search, instance, and limit. Service/request concepts (`type`, level, host, HTTP method/status/path, build, and pre-deploy) return a named 400 instead of being ignored.
- Results are oldest-first after the chosen direction applies the limit, exactly like service logs. Each line carries `resource=dpg-…`, `type=postgres`, the CNPG pod as `instance`, and `container=postgres` when supplied by the source.
- The dashboard detail route owns its selected tab in the URL: `/databases/<dpg-id>?tab=logs`. Its controls are Last hour / 6 hours / 24 hours, observed CNPG instance, and debounced text search. Loading, empty, filtered empty, unauthorized, unconfigured-source, and upstream-error states are distinct.

## Attribution and authorization invariants

1. `AuthorizeDatabase(ctx, can_view_logs, dpg-id)` completes before a Loki or Kubernetes pod selector is built. The Database CR's tenant label, not its namespace or display name, selects the authorization object.
2. The durable selector is exact equality on `{namespace="…", database="dpg-…"}`. `database` comes from the already authorized Database CR's immutable metadata name, never a free-form caller selector.
3. The live fallback selects pods by CNPG's exact `cnpg.io/cluster=dpg-…` label and reads only the `postgres` container.
4. The operator stamps `app.bex.co/component=database` into tenant Database CNPG pod metadata. Alloy requires that marker before ingesting the stream, so platform/auth/control-plane CNPG clusters are excluded. It derives the Loki `database` label from `cnpg.io/cluster`; it never uses the mutable display name.
5. Tests seed two database pod sets and prove one query cannot mix the other, and prove REST, GraphQL, and MCP reject a foreign-workspace database before the history source is called.

## Durability and degraded behavior

With `BEX_LOKI_URL`, the standard seven-day Loki history survives CNPG pod restarts. Without Loki, bex reads the current CNPG pod log buffer: search, range, direction, limit, and instance still work, but restarted-pod history is gone. With neither history nor pod-log source the APIs return 503 `logs source not configured`, and the dashboard explains that the installation has no database-log source. An unreachable configured Loki surfaces its error; bex does not silently switch sources and disguise an outage.

Live subscription remains App/build-only in w3/m28. Render's captured Postgres requirement was historical viewing with range/search/instance controls; bex does not route a `dpg-…` request through the App SSE path or pretend it is live.
