# w1 · m155 — Render's metrics contract is partly unserved on REST and MCP: `aggregateBy` is silently ignored, eight Render paths return 404, and MCP rejects Render's metric names

**Worker:** worker1 **Goal:** every metrics request shaped the way Render documents it gets either Render's answer or an explicit, coded refusal, never a bare `404` or a plausible wrong series.
- REST honors `aggregateBy`.
- REST serves Render's limit, filter, datastore and bandwidth-source paths, or each is an explicit divergence recorded in ADR018.
- MCP `get_metrics` accepts Render's metric-type names.

**Status:** done (2026-09-15). Every task is complete, and the definition of done passed live on production (images pinned to `c4212ec71`).

## Tasks (in order)

| id   | title                                                                                                             | est | depends_on                   |
| ---- | ----------------------------------------------------------------------------------------------------------------- | --- | ---------------------------- |
| t001 | REST honors Render's `aggregateBy` (per-status-code series; `host` refused with a code) and retires the dead `groupBy` — **DONE** | 30m | —                            |
| t002 | REST serves `/v1/metrics/cpu-limit` and `/v1/metrics/memory-limit` — **DONE**                                                 | 15m | —                            |
| t003 | REST serves `/v1/metrics/filters/{application,http,path}` from the existing filter verbs — **DONE**                           | 40m | —                            |
| t004 | Render's `active-connections`, `disk-usage` and `bandwidth-sources` paths: serve them or record the divergence — **DONE**     | 30m | —                            |
| t005 | MCP `get_metrics` accepts Render's `metricTypes` names — **DONE**                                                             | 25m | —                            |
| t006 | Blast radius: a composed-server test walking every Render `/metrics` path and parameter — **DONE**                            | 30m | t001, t002, t003, t004, t005 |
| t007 | Render parity — **DONE**                                                                                                     | 20m | t006                         |
| t008 | Simplify — **DONE**                                                                                                          | 15m | t007                         |
| t009 | Test coverage — **DONE**                                                                                                     | 40m | t007                         |
| t010 | Closeout — **DONE**                                                                                                          | 10m | t009                         |

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

## Implementation (2026-09-14)

**`aggregateBy` (t001).** `requestGroupBy(param, value)` (`lego/backend/internal/metrics/mcp.go`) is the one mapping for REST `aggregateBy` and MCP `aggregateHttpRequestCountsBy`: `statusCode` → the status breakdown, `host` → a coded 400 naming the parameter. `parseMetricParams` now reads `aggregateBy`, and the unreachable non-Render `groupBy` read is gone.

- **Label name.** The REST series carry `statusCode`, relabelled from the sources' `code`. Render's spec does not name the label, so this follows Render's own REST vocabulary for the breakdown (the `aggregateBy` value and the `filters/http` field). GraphQL keeps `code` as the control. Not confirmed against a Render capture.
- **`aggregationMethod`.** The t006 walk found REST also accepted and ignored Render's `aggregationMethod` on `/metrics/cpu`. It now goes through the same AVG-only rule as MCP's `cpuUsageAggregationMethod` (`cpuAggregation`).
- **`service`.** It is accepted as an alias of `resource` on every metrics path (`requestedResources`).

**Limits (t002).** `cpu-limit` and `memory-limit` are in `metricPaths`, served by the same Metrics verb GraphQL `CPU_LIMIT`/`MEMORY_LIMIT` and MCP `cpu_limit` use.

**Filters (t003).** Three routes over `MetricsFilters`, the verb behind GraphQL `metricsFilters`:

- `filters/application` returns `[{filter: instance, values}]`.
- `filters/http` returns `[{filter: statusCode, values}, {filter: host, values}]`. Host values stay empty, as in GraphQL, because they are discovered from the logs label read. Narrowing by `statusCode`/`host` is refused with a 400 rather than ignored.
- `filters/path` returns `[]`: bex has no path suggestions (GraphQL `metricsPathFilterSuggestions` is the same).
- Every one authorizes its resource, so an unknown resource is 404, never an empty list.
- **Declined:** a 503 when the status-code source is unwired. The shared verb answers `[]` there for GraphQL too, and changing it would split the two surfaces.

**Datastore and bandwidth paths (t004).**

- `disk-usage` and `active-connections` are served, and `datastoreKindFor` infers the kind from the resource id prefix via `id.KindOf`: `red-` Key Value, `srv-` a service disk, otherwise Postgres. `active-connections` maps to `db_connections` or `kv_connections`, and a service is a coded 400.
- `disk-capacity` and `replication-lag` get the same inference. Before m155 Render's parameter shape could not name a Key Value at all: `kind` is not a Render parameter, so the request validator refused it.
- **Divergence, `bandwidth-sources`:** a coded 501 that names `/v1/metrics/bandwidth` and GraphQL `monthToDateBandwidth`. bex keeps per-source bandwidth as month-to-date totals, not time series.

**MCP names (t005).** `renderMetricTypes` maps `cpu_usage`, `memory_usage`, `http_request_count` and `bandwidth_usage` onto bex ids; the other Render names already coincide, and bex's own ids keep working. `active_connections` is answered from the datastore verb for a `dpg-`/`red-` resource and refused for a service. Each series is labelled with the name the caller asked for. The `metricTypes` schema lists Render's names.

**Blast radius (t006).** `api/render_metrics_contract_test.go` generates the matrix from the pinned spec: every `/metrics` path except the two workflow non-goals (`task-runs-*`), with each documented query parameter. Every parameter must have a verdict (served, refused, or ignored with a reason), no path may be a bare mux 404, and no documented parameter may be refused as unknown. Ignored with a reason: the time window on `filters/application` and `filters/http` (discovery lists what is queryable now), and every narrowing parameter on `filters/path` (always `[]`). The cross-workspace route inventory and the Render route-intersection pin (131 → 139 operations) list the eight new routes.

**Parity (t007, docs).** ADR018's core and extended metrics rows and ADR010's REST parameter list describe what REST serves, including the `bandwidth-sources` divergence. The live cross-surface comparison is in § Live verification.

**Simplify (t008).** Applied: MCP `get_datastore_metrics` infers the kind the way REST does (`datastoreKindFor`); `filterValuesOrEmpty` coalesces a nil answer to `[]` in one place, so `orEmpty` is gone; the `applyCPUAggregation` wrapper and the GraphQL `"status"` literal are replaced by `cpuAggregation` and `groupByStatus`; the matrix test asserts status per verdict instead of only "not a bare 404".

- **Declined:** a non-member test per `filters/*` route. The verb-level authz sweeps (`TestAuthzGuardsEveryVerb`, the cross-workspace route inventory) already cover them.
- **Kept on purpose:** MCP's `aggregateHttpRequestCountsBy=statusCode` series keep the `code` label. Each adapter mirrors its own Render counterpart, and Render's MCP server does not name a `statusCode` label, so only REST is relabelled.

**Test coverage (t009).** `metrics/render_contract_test.go` covers `aggregateBy` (statusCode series labels, host refusal), `aggregationMethod`, the limit paths, the three filter shapes and their refusals, datastore kind inference, `active-connections` on a service (400), `bandwidth-sources` (501), and every Render MCP metric type, including the service refusal of `active_connections`. `mcp_alias_test.go` pins `requestGroupBy` and `cpuAggregation`. Pre-fix, the api matrix test failed with a bare 404 on all eight new paths. Backend `go test ./...` passes. `make lint` reports only two findings that predate m155 (`api/scope_matrix.go:163` unused `writeGraphQLErrors`, `operator/internal/publish/publish.go:611` modernize).

## Live verification (2026-09-15, production pinned to `c4212ec71`)

- **Fixture.** `qa-20260915-m155` (`srv-dakempnnonls738jbnb0`), a free web service built from `examples/hello-python`, docker, with Docker Command `python -u -m http.server $PORT`. From 06:55Z it received `GET /` (200) and `GET /qa-missing-N` (404) every ~2 s.
- **Serving check.** At 07:29:25Z, `GET /v1/metrics/cpu-limit` answered 200 with a series; before this milestone it was a bare 404.
- **Probes** run at 07:47:04Z from a signed-in dashboard page over the last hour. MCP went over streamable HTTP at `https://api.bex.co/mcp`.

```text
GET http-requests?aggregateBy=statusCode&resolutionSeconds=60 → 200, 3 series:
    statusCode=200 (last 0.311) · statusCode=404 (last 0.333) · statusCode=501 (last 0)
GET http-requests?aggregateBy=host  → 400 "aggregateBy=host is unsupported — neither Traefik Prometheus counters nor the Loki request-log path expose a host group-by axis (filter by host instead)"
GET http-requests?aggregateBy=bogus → 400 invalid query parameter "aggregateBy"            (control)
GET http-requests (no aggregateBy)  → 200, 1 series resource=srv-…                        (control)
GET cpu-limit    → 200, 1 series, value 0.1
GET memory-limit → 200, 1 series, value 536870912
GET filters/http        → 200 [{"filter":"statusCode","values":["200","404","501"]},{"filter":"host","values":[]}]
GET filters/application → 200 [{"filter":"instance","values":["srv-dakempnnonls738jbnb0-a2ub95kcj33q9ahv8blm"]}]
GET filters/path        → 200 []
GET filters/http&statusCode=200 → 400 "narrowing filter values by statusCode or host is unsupported: omit them to list every queryable value"
GET active-connections?resource=srv-… → 400 "active connections need a Postgres (dpg-) or Key Value (red-) resource, not "srv-…""
GET disk-usage?resource=srv-…         → 200 []   (a service with no disk)
GET bandwidth-sources → 501 "bandwidth-sources is not served: bex keeps its per-source bandwidth (http, nat, websocket) as month-to-date totals, not time series — read /v1/metrics/bandwidth …"
GET cpu → 200 (3 instance series) · cpu&aggregationMethod=MAX → 400 (only AVG) · instance-count → 200, 1 series   (controls)
MCP tools/call get_metrics {metricTypes:["cpu_usage"]} → series labelled metric=cpu_usage
MCP get_metrics {metricTypes:["memory_usage","http_request_count","bandwidth_usage","instance_count"]} → series
MCP get_metrics {metricTypes:["active_connections"]} on srv-… → isError "bad request: active connections need a Postgres (dpg-) or Key Value (red-) resource …"
MCP get_metrics {metricTypes:["cpu"]} → series                                                (bex id control)
```

- **Every DoD bullet passed.**
  - REST `aggregateBy=statusCode` splits by status with Render's `statusCode` label.
  - `aggregateBy=host` is a coded 400.
  - The limit paths answer, and the filter paths return Render's filter arrays.
  - `active-connections` on a service is a coded refusal, and `bandwidth-sources` is the recorded 501 divergence.
  - MCP accepts Render's metric names.
  - The controls did not change.
- **Not probed live.** A Postgres or Key Value resource for `active-connections` and `disk-usage`: no datastore fixture was created. The composed-server and unit tests cover the kind inference.

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
