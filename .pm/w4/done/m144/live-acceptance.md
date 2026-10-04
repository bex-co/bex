# m144 operational acceptance — 2026-10-02 (UTC 2026-10-03T07:22–07:41Z)

All checks were read-only: kubectl get/logs/top, a `kubectl port-forward` to the Prometheus read API (stopped afterwards), and the signed-in QA session's API reads. Nothing was restarted, scaled, edited or deleted, and no fixture was created. Platform image train: GitOps pin `20fb64867` → `1263d12ae` (contains `0436d563d`, the production values fix).

## Retained-data restart under the new budget

- Pod `monitoring/prometheus-server-5db4bfcd9f-lv2kj`, created 2026-10-02T08:35:57Z. Container `prometheus-server` has requests `cpu 50m / memory 1Gi` and limits `cpu 500m / memory 2Gi`, with args `--storage.tsdb.retention.time=3d --storage.tsdb.path=/data`. The volume is the existing PVC `prometheus-server` (10Gi, created 2026-07-10).
- Startup log: 16 healthy persisted blocks found. `Replaying WAL` started 08:36:01Z over segments 1016→1595 (580 retained segments). `WAL replay completed checkpoint_replay_duration=5.71s wal_replay_duration=53.92s` at 08:37:00Z, then `TSDB started` and `Server is ready to receive web requests` at 08:37:04Z, followed by normal compaction and checkpoint. WAL was not cleared, and `prometheus_tsdb_wal_corruptions_total` is 0.
- This was a real restart over retained data with the new budget. It came from the GitOps rollout, not from this run. `--previous` logs: `previous terminated container … not found` (no crash since start).

## Stability and readiness

| UTC | restartCount (server / reload) | ready | EndpointSlice `prometheus-server-pmwtk` |
| --- | --- | --- | --- |
| 07:22:17 | 0 / 0 | true | — |
| 07:25:50 | 0 / 0 | true | ready=true serving=true |
| 07:38:47 | 0 / 0 | true | — |
| 07:41:14 | 0 / 0 | true | ready=true serving=true |

The 15-minute observation window passes, on top of about 23 h of uptime with zero restarts. `kube_pod_container_status_restarts_total` = 0.

## Measured memory, cardinality and capacity

- cAdvisor working set for the server container: first post-replay sample 534 MiB (08:38Z), 23 h minimum 437 MiB, **23 h maximum 760 MiB** (797,450,240 B), and 557–601 MiB current. `process_resident_memory_bytes` is 563 MiB and Go heap in use is 467 MiB.
- The replay peak itself cannot be read back: Prometheus does not scrape its own pod while replaying, and no restart was performed. It is bounded by the completed 580-segment replay within the 2 GiB limit with no OOMKill. Before the change, the replay OOMed at 512 MiB.
- Head series 114,635 (11,695 label pairs, 184,656 chunks). Largest metric families: `traefik_router_request_duration_seconds_bucket` 5,640, `traefik_service_request_duration_seconds_bucket` 3,480. Blocks on disk 320 MiB. Scrape samples per cycle 150,688. Query p99 inner_eval 11 ms.
- Node `bex-platform-h7t89-vhv2v`: 5042Mi used (65%) of 7.47Gi allocatable, MemoryPressure=False. Requests 82% and limits 251% (overcommitted, same as before). The 2 GiB cap leaves about 2.6× headroom over the measured runtime peak.

## DoD API/UI replay (fresh window 2026-10-03T06:20–07:20Z, `srv-d9bj8s3eg85c7390eb9g`)

- GraphQL `QaMetrics`: `errors: null`. CPU has 13 points (unit `cpu`), MEMORY 13 points (`bytes`), INSTANCES 13 points (`count`), and CPU_LIMIT returns 1 cpu (the pod limit).
- REST `GET /v1/metrics/cpu?...resolutionSeconds=300` returns HTTP 200 with one instance-labelled series. MCP `get_metrics` (`cpu_usage`, `memory_usage`, `instance_count`) returns `isError` null with 13 points each, scoped to the same instance and resource.
- The historical outage window 2026-10-02T06:40–07:40Z returns `errors: null` and **zero points**. The gap stays a gap; nothing was backfilled or zero-filled.
- Dashboard `/services/srv-d9bj8s3eg85c7390eb9g/scaling`, loaded in an independent headless Chromium and then hard-reloaded: every `metrics` GraphQL response was 200 with no errors (series with 62/88/58 points, plus limits). Zero "Couldn't load this metric", 0 console errors, and the Memory/CPU/Instances charts render real curves.

## Shared consumer matrix (live)

| Consumer | Live outcome |
| --- | --- |
| RequestMetrics / MetricsFilterValues | `traefik_service_requests_total` 608 series, router duration buckets 5,130 |
| ResourceMetricsRange / ResourceLimitRange | tenant cAdvisor CPU 27 / memory 27 series; `kube_pod_container_resource_limits` 77; API replay above passes |
| MonthToDateBandwidth / usage egress | router bytes 1,026, WebSocket ingress/egress 570/570, direct-App 547, `bex_kv_proxy_egress_bytes_total` 6. The `bex-pg-proxy` job exports `bex_pg_proxy_egress_bytes_total` with health and start-time series (no current samples in the instant vector). |
| DiskUsage / operator disk growth | `kubelet_volume_stats_used_bytes` 6 tenant series |
| DBConnections / ReplicationLag | `cnpg_backends_total` 18, `cnpg_pg_replication_lag` 9; all cnpg exporters up |
| **KeyValueStats** | **Partial — separate root cause.** Target `valkey-instances` for production `red-d9p49kdrtmes73c34ovg-0` (beancount-forum-redis) is down with `context deadline exceeded`: scrape duration 10.0 s (equal to the timeout), 23 h availability 0.2–6%. The exporter sidecar is CFS-throttled 98.5% under its `guaranteedResources("10m","32Mi")` budget (`lego/operator/internal/controller/keyvalue_controller.go:171`). The other production store scrapes in 3.1 s at 93% throttling. Prometheus is healthy, so this is not the OOM outage; it needs its own filing. |
| Operator free-service activity | auto-hibernate reconciles normally (e.g. 07:07:47Z); no Prometheus read errors in 22 h of manager logs |
| Usage collector | running; records degraded egress sources per hour ("unhealthy … skipped this hour") as designed; no Prometheus transport errors in 22 h of bex-api logs. No billing repair or backfill is claimed. |
| Rule evaluator | 4 groups / 114 rules from `alerting_rules.yml` and `platform_gitops_expected.yml`, **0 unhealthy** |
| Grafana | pod Running for 4 days, no datasource or Prometheus errors in 24 h of logs. The datasource health API was not called (needs Grafana admin auth). |
| Scrape health overall | 78 targets up, 1 down (the Key Value exporter above) |
