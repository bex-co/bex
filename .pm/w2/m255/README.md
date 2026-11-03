# w2 · m255 — Disk usage over 80% notifies, as on Render

**Worker:** worker2 **Goal:** when a service's persistent disk usage exceeds 80% of its allocated size, and when it drops back below, owners get exactly one notice each way. **Status:** in progress — t001–t003, t005–t007 done; t004 (live walk on production) next, then t008

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Disk usage evaluator with hysteresis — **DONE** | 1.5h | — |
| t002 | Durable crossing facts in the service events feed — **DONE** | 1h | t001 |
| t003 | Email both directions plus push — **DONE** | 45m | t002, w2/m252/t005 |
| t004 | dev-2 and production walk | 30m | t003 |
| t005 | Render parity — **DONE** | 15m | t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 30m | t005 |
| t008 | Closeout | 10m | t006, t007 |

## Definition of done

- On dev-2 and production, filling a service disk past 80% produces one "above 80%" email per eligible member; freeing it below produces one "back below" email (to `notificationsToSend: all` services, Render's level).
- Writing and deleting around the threshold does not produce repeated notices (hysteresis).
- The crossings appear in the service events feed; any Render events/webhook naming divergence is recorded in ADR018.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-11-02 #5. render.com/docs/notifications: "A service's persistent disk usage exceeds 80% of the disk's allocated space" (Email ✓) and "… drops back below 80%" (Email ✓). bex has no such event; the signal exists (`kubelet_volume_stats_used_bytes`, `lego/backend/internal/metrics/source.go:712`).
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first) for the ADR082 persistent disks shipped in w1/m83–m86.
- **Expected outcome:** a disk filling up warns its owner before the app breaks.
- **Why now:** disks are sold; a full disk silently breaks a tenant app; this is the last Render notification row w2/m252 does not cover.
- **Render parity included:** events feed, notifications and settings are tenant-facing.

## Work log — 2026-11-03

- **t001:** `internal/diskusage.Sweeper` (bex-api, every 5 min) reads the kubelet's used bytes for each running service disk (`metrics.ServiceDiskPVCPattern`) against the disk's own size from `service_disks`. A crossing of 80% is first recorded as pending (`service_disk_usage_state`) and confirmed only by the next sweep within two intervals, at exactly 80% in both directions; a stale sighting (service parked or series missing in between) starts again.
- **t002:** Render's `service_disk_usage_high` / `service_disk_usage_recovered` facts (migration 0183, renumbered past a concurrent session's 0179–0182) with a typed side table `service_event_disk_usage` attached to the feed after the read (the `attachDeployEnds` pattern), so the UNION projection is unchanged. REST/GraphQL/MCP event details are exactly Render's `{diskId, diskName, mountPath, usagePercent, thresholdPercent}`; the Events tab shows a label and detail line (en/zh). They are Events-API types only in Render's contract (`eventTypeParam`), not webhook types, so no webhook vocabulary changed.
- **t003:** `DiskUsageNotifier` enqueues through the failure-alert queue before the crossing is recorded (keyed by the crossing). High is failure-level; back-below goes only to `notificationsToSend: all`, Render's "All notifications" level. The legacy `notifyOnFail: notify` now folds to `failure` rather than `all` (identical for failures, and it no longer opts into non-failure mail).
- **DoD refinement (2026-11-03, worker2):** the first bullet's "and push" is dropped. Render delivers these two notifications by email and Slack only (render.com/docs/notifications); bex's push channel has no disk event in its vocabulary and Render gives no basis to add one. Email both ways, plus the event, is the parity behavior.
- **t005 Render parity:** ADR018 Notifications row back to ✅ (every Render notification row is now covered or a recorded non-goal: Slack, one-off jobs).
- **t006/t007:** review fixes (stale-sighting guard, confirm only the pending crossing, shared metrics helpers); tests: `internal/diskusage` (two-sweep confirmation both ways, no flap on a one-sweep dip, stale sighting, unmounted/suspended/hibernated skipped, notify-before-record), `TestDiskUsageCrossingsPG`, `TestDiskUsageEventRendersRenderDetails` (REST/GraphQL/MCP), `TestDiskUsageNotifierLevels`.
