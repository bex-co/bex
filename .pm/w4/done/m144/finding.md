# Major — resource charts fail because production Prometheus cannot finish startup

Why: users cannot inspect service CPU, memory or instance history through the dashboard or API.

## Reproduction

2026-10-02 07:34–07:47 UTC, `bex` workspace `tea-d98210cbbpdc73dcrkvg`. The new free Go QA service reached Live and served its updated MESSAGE over HTTP 200, but Scaling showed three "Couldn't load this metric" messages. A fresh login and read-only visit to existing `beancount-cms-v2` at `https://dashboard.bex.co/services/srv-d9bj8s3eg85c7390eb9g/scaling` reproduced the same three errors; a hard reload reproduced them again.

Expected: ranged CPU, memory and instance samples for an active, authorized service. Actual: GraphQL internal errors, REST 500 and an MCP error result. CPU_LIMIT remains successful. The dashboard truthfully displays source failure; do not replace it with fake zero/empty success.

The screenshot `.playwright-mcp/qa-metrics-r2-1.png` and JSON `.playwright-mcp/qa-metrics-r2-1.json` were checked on disk. These are local, gitignored artifacts; the durable API evidence follows.

## Exact API probes and complete responses

GraphQL: POST `https://api.bex.co/graphql`, JSON Content-Type, browser credentials included. REST: authenticated browser GET to the recorded URL.

```json
{
  "date": "2026-10-02T07:44:41.837Z",
  "graphql": {
    "request": {
      "query": "query QaMetrics { cpu: metrics(query:{name:\"CPU\",filters:[{field:\"RESOURCE\",values:[\"srv-d9bj8s3eg85c7390eb9g\"]}],start:\"2026-10-02T06:40:00Z\",end:\"2026-10-02T07:40:00Z\",resolution:300}){unit values{time value}} memory: metrics(query:{name:\"MEMORY\",filters:[{field:\"RESOURCE\",values:[\"srv-d9bj8s3eg85c7390eb9g\"]}],start:\"2026-10-02T06:40:00Z\",end:\"2026-10-02T07:40:00Z\",resolution:300}){unit values{time value}} instances: metrics(query:{name:\"INSTANCES\",filters:[{field:\"RESOURCE\",values:[\"srv-d9bj8s3eg85c7390eb9g\"]}],start:\"2026-10-02T06:40:00Z\",end:\"2026-10-02T07:40:00Z\",resolution:300}){unit values{time value}} cpuLimit: metrics(query:{name:\"CPU_LIMIT\",filters:[{field:\"RESOURCE\",values:[\"srv-d9bj8s3eg85c7390eb9g\"]}]}){unit values{time value}} }"
    },
    "status": 200,
    "response": {
      "data": {
        "cpu": null,
        "cpuLimit": [
          {
            "unit": "cpu",
            "values": [
              {
                "time": "2026-10-02T07:44:41Z",
                "value": 1
              }
            ]
          }
        ],
        "instances": null,
        "memory": null
      },
      "errors": [
        {
          "message": "internal error",
          "locations": [
            {
              "line": 1,
              "column": 413
            }
          ],
          "path": ["instances"]
        },
        {
          "message": "internal error",
          "locations": [
            {
              "line": 1,
              "column": 19
            }
          ],
          "path": ["cpu"]
        },
        {
          "message": "internal error",
          "locations": [
            {
              "line": 1,
              "column": 213
            }
          ],
          "path": ["memory"]
        }
      ]
    }
  },
  "rest": {
    "url": "https://api.bex.co/v1/metrics/cpu?resource=srv-d9bj8s3eg85c7390eb9g&startTime=2026-10-02T06:40:00Z&endTime=2026-10-02T07:40:00Z&resolutionSeconds=300",
    "status": 500,
    "response": {
      "error": "internal error",
      "id": "internal_error",
      "message": "internal error"
    }
  }
}
```

MCP: POST `https://api.bex.co/mcp`, JSON Content-Type, Accept `application/json, text/event-stream`, browser credentials included:

```json
{
  "jsonrpc": "2.0",
  "id": 20261002,
  "method": "tools/call",
  "params": {
    "name": "get_metrics",
    "arguments": {
      "resourceId": "srv-d9bj8s3eg85c7390eb9g",
      "metricTypes": ["cpu_usage"],
      "startTime": "2026-10-02T06:40:00Z",
      "endTime": "2026-10-02T07:40:00Z",
      "resolution": 300
    }
  }
}
```

HTTP 200, complete response body:

```text
event: message
data: {"jsonrpc":"2.0","id":20261002,"result":{"content":[{"type":"text","text":"internal error"}],"isError":true}}

```

Only the deliberate REST probe added a console HTTP 500. The GraphQL requests returned HTTP 200 with errors; MCP likewise correctly distinguishes transport status from tool failure. No rate-limit response or hanging request explains this reproduction.

## Runtime attribution

These were read-only commands; no pod, deployment, volume or resource budget was changed:

```sh
kubectl --context hetzner-prod --request-timeout=20s get pod -n monitoring prometheus-server-5797f5666d-sfzpg -o 'jsonpath={range .status.containerStatuses[*]}{.name}{"\t"}{.restartCount}{"\t"}{.ready}{"\t"}{.lastState.terminated.reason}{"\t"}{.lastState.terminated.finishedAt}{"\n"}{end}'
kubectl --context hetzner-prod --request-timeout=20s logs -n monitoring prometheus-server-5797f5666d-sfzpg -c prometheus-server --previous --tail=4
kubectl --context hetzner-prod --request-timeout=20s get endpointslice -n monitoring -l kubernetes.io/service-name=prometheus-server -o 'jsonpath={range .items[*]}{.metadata.name}{"\t"}{.endpoints[*].conditions.ready}{"\t"}{.endpoints[*].addresses}{"\n"}{end}'
```

Complete selected status/log outputs:

```text
prometheus-server  572  false  OOMKilled  2026-10-02T07:42:24Z
prometheus-server-configmap-reload  0  true
ts=2026-10-02T07:42:23.414Z caller=head.go:793 level=info component=tsdb msg="WAL segment loaded" segment=1438 maxSegment=1586
ts=2026-10-02T07:42:23.418Z caller=head.go:793 level=info component=tsdb msg="WAL segment loaded" segment=1439 maxSegment=1586
ts=2026-10-02T07:42:23.461Z caller=head.go:793 level=info component=tsdb msg="WAL segment loaded" segment=1440 maxSegment=1586
ts=2026-10-02T07:42:23.899Z caller=head.go:793 level=info component=tsdb msg="WAL segment loaded" segment=1441 maxSegment=1586
prometheus-server-pmwtk  false  ["10.244.35.247"]
```

The container's last exit code was 137. The count is the pod's lifetime restart count; it does not prove that every earlier termination had the same cause. Live image: `quay.io/prometheus/prometheus:v2.54.1`. Live resources: requests CPU 50m/memory 256Mi; limits CPU 500m/memory 512Mi. Args include `--storage.tsdb.retention.time=3d`, `--storage.tsdb.path=/data`. These match the committed values.

At 07:46 the hosting node reported MemoryPressure=False and Ready=True, with `kubectl top node bex-platform-h7t89-vhv2v` reporting 4070Mi / 53% memory used. This supports investigating the container budget; it is not a historical proof about all host OOM events.

The bex-api logs tie the exact 07:44:41 requests to `http://prometheus-server.monitoring.svc.cluster.local/api/v1/query_range` with `dial tcp 10.103.143.82:80: connect: connection refused`. The query selectors name `tea-d98210cbbpdc73dcrkvg-beancount-cms-v2`, and the window matches the API probe. Together with the non-ready EndpointSlice, this establishes source unavailability rather than malformed input or UI mishandling.

## Root cause, consumer and target behavior

Source inspected at `1589e8459`:

- `deploy/gitops/base/prometheus.yaml:221-235`: a single server uses a persistent volume and three-day retention but only a 512 MiB memory limit / 256 MiB request. The comment still assumes the workload is disk-bounded. Production inherits this budget; no production override was found. The live generated pod matches it.
- `lego/backend/cmd/api/main.go:547-565`: production wires the ranged metric sources to that Prometheus. Configured-but-unreachable Prometheus deliberately remains an error; it is not silently replaced by a metrics-server snapshot.
- `lego/backend/internal/metrics/service.go:632-635,799-815,953-955` and `source.go:169-198,217-225`: CPU/memory and instance reads reach query_range and propagate transport failure. `internal/api/server.go:1568-1569` redacts internal details.
- **Control verified for its real reason:** `service.go:895-925` implements CPU_LIMIT/MEMORY_LIMIT directly from pod specs. It never requires Prometheus. The initial capture's successful MEMORY_LIMIT/CPU_LIMIT responses and the durable CPU_LIMIT control are therefore consistent with the source failure.
- UI `dashboard/src/features/services/components/scaling-recent-metrics.tsx:49-59` asks the corresponding resource and limit queries; no chart-layer fix is needed for the observed failure.

The running dependency was read at the exact version: [Prometheus v2.54.1 Head.Init](https://github.com/prometheus/prometheus/blob/v2.54.1/tsdb/head.go#L612) replays segments through `loadWAL` (around 763–795), logging each completion; replay completion is logged at 831. [main.go](https://github.com/prometheus/prometheus/blob/v2.54.1/cmd/prometheus/main.go#L1172) opens storage before publishing it as available. The last log ends at segment 1441 of 1586 immediately before the recorded OOM. This supports an OOM during replay; the specific cardinality/series growth and required peak memory remain unmeasured.

**Fix target:** size a bounded production request/limit using the retained dataset's replay and steady-state/query peaks plus platform-node capacity; recover through the owning GitOps configuration while preserving the PVC/WAL. Put production-specific sizing in the production overlay unless measurements justify changing the shared baseline. Re-evaluate expensive scrape/label families only from measured contribution and preserve consumers that need those series. Verify a restart against the existing dataset, not only an empty-PVC startup. Do not "fix" the API by disabling BEX_PROM_URL, falling back to fake history, deleting the WAL, or relabelling failure as zero usage.

[Prometheus storage documentation](https://prometheus.io/docs/prometheus/latest/storage/) explains that head data is in memory and WAL is replayed after restart. Its retention controls do not establish a safe memory budget for this dataset. Reducing retention alone is not evidence that startup replay will fit.

## Blast radius, aliases and adjacent classes

The constructor search found **nine production metrics-source wiring calls**, all in `backend/cmd/api/main.go:554-565`: RequestMetrics, ResourceMetricsRange, ResourceLimitRange, MonthToDateBandwidth, MetricsFilterValues, DiskUsage, DBConnections, ReplicationLag and KeyValueStats. **One** production call wires NewPrometheusResourceSource; its shared service paths cover CPU, memory and instance count. REST `/v1/metrics/cpu`, GraphQL `metrics`, and MCP `get_metrics` (Render `cpu_usage`, legacy `cpu`) converge on it.

The backend usage service also reads the shared Prometheus (`internal/usage/service.go:81,1092,1117,1277`); usage failures were seen in logs, but billing consequences were not assessed. The operator has **two BEX_PROM_URL reads** (`cmd/manager/main.go:67,549`) wiring free-service activity and database disk usage. Their behavior was not exercised during this outage. Grafana and the shared rule evaluator are additional infrastructure consumers.

Resource-family matrix to verify: web/private/worker resource charts; cron's permitted/between-run states; static's applicable HTTP metrics and absent compute metrics; Postgres and Key Value datastore metrics. This is a production backend recovery, not an allowlist of user resources. Preserve the smaller local stack's intended sizing/isolation.

Forbidden/unauthenticated resources must remain denied; unknown resources retain not-found semantics; bad metric names/windows remain validation errors. A healthy source with no matching samples returns an honest empty result. An unavailable source or timeout remains a failure, not a successful empty series. Recovery must not backfill nonexistent samples across the outage.

**Unverified:** exact replay peak and series/cardinality driver, oldest retained history and full outage duration, datastore/operator/usage effects, all prior restart causes, post-recovery behavior and restart stability. No recovery was performed by this hunt.

## Dedupe and Render

Open and done workstreams and Prometheus history were searched for OOMKilled, CrashLoop, WAL replay and the memory cap. No open item schedules this startup failure. w7/m156 concerns CNPG replica metric collection; w7/m159–m160/m164 add other observability series; w7/047 is a build-cache trial decision. None covers the observed Prometheus OOM. w4/m139's older billing watermark problem is not attributed to this incident without its own evidence. The current manifest still declares the observed budget, so no landed recovery is merely waiting to deploy. No roadmap anti-goal conflicts.

[Render's service metrics](https://render.com/docs/service-metrics) exposes CPU and memory on the service Metrics page. This finding restores that supported bex journey; no external metric drain is proposed.

## Hunt cleanup

This follow-up sweep was read-only and created no resources. It reused neither another user's session nor bulk logout; its own session was successfully revoked at round close and its local cookie files were removed. The prior sweep's Docker deletion finalizer is tracked separately in m142.
