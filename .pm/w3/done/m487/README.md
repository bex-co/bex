# w3 · m487 — Turning on edge caching changes neither a service's Metrics charts nor its autoscaling, as on Render

**Worker:** worker3 **Goal:** an edge-cached service's CPU and memory usage, its limit line and its autoscaling decisions count only its own container, as on Render, where the cache is not inside the instance. **Status:** done (2026-11-19; verified on production pin `7af50a28168d`, § Production record)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Failing tests: autoscaling and the Metrics queries count the edge-cache sidecar — **DONE** | 30m | — |
| [t002](done/t002.md) | One shared sidecar name; the pod-metrics decoder leaves the sidecar out for the autoscaler and bex-api — **DONE** | 45m | t001 |
| [t003](done/t003.md) | Service usage and limit-history PromQL leave the sidecar out — **DONE** | 45m | t001 |
| [t004](done/t004.md) | Record it in ADR004 and ADR010, ship and verify on production — **DONE** | 45m | t002, t003 |
| [t005](done/t005.md) | Render parity — **DONE** | 20m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 30m | t005 |
| [t008](done/t008.md) | Closeout — **DONE** | 10m | t006, t007 |

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

Recorded in [ADR018](../../../../docs/ADR018-render-parity.md)'s Edge caching row, [ADR004](../../../../docs/ADR004-app-deployment.md) § Edge caching and [ADR010](../../../../docs/ADR010-observability.md) § Metrics. No drift to file.

## Gate (2026-11-18)

- **Gate (clears by itself):** a production deploy carrying `8a37b982f` (the implementation). `deploy.yml` deploys at most every 6 hours; the last pin, `e5ace7826` (2026-11-18 21:22Z), makes the earliest one about 2026-11-19 03:30Z plus its tests and build. The check: the newest `chore(deploy): pin platform images to <sha>` commit on `origin/main` with `git merge-base --is-ancestor 8a37b982f <sha>`. No user action is needed.
- **Then (t004 steps 2–3):** production had no App with a caching profile on 2026-11-18, so create an edge-cached fixture (a paid web service with `origin-controlled`) in `bex-canary` as the QA user, or reuse `w3/m486`'s t006 fixture if it is still up. Compare REST `GET /v1/metrics/memory` and `/cpu` with Prometheus `container_memory_working_set_bytes{container="app"}` and the app container's CPU rate for the same pods, and `/memory-limit` and `/cpu-limit` with the plan's; record it here, then t008 closes out.

## Unblocked (2026-11-19, `/loopx w3` routine `pin-gated-recheck`)

- **Affected:** t004, t008. **Source:** verified evidence, no user decision.
- **Finding:** the newest pin on origin/main, `826d32e42` (`chore(deploy): pin platform images to 7af50a28168d`, which moves the `bex-operator` image), contains `8a37b982f`: `git merge-base --is-ancestor 8a37b982f 7af50a28168d` holds. The release condition in § Gate is met.
- **Before measuring:** The pin was pushed by `deploy.yml` run 44451653443, whose `deploy` job was still running at 2026-11-19 03:50Z: confirm that run finished green and that `bex-system`'s images match the pin before measuring. The pin does not carry `a6b1658fb` (`w3/m489`'s per-service tier), so production still runs the per-pod edge-cache sidecar this milestone changed; the deploy that carries `a6b1658fb` moves caching services to the tier. The comparison is meaningful on this pin because the sidecar is still inside each instance.
- **Status:** pending verification. A deployed fix is not proof that the definition of done holds on production.
- **Next:** t004 steps 2–3 as written in § Gate (reuse `w3/m486`'s t006 fixture if it is up), record the comparison here, then t008.

## Production record (t004, 2026-11-19)

**Release.** `deploy.yml` run 44451653443 finished green, and every `bex-system` Deployment, `bex-api` and `bex-controller-manager` included, ran `bex-operator@sha256:7ce6ae1e…`, the digest pin `826d32e42` (`7af50a28168d`) writes. The pin contains `8a37b982f`. It does not carry `a6b1658fb`, so the fixture ran the per-pod `edge-cache` sidecar this milestone excludes.

**Fixture.** `m487-t004-cache` (`srv-dbv7cqna346c73cf1o90`) was a starter web service in `bex-canary` running `docker.io/traefik/whoami:v1.10.1`. The QA user (`scripts/qa-login.sh`) created it through REST at 04:09:14Z and turned caching on with REST `PATCH` `serviceDetails.cache.profile=origin-controlled`. Its pod `…-7c4ffb68f7-wg865` ran `app` (500m / 512Mi, the starter plan) beside `edge-cache` (100m / 160Mi). From 04:13:10 to 04:17:10, 16 concurrent requests per round went through the edge: dynamic `/?n=…` paths and 50 `.css` paths (`MISS`, then `HIT`).

**Comparison.** The REST reads were `GET /v1/metrics/{memory,cpu}` with `startTime=04:11:00Z`, `endTime=04:18:00Z` and `resolutionSeconds=60`, plus `percentage=true`, and GraphQL `metrics` with `MEMORY` and `CPU` over the same window. They were compared with Prometheus (`prometheus-server`, the `BEX_PROM_URL` bex-api reads) on that pod at the same steps. Memory used `container_memory_working_set_bytes`, and CPU used `rate(container_cpu_usage_seconds_total[60s])` (shown in millicores, m), for `container="app"` and `container="edge-cache"` separately.

| Step (UTC) | REST = GraphQL memory | Prometheus `app` / `edge-cache` | REST memory % (app ÷ 512Mi) | REST = GraphQL CPU | Prometheus `app` / `edge-cache` | REST CPU % (app ÷ 0.5) |
| --- | --- | --- | --- | --- | --- | --- |
| 04:12 | 1380352 B | 1380352 / 1695744 B | 0.2571 | 0.145 m | 0.145 / 0.352 m | 0.0290 |
| 04:14 | 6545408 B | 6545408 / 6610944 B | 1.2192 | 2.931 m | 2.931 / 5.105 m | 0.5862 |
| 04:16 | 6950912 B | 6950912 / 7188480 B | 1.2947 | 2.633 m | 2.633 / 3.265 m | 0.5266 |
| 04:17 | 7172096 B | 7172096 / 7229440 B | 1.3359 | 3.825 m | 3.825 / 4.790 m | 0.7651 |

- **Usage.** At each of the seven steps from 04:11 to 04:17, REST and GraphQL memory equal the `app` container's working set to the byte. Their CPU equals the `app` container's rate to the millicore. The 04:11 step also carries the pre-caching pod (`…-5dc7876997`, as its own instance), which was rolled away when the sidecar was added, the residual `w3/m489` owns. Before this milestone the reads would have added the sidecar: 14.4 MB at 04:17 instead of 7.2 MB, and 8.6 m of CPU instead of 3.8 m.
- **Limits.** REST `/v1/metrics/memory-limit` and `/cpu-limit`, and GraphQL `MEMORY_LIMIT` and `CPU_LIMIT`, read 536870912 B and 0.5 CPU, the starter plan's (`tiers`: `0.5c-512mb`). The `percentage=true` reads divide by the Prometheus limit history, and every step equals `app ÷ plan`: 1.3359 % at 04:17, where the old selector would have read (app + sidecar) ÷ 672Mi = 2.0438 %. kube-state-metrics held the sidecar's own limit, 167772160 B and 0.1 CPU, which no read included.
- **Not measured live.** MCP `get_metrics` and the dashboard read the same `Service.Metrics` call as REST and GraphQL (§ Render parity), and the fixture had no autoscaling. `TestAutoscalingLeavesTheEdgeCacheSidecarOut` covers the autoscaler, and `TestTheSnapshotPathLeavesTheEdgeCacheOut` covers the pod-spec fallback, which production does not take while Prometheus answers.
- **Cleanup.** The fixture was deleted (`204`, then `GET` `404`) and its pod terminated. This run's QA session was logged out (`--logout` `ok`, then `401`). The other caching App in production, w6/m484's `m484-t005-cache`, was not touched.

**Definition of done.** Each item holds:

- REST, GraphQL, MCP and the dashboard share one read, which reported the fixture's usage from its `app` container alone. Its limit was the plan's on both the history and the pod-spec read.
- `autoscaleDesired` leaves the sidecar out (test).
- On production, the reported memory equals Prometheus `container_memory_working_set_bytes{container="app"}`, and the limit equals the plan's, as recorded above.
