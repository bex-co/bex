# w4 · m378 — Fifteen of Render's shared MCP tools answer in bex's own shapes, not what Render's server returns: list envelopes, `list_logs`/`get_metrics` series, service creates without `deployId`, and datastores without `owner`/`options`

**Worker:** worker4 **Goal:** every MCP tool that bex shares with Render's official server (`render-oss/render-mcp-server`) returns the text that server returns for the same call. A Render-trained agent, prompt or skill parses bex's answers without a special case, and a test pins each tool's result shape so it cannot drift again. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | List tools answer in Render's text: `list_services`, `list_postgres_instances`, `list_key_value`, `list_workspaces` and `list_log_label_values` as a bare JSON array, and `list_deploys` as an array plus `\n\n cursor: <cursor>` | 45m | — |
| [t002](t002.md) | `list_logs` returns Render's (REST's) line shape: `id`, `labels` as `[{name, value}]` with `resource`, `message`, `timestamp` | 30m | — |
| [t008](t008.md) | `get_metrics` returns Render's `MetricsResponse` (`resourceId`, `timeRange`, `metrics[{type, data}]`, with REST's `labels[{field, value}]` and `values`), not bex's `{series:[{labels:{…}, points}]}` | 30m | — |
| [t009](t009.md) | `create_web_service`, `create_static_site` and `create_cron_job` return Render's `{service, deployId}` (`ServiceAndDeploy`), as REST already does, not the bare service | 20m | — |
| [t010](t010.md) | Key Value and Postgres tools (`list_key_value`, `get_key_value`, `create_key_value`, `list_postgres_instances`, `get_postgres`, `create_postgres`) return REST's Render-shaped detail: an `owner` object, and Key Value's `options`, not bex's flat view | 40m | — |
| [t003](t003.md) | Pin each shared tool's result shape against Render's server, so a wrapped or re-keyed answer fails CI | 45m | t001, t002, t008, t009, t010 |
| [t004](t004.md) | Render parity | 20m | t003 |
| [t005](t005.md) | Simplify | 15m | t004 |
| [t006](t006.md) | Test coverage | 30m | t004 |
| [t007](t007.md) | Closeout (live replay on production) | 20m | t005, t006 |

## Definition of done

All on production, workspace `bex-canary`. Use streamable HTTP to `https://api.bex.co/mcp`, with a session cookie or an API key, as in § Finding.

1. **Lists are Render's text.** Each `tools/call` result's first `text` content parses as shown:
   - `list_services {}`, `list_postgres_instances {}`, `list_key_value {}`, `list_workspaces {}` and `list_log_label_values {"resource":[<id>],"label":"type"}`: a JSON **array**;
   - `list_deploys {"serviceId":<id>,"limit":2}`: a JSON array followed by `\n\n cursor: <cursor>`.

   Today each is an object (`{"services":[…]}`, `{"postgres":[…]}`, `{"keyValues":[…]}`, `{"workspaces":[…]}`, `{"values":[…]}`, `{"cursor":…,"deploys":[…]}`).
2. **Log lines are Render's.** `list_logs {"resource":[<id>],"limit":2,"type":["build"]}` returns the REST envelope. Each entry carries `id` and `labels: [{"name":"resource","value":<id>}, {"name":"instance",…}, {"name":"type","value":"build"}]`, byte-for-byte what `GET /v1/logs` returns for the same window.
3. **Metrics are Render's.** `get_metrics {"resourceId":<id>,"metricTypes":["cpu_usage","http_request_count"]}` returns `{"resourceId":<id>,"timeRange":{…},"metrics":[{"type":"cpu_usage","data":[{"labels":[{"field":"resource",…}],"unit":"cpu","values":[{"timestamp":…,"value":…}]}]},…]}`. The `data` items equal `GET /v1/metrics/cpu?resource=<id>` for the same window, and an `aggregateHttpRequestCountsBy: statusCode` series is labeled `statusCode`, not `code`.
4. **Creates carry the deploy.** `create_web_service {"name":"qa-…","runtime":"go","repo":"https://github.com/bex-co/bex","branch":"main","buildCommand":…,"startCommand":…,"plan":"free"}` returns `{"service":{…},"deployId":"dep-…"}`, the same envelope as `POST /v1/services`. `create_static_site` and `create_cron_job` do the same.
5. **Datastores are Render's details.** `get_key_value`, `create_key_value` and every `list_key_value` item carry `owner: {id, name, email, type}` and `options: {maxmemoryPolicy, persistenceMode}`, as `GET /v1/key-value/{id}` does, with no flat `ownerId`/`maxmemoryPolicy`/`persistenceMode`. `get_postgres`, `create_postgres` and every `list_postgres_instances` item carry the `owner` object.
6. **Unchanged:**
   - `list_events` keeps its array-plus-cursor text;
   - `get_service`, `get_deploy` and `update_environment_variables` keep their current text, which already matches;
   - bex-only tools are not touched.
7. **Guarded:** the t003 test fails when any of these tools' text is rewrapped in an object or a log label is renamed.
8. **Fixtures:** every fixture is deleted, and each GET answers 404.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs w4` loop, round 295, 2026-11-13 13:41–14:00 UTC (`muse.env`, workspace `bex-canary`; pin `5de3d5bcce8e`). The probes are under § Finding.
- **Governing docs:**
  - [ADR018](../../../docs/ADR018-render-parity.md) § "MCP parity is pinned and measured" (w1/m70). The pin, `lego/backend/internal/api/openapi/render-mcp-tools.json`, `render_mcp.go`, records tool names, argument names, types and annotations. It excludes result shapes, so nothing checks them.
  - [ADR006](../../../docs/ADR006-bex-api.md): REST, GraphQL and MCP share one core.
- **Goal linkage:** ADR008's AI-native goal and Render parity. Render's own MCP server is the contract agents and the `render-oss/skills` pack are written against. bex ships that pack as its CLI skills (`lego/cli/internal/skills/pack`), so a skill that reads Render's `labels` array or a bare service array meets a different shape on bex.
- **Expected outcome:** a Render-trained agent reads bex's MCP answers unchanged. The one place bex already got this right, `list_events`, becomes the rule rather than the exception.
- **Why now:** found live, cheap to fix, and widening. Every new shared tool copies whichever envelope its neighbour uses, and no test tells them apart.
- **Render parity task included (t004):** the MCP surface changes.

## Finding (round 295)

- **Fixture:** Free web service `qa-20261113-r295-mcp` (`srv-dbrh7b1ens8c73fghnrg`), `docker.io/library/busybox:1.37`. Deleted afterwards (`DELETE` 204, then `GET` 404).
- **Transport:**
  - `POST https://api.bex.co/mcp` with the dashboard session cookie (`initialize` → `protocolVersion 2025-06-18`, stateless, no `Mcp-Session-Id`). Then `tools/call`.
  - `tools/list` returned 249 tools. Every Render tool except the Workflows family and `select_workspace`/`get_selected_workspace` is present, and both of those are recorded non-goals.
- **What bex returned:** the first `text` content of each result, parsed.

  | tool | bex text (top level) | Render's server (source, `main`, read 2026-11-13) |
  | --- | --- | --- |
  | `list_services` | object `{"services":[…]}` | `json.Marshal(response)`, `response` being `[]*client.Service`: a bare array (`pkg/service/tools.go:62-67`, `repo.go:32-41`) |
  | `list_deploys` | object `{"cursor":…,"deploys":[…]}` | `json.Marshal(deploys)` + `"\n\n cursor: "` + cursor (`pkg/deploy/tools.go:73-90`) |
  | `list_postgres_instances` | object `{"postgres":[…]}` | `json.Marshal(postgres)`, a bare array (`pkg/postgres/tools.go:52`) |
  | `list_key_value` | object `{"keyValues":[…]}` | `json.Marshal(keyValues)`, a bare array (`pkg/keyvalue/tools.go:48`) |
  | `list_workspaces` | object `{"workspaces":[…]}` | `json.Marshal(workspaces)`, a bare array (`pkg/owner/tools.go:44`) |
  | `list_log_label_values` | object `{"values":["request"]}` | `json.Marshal(values)`, `[]string` (`pkg/logs/tools.go`, the `listLogLabelValues` handler) |
  | `list_logs` | `{hasMore, logs:[{labels:{"instance":…,"service":…,"type":"build"}, message, timestamp}], nextEndTime, nextStartTime}`, with no `id` | `json.Marshal(logs.JSON200)`, the REST `Logs200Response` (`pkg/logs/tools.go:248-253`, `repo.go:17-28`) |
  | `list_events` (control) | `[…]\n\n cursor: MjAy…` | the same (`pkg/event/tools.go:110-122`) |
  | `get_service` (control) | the REST service object, the same keys as `GET /v1/services/{id}` minus `owner` | `json.Marshal` of the service |

  For the same window, `GET /v1/logs?resource=srv-dbrh7b1ens8c73fghnrg&type=build&limit=2` returned:

  ```
  {"id":"srv-dbrh7b1ens8c73fghnrg-rs33m9r4o4983kig4c4n-2026-11-13T13:45:05.954875Z-78da67b1","message":"==> Your service is live 🎉","timestamp":"2026-11-13T13:45:05.954875Z","labels":[{"name":"resource","value":"srv-dbrh7b1ens8c73fghnrg"},{"name":"instance","value":"srv-dbrh7b1ens8c73fghnrg-rs33m9r4o4983kig4c4n"},{"name":"type","value":"build"}]}
  ```

  MCP `list_logs` returned the same line as:

  ```
  {"labels":{"instance":"srv-dbrh7b1ens8c73fghnrg-rs33m9r4o4983kig4c4n","service":"srv-dbrh7b1ens8c73fghnrg","type":"build"},"message":"==> Your service is live 🎉","timestamp":"2026-11-13T13:45:05.954875Z"}
  ```

- **Root cause:**
  - The six list tools return a typed `Out` struct (for example `listLogLabelValuesResult{Values}`, `lego/backend/internal/logs/mcp.go:90-94`) with a nil `CallToolResult`. go-sdk v1.8.0 (`lego/backend/go.mod:23`, `mcp/server.go:440-485` in the module cache) then marshals `Out` into `StructuredContent` and copies the same JSON into the only `TextContent`. So the text is bex's struct.
  - The comment at `logs/mcp.go:90-91` gives the reason: "Render's tool returns a bare array; MCP tool results are objects, so bex names the field". That no longer holds at this SDK version, which accepts array structured output with a text fallback (the SEP-2106 comment at `server.go:473-476`). bex's own `list_events` (`lego/backend/internal/events/mcp.go:148-155`) already returns Render's text by setting `Content` itself.
  - `list_logs` returns `listLogsResult{Logs []LogEntry}` (`logs/mcp.go:83-88`). That is bex's internal `LogEntry`, whose labels are a map keyed by the core's `service`. The REST adapter maps it to Render's `resource` and to an ordered `[{name, value}]` with an `id` (`lego/backend/internal/logs/render.go:46-60`). `mcpLogEntries` (`logs/mcp.go:203-215`) filters keys but keeps the map.
  - The tools live in `apps/mcp.go:665` (`list_services`), `deploys/mcp.go:117` (`list_deploys`), `postgres/mcp.go:108` (`list_postgres_instances`), `keyvalue/mcp.go:109` (`list_key_value`), `workspaces/mcp.go:52` (`list_workspaces`) and `logs/mcp.go:138`/`:173` (`list_logs`, `list_log_label_values`).

## Round 296 extension — `get_metrics` has bex's shape too (2026-11-13 13:52–13:58 UTC, pin `5de3d5bcce8e`)

- **Fixture:** Free web service `qa-20261113-r296-met` (`srv-dbrhc2pens8c73fghodg`), `busybox:1.37`, with 30 requests sent. Deleted afterwards (`DELETE` 204, then `GET` 404).
- **bex `get_metrics`:** `{"resourceId":"srv-dbrhc2pens8c73fghodg","metricTypes":["cpu_usage","http_request_count","instance_count"]}` returned:

  ```
  {"series":[{"labels":{"instance":"srv-…-m70f97s6mj0lje4rr4eq","metric":"cpu_usage","resource":"srv-dbrhc2pens8c73fghodg"},"points":[{"timestamp":"2026-11-13T13:53:00Z","value":0.00025…},…],"unit":"cpu"},{"labels":{"metric":"http_request_count",…},"points":[…],"unit":"count"},{"labels":{"metric":"instance_count",…},…}]}
  ```

  With `aggregateHttpRequestCountsBy: "statusCode"`, each series is labeled `"code":"200"`.
- **REST, the same service:**
  - `GET /v1/metrics/cpu?resource=…` → `[{"labels":[{"field":"instance",…},{"field":"resource",…}],"unit":"cpu","values":[{"timestamp":…,"value":…}]}]`.
  - `GET /v1/metrics/http-requests?…&aggregateBy=statusCode` labels `{"field":"statusCode","value":"200"}`.
- **Render's server:** `json.Marshal(response)` of `MetricsResponse{ResourceID, TimeRange{Start, End}, Metrics []MetricData{Type, Data TimeSeriesCollection}}`. See `pkg/metrics/tools.go:233-238` and `pkg/metrics/repo.go:64-75`. `Data` is REST's time-series collection.
- **Root cause:** `getMetricsResult{Series []MetricSeries}` (`lego/backend/internal/metrics/mcp.go:73-79`) is returned through the typed path. That is the same go-sdk mechanism as § Finding: the internal `MetricSeries` (a labels map, `points`, the core's `code` label) becomes the text.
  - The tool's own description (`mcp.go:171-172`) says "as Render-shaped time-series".
- **Added:** task t008. t003 now also depends on it.

## Round 297 extension — create tools drop `deployId` (2026-11-13 14:07–14:12 UTC, pin `5de3d5bcce8e`)

- **Fixture:** `qa-20261113-r297-mcpc` (`srv-dbrhj91ens8c73fghp60`), created through MCP `create_web_service` (Go, `bex-co/bex` main, Free). Deleted afterwards (`DELETE` 204, then `GET` 404).
- **bex:**
  - The call returned the bare service object (keys `autoDeploy … updatedAt`, `id: srv-dbrhj91ens8c73fghp60`), with no `service` wrapper and no `deployId`.
  - Its first deploy existed (`dep-dbrhj91ens8c73fghp6g`, `bexTrigger: create`), but an agent had to call `list_deploys` to find it.
- **Render's server:** `create_web_service` returns `json.Marshal(response)` of `CreateService`'s `*client.ServiceAndDeploy` (`pkg/service/tools.go:200-210`, `repo.go:124-135`), that is `{service, deployId}`.
- **bex REST:** the create already answers that envelope: `serviceAndDeploy{Service, DeployID: app.LatestDeployID}` at `lego/backend/internal/apps/rest.go:1247`.
- **Root cause:** `renderServiceResult` (`lego/backend/internal/apps/mcp.go:1108-1114`) returns `toRenderService(app)` alone. It has 3 create callers (`create_web_service` `:692`, `create_cron_job` `:703`, `create_static_site` `:714`), so all three drop the deploy id.
- **Control:** `trigger_deploy` returns the deploy object, as Render's does (`pkg/deploy/tools.go:177-191`).
- **Added:** task t009. t003 now also depends on it.

## Round 298 extension — datastore tools return bex's flat views (2026-11-13 14:22–14:30 UTC, pin `5de3d5bcce8e`)

- **Fixtures:**
  - Free Key Value `qa-20261113-r298-kv` (`red-dbrhq80k58is73auob2g`), created with MCP `create_key_value`;
  - Free Postgres 18 `qa-20261113-r298-pg` (`dpg-dbrhq81ens8c73fghph0`), created with `create_postgres`, which defaulted to `0.0.0.0/0` and `::/0`, as Render's Postgres does.

  Both were deleted afterwards (`DELETE` 204, then `GET` 404).
- **Key Value:**
  - MCP `get_key_value`, `create_key_value` and the `list_key_value` items have keys `createdAt, dashboardUrl, id, internalAuth, ipAllowList, maxmemoryPolicy, name, ownerId, persistenceMode, plan, public, region, status, suspended, updatedAt, version`. `list_key_value` items add `blueprintId` and `projectId`, and `create_key_value` adds `externalAccessRestarts`.
  - REST `GET /v1/key-value/{id}` has `owner` and `options` in their place.
  - Render's `keyValueDetail` requires `owner` and `options`. Render's server returns `json.Marshal(keyValue)` of that client type (`pkg/keyvalue/tools.go:85`, `:182`).
- **Postgres:**
  - MCP `get_postgres`, `create_postgres` and the list items have `ownerId` and no `owner`.
  - REST has both. Render's `postgresDetail` requires `owner` (`pkg/postgres/tools.go:89`, `:193`).
- **Root cause:**
  - The tools return `KeyValueView`/`PostgresView` directly (`lego/backend/internal/keyvalue/mcp.go:109-140`, `lego/backend/internal/postgres/mcp.go:108-140`).
  - REST maps through `renderKeyValue` (`lego/backend/internal/keyvalue/rest.go:45-…`). That mapper's own comment says bex's flat `ownerId`/`maxmemoryPolicy`/`persistenceMode` is "the shape GraphQL/MCP read directly off KeyValueView" and "silently zero-valued or failed to decode client-side because Render's real contract nests owner/options".
  - ADR018:423 records that REST fix (2026-07-15). MCP was left on the old shape.
- **Added:** task t010. t003 now also depends on it.

## Unverified

- **Render's server at its pinned commit.** The pin (`render-mcp-tools.json` `pin`) is `main` at `23fb336` (2026-11-12). The formatting above was read on `main` a day later, and t003's capture must use the pinned commit.
- **Whether a client reads `structuredContent` rather than the text.** Render's server sends no structured content, so the text is the contract. t001 decides whether bex keeps a structured object beside Render's text.
- **The other shared tools** (`get_postgres`, `get_key_value`, `get_metrics`, `list_logs`' `hasMore` paging, the create tools) were not compared field by field this round. t003 covers them.
