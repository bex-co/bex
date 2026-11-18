# w3 · m487 — Turning on edge caching changes neither a service's Metrics charts nor its autoscaling, as on Render

**Worker:** worker3 **Goal:** an edge-cached service's CPU and memory usage, its limit line and its autoscaling decisions count only its own container, as on Render, where the cache is not inside the instance. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Failing tests: autoscaling and the Metrics queries count the edge-cache sidecar | 30m | — |
| [t002](t002.md) | One shared sidecar name; the pod-metrics decoder leaves the sidecar out for the autoscaler and bex-api | 45m | t001 |
| [t003](t003.md) | Service usage and limit-history PromQL leave the sidecar out | 45m | t001 |
| [t004](t004.md) | Record it in ADR004 and ADR010, ship and verify on production | 45m | t002, t003 |
| [t005](t005.md) | Render parity | 20m | t004 |
| [t006](t006.md) | Simplify | 15m | t005 |
| [t007](t007.md) | Test coverage | 30m | t005 |
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
