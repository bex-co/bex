# w2 · m252 — Failure alerts reach email, as on Render

**Worker:** worker2 **Goal:** every failure event Render emails about — a running service becoming unhealthy, a cron run failing, an image pull failing, a Blueprint sync failing — bex emails too, under the existing `notificationsToSend` and member settings. **Status:** done (D1=A, 2026-11-07)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Correct ADR018's Notifications row with a per-event matrix — **DONE** | 20m | — |
| t002 | Email for server_failed with a flap digest (and server_available per Render's level) — **DONE** | 1.5h | t001 |
| t003 | Email for cron_failed and image_pull_failed — **DONE** | 45m | t001 |
| t004 | Email for Blueprint sync failure — **DONE** | 45m | t001 |
| t005 | Apply notificationsToSend and member settings per event on every surface — **DONE** | 1h | t002, t003, t004 |
| t006 | dev-2 walk and production check — **DONE** | 45m | t005 |
| t007 | Render parity — **DONE** | 15m | t006 |
| t008 | Simplify — **DONE** | 15m | t007 |
| t009 | Test coverage — **DONE** | 30m | t007 |
| t010 | Closeout — **DONE** | 10m | t008, t009 |

## Definition of done

- On production, a workspace member with **no push device** receives an email for: a crashing (unhealthy) service, a failed cron run, a failed image pull, and a failed Blueprint sync.
- A service flapping failed/available 20 times in an hour produces a digest, not 20 emails; no failure is silently dropped (the digest lists them).
- With `notificationsToSend: none` on the service, none of these emails is sent; with `failure` they are; recovery (`server_available`) email follows Render's documented level.
- `docs/ADR018-render-parity.md` Notifications row carries a per-event matrix against render.com/docs/notifications and is `✅` only where true.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-11-02 #2. render.com/docs/notifications (fetched 2026-11-02) emails on: build/deploy failure, Docker image pull failure, cron job failure, a running service becoming unhealthy, persistent disk usage >80% (and back below), Blueprint sync failure, and suspension after repeated startup failures. bex email today covers deploy lifecycle, billing, free hours and database expiry only (`lego/backend/internal/notifications/service.go`); `server_failed`/`cron_failed` reach push only. Absorbs FUTURE-MAYBE "Availability-alert delivery + CPU-throttle visibility" part (a) (w2/028; eden-cms-v2 logged 1,399 failed/available pairs).
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first); V0 goal 2 (basic obs for operation).
- **Expected outcome:** tenants learn about outages without installing the mobile app or enabling web push.
- **Why now:** ADR018 currently claims `✅` for notifications while email is deploy-only; the old deferral trigger predates the parity-first rule (2026-10-17/20), under which an unsigned divergence does not stand. w2/m253 and w2/m255 reuse this email path.
- **Render parity included:** notification settings and email behavior are tenant-facing on REST, GraphQL, MCP and the dashboard.

## Work log — 2026-11-03

- **Design:** `AlertEmailWorker` (`internal/notifications/alert_email_worker.go`) tails the composed event feed (its own `alert_email_watermarks` row, `store.TailFeed`) and failed Blueprint syncs (`blueprint_syncs.state = 'error'`, second watermark), projecting `server_failed`/`server_hardware_failure`, failed `cron_job_run_ended`, `image_pull_failed` and failed syncs into one `alert_email_streams` row per (workspace, member, resource, event). `alert_email_events` records each source event once per member, so a page re-read by a stale replica counts nothing twice. The first failure is due at once; failures within an hour after a mail are folded into one digest that counts them and lists the newest 20 times. Sends lease streams (`FOR UPDATE SKIP LOCKED`), retry transient SMTP/lookup failures with backoff (cap 8), and treat 550/551/553 and no-address as final. Both tables cascade from `tenant_members` (workspace and account-deletion censuses).
- **Policy (t005):** a service's `notificationsToSend` applies exactly as for a failed deploy (`failureNotificationPolicy`, now shared with `NotifyDeploy`); `default` follows each member's existing failure preference, so no new setting was needed. The Settings toggle is relabelled "Failures" / "故障" with a hint naming every event, and the MCP field description matches. Recovery is never emailed (Render: Slack only).
- **Found and fixed:** push "Cron job failed" was driven by `job_run_ended` (retired one-off jobs) and never fired for real cron runs; it now reads `cron_job_run_ended`.
- **t001:** ADR018 Notifications row is ◐ with the per-event matrix (disk 80% → w2/m255, crash-loop suspension → w2/m253).
- **Tests:** `TestAlertEmailWorkerMailsFailures` and `TestAlertEmailStreamLifecycle` (real Postgres), `alert_email_worker_test.go` (classification, policy matrix, copy, send outcomes), push vocabulary tests updated.

## Blocked (2026-11-03)

All implementation is done (t001–t005 and t007–t009 done; t006 (live walk) blocked). The remaining walk needs a gate this session cannot clear: user: (1) choose how the local CAPD cluster runs the egress nft rule (drop it locally, or rework it to bind in nested kind nodes) so dev-2 comes up for the crash / cron / image-pull / Blueprint-failure emails in Mailpit; and (2) approve a paid production canary (cron jobs are paid) plus an inbox to read the alert, for the production repeat. Run `/unblock w2` to decide. The milestone returns to the queue once the gate clears.

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
- **仍待决定（生产部分）：** DoD 要求在生产上验证，而这需要付费资源（付费 cron 的失败运行，以及一个 worker 能读到告警邮件的收件箱），用户这次没有批准（选了 A 而不是 C）。worker 完成 dev-2 部分后，如果仍缺生产条件，就重新 blocked 并把这个问题交回用户：是否接受 dev-2 证据加 envtest 作为验收，或者批准付费金丝雀。

## dev-2 walk — 2026-11-05 (`/loopx w2`)

Local CAPD mock (reprovisioned with the D1 rule), `bex-operator:dev` from main, dev-2 bex-api, dev-only identity `m252-walk@bex.test`. Email evidence from Mailpit; facts from bex-db.

| DoD point | Result |
| --- | --- |
| Unhealthy service emails | **Proven**: `server_failed` 12:09:50Z → "Service unavailable: m252-flapper" 12:10:12Z |
| Failed image pull emails | **Proven**: `image_pull_failed` 12:19:04Z → "Image pull failed: m252-pull2" 12:20:02Z |
| Failed Blueprint sync emails | **Proven**: sync `bsr-dbm76p9jg4r9e0s3ojm0` error 12:15:33Z → "Blueprint sync failed: m252-bp" 12:25:37Z, with the error text and an `/blueprints/exs-…` link. The ~10 min gap is the designed `alertEmailBlueprintLag`. |
| Failed cron run emails | Not walked on dev-2: cron deploys failed on a missing `bex-tenant-operator` ClusterRole. Fixed in `dev-env.sh up` (`33ad3ad5f`). |
| 20× flap → one digest, nothing dropped | **Proven with real pods**: 20 `server_failed` + 20 `server_available` facts from 12:09:50Z to 12:38:34Z produced two emails. The first came at 12:10:12Z; then "Service unavailable: m252-flapper (19 times)" at 13:10:12Z listed all 19 later timestamps. |
| `none` suppresses / `failure` sends | **Failed on that build**: the policy was looked up by CR name. Already fixed on main (`c2f9a3c38`, `27099feda`). `8a7ccafd9` adds migration 0195, which carries open digests across the rekey, plus realistic-id tests. Re-check on production. |
| Recovery not emailed | **Proven**: more than 20 `server_available` facts, zero recovery emails. |

A missing tag (`busybox:no-such-tag`) never reaches `image_pull_failed`. The operator's image-metadata pre-check fails the deploy first, so the member gets "Deploy failed".

## Blocked (2026-11-05)

Every implementable task is done and dev-2 evidence is above. t006's production half needs paid resources (a paid cron's failing run) and a member inbox the worker can read, which D1=A did not approve. **User decision:** accept dev-2 + unit/envtest evidence as acceptance (then t006 and t010 close), or approve a paid production canary. Run `/unblock w2`.

## 解阻决定（2026-11-07，来源：用户决定 D1=A）

- **决定：** 接受 dev-2 走查（2026-11-05，证据见上文）加单元测试和 envtest 作为验收，不做付费生产金丝雀。
- **对标依据：** Render 在这些场景都会发送邮件，代码已经对齐。这个决定只关系到验收证据，客户行为不变。
- **结果：** 剩余任务按此关闭，里程碑移入 `done/`。
