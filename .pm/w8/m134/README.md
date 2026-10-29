# w8 · m134 — New private Key Value stores never become available (regression from w8/181)

**Worker:** worker8 **Goal:** A Key Value store created without an IP allowlist (the default on every surface) is `available` within about a minute, as before w8/181. Its first external-access enable still restarts nothing. **Status:** todo（2026-10-29 解除阻塞；原记录：t004, t005, t006 done 2026-10-23; t001 (the blocker hotfix) shipped in db4f548e1 and needs the release pipeline to deploy the operator; then QA runs t002/t007 live: a fresh `bex kv create --plan free` is available within 90 s, inventory and roll any store created since 2026-10-23 02:28 UTC still `creating`, a store created with an allowlist is reachable over TLS. t003 (restart-free first enable without a birth listener) is NOT done and needs your sign-off on the design: Valkey 7.2.14 and 8.1.9 both accept `CONFIG SET tls-cert-file/tls-key-file/tls-ca-cert-file` then `tls-port` at runtime and `tls-port 0` turns it off (verified in Docker 2026-10-23), but using it means an optional Secret mount plus either a sh wrapper that adds `--tls-port` when the cert files exist or an operator re-apply after every pod restart, inside the live-handoff engine that guards customer data

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 — **DONE** | Hotfix: a new store starts with the TLS listener only when its Certificate can exist | 30m | — |
| t002 | Unstick stores created since the regression | 20m | w8/m134/t001 |
| t003 | Restart-free first enable without a birth listener (w8/181's goal) | 60m | w8/m134/t001 |
| t004 — **DONE** | Render parity: ADR021/ADR018 and create/enable timings on every surface | 15m | w8/m134/t003 |
| t005 — **DONE** | Simplify the changed code | 15m | w8/m134/t004 |
| t006 — **DONE** | Test coverage: private create becomes Ready; public create; first enable | 40m | w8/m134/t004 |
| t007 | Close out the milestone | 30m | w8/m134/t005, w8/m134/t006 |

## Definition of done

Live on production after the release pipeline deploys the operator, with owned Free fixtures deleted by ID with 404 proofs (one free store per workspace, so serially):

- `bex kv create --name <n> --plan free --confirm -o json` → `available` within 90 s. `bex kv-cli <id> -- PING` works from inside the workspace (or via an allowlist enabled afterwards).
- On that store (persistence `off` and default), the first `bex kv update <id> --ip-allow-list "cidr=<ip>/32,description=qa" --confirm -o json` succeeds without `KEY_VALUE_RESTART_CONFIRMATION_REQUIRED`. `INFO server` `uptime_in_seconds` keeps increasing across the change, and a key written before it is still there (w8/181's DoD, rechecked).
- A store created with `--ip-allow-list` is `available` and reachable over TLS (control).
- No store created after the regression remains in `creating` (t002 inventory).

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` loop sweep 8b3c, 2026-10-23 UTC: bex v0.3.3, pin Render CLI v2.27.0 `a764810a7682`, `https://api.bex.co/v1/`, `bex-canary` (`tea-daif693dqjvc73e7as3g`), human device login. Regression from **w8/181** ([done/181.md](../done/181.md), commit `db739b089` "fix(operator): start new Key Value stores with the TLS listener"), live since the platform pin `eaaccb6f6fd4` (2026-10-23 02:28 UTC; `fa8ac5bba24a` since 03:39 UTC). The same create in sweep 8004 (pin `b3cd5bc74de8`, before the regression) was `available` within ~10 s.
- **Goal linkage:** managed Key Value ([ADR021](../../../docs/ADR021-keyvalue-management.md)); Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)): a new Render Key Value is private by default and is available in minutes.
- **Expected outcome:** creating a Key Value works again on every surface (CLI, REST, GraphQL, MCP, dashboard, Blueprints), and w8/181's restart-free first enable is kept.
- **Why now:** **blocker.** Every new store created without an allowlist (the CLI, API and dashboard default) stays `creating` indefinitely, with no logs and no reason shown. On Free the stuck store also holds the workspace's one free slot. Render parity task included: create/status behavior is tenant-facing on every surface.

## Evidence (sweep 8b3c)

Store A `red-dbdeltvsf6rc738q75qg` (`qa-20261022-8b3c-kv`): `bex kv create --name qa-20261022-8b3c-kv --plan free --confirm -o json` at 05:04:55 UTC. It was still `creating` at 05:07:38, when `bex kv update … --ip-allow-list "cidr=<ip>/32,description=qa8b3c"` was sent. It became `available` at 05:09:31 (`INFO server` `uptime_in_seconds:37` shortly after). Deleted, 404.

Store B `red-dbder988dkas73a0v4ig` (`qa-20261022-8b3c-kv2`), the control with nothing touched. Created 05:16:21 UTC (the first retry got `400 WORKSPACE_RESOURCE_LIMIT` while A terminated).

```text
05:16:21 → 05:34:59  status "creating" (polled every 5–15 s; 18 m 38 s)
                     bex logs -r red-dbder988dkas73a0v4ig → no lines at all (the process never started)
                     dashboard /keyvalue/red-dbder988dkas73a0v4ig: "Creating", no reason
05:34:59             bex kv update red-dbder988dkas73a0v4ig --ip-allow-list "cidr=<ip>/32,description=qa8b3c" → 200, status "creating"
05:37:18             status "available"   (2 m 19 s after the store became public)
```

Control in the same sweep: on store A, a later allowlist edit (adding `203.0.113.0/24`) answered `available` with no restart. `uptime_in_seconds` went 37 → 65 across it (w4/m233 holds).

## Root cause

- `lego/operator/internal/controller/keyvalue_controller.go:1024-1025`: the workload intent's `tlsListener` is true for a brand-new store (`keyValueIsNew`, `:430-437`, added by `db739b089`). The pod template then mounts the TLS Secret (`:916-927`, `--tls-port`/`--tls-cert-file /tls/tls.crt`, a non-optional `SecretVolumeSource`), so the pod cannot start until that Secret exists.
- `reconcileKeyValueTLS` (`:447-475`) creates the cert-manager Certificate only when the store is **public**. For `listener && !public` it returns early ("A private store's kept listener stays on the Certificate as issued"), an assumption true for a store that was public once but false for a newborn private store. No Certificate means no Secret, so the pod stays `ContainerCreating` and the status stays `creating`, with no failure reason.
- Enabling an allowlist makes the store public, the Certificate is created and issued (~2 min), and the pod starts. That is why store A came up and B only did after the enable.
- `keyvalue_test.go` (the `db739b089` tests) asserts the template has the listener, but not that a private newborn store's pod can start.

## Fix (target)

- t001 (hotfix, ship first): `keyValueIsNew` alone must not add the listener. Start with it only when the store is created public (its Certificate is created in the same reconcile). A private newborn starts without the listener, as before `db739b089`. This reopens w8/181's first-enable restart, which t003 removes properly.
- t003: make the first enable restart-free without a birth listener. Recommended: mount the TLS Secret with `optional: true` from birth (no `--tls-port`). On the first enable, wait for the Secret files, then turn the listener on at runtime (`CONFIG SET tls-cert-file/tls-key-file/tls-port` through the platform ACL user, which already runs live persistence handoffs, ADR021:187). This also avoids issuing a public certificate for every private store, which would add ACME latency to every create and spend the per-registered-domain issuance quota on stores never exposed. Verify on the pinned Valkey 7/8 images that `tls-port` is runtime-settable; if not, fall back to issuing per-store certificates at birth and record the quota analysis.
- t002: list stores created since 2026-10-23 02:28 UTC still in `creating`, and after t001 roll them (they have no data). Report the count.

## Blast radius

Every Key Value created without an allowlist since the `eaaccb6f6fd4` pin, every plan, every workspace, on every surface (CLI/REST/GraphQL/MCP/dashboard/Blueprint, all through the operator). Postgres is not affected (separate controller). Existing stores are not rolled by `db739b089` and keep working.

## Unverified

- Whether any customer store is currently stuck (t002 counts).
- Valkey runtime `tls-port` support on both pinned majors (t003 checks).

## Dedupe

Regression of w8/181 (done 2026-10-22). Its DoD covered the first enable on a store created after the change but not that the store becomes available while private. Searched open `.pm` items across workstreams and the last 6 h of `origin/main` for Key Value / TLS listener / stuck `creating`: no other filing.

## Partial live evidence (2026-10-23 11:13–11:18 UTC, platform pin `29d63e1f7bea`, bex v0.3.3, `bex-canary`, `/qa-find-bugs-cli` sweep d4e8)

`bex kv create --name qa-20261023-d4e8-kv --plan free --confirm -o json` (no allowlist) → `red-dbdk2kr17lic73al6nc0` created 11:13:23, `available` by 11:14:18 (< 55 s; before t001 it stayed `creating` 18 m 38 s). Not covered: first enable restart-free (t003, parked), t002 inventory. DoD (t007) still open.


## 解除阻塞记录（2026-10-29，`/unblock w8`）

- **涉及 ID：** w8/m134
- **结论来源：** 用户明确决定（D3）+ 本次复核证据。
- **复核结果：** t001 的修复 `db4f548e1` 已在生产 pin `b33dd43c4428` 中，原「等部署」卡点已消失；t002/t007 线上验收由 worker 完成。
- **结论（t003）：** 采用 sh 包装方案：可选挂载证书 Secret；启动脚本在证书文件存在时加 `--tls-port`；首次开启外部访问时用 `CONFIG SET tls-cert-file/tls-key-file/tls-ca-cert-file` 再 `CONFIG SET tls-port` 在线打开，免重启（关闭用 `tls-port 0`）。不采用「operator 每次重启后重新 apply」，也不维持「首次开启会重启」。
- **要求：** 改动在保护用户数据的 live-handoff 引擎内，必须覆盖：无持久化实例开启时数据不丢、pod 重启后 TLS 仍生效、关闭外部访问后端口确实关闭。
- **用户门槛：** 无。
- **worker 下一步：** loopx 实施 t003 并补测试，然后在生产完成 t002/t007 验收（fresh `bex kv create --plan free` 90 秒内可用；排查 2026-10-23 02:28 UTC 之后仍处于 `creating` 的 store 并处理；allowlist store 可经 TLS 连接）。fixture 按 ID 删除并留 404 证据。
