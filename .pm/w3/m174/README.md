# w3 · m174 — Alerting and backups stop failing silently

**Worker:** worker3 **Goal:** when the alert path itself breaks, a tenant disk snapshot or a Postgres base backup stops, or tenant pods cannot be scheduled, a human is told — through a path that does not depend on the broken component **Status:** todo — t001–t006, t008, t009 done; 2026-10-29 unblock: new t011 fixes the heartbeat (it has reported FAIL every hour since 2026-10-21), then t007 drill on a rebuilt mock cluster + production checks, t010 closeout

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Watchdog rule and self-monitoring for Prometheus and Alertmanager — **DONE** | 45m | — |
| t002 | Out-of-cluster dead-man check that opens a GitHub tracking issue — **DONE** | 1h | t001 |
| t003 | Watch the tenant disk-snapshot CronJobs (`dskbak-*`) — **DONE** | 30m | — |
| t004 | Alert on Postgres base-backup age — **DONE** | 1h | — |
| t005 | Alert on unschedulable tenant pods and a full serving pool — **DONE** | 40m | — |
| t006 | Panels, coverage inventory and runbook links for the new alerts — **DONE** | 40m | t001, t003, t004, t005 |
| t011 | Heartbeat actually reaches Alertmanager/Prometheus in production | 1h | t002 |
| t007 | Fault-injection proof on the mock cluster | 1h | t002, t006, t011 |
| t008 | Simplify — **DONE** | 15m | t007 |
| t009 | Test coverage — **DONE** | 30m | t007 |
| t010 | Closeout | 10m | t008, t009 |

## Definition of done

- Every new rule has a firing and a non-firing case in the existing rule-test harness (`scripts/test_platform_metrics.py` / `scripts/alerts-verify.sh`), run in CI.
- On the mock cluster, each injected failure produces its signal, recorded in `docs/drills/`: Alertmanager scaled to 0 ⇒ the out-of-cluster check opens its GitHub tracking issue within its window and closes it after recovery; a `dskbak-*` CronJob with no success past threshold ⇒ `BackupCronJobStale`; a failed or missing base backup ⇒ the base-backup-age alert; an unschedulable tenant pod ⇒ `TenantPodUnschedulable`.
- `scripts/obs-coverage-check.sh` passes with the new alerts inventoried and paneled; every new alert links an existing runbook anchor; the 11 billing alerts' broken anchor is fixed.

## Evidence (2026-10-19)

- No `Watchdog`/heartbeat rule, no Prometheus or Alertmanager self-scrape, no rule on failed notifications or failed rule evaluations (0 matches in `deploy/gitops/base/prometheus.yaml`; the scrape jobs at :263–742 cover neither). Delivery is one Alertmanager → SendGrid (`:122-158`) → one inbox; [`w8/m30` t004](../../w8/done/m30/done/t004.md) records 512 alert emails lost to a no-MX domain and two days of a critical backup alert never seen.
- [ADR088](../../../docs/ADR088-platform-observability-ui.md):23,:220 rely on an "external uptime check" for whole-cluster outages that is defined nowhere in the repo.
- `BackupCronJobStale` matches `etcd-backup|openbao-backup|kvbak-.*` only (`prometheus.yaml:1687-1694`), so the operator's `dskbak-*` CronJobs (`lego/operator/internal/controller/disk_backup.go:48`) — the only recovery point for tenant disks ([ADR082](../../../docs/ADR082-persistent-disks.md)) — are unwatched; ADR031's store tables have no disk row.
- `PlatformDatabaseBackupStale` reads WAL-archiver time only (`prometheus.yaml:1761-1762`); no CNPG base-backup metric is scraped, so a failing nightly base backup under healthy WAL archiving leaves every alert green.
- No rule on Pending/unschedulable tenant pods (only `BuildQueuedTooLong` for builds); the serving pool caps at 3 nodes, and `infra/clusterapi/overlays/hetzner-caph/cluster.yaml:373-376` records rollout pods that already went unschedulable.
- 11 billing alerts link `docs/runbooks/stripe-billing-setup.md#operations-reconciliation-and-repair`, but the heading is `## 5. Operations: reconciliation and repair` (`:182`).

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w3` 2026-10-19, item 3 (approved by the user the same day); verified against `prometheus.yaml`, `disk_backup.go`, ADR031/ADR082/ADR088 and `w8/m30`.
- **Goal linkage:** V0 roadmap 2 (basic observability for operation); [ADR088](../../../docs/ADR088-platform-observability-ui.md) coverage and [ADR031](../../../docs/ADR031-platform-data-backup.md) backup detection bounds, which assume a working notification path.
- **Expected outcome:** the four silent failure classes above each reach a human; the alert path has an independent heartbeat, so "no alerts" again means "nothing is wrong".
- **Why now:** alert delivery has already failed silently once; ADR082 disks are live in production and their only backups are unwatched; ADR031's annual all-store re-drill (next 2027-07-31) makes continuous detection the only near-term safety net.
- **Render parity omitted:** platform-internal alerting, scrape and runbook changes — no REST/GraphQL/MCP/UI surface.
- **Relation to `038`:** independent. `w3/done/038` (won't do, 2026-10-29: the warning digest stays on email); this milestone's heartbeat uses GitHub issues and needs no new credential. 

## Unblock 记录（2026-10-29，/unblock w3，origin/main `e4d9c65d6`）

- **涉及：** t007、t010，新增 t011。
- **用户明确决定 D1（2026-10-29）：** loopx w3 可以用本机 `.env` 的 `HCLOUD_TOKEN` + `~/.ssh/bex` 经 `scripts/fetch-app-kubeconfig.sh`（kubeconfig 写到 `mktemp -d` 下，用完删除）对生产做**只读**检查：`kubectl get/logs/events`、`kubectl port-forward` 查询 Prometheus/Alertmanager。禁止 apply/patch/exec/delete/scale，禁止打印凭据。注：本机 `.env` 为 0644 时 `bex_load_env` 会拒绝加载，这时只取 `HCLOUD_TOKEN` 这一个值传给脚本，不要 cat 整个文件。
- **现有规则（AGENTS.md「Local dev environments are pre-approved」）：** `scripts/mock-cluster.sh` 的完整重建（包括 reprovision）不需要用户批准，mock cluster 宕机不构成 blocked。2026-10-29 复核：`docker ps` 仍只剩 `bex-tenant-0-…` 一个容器，worker 需先重建，并在交接里报告重建了什么。
- **本次复核证据（生产只读）：** Alertmanager 中 `Watchdog` 为 `active`；`bex_cnpg_backup_stopped_at_seconds` 覆盖全部 4 个平台集群（`bex-db` 177、`kratos-db`/`hydra-db`/`openfga-db` 各 8 条 Backup 序列；最新 base backup 在 17.8–19.5h 前）；`PostgresBaseBackupStale` 未 firing。t007 这三项生产检查已满足。
- **新发现的真缺陷 → t011：** `alerting-heartbeat.yml` 每次运行都是 success，但脚本实际输出 `FAIL: cluster API or Alertmanager unreachable … / FAIL: Prometheus unreachable through the API server`（例如 run `40955985179`，2026-10-29T21:13Z），issue #99 自 2026-10-21 一直开着。kubeconfig 能取到（`via CP 91.107.197.144`），但本地复现 `kubectl get --raw /api/v1/namespaces/monitoring/services/prometheus-server:80/proxy/api/v1/rules` 超时，而 `kubectl port-forward svc/prometheus-server` 正常。所以问题在 apiserver→monitoring Pod 的 service-proxy 路径（可能是 CP 主机网络到 Pod 的路由，或 NetworkPolicy/Cilium 拦截了 apiserver 入站），`scripts/alerting-heartbeat.sh:47` 的 `2>/dev/null` 把错误吞掉了。「workflow 是绿的」**不能**当作 heartbeat 通过的证据。
- **剩余阻塞：** 无。worker 依次做 t011 → t007（重建 mock cluster 后做四个注入，并补「heartbeat 首次 down=false 且 #99 自动关闭」的 run id）→ t010。
