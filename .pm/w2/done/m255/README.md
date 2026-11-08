# w2 · m255 — Disk usage over 80% notifies, as on Render

**Worker:** worker2 **Goal:** when a service's persistent disk usage exceeds 80% of its allocated size, and when it drops back below, owners get exactly one notice each way. **Status:** done (2026-11-08, D1=A)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Disk usage evaluator with hysteresis — **DONE** | 1.5h | — |
| t002 | Durable crossing facts in the service events feed — **DONE** | 1h | t001 |
| t003 | Email both directions plus push — **DONE** | 45m | t002, w2/m252/t005 |
| t009 | dev-env runs Prometheus so the disk-usage sweeper runs on dev-N — **DONE** | 1h | t003 |
| t004 | dev-2 walk (D1=A: dev-2 + tests are the acceptance) — **DONE** | 30m | t009 |
| t005 | Render parity — **DONE** | 15m | t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 30m | t005 |
| t008 | Closeout — **DONE** | 10m | t004, t006, t007 |

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

## Blocked (2026-11-03)

All implementation is done (t001–t003 and t005–t007 done; t004 (disk-fill walk) blocked). The remaining walk needs a gate this session cannot clear: user: choose how the local CAPD cluster runs the egress nft rule so dev-2 comes up, or approve a paid disk-bearing production canary (QA rules forbid buying one). Run `/unblock w2` to decide. The milestone returns to the queue once the gate clears.

## 解阻复核（2026-11-04，复核证据，待用户决定）

- **根因：** 本地 CAPD 节点用的是 Docker Desktop 的 linuxkit 内核（`7.0.12-linuxkit`），它不支持 nft 的 `socket cgroupv2` 匹配。
- **证据：** 在临时 kind 节点里实测，`--cgroupns=private` 下各层级都失败；`--cgroupns=host` 下用绝对路径和正确层级（level 4）同样报 `Could not process rule: No such file or directory`。纯地址规则 `ip daddr 169.254.169.254 reject` 可以加载。生产的 Hetzner 节点不受影响。
- **可行解法（待用户选）：**
  - (A) local-capd overlay 改用不依赖 cgroup 的 metadata 地址拦截规则，生产规则不变；本地本来就没有 metadata 服务。
  - (B) 本地改用内核支持 nft socket cgroupv2 的容器运行时（如 colima/OrbStack 的 VM）。
  - (C) 不修本地，改走生产付费金丝雀。

## 解阻决定（2026-11-04，来源：用户决定 D1=A）

- **涉及：** m252 t006、m253 t005、m255 t004。
- **决定：** 只在 `infra/clusterapi/overlays/local-capd/cluster.yaml` 里，把依赖 `socket cgroupv2` 的 containerd 出站规则换成不依赖 cgroup 的 metadata 地址拦截（`ip daddr 169.254.169.254 reject`，必要时加 IPv6 的 metadata 地址）。生产 overlay（hetzner-caph）的规则一字不改。
- **理由：** Docker Desktop 的 linuxkit 内核不支持 nft `socket cgroupv2`（2026-11-04 实测）。本地仍然保留一道 metadata 拦截，不是移除安全控制；客户无感知。
- **worker 下一步：**
  1. 改 local-capd overlay 里所有出现这条规则的位置，并同步 `scripts/clusterapi-validate.sh` 对本地的断言。
  2. 用 `scripts/mock-cluster.sh` 重建本地集群，确认节点上 containerd 为 `active`、拦截表已加载。
  3. 用 `scripts/dev-env.sh 2 up` 起 dev-2，执行本里程碑剩余的 dev-2 验收。
- **验收：** `clusterapi-validate.sh` 通过；dev-2 起来；本里程碑的 dev-2 证据写入对应任务。
- **仍待决定（生产部分）：** DoD 要求在生产上验证，而这需要付费资源（带磁盘的付费服务），用户这次没有批准（选了 A 而不是 C）。worker 完成 dev-2 部分后，如果仍缺生产条件，就重新 blocked 并把这个问题交回用户：是否接受 dev-2 证据加 envtest 作为验收，或者批准付费金丝雀。

## dev-2 walk — 2026-11-05 (`/loopx w2`): blocked locally

The disk service could not provision on a fresh mock: the `bex-tenant-operator` ClusterRole was missing, which is now fixed in `dev-env.sh up` (`33ad3ad5f`). Even with that fixed, dev-2 runs no Prometheus and has no `BEX_PROM_URL`, so the disk-usage sweeper never starts. It is also unverified whether local-path volumes publish `kubelet_volume_stats_used_bytes`. So no DoD point can be shown on dev-2.

## Blocked (2026-11-05)

The production walk needs a paid disk-bearing canary, which D1=A did not approve, and dev-2 cannot exercise the sweeper. **User decision:**

- approve a paid production canary (a Starter service with a 1 GB disk, filled past 80% and then freed, then deleted), or
- approve adding Prometheus with `BEX_PROM_URL` to dev-env so the sweeper runs locally, or
- accept the unit/envtest coverage.

Run `/unblock w2`.

## 解阻决定（2026-11-07，来源：用户决定 D1=A）

- **决定：** 不做付费金丝雀。worker 给 dev-env 加上 Prometheus（本地，零成本），让磁盘用量巡检在 dev-2 上真正运行。
- **验收步骤：** 在 dev-2 上完整走一遍。
  1. 写满磁盘超过 80%，收到邮件和事件。
  2. 在阈值附近抖动，不重复发送。
  3. 释放到阈值以下，收到恢复通知。
  4. 验收完成后关闭。
- **worker 下一步：**
  1. 新增任务 t009：dev-env 部署 Prometheus 并设置 `BEX_PROM_URL`。
  2. 确认 local-path 卷能产出 `kubelet_volume_stats_used_bytes`；如果不能，选用等价的指标来源，并记录理由。
  3. 走查 t004 的 dev-2 部分。
- **对标依据：** Render 在磁盘超过 80% 时会通知，代码已经对齐。这个决定只关系到验收证据。

## dev-2 walk — 2026-11-08 (`/loopx w2`): passed; closed (D1=A)

- **t009:**
  - `dev-env.sh up` now runs a shared Prometheus in `monitoring` (production image, the production `kubernetes-kubelet` job verbatim) and sets dev-N's `BEX_PROM_URL` (dev-2 on :48020). `status` checks that it is ready and that volume series exist.
  - **Finding:** local-path (hostPath) volumes publish no `kubelet_volume_stats_used_bytes`. Zero series were found on all three CAPD kubelets. A `du` DaemonSet (`scripts/dev-env/prometheus/volume-usage.sh`) therefore publishes the same series and labels on dev, and the production query and behavior are unchanged.
- **t004 walk:** workspace `tea-dbo1091jg4r0tg69b330`; `background_worker` `srv-dbo10b1jg4r0tg69b34g` with a 1 GB disk `dsk-dbo10b1jg4r0tg69b35g` at `/data`, set to `notificationsToSend: all`.
  - **Over 80%:** filled to 85% at 06:03:08Z. Event `evt-9l7v2vq70q6c74kbjtf8` `service_disk_usage_high` fired at 06:09:56Z, with `{diskId, diskName walk-data, mountPath /data, usagePercent 85, thresholdPercent 80}`. Mailpit received "Disk usage high: m255-disk-walk" at 06:10:02Z.
  - **No repeats:** six 78%/85% oscillations (06:10:58–06:12:39Z), then a one-sweep dip, produced no extra event or email.
  - **Back below:** dropped to 50% at 06:20:38Z. Event `evt-e5bu4uiofk7rrc2e60ve` `service_disk_usage_recovered` fired at 06:29:56Z, and "Disk usage back to normal" arrived at 06:30:02Z.
  - **Surfaces:** REST `GET /v1/services/{id}/events` and `GET /v1/events/{id}`, GraphQL `serviceEvent`/`serviceEvents`, and MCP `list_events`/`get_service_event` all show both events with Render's exact `details`.
  - **Cleanup:** the service and disk were deleted (PVC and PV gone).
- **Acceptance (D1=A, 2026-11-07):** dev-2 plus tests. Every DoD point holds; no finding contradicts it.
