# Observability — logs and metrics

bex makes a running platform **observable** without `kubectl`: an operator or an AI agent debugging a deploy reaches App logs and metrics through the same bearer-authed [bex-api](bex-api.md) it uses for lifecycle verbs. This is `GOAL.md` #2 ("basic obs for operation") and the AI-native pillar in [vision.md](vision.md) — agents can't fix a failing deploy they can't read.

Logs shipped first: highest operational value, and the simplest backend (pod logs, no metrics-server dependency). Metrics follow, over the same one-Core-many-adapters shape.

## Logs

One `Core` logs read, three adapters — the [bex-api](bex-api.md) invariant. The MCP `list_logs` tool is the agent surface; REST and GraphQL expose the same read to the public API and dashboard.

```mermaid
flowchart LR
  rest["REST GET /v1/logs<br/>GET /v1/logs/subscribe"] --> core
  gql["GraphQL logs(...)"] --> core
  mcp["MCP list_logs"] --> core
  core["Core.Logs / QueryLogs / FollowLogs"] --> src["PodLogSource / PodLogStream"]
  src --> pods["App pods (app.bex.co/app label)"]
```

- **`Core.Logs(name, tail)`** — tail-N aggregation across replicas; the unfiltered convenience read. Returns `LogEntry{timestamp, message, labels}` (labels `service`/`instance`/`container`).
- **`Core.QueryLogs(LogQuery)`** — adds Render's filters (type/text/time) and paging; the read path all three adapters (REST, GraphQL, MCP `list_logs`) go through.
- **`Core.FollowLogs(LogQuery, emit)`** — live tail; the SSE stream.

`PodLogSource` (and its follow sibling `PodLogStream`) is the one dependency Core reaches past the generic client for — the `pods/log` subresource controller-runtime's client can't serve. It's injected (`NewPodLogSource` / `NewPodLogStream` in `podlogs.go`), so the domain layer stays clientset-free and every read is faked in tests with no cluster.

### REST surface

| method + path | effect |
| --- | --- |
| `GET /v1/logs` | historical query → `{hasMore, next*Time, logs}` |
| `GET /v1/logs/subscribe` | live tail over Server-Sent Events |
| `graphql { logs(...) }` | same query, flat `LogEntry` rows |
| MCP `list_logs` | agent read (Core.QueryLogs), `resource` array + `type`/`text`/`startTime`/`endTime`/`limit` |

Query params (Render vocabulary): `resource` (App id, repeatable), `type` (repeatable), `text` (case-insensitive substring), `startTime`/`endTime` (RFC3339), `limit` (default 20, max 100 — Render's paging range). `Core` re-applies every filter, sorts oldest-first, and keeps the newest `limit`.

The REST log object is Render's public-API `log` (all fields required): `{id, message, timestamp, labels[]}`. bex synthesizes a stable `id` from instance + timestamp + a message hash, and renders Core's map labels as Render's `{name,value}` array — `type` (value `app`), `resource` (Core's `service`), `instance`. The envelope carries all four required fields: `hasMore`, `nextStartTime` (newest line), `nextEndTime` (oldest line, the backward-page cursor), `logs`. (MCP instead returns `LogEntry` with map labels verbatim, matching Render's MCP server — each adapter mirrors its own Render counterpart.)

### Log types

Render's `type` is `app`/`request`/`build`. **bex only sources application (`app`) logs today** — the App's own container stdout/stderr, read from every replica pod (label `app.bex.co/app=<name>`) and aggregated. `request` (Traefik access logs) and `build` (bex builds in a separate plane, see [go-and-gitops](go-and-gitops.md)) have no backend here, so those types resolve to an empty page rather than an error — a Render-shaped client filtering by them sees an empty result, never a break. `application` is accepted as an input alias for `app`.

### Live tail (SSE)

`GET /v1/logs/subscribe` streams `text/event-stream`, one `data: <log JSON>` frame per new line, following a single `resource`. bex uses **SSE, not Render's WebSocket**: no extra dependency, works with `curl -N`, same "stream new lines live" contract. The handler clears the server write deadline (`http.NewResponseController`) so the long-lived stream isn't killed by the api's `WriteTimeout`; the stream ends when the client disconnects (request context cancelled).

### Render compatibility

Shapes verified against `render-public-api-1.json`: the `type`/`resource`/`text`/`startTime`/`endTime`/`limit` params, the `{hasMore, nextStartTime, nextEndTime, logs}` envelope, and the `{id, message, timestamp, labels[]}` log object (with the label-name enum). Known, intentional deviations:

- **subscribe transport** — SSE vs Render's WebSocket (`101` upgrade).
- **`ownerId`** — Render requires it; bex is single-tenant and omits it.
- **unimplemented filters** — Render's request-log filters `instance`, `level`, `host`, `statusCode`, `method`, `path`, and the `direction` (forward/backward) param are not wired yet; `text`/`type`/time filtering is.

### RBAC

The api ServiceAccount reads `pods` (`get`/`list`/`watch`) and `pods/log` (`get`) — added with the logs verb in `lego/operator/config/api/rbac.yaml`. No clientset lives in Core; only `podlogs.go` (and its `main.go` wiring) touch it.

## Metrics

The same one-Core-many-adapters shape as logs. `Core.Metrics(MetricQuery)` is the single read; REST (`rest_metrics.go`) and GraphQL (`graphql.go`) are the surfaces. Two backends, each an injected source so Core stays clientset-free (like `PodLogSource`):

```mermaid
flowchart LR
  rest["REST GET /v1/metrics/{cpu,memory,instance-count,<br/>http-requests,http-latency,bandwidth}"] --> core
  gql["GraphQL metrics(...)"] --> core
  core["Core.Metrics"] --> rr["ResourceMetricsRange source"] --> promc["Prometheus ← cAdvisor (kubelets)"]
  core -.fallback.-> rm["ResourceMetrics source"] --> ms["metrics-server (metrics.k8s.io)"]
  core --> pods["App pods (limits + fallback instance count)"]
  core --> qm["RequestMetrics source"] --> prom["Prometheus ← Traefik"]
```

- **Resource** — `cpu` / `memory` / `instance_count` come from **cAdvisor scraped by Prometheus** (`NewPrometheusResourceSource`, gated by `BEX_PROM_URL` like the request metrics): a `query_range` over `container_cpu_usage_seconds_total` (per-pod rate → cores) and `container_memory_working_set_bytes` (per-pod sum → bytes; also counted for `instance_count`), one stepped series per replica tagged `instance` + `resource`, honoring `startTime`/`endTime`/`resolutionSeconds`. Since kubelet metrics carry pod names but not pod labels, an App's pods are matched by the Deployment pod-name shape (`<app>-<rs-hash>-<suffix>`) — anchored, so `web` never matches `web-api` pods. With `percentage=true`, every point is divided by the pod's limit (read from the pod spec) and reported 0..100; an instance with no limit — including a pod that no longer exists — is omitted rather than faked. Every tiered App has one (see [control-plane.md's tier catalog](control-plane.md#tiers-plans--pod-resources--machine-provisioning)); this path only fires for a bare-CR App with no `spec.tier` set.
- **Resource fallback** — without `BEX_PROM_URL`, `cpu` / `memory` come from **metrics-server** (`metrics.k8s.io/v1beta1`, via `NewResourceMetricsSource`): a point-in-time snapshot, so each series carries a **single current point** regardless of the requested range. `instance_count` then derives from the App's pods directly, needing **no** source at all. When Prometheus is configured but unreachable at query time, the error surfaces (no silent fallback — same contract as request metrics).
- **Request** — `http_requests` / `http_latency` / `bandwidth` come from **Traefik scraped by Prometheus** (`NewPrometheusRequestSource`, gated by `BEX_PROM_URL`): a `query_range` over `traefik_service_requests_total` (rate), `_request_duration_seconds_bucket` (`histogram_quantile`, default p95), and `_responses_bytes_total` (rate). `statusCode` filters the `code` label (`2xx` → `2..`); `groupBy` (`status`/`method`) breaks the result into per-label series.

When a metric's source isn't wired at all (request metrics without `BEX_PROM_URL`, cpu/memory with neither Prometheus nor metrics-server), the endpoint returns **503** (`ErrMetricsUnavailable`) — the App exists, the data source doesn't.

### REST surface

| method + path | metric |
| --- | --- |
| `GET /v1/metrics/cpu` | per-instance CPU (cores, or % of limit) |
| `GET /v1/metrics/memory` | per-instance memory (bytes, or % of limit) |
| `GET /v1/metrics/instance-count` | running replica count |
| `GET /v1/metrics/http-requests` | request rate (req/s) |
| `GET /v1/metrics/http-latency` | latency percentile (seconds) |
| `GET /v1/metrics/bandwidth` | outbound bytes/s |

Query params (Render vocabulary): `resource` (App id, repeatable), `startTime`/`endTime` (RFC3339), `resolutionSeconds`, `quantile` (0..1, latency), `statusCode`/`host`/`path`/`groupBy` (request filters), and a bex extra `percentage=true` (cpu/memory as a fraction of limit). Each endpoint returns Render's metrics array — `[{labels:[{field,value}], unit, values:[{timestamp,value}]}]`. GraphQL: `metrics(resource, metric, startTime, endTime, resolutionSeconds, quantile, percentage, statusCode, host, path, groupBy)` → `MetricSeries { unit, labels{field,value}, points{timestamp,value} }`, `metric` being `cpu`/`memory`/`instance_count`/`http_requests`/`http_latency`/`bandwidth`. The MCP `get_metrics` tool (`resource[]` + `metricTypes[]`) exposes the same read to agents — three-adapter parity, like `list_logs`.

### Render compatibility

Shapes track Render's metrics endpoints (per-metric path segments; the `{labels, unit, values}` time-series). With Prometheus configured (`BEX_PROM_URL`), **all six metrics are resolution-stepped series honoring `startTime`/`endTime`/`resolutionSeconds`** — Render metrics-page parity. Known, intentional deviations:

- **snapshot fallback** — without `BEX_PROM_URL`, resource metrics fall back to metrics-server and return a single current point (metrics-server has no history); Render always returns a stepped series.
- **`cpu_limit`/`memory_limit` stay single-point** — limits come from the current pod spec, and bex won't fabricate a history for a value it only knows _now_ (a past limit may have differed). Safe for Render-style clients, which fetch the limit alongside the usage series and divide client-side using its latest value.
- **`host`/`path` filters** — accepted (Render vocabulary) but not applied to request metrics: Traefik's per-service counters carry only `code`/`method`, not host/path (host/path live on router-level metrics). Documented like the logs adapter's unimplemented request filters.
- **Traefik service selector** — the App→Traefik-service match is a heuristic (`service=~".*<app>.*"`); a real cluster may need it tuned to the ingress's actual service label. The resource-metrics pod match (`pod=~"<app>-[a-z0-9]+-[a-z0-9]{5}"`) is its (stricter) cAdvisor sibling.

### RBAC

The metrics-server fallback adds read on `metrics.k8s.io` `pods` (`get`/`list`) to the api ServiceAccount (`lego/operator/config/api/rbac.yaml`); percentage mode reuses the existing `pods` read for limits. The Prometheus-backed metrics (resource history and request) reach Prometheus over HTTP (`BEX_PROM_URL`), not the kube API, so they need no extra RBAC on bex-api — the Prometheus ServiceAccount's chart-default ClusterRole covers the cAdvisor scrape (`nodes`, `nodes/proxy`, `nodes/metrics`).

### Cluster enablement

`deploy/gitops/base/prometheus.yaml` runs the one Prometheus behind both history-backed metric families, with two scrape jobs: `traefik` (request counters, via `deploy/gitops/base/traefik.yaml`'s `metrics` entrypoint `:9100` with `addServicesLabels`) and `kubernetes-cadvisor` (per-container cpu/memory, scraped through the apiserver proxy so it works even where the pod network can't reach every kubelet). `BEX_PROM_URL` points bex-api at it and enables both. `deploy/gitops/base/metrics-server.yaml` installs metrics-server — now only the snapshot fallback (and `kubectl top`).

## Verify (mock cluster)

```sh
# deploy a sample App, then:
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8090/v1/logs?resource=<app>&type=app" | jq .

# resource metrics — with BEX_PROM_URL set these are stepped history; a ranged
# query returns one point per resolution step per instance:
curl -s -H "Authorization: Bearer $BEX_API_TOKEN" \
  "http://localhost:8090/v1/metrics/cpu?resource=<app>&percentage=true" | jq .
curl -s -H "Authorization: Bearer $BEX_API_TOKEN" \
  "http://localhost:8090/v1/metrics/memory?resource=<app>&startTime=$(date -u -v-1H +%Y-%m-%dT%H:%M:%SZ)&endTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)&resolutionSeconds=60" \
  | jq '.[0].values | length'   # ≈60 points over the hour (where data exists)
curl -s -H "Authorization: Bearer $BEX_API_TOKEN" \
  "http://localhost:8090/v1/metrics/instance-count?resource=<app>" | jq .
```
