# Managed Key Value logs contract

**Captured:** 2026-07-16 · **Implemented:** w3/m30

## Source evidence

Render's current [List logs API](https://api-docs.render.com/reference/list-logs) uses one repeatable `resource` filter for every log-producing resource and explicitly lists Key Value (`red-…`) alongside servers, cron jobs, jobs, Postgres, and workflows. Its documented controls include `startTime`, `endTime`, `direction`, `resource`, `instance`, `text`, and `limit`. Render's [logging guide](https://render.com/docs/logging) shows a default recent window, time-range selection, search, and per-instance filtering. The [managed Key Value guide](https://render.com/docs/redis) states that Valkey store output is available in logs.

The same artifact that documented Postgres logs (`docs/render-artifacts/postgres-logs.md`) covers Key Value: Render's List-logs API routes `resource=red-…` through the same endpoint and contract as `resource=dpg-…`. bex mirrors that unification — one shared generic query, one dedicated compatibility API.

## bex contract

- REST `GET /v1/logs?resource=red-…`, GraphQL `logs(resource: "red-…")`, and MCP `list_logs(resource: ["red-…"])` all call the same keyvalue-scoped core query. The dedicated Key Value REST, GraphQL, and MCP compatibility adapters remain available and delegate typed ids to that same production core.
- Supported Key Value filters are RFC3339 start/end time, direction, text search, instance, and limit. Service/request concepts (`type`, level, host, HTTP method/status/path, build, and pre-deploy) return a named 400 instead of being ignored.
- Results are oldest-first after the chosen direction applies the limit, exactly like service logs and Postgres logs. Each line carries `resource=red-…`, `type=keyvalue`, the Valkey pod as `instance`, and `container=valkey` when supplied by the source.
- The dashboard detail route owns its selected tab in the URL: `/key-value/<red-id>?tab=logs`. Its controls are Last hour / 6 hours / 24 hours, observed Valkey instance, and debounced text search. Loading, empty, filtered empty, unauthorized, unconfigured-source, and upstream-error states are distinct.

## Attribution and authorization invariants

1. `AuthorizeKeyValue(ctx, can_view_logs, red-id)` completes before a Loki or Kubernetes pod selector is built. The KeyValue CR's tenant label, not its namespace or display name, selects the authorization object.
2. The durable selector is exact equality on `{namespace="…", keyvalue="red-…"}`. `keyvalue` comes from the already authorized KeyValue CR's immutable metadata name, never a free-form caller selector.
3. The live fallback selects pods by the operator-stamped `app.bex.co/keyvalue=red-…` label and reads only the `valkey` container.
4. The operator stamps `app.bex.co/keyvalue=<name>` into tenant Valkey pod metadata. Alloy requires that marker before ingesting the stream, so platform/infrastructure Valkey instances without that label are excluded. It derives the Loki `keyvalue` label from `app.bex.co/keyvalue`; it never uses the mutable display name.
5. Tests seed two key-value pod sets and prove one query cannot mix the other, and prove REST, GraphQL, and MCP reject a foreign-workspace key-value store before the history source is called.

## Durability and degraded behavior

With `BEX_LOKI_URL`, the standard seven-day Loki history survives Valkey pod restarts. Without Loki, bex reads the current Valkey pod log buffer: search, range, direction, limit, and instance still work, but restarted-pod history is gone. With neither history nor pod-log source the APIs return 503 `logs source not configured`, and the dashboard explains that the installation has no key-value log source. An unreachable configured Loki surfaces its error; bex does not silently switch sources and disguise an outage.

Live subscription remains App/build-only. Render's captured Key Value requirement was historical viewing with range/search/instance controls; bex does not route a `red-…` request through the App SSE path or pretend it is live.
