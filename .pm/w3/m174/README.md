# w3 · m174 — Alerting and backups stop failing silently

**Worker:** worker3 **Goal:** when the alert path itself breaks, a tenant disk snapshot or a Postgres base backup stops, or tenant pods cannot be scheduled, a human is told — through a path that does not depend on the broken component **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Watchdog rule and self-monitoring for Prometheus and Alertmanager | 45m | — |
| t002 | Out-of-cluster dead-man check that opens a GitHub tracking issue | 1h | t001 |
| t003 | Watch the tenant disk-snapshot CronJobs (`dskbak-*`) | 30m | — |
| t004 | Alert on Postgres base-backup age | 1h | — |
| t005 | Alert on unschedulable tenant pods and a full serving pool | 40m | — |
| t006 | Panels, coverage inventory and runbook links for the new alerts | 40m | t001, t003, t004, t005 |
| t007 | Fault-injection proof on the mock cluster | 1h | t002, t006 |
| t008 | Simplify | 15m | t007 |
| t009 | Test coverage | 30m | t007 |
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
- **Relation to `038`:** independent. `w3/blocked/038` chooses a low-noise channel for the warning digest; this milestone's heartbeat uses GitHub issues and needs no new credential. When 038 lands, the heartbeat may also post there.
