# w7 · m160 — Add etcd latency and scrape-health coverage

**Worker:** worker7 **Goal:** Deterministic deployment reconciliation depends on a responsive Kubernetes control plane. Monitor degraded latency before it becomes an opaque hosting failure. **Status:** done

**Estimate:** 180m implementation; 250m including closing tasks. Runtime observation windows may exceed active effort.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Establish secure per-member metrics access — **DONE** | 60m | — |
| t002 | Add latency, quorum/leader and scrape-health coverage — **DONE** | 60m | t001 |
| t003 | Validate thresholds and deployed member coverage — **DONE** | 60m | t002 |
| t004 | Simplify — **DONE** | 20m | t003 |
| t005 | Test coverage — **DONE** | 40m | t003 |
| t006 | Closeout — **DONE** | 10m | t004, t005 |

## Definition of done

- [x] Each member has an authenticated or otherwise appropriately isolated metrics path; no public listener or cluster-admin credential is introduced for monitoring.
- [x] Every member is represented; sustained latency, cluster-health symptoms and telemetry loss have distinguishable actionable alerts.
- [x] All members are scraped; alerts fire and resolve for their tested conditions, and missing monitoring is visible independently of etcd health.

- [x] Required checks and closing tasks are complete, with dated runtime evidence for production-dependent outcomes.

## Source + Goal linkage

- **Source:** user-approved second platform-log brainstorm, 2026-09-28; `$pm all for w7`. Observations were captured against 016391810; materialization checkout is 242fe0c83. Revalidate before implementation.
- **Evidence:** The September 28 capture contains 356 slow-apply warnings, 23 delayed ReadIndex responses, three slow fdatasync warnings and two delayed leader heartbeats. The checked-in Prometheus configuration at brainstorm revision 016391810 has no explicit etcd scrape job or etcd alert rules. This is evidence for visibility, not proof that disks must be replaced.
- **Local evidence:** `/tmp/bex-platform-log-audit/` (manifest, events, node snapshots, container logs and Loki history). Temporary files can expire; the sanitized finding is preserved here.
- **Goal linkage:** ADR008 reliable self-hosted hosting and deterministic platform operation. Deterministic deployment reconciliation depends on a responsive Kubernetes control plane. Monitor degraded latency before it becomes an opaque hosting failure.
- **Expected outcome:** All members are scraped; alerts fire and resolve for their tested conditions, and missing monitoring is visible independently of etcd health.
- **Why now:** Deterministic deployment reconciliation depends on a responsive Kubernetes control plane. Monitor degraded latency before it becomes an opaque hosting failure.
- **Deduplication:** Complements w3/m6 node readiness alerts and existing etcd backup work. m159 handles tenant-node pressure; this milestone covers etcd members, latency and quorum/scrape health only.
- **Render parity omitted:** internal registry/platform monitoring only; no tenant-facing REST/GraphQL/MCP/UI contract change.

## Scheduling and boundaries

Approved second-round priority 3; keep existing m154–m157 work intact. The first round's root GitOps blocker m153 is now done; verify current sync before relying on it. These milestones do not depend on one another, although changes to shared Prometheus configuration must be coordinated. Implementation uses isolated dev-7 where applicable; do not disturb other workstreams' stacks. This filing performs no production mutation.

## Evidence (2026-09-28 UTC, worker7)

**Inventory (t001):** three kubeadm stacked etcd members (`bex-control-plane-{hpjqx,m48fw,v94f8}`, InternalIPs 10.10.0.15/.14/.12, image `registry.k8s.io/etcd:3.6.5-0`). `--listen-metrics-urls=http://127.0.0.1:2381` (loopback only); the client port requires an etcd client certificate (full keyspace), rejected as a monitoring credential. Changing the kubeadm flag would roll every control-plane machine. Chosen path: `kube-rbac-proxy` v0.22.1 (digest-pinned) per CP node, host network bound to `status.hostIP`, TokenReview + SubjectAccessReview for `GET /metrics`, cert-manager private CA with serving SAN `etcd-metrics.monitoring.svc` pinned by Prometheus `server_name`; Prometheus mounts only `ca.crt`. Allowlisted metric names verified against the etcd 3.6.5 image (peer RTT registers only with peers; present in prod).

**Implemented (t002), shipped `3cd06f1cc`:** `deploy/gitops/charts/etcd-metrics` + Argo app, `etcd` scrape job, rules EtcdMetricsMissing (warning, per node + absent), EtcdNoLeader (critical), EtcdFrequentLeaderChanges, EtcdHighFsyncLatency (p99 > 500ms), EtcdHighCommitLatency (p99 > 250ms), EtcdDatabaseNearQuota (> 80%), two Cluster capacity panels, ADR010 section; `EtcdMetricsMissing` waived in the panel audit with a reason (context-series only; the panel draws `up{job="etcd"}`).

**Tests (t005):** promtool — per-node scrape loss raises EtcdMetricsMissing for that node only and does **not** raise EtcdNoLeader; no target at all raises it; leader loss pages at 6m and clears by 10m; sustained fsync ≈ 990ms fires at 30m while the first minutes do not. `gitops-validate.sh` passes (render, promtool, panel audit).

**Production (t003, 07:38–07:41Z):** DaemonSet 3/3 on the CP nodes; both Certificates Ready; all three `etcd` targets `up` with no scrape error, ~330 allowlisted series. Security, from a throwaway pod in `monitoring` (deleted after): no token → **401**; the namespace `default` SA token → **403**; `/health` → **404** (`--allow-paths`); TLS without the pinned name → curl exit 60; nothing listens on the public IPs (46.225.151.66, 178.105.18.185 :9979 closed). Live values: WAL fsync p99 3–5ms, commit p99 4–9ms, DB 10–11% of quota, exactly one leader (m48fw); all six rules loaded and **inactive** (thresholds do not false-fire). Cumulative `etcd_server_slow_apply_total` is 298 / 1864 / 2963 and leader changes 3 / 3 / 20 — the audit's slow-apply log lines are real but steady-state latency is far below the alert thresholds; no disk action is indicated. Failure paths were exercised in promtool only (no production quorum disruption); live critical-page delivery was not exercised.

**Simplify (t004):** inline review; the proxy carries no etcd credential and one allowlist.
