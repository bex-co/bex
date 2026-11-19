# w3 · m487 — Turning on edge caching changes neither a service's Metrics charts nor its autoscaling, as on Render

**Worker:** worker3 **Goal:** an edge-cached service's CPU and memory usage, its limit line and its autoscaling decisions count only its own container, as on Render, where the cache is not inside the instance. **Status:** todo (t001–t003 and t005–t007 done; t004's ADR edits done, its production check waits for a deploy carrying the change)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Failing tests: autoscaling and the Metrics queries count the edge-cache sidecar — **DONE** | 30m | — |
| [t002](done/t002.md) | One shared sidecar name; the pod-metrics decoder leaves the sidecar out for the autoscaler and bex-api — **DONE** | 45m | t001 |
| [t003](done/t003.md) | Service usage and limit-history PromQL leave the sidecar out — **DONE** | 45m | t001 |
| [t004](t004.md) | Record it in ADR004 and ADR010, ship and verify on production | 45m | t002, t003 |
| [t005](done/t005.md) | Render parity — **DONE** | 20m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 30m | t005 |
| [t008](t008.md) | Closeout | 10m | t006, t007 |

## Definition of done

- REST `/v1/metrics/cpu` and `/memory`, GraphQL `metrics`, MCP `get_metrics` and the dashboard Metrics page report an edge-cached service's usage from its own container only; its limit line equals its plan's on both the Prometheus history and the pod-spec fallback.
- `autoscaleDesired` computes the same desired instance count with and without the `edge-cache` container's usage (test).
- On production (t004), an edge-cached fixture's reported memory equals Prometheus `container_memory_working_set_bytes` for its app container, and its limit equals its plan's; recorded in this README.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` for w3 2026-11-18 (topic: one aspect of Render parity, web-service edge caching), item 2; filed by `/pmx them to w3` 2026-11-18. Evidence on `origin/main` `4e3c87031`:
  - `lego/types/podmetrics` `Parse` sums every container in a pod; `lego/operator/internal/controller/autoscale.go:219-260` compares that sum with the plan's `tierLimits(tier)`, so the sidecar (limit 160Mi) counts against the plan and can trigger a scale-out the customer pays for.
  - `lego/backend/internal/egressquery/podname.go:125` (`container!=""`) feeds the service usage query and the limit history (`lego/backend/internal/metrics/source.go`), so both include the sidecar.
  - `lego/backend/internal/metrics/service.go:1238-1262` (`podResourceLimits`) already skips `edge-cache` ("its resources are the platform's, never counted against the plan"), so the fallback and the history disagree by 160Mi.
- **Goal linkage:** ADR008 § Render parity first. Render's cache is "powered by the same global CDN as Render static sites" ([web-service-caching](https://render.com/docs/web-service-caching)), outside the instance, so enabling it changes no instance metric.
- **Expected outcome:** enabling caching no longer moves the Metrics charts or scales a service out on the sidecar's memory.
- **Why now:** every caching service's charts are wrong today, and memory autoscaling can bill extra instances for it. The fix is small, and `w3/m489`'s resource accounting builds on the shared exclusion.
- **Render parity included:** reported metric values change on REST, GraphQL, MCP and the dashboard.

## Implementation (2026-11-18)

- **One name.** `appv1alpha1.EdgeCacheContainer` (`lego/types/v1alpha1/app_types.go`) names the sidecar. The operator's `edgeCacheContainerName` is that constant, and bex-api's own `edgeCacheContainer` copy is gone.
- **The autoscaler and bex-api's current usage.** `podmetrics.Parse` skips the container, so `autoscaleDesired` and the metrics-server fallback read the service's own containers. `TestAutoscalingLeavesTheEdgeCacheSidecarOut` buys a third instance on the old code (2 × 450Mi against starter's 80% memory target) and two on the new.
- **The Prometheus reads.** `serviceContainers` (`lego/backend/internal/metrics/source.go`) adds `container!="edge-cache"` to `egressquery.PodNameMatcher` for the CPU and memory usage queries, the limit history a percentage divides by and, as a deviation from t003, a service's disk activity (the sidecar mounts no disk, so it changes no value; it keeps every per-container service read on one selector). `instance_count` keeps `PodNameMatcher`: it counts pods, and the usage meter bills instance-seconds by that selector. `TestServiceResourceQueriesLeaveTheEdgeCacheOutOnPrometheus` evaluates the queries in Prometheus' own engine (`promtool`): on the old code they read 0.38 cores, 450Mi, a 0.6-core and a 672Mi limit for a pod whose app runs 0.3 cores and 300Mi under 0.5 / 512Mi.
- **The pod-spec fallback** already skipped the sidecar (`podResourceLimits`), so the Prometheus history and the fallback disagreed by 160Mi until now. `TestTheSnapshotPathLeavesTheEdgeCacheOut` pins usage, percentage and both limits on that path.
- **Production before the deploy:** no production App had a caching profile on 2026-11-18 (`kubectl get apps.app.bex.co -A`, `spec.cacheProfile`), so no customer's charts or autoscaling were moved by the sidecar at the time.

## Render parity (t005, 2026-11-18)

Render: the cache is "powered by the same global CDN as Render static sites", and on a hit "the request never reaches your web service" ([web-service-caching](https://render.com/docs/web-service-caching), fetched 2026-11-18). The page says nothing about metrics or autoscaling, so "Render's charts do not move" is inferred from the cache being outside the instance, not documented.

| Surface | Render | bex after this milestone | Verdict |
| --- | --- | --- | --- |
| REST `/v1/metrics/cpu`, `/memory`, GraphQL `metrics`, MCP `get_metrics` | the instance's own usage; the cache is on the CDN | the service's own containers, on the Prometheus history and the metrics-server fallback alike (one `Service.Metrics` read behind all three) | matched (inferred) |
| `percentage=true` | — (bex extension) | divides by the plan's limit on the history and the fallback alike | consistent |
| REST `/v1/metrics/cpu-limit`, `/memory-limit` | the plan's | the plan's (the pod-spec reader already skipped the sidecar) | matched |
| `instance-count` | instances | pods, unchanged | matched |
| Dashboard Metrics page | the instance's own usage | reads GraphQL `metrics`, no client-side change | matched (inferred) |
| Autoscaling | scales on the instances' CPU and memory | `autoscaleDesired` reads the service's own containers | matched (inferred) |

Recorded in [ADR018](../../../docs/ADR018-render-parity.md)'s Edge caching row, [ADR004](../../../docs/ADR004-app-deployment.md) § Edge caching and [ADR010](../../../docs/ADR010-observability.md) § Metrics. No drift to file.

