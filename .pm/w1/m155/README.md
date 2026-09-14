# w1 · m155 — Render's metrics contract is partly unserved on REST and MCP: `aggregateBy` is silently ignored, eight Render paths return 404, and MCP rejects Render's metric names

**Worker:** worker1 **Goal:** every metrics request shaped the way Render documents it gets either Render's answer or an explicit, coded refusal, never a bare `404` or a plausible wrong series.
- REST honors `aggregateBy`.
- REST serves Render's limit, filter, datastore and bandwidth-source paths, or each is an explicit divergence recorded in ADR018.
- MCP `get_metrics` accepts Render's metric-type names.

**Status:** todo

## Tasks (in order)

| id   | title                                                                                                             | est | depends_on                   |
| ---- | ----------------------------------------------------------------------------------------------------------------- | --- | ---------------------------- |
| t001 | REST honors Render's `aggregateBy` (per-status-code series; `host` refused with a code) and retires the dead `groupBy` | 30m | —                            |
| t002 | REST serves `/v1/metrics/cpu-limit` and `/v1/metrics/memory-limit`                                                 | 15m | —                            |
| t003 | REST serves `/v1/metrics/filters/{application,http,path}` from the existing filter verbs                           | 40m | —                            |
| t004 | Render's `active-connections`, `disk-usage` and `bandwidth-sources` paths: serve them or record the divergence     | 30m | —                            |
| t005 | MCP `get_metrics` accepts Render's `metricTypes` names                                                             | 25m | —                            |
| t006 | Blast radius: a composed-server test walking every Render `/metrics` path and parameter                            | 30m | t001, t002, t003, t004, t005 |
| t007 | Render parity                                                                                                     | 20m | t006                         |
| t008 | Simplify                                                                                                          | 15m | t007                         |
| t009 | Test coverage                                                                                                     | 40m | t007                         |
| t010 | Closeout                                                                                                          | 10m | t009                         |

## Definition of done

Run every call from a signed-in dashboard page (`fetch(…, {credentials:'include'})`) against a throwaway free web service that has served a mix of `200` and `404` requests in the last hour. Only states observed at filing time are listed:

- **REST `aggregateBy=statusCode` breaks requests down by status.** `GET /v1/metrics/http-requests?resource=<srv>&startTime=<now-1h>&endTime=<now>&resolutionSeconds=60&aggregateBy=statusCode` returns one series per status code, each carrying Render's status-code label.
  - At filing time it returned **one** series labelled only `resource=<srv>` (last value 0.533).
  - GraphQL `metrics(query:{name:"HTTP_REQUESTS", aggregateBy:["STATUS_CODE"], …})` over the same window returned three series, `code=200` (0.400), `code=404` (0.133) and `code=501` (0), so the data existed.
- **REST `aggregateBy=host` is never silently ignored.** It returns per-host series, or a coded `400` naming the unsupported breakdown, the way MCP's `aggregateHttpRequestCountsBy=host` already refuses. At filing time it returned the same single series.
- **The limit paths answer.** `GET /v1/metrics/cpu-limit?resource=<srv>&…` and `/memory-limit` return `200` with series.
  - At filing time both were `404` with text `404 page not found`.
  - The same metric already worked elsewhere: GraphQL `CPU_LIMIT` returned `unit: cpu`, value `0.1`, and MCP `get_metrics(metricTypes:["cpu_limit"])` returned a series.
- **The filter paths answer.** `GET /v1/metrics/filters/http`, `/filters/application` and `/filters/path` (`?resource=<srv>&…`) return Render's filter arrays, for example the `statusCode` values `200`, `404`. At filing time all three were bare `404`.
- **Datastore and bandwidth paths are served or on the record.** `GET /v1/metrics/active-connections`, `/disk-usage` and `/bandwidth-sources` either answer with Render's shape, or `docs/ADR018-render-parity.md` records each as a divergence with its reason and the bex path to use instead. At filing time all three were bare `404`, while ADR018:198 marks "cpu/mem limit & target · disk · active-connections" ✅ on REST.
- **MCP accepts Render's names.** `tools/call get_metrics {resourceId:<srv>, metricTypes:["cpu_usage"]}` returns a series. At filing time: `{"isError":true,"content":[{"text":"bad request: unknown metric \"cpu_usage\""}]}`. The same call with `["cpu"]` or `["cpu_limit"]` succeeded.
- **Controls stay green.** REST `cpu`, `instance-count` and `http-requests` without `aggregateBy` return what they returned at filing time (one series each). GraphQL `aggregateBy:["STATUS_CODE"]` keeps splitting by code. `aggregateBy=bogus` keeps its `400 invalid query parameter "aggregateBy"`.

## Evidence (probes run 2026-09-14, production, workspace `bex` / `tea-d98210cbbpdc73dcrkvg`)

- **Fixture:** `qa-20260914-shell` (`srv-dak50vq6m8ac739r6400`), a free web service built from `examples/hello-python` with Docker Command `python -u -m http.server $PORT`.
  - Created 19:52:34Z and live 19:54:19Z.
  - Traffic from 19:54:19: `GET /` every ~2 s, plus `GET /qa-missing-N` (a 404) every third iteration.
  - Deleted 19:57:50Z (`DELETE` → `204`, then `GET` → `404`).

```text
19:55:18Z GET /v1/metrics/{cpu-limit,memory-limit,filters/http,filters/application,filters/path,bandwidth-sources,active-connections,disk-usage}?resource=srv-dak50vq6m8ac739r6400&startTime=<now-1h>&endTime=<now>
          → 404 "404 page not found" (all eight)
          GET /v1/metrics/{cpu,instance-count,http-requests?aggregateBy=statusCode,http-latency?quantile=0.95} → 200 (controls)
19:55:36Z GraphQL metrics(query:{name:"CPU_LIMIT",filters:[{field:"RESOURCE",values:[srv]}],start,end}) → unit "cpu", value 0.1
19:56:37Z GET /v1/metrics/http-requests?…&resolutionSeconds=60&aggregateBy=statusCode → 200 [{labels:"resource=srv-…", last 0.533}]
          GET …&aggregateBy=host                                   → 200 [{labels:"resource=srv-…", last 0.533}]   (same single series)
          GET …&groupBy=status   /   …&groupBy=statusCode           → 400 "request contains an unsupported query parameter"
          GET …&statusCode=404                                     → 400 "request contains an unsupported query parameter"
          GET …&aggregateBy=bogus                                  → 400 "invalid query parameter \"aggregateBy\""
          GraphQL metrics(query:{name:"HTTP_REQUESTS",aggregateBy:["STATUS_CODE"],resolution:60,…})
                                                                   → code=200 (0.400) · code=404 (0.133) · code=501 (0)
~19:57Z   MCP POST https://api.bex.co/mcp initialize → 200; tools/call get_metrics
          {resourceId:srv, metricTypes:["cpu_usage"]} → isError "bad request: unknown metric \"cpu_usage\""
          {metricTypes:["cpu_limit"]} → series · {metricTypes:["cpu"]} → series
```

**Render's contract.**
- **REST** (`lego/backend/internal/api/openapi/render-public-api-1.json`): 21 paths under `/metrics`.
  - `/metrics/http-requests` takes `aggregateBy` with enum `statusCode, host`.
  - `/metrics/filters/http` is "List queryable status codes and host values", `/filters/application` is "List queryable instance values", and `/filters/path` is "List queryable paths".
  - `/metrics/bandwidth-sources` is "Get bandwidth usage breakdown by traffic source".
- **MCP** (`render-oss/render-mcp-server` `pkg/metrics/tools.go:138-141`): `get_metrics` `metricTypes` accepts `cpu_usage, memory_usage, http_request_count, active_connections, instance_count, http_latency, cpu_limit, cpu_target, memory_limit, memory_target, bandwidth_usage`.

## Root cause

- **REST reads the wrong parameter.** `parseMetricParams` reads `groupBy` (`lego/backend/internal/metrics/rest.go:172-190`, `:185`) and never `aggregateBy`.
  - The strict Render request validator refuses `groupBy`, because it is not in Render's spec: `hasUnknownRenderQuery` → `400 "request contains an unsupported query parameter"` (`lego/backend/internal/api/render_openapi.go:386-388`). The only REST group-by path is therefore unreachable.
  - Render's `aggregateBy` passes validation (the validator even enum-checks it) and reaches a handler that drops it.
  - `docs/ADR010-observability.md:188` still documents `statusCode`/`groupBy` as the REST params.
  - The group-by predates the strict validator: `w3/done/m4.5` mapped GraphQL `aggregateBy` "exactly like REST's `groupBy` param".
- **The limit metrics have no REST route.** `metricPaths` (`rest.go:35-47`) maps eight App segments: `cpu`, `memory`, `instance-count`, `http-requests`, `http-latency`, `bandwidth`, `cpu-target`, `memory-target`. `MetricCPULimit`/`MetricMemoryLimit` exist (`metrics/service.go:46`, `:48`) and GraphQL maps `CPU_LIMIT`/`MEMORY_LIMIT` (`graphql.go:40`, `:42`), but `cpu-limit`/`memory-limit` are unmapped.
- **Other Render paths fall through to a plain 404.** `datastoreMetricPaths` (`rest.go:49-56`) uses bex names: `disk`, `disk-capacity`, `db-connections`, `replication-lag`, `kv-memory`, `kv-connections`. A Render path with no bex handler falls through the validator to the mux's plain `404` (`render_openapi.go:361-366`), the same mechanism as `w1/093`.
- **Filters exist only on GraphQL.** `metricsFilters` (`graphql.go:345`) and `metricsPathFilterSuggestions` (`:362`) have no REST route.
- **MCP forwards names without mapping.** `get_metrics` documents bex ids only (`metrics/mcp.go:41`) and passes each name straight to `Metrics`, which returns `unknown metric %q` (`service.go:447`). No Render-name alias exists: grepping for `"cpu_usage"`, `"memory_usage"`, `"http_request_count"`, `"bandwidth_usage"` and `"active_connections"` across `lego/backend/internal` (non-test) finds 0 hits.
- **Why the pins missed it.** The MCP parity pin counts `get_metrics` as a Superset (`api/mcp_parity_test.go:269`, `w2/m91` "Render arg names"): it compares argument names, not the metric-type values.
- **Ledger claims.** ADR018:197 ("6 REST endpoints under `/v1/metrics/*`") and :198 (limit and active-connections ✅ on REST) claim more than production serves.

## Blast radius

- **Render's 21 `/metrics` paths**, against production:
  - **Served under Render's own name:** `cpu`, `memory`, `instance-count`, `http-requests`, `http-latency`, `bandwidth`, `cpu-target`, `memory-target`, `disk-capacity`, `replication-lag`. The last two were not probed with Render's parameter shape; bex documents `?resource=&kind=`.
  - **Bare `404`, probed:** `cpu-limit`, `memory-limit`, `filters/application`, `filters/http`, `filters/path`, `bandwidth-sources`, `active-connections`, `disk-usage`.
  - **Recorded non-goals:** `metrics-stream/{ownerId}` (`.pm/DO_NOT_DO.md:24`, external drains), and `task-runs-queued` / `task-runs-completed` (`DO_NOT_DO.md:26`, workflows).
- **Parameters.** `aggregateBy` is silently dropped. t006 walks every other Render parameter of the served paths (`service`, `instance`, `host`, `path`, `aggregationMethod`, `quantile`) to find any other accepted-but-ignored one.
- **MCP names.** Five of Render's MCP names are unmapped: `cpu_usage`, `memory_usage`, `http_request_count`, `bandwidth_usage`, `active_connections`. Only `cpu_usage` was probed; the rest were read from code.
- **Unaffected.** GraphQL and the dashboard (a GraphQL client) are not affected. They are the controls.

## Adjacent classes

- **`host` breakdown.** Traefik counters and the Loki request path have no host axis (`mcp.go:124-135`), so REST must refuse `aggregateBy=host` with the same coded message rather than answer as if it were ungrouped.
- **Label vocabulary.** GraphQL relabels status to `code` (`lokisource.go:143-155`); Render's REST label name for the breakdown must be confirmed from the spec and captures. Do not copy GraphQL's name onto REST.
- **Filters that need the log store.** Without Loki these answer `503`, as the logs filters do, never an empty list presented as "no values".
- **Tenancy.** Filters scope to the caller's workspace (`w3/done/m18/t004`'s `ownerId` decision). A resource in another workspace is `403`/`404` under the existing authz seam, never an existence oracle.
- **Datastore resources.** Render's `active-connections` and `disk-usage` take a Postgres or Key Value id with no `kind`, so a mapping must infer the kind from the id prefix (`dpg-`/`red-`) or refuse clearly.

## Unverified (reasoned, not probed this run)

- **Render's REST label name** for `aggregateBy=statusCode` series, and the exact response shapes of the `filters/*` and `bandwidth-sources` paths.
- **MCP names** other than `cpu_usage` (read from `service.go:447` and the missing aliases).
- **`disk-capacity` / `replication-lag`** called with Render's parameter shape (no `kind`).
- **Real callers.** Whether the official Render CLI or common SDKs call these paths; the Render MCP server certainly uses the metric names above.
- **Shell refusal wording (same run, not filed).** On the free fixture, `serviceDetails.sshAddress` was null and `POST /v1/services/<srv>/shell-ticket` returned `409 "service is not eligible and running"`. That is honest, but one message covers both "free plan" and "not running". It is recorded here only.

## Dedupe

- `grep -rli 'aggregateBy|cpu-limit|cpu_usage|http_request_count|filters/http' .pm` hits only done items:
  - `w3/done/m18/t004` (GraphQL `aggregateBy` instance decision);
  - `w3/done/m4.5` (GraphQL group-by mapped "like REST's `groupBy`", before the strict validator);
  - `w3/done/m4` (cAdvisor);
  - `w2/done/028` (dashboard CPU-limit visibility deferred).
- `w4/086` states that "the four read families (logs, metrics, usage/billing, service events) were not walked"; this fills that gap for metrics.
- `w1/093` is the same validator-to-plain-404 mechanism on a different route.
- `.pm/DO_NOT_DO.md` covers only `metrics-stream` and task runs.
- No open item covers this. Not fixed on `main` as of `e982c430e`.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `https://dashboard.bex.co`, 2026-09-14 pass 29, journeys 7 (metrics) and 8 (shell). The shell side is clean on the free plan: the page reads "Shell access requires a running paid web, private, or background service", and REST refuses the ticket with `409`.
- **Goal linkage:** `docs/ADR006-bex-api.md` (one Render-compatible contract across REST, GraphQL and MCP), and the metrics rows of `docs/ADR018-render-parity.md` (197–198). It is also the metrics half of the read-family walk `w4/086` left open.
- **Expected outcome:** an agent or script following Render's API docs or Render's MCP vocabulary gets real metrics from bex, or a clear refusal. The ledger's metrics rows then describe what production serves.
- **Why now:** `aggregateBy=statusCode` returns a plausible but wrong answer, which is the most dangerous kind of parity gap. A caller asking "how many 404s" silently receives all requests. The MCP name rejection hits every agent that learned Render's `get_metrics`.
- **Render parity:** included (t007). This milestone changes REST and MCP behavior and ADR018/ADR010 records.
