# w4/m144 verification — 2026-10-02

## Observed failure and capacity

Read-only production evidence at approximately 08:17–08:20 UTC:

| Fact | Observed value |
| --- | --- |
| Pod | `monitoring/prometheus-server-5797f5666d-sfzpg` |
| Image | `quay.io/prometheus/prometheus:v2.54.1` |
| Last exit | `OOMKilled`, code 137, finished `2026-10-02T08:14:24Z` |
| Restarts / endpoint | 577 / ready=false, serving=false |
| Last previous-log segment | 1392 of 1591 at `08:14:10.816Z`; no replay-complete line |
| Existing budget | memory request/limit 256/512 MiB, CPU 50/500m |
| PVC | `monitoring/prometheus-server`, Bound, RWO, `hcloud-volumes`, provisioned 10 GiB |
| PV identity | `pvc-c3292b20-2d80-471a-a851-ccd22623cd00` |
| Data usage | 1,590,988,800 bytes, approximately 1.48 GiB |
| Node | `bex-platform-h7t89-vhv2v`, Ready=True, MemoryPressure=False |
| Node allocatable / pod requests | 7,834,828 KiB / 5,526 MiB |
| Node available / working set | 3,773,476,864 / 4,354,244,608 bytes at `08:18:19Z` |
| Pod cgroup maximum | 567,005,184 bytes; includes sidecar/pod overhead, **not** a successful server replay peak |
| Rollout strategy | Recreate; existing PVC claim retained |

Commands used the existing production kubeconfig without printing it: `get pod`,
`logs --previous --tail=14`, `get endpointslice`, `top nodes`, `get pvc`,
`describe node`, and kubelet `/stats/summary` / `/metrics/cadvisor` read endpoints.
No production object, volume or retention setting was changed by these probes.
The latest kill remains a lower bound on required memory; series cardinality and
successful replay/runtime/query peaks are not yet known.

## Bounded recovery change

The production Application alone layers `server.resources.requests.memory: 1Gi`
and `limits.memory: 2Gi` through Helm parameters. CPU, image/chart identity,
PVC, WAL, retention, placement, scrape selection and rule files are unchanged.
Base retains 256/512 MiB; local now correctly inherits that intended budget,
disables Alertmanager, and keeps its CAPD filesystem-discovery override.

The extra request is 768 MiB: node reservations become 6,294 MiB, leaving about
1,357 MiB allocatable. A 2 GiB server working set would leave roughly 1.5 GiB of
the observed available memory. Other containers' limits are already overcommitted
and some have no requests; those numbers do not guarantee simultaneous peaks fit.
This is a schedulable recovery attempt with a fixed ceiling, not a declaration
that 2 GiB has been proven sufficient. Reassess capacity and measured peaks at
rollout before calling the incident resolved.

## Shared consumer matrix

All nine sources are wired from `cfg.PromURL` in
`lego/backend/cmd/api/main.go:555–568`. The source constructors remain unchanged.
The following are **source audits**, not live recovery successes.

| Source | Required series / behavior |
| --- | --- |
| RequestMetrics | Traefik service/router request counters and latency buckets; egress counters below. HTTP counts prefer Loki and use Prometheus as fallback; latency remains Prometheus-backed. |
| ResourceMetricsRange | `container_cpu_usage_seconds_total`, `container_memory_working_set_bytes`; namespace and workload pod selectors. |
| ResourceLimitRange | `kube_pod_container_resource_limits`, used for historical percentage denominators. |
| MonthToDateBandwidth | Router responses, WebSocket, direct-App and public datastore egress counters plus source-health evidence. |
| MetricsFilterValues | Scoped `code` discovery from Traefik service/router request counters. |
| DiskUsage | `kubelet_volume_stats_used_bytes` and exact resource PVC patterns. |
| DBConnections | `cnpg_backends_total`, namespace and CNPG cluster scoped. |
| ReplicationLag | `cnpg_pg_replication_lag`; queried only for HA databases. |
| KeyValueStats | `redis_memory_used_bytes`, `redis_connected_clients`, scoped by namespace and StatefulSet ordinal. |

Egress selectors remain centralized in `internal/egressquery/query.go`: router
`traefik_router_responses_bytes_total`; `bex_websocket_egress_bytes_total`;
`bex_app_direct_egress_bytes_total`; `bex_pg_proxy_egress_bytes_total` /
`bex_kv_proxy_egress_bytes_total`, with the existing job `up`, health,
process-start and counter-loss series. No scrape family or rule is removed.

| Additional consumer | Preserved contract / remaining live check |
| --- | --- |
| Operator free-service activity | Traefik requests plus WebSocket ingress/egress; read failure keeps the service awake. Verify healthy activity reads after recovery. |
| Operator database disk growth | Used/capacity PVC metrics; failure records an unavailable sample and defers growth. Verify sample availability without inducing datastore growth. |
| Usage collector | cAdvisor presence, PVC usage and shared egress sources; transport errors defer an hour, degraded source health is recorded. Recovery is not proof of repaired billing or historical backfill. |
| Grafana | Existing default Prometheus datasource and dashboards; verify source health after recovery. |
| Rule evaluator | Both existing rule files and the full platform/bex/log-delivery rule groups remain; verify loaded rules and evaluation health after recovery. |

Resource families: web/private/worker use the shared resource sources; cron can
have honest empty between-run windows. Ordinary scheduled cron pod names fit the
shared matcher, but shortened long CronJob names and manual `-run-<hash>` jobs
were not live-verified and are not guaranteed by this budget fix. Static UI uses
network metrics, excluding compute charts. Postgres uses disk/connections and
HA-only lag; Key Value uses disk/memory/connections; attached App disks use their
exact PVC. No live success is claimed for these families before source recovery.

Authorization still precedes source access. Unknown resources, forbidden access,
bad names/windows, unavailable sources and healthy empty matrices remain distinct.
Configured Prometheus failure does not activate the metrics-server fallback.
CPU_LIMIT/MEMORY_LIMIT, autoscaling targets and logical disk capacity remain
source-independent. No backend/dashboard/query-contract code changed.

## Render comparison

[Render service metrics](https://render.com/docs/service-metrics), read
2026-10-02, includes the CPU/memory journey. This change restores infrastructure
capacity for Bex's existing contracts and extensions; it changes no wire fields,
units or authorization. ADR018 now qualifies its implementation claims with the
observed outage. Production API/UI parity remains pending the fresh-window replay.

## Automated validation

The existing `scripts/test_platform_metrics.py` renderer is reused. It now
matches [Argo's documented precedence](https://argo-cd.readthedocs.io/en/stable/user-guide/helm/#helm-value-precedence):
`valuesObject` replaces inline `values`; parameters override the selected
representation. Production memory and local inventory/exporter changes use
parameters so the shared inline configuration survives.

A locked-chart render of the **original** local overlay with actual Argo
precedence exposed the pre-existing bug: no server resources, 15-day default
retention, 10 default scrape jobs, and missing base alert rules. The corrected
local overlay renders 256/512 MiB, three-day retention, 21 scrape jobs and both
rule files. This is local rendered evidence, not a claim about a live local
cluster.

The one-off proof reused `render_application` and compared actual Kubernetes
objects: production ConfigMap and PVC equal base; the production Deployment
matches base after normalizing only container resources; all three server flag
lists match; local resources/PVC/base alert rules equal base. Base/prod/local
all render 21 jobs, the same existing PVC claim and Recreate strategy. The
configured claim remains 8 GiB (the existing production provisioner allocated
10 GiB); it is not recreated or resized by this change. Results:
`/tmp/bex-w4-m144-render-proof.json`.

The existing platform-metrics suite passed both overlays, including 32 production
and 28 local Application identities plus four database identities per overlay.
All three final simplify reviewers reported clean. Full
`bash scripts/gitops-validate.sh` passed, including locked-chart renders, both
platform-metrics overlays, 113 alert rules plus the inventory recording rule and
their promtool fixtures, and the OpenFGA model transform. Logs:
`/tmp/bex-w4-m144-gitops-final.log` and
`/tmp/bex-w4-m144-platform-final.log`. Markdown formatting, `git diff --check`
and the skill-layout validator also passed. These checks verify configuration
behavior; they do not substitute for the retained-data production restart.

## Remaining release and operational gate

Owner: production GitOps release operator, then QA/platform operator. The root
`bex-platform-prod` Application consumes the production overlay; its Prometheus
child must apply the new budget to the existing claim. Record that applied budget
and PVC identity, then:

1. Observe WAL replay completion, Ready pod and ready EndpointSlice on retained
   data; measure cgroup/process replay peak, steady-state/query peak and head
   cardinality, and reassess colocated node headroom.
2. Verify a restart with that same data and a stable restart count over 15
   minutes. An empty volume is not this regression check.
3. Repeat the exact REST/GraphQL/MCP and Scaling hard-reload probes in finding.md
   with a fresh window; require real scoped samples where collected, retain
   CPU_LIMIT/MEMORY_LIMIT controls and honest outage gaps.
4. Check the named consumer/family matrix, query-error/empty/authorization
   boundaries, rule evaluation, Grafana source and collector health. Record
   unverified cases explicitly; do not infer billing repair.

The ship workflow ends at a successful push and prohibits watching CI/deployment;
this run therefore records the release/operational gate rather than asserting
post-push recovery. This item created no production fixtures or QA session, so
there is no drill-specific credential or resource cleanup outstanding.
