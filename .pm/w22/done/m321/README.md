# w22 · m321 — The storage meter's registry credentials: the operator mints a read user and a delete user for bex-api

**Worker:** worker22 **Goal:** bex-api can read a suspended sandbox's image size from Zot and delete a terminated sandbox's `snap-gen` tags, through two operator-managed Zot users scoped to `snapshots/**`, with no admin credential in bex-api **Status:** done (2026-11-15)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Design: where the two Zot credentials live, and how the operator may write them — **DONE** | 45m | — |
| t002 | Operator: mint the two Zot users, their ACL entries and the Secret — **DONE** | 1h30m | w22/m321/t001 |
| t003 | Hook it into the operator: reconcile, RBAC and manifests — **DONE** | 1h | w22/m321/t002 |
| t004 | bex-api: read the Secret into the storage meter's environment — **DONE** | 45m | w22/m321/t003 |
| t005 | Production check: a suspended sandbox is sized, a terminated one's images are deleted — **DONE** | 1h | w22/m321/t004 |
| t006 | Simplify — **DONE** | 20m | w22/m321/t005 |
| t007 | Test coverage — **DONE** | 40m | w22/m321/t005 |
| t008 | Closeout — **DONE** | 10m | w22/m321/t006, w22/m321/t007 |

## Definition of done

- Two Zot users exist, minted by the operator: `bex-sandbox-meter` with `read` and `bex-sandbox-reaper` with `read` and `delete`, both on `snapshots/**` and nothing else; the Secret bex-api reads them from is created and self-heals
- bex-api starts with and without the Secret; with it, retained images are sized and a terminated sandbox's tags are deleted
- A production check shows one image sized and one terminated sandbox's tags deleted, and a repository the controller did not write to untouched

## Source + Goal linkage

- **Source:** w22/006 (filed by w22/m313 t004 on 2026-11-10), promoted 2026-11-10 by `/loopx w22`; [docs/ADR023-usage-metering.md](../../../../docs/ADR023-usage-metering.md) § Sandbox snapshot storage, [docs/ADR104-sandbox-render-parity.md](../../../../docs/ADR104-sandbox-render-parity.md) D11
- **Goal linkage:** Render parity first (ADR008): a suspended sandbox pays snapshot storage as on Render, which needs the meter to read sizes; D11's bound on registry growth needs the reaper
- **Expected outcome:** the storage meter and the reaper work on production without anyone hand-setting a credential, and the registry stops keeping a terminated sandbox's images forever
- **Why now:** w22/m313 shipped the meter and the reaper with no credential to run them: sizing is off, so no storage is billed and `SandboxStorageUnsized` fires once a sandbox is suspended
- **Render parity:** omitted. No REST, GraphQL, MCP or UI surface changes.
- **Open decision, mine to make in t001, not the user's:** where the Secret lives (the operator caches Secrets only in the apps namespace, and bex-api is in `bex-system`)
- **Safety:** the reaper is the first registry writer bex-api has had; it deletes only the controller's `snap-gen<N>` tags and its credential covers `snapshots/**` only

## 解阻记录 2026-11-14（/unblock w22）

- **来源：** 用户决定（D1，2026-11-14）加复核证据。
- **决定：** 用户授权 w22 的 worker 在生产上做短时取证：加探针并手动 `gh workflow run` 派发 `isolation-matrix.yml`（环境 `production-observe`），只用一次性、不计费的 workspace（ADR046 billing-exclude，与周跑相同），读 Zot 大小、Prometheus、kubelet 统计。m314 可以对一个 sandbox 加几分钟 CPU 负载，m313 可以挂起一个 sandbox 等至少一小时的汇总。授权只到这里：更长的负载、多个 sandbox、写生产业务数据都要再问用户。
- **已核实（访问不是计划，是已有的）：** (1) 该工作流的 `Get app cluster kubeconfig` 步骤用 `scripts/fetch-app-kubeconfig.sh` 取 `admin.conf`，所以 Zot 的 Secret、Prometheus、kubelet 统计都读得到；(2) `gh` 已登录，`workflow_dispatch` 已有成功运行（43577969524、43579027377、43580573152，2026-11-14）；(3) `scripts/qa-login.sh` 2026-11-14 实测通过（exit 0，已 `--logout`），w32/m284 记的"QA 登录坏了"已过时。
- **约束：** 与周跑共用 `concurrency: isolation-matrix`，不要和每周二 03:41 UTC 的 cron 撞；不给 `production-*` 环境加人工闸门（DO_NOT_DO #ENV-REVIEWERS）；证据只记摘要和 run id，原始日志不入库（#NO-ARTIFACTS-IN-GIT）；探针结束时必须清理一次性 workspace 和 sandbox。如果发现某项读不到，把缺口写成精确的新闸门再停靠，不要猜。

- **本项的状态：** t001–t004、t006、t007 已完成并发布。Zot 用户覆盖各 workspace 自己的 `snapshots/<ws>-sandbox/**` 的修复是 `69b0b4c0b`（2026-11-11），之后的 `deploy (bex via Argo)` 已多次成功，修复已部署。
- **worker 的下一步（t005）：** 在探针里先确认两个 Zot 用户对每个 workspace 的 pattern 都有权限；挂起一个一次性 sandbox，确认 `bex_sandbox_storage_objects{state="unsized_open"}` 回到 0、字节数与 Zot manifest 一致；终止后确认 `reap_pending` 清零、该 sandbox 的 `snap-gen` tags 被删、控制器没写过的 repository 不受影响；确认 6 条 QA reap 行已清零。摘要写进 t005，再做 t008 收尾。
- **验收：** DoD 第三条有生产证据：一个镜像被计量、一个终止 sandbox 的 tags 被删、其他 repository 不动。
- **仅剩的机械卡点：** `scripts/pm-board-validate.py` 会把本目录从 `blocked/` 移回 `w22/m321` 判成"新增了 w4 已用的编号 m321"（它文档里写同一 workstream 内移动不算重复，实现却按目录比较）。已记为 w22/015；它发布后，`/pm unblock w22/m321` 把本目录移回 `w22/m321`，worker 做 t005 和 t008。
- **机械卡点已清（2026-11-14）：** w22/015 已发布（`84c61a59c`），`/pm unblock` 把本目录移回 `w22/m321`；`pm-board-validate` 对这次移动通过。worker 做 t005 和 t008。

## Outcome (2026-11-15)

- **Shipped and verified on production:** two operator-minted Zot users, the Secret bex-api reads them from, sizing and reaping.
- **Production evidence (2026-11-15, [sandbox-measure run 43693931198](https://github.com/bex-co/bex/actions/runs/43693931198); the first half also in run 43691298476).** One disposable, billing-excluded workspace, two sandboxes writing about 100 MB of incompressible data and paused:
  1. **Sized, exactly.** The registry's newest image (`snap-gen3`, config plus layers as bex-api sizes it) was 111,698,860 and 110,654,096 bytes for the two runs' first sandboxes (and 111,698,869 and 111,698,868 for the others); the meter's `sandbox_storage_objects.bytes` was identical in every case, and `bex_sandbox_storage_objects{state="unsized_open"}` read 0.
  2. **Accrued, only storage.** After the meter's 15-minute interval each suspended sandbox carried `sandbox_snapshot_gb_seconds` 99 to 102 (about 0.11 GB × 900 s), `GET /v1/usage` showed it under `resourceKind: sandbox` next to the unchanged `sandbox_compute_seconds` and `sandbox_memory_gb_seconds`; no other usage kind grew while suspended.
  3. **Stops on resume.** After a resume the storage total was 102 at 90 s and 102 at 240 s. A terminate while suspended ended the object (`reap_pending` set, then cleared after the delete); the usage total after that end was not re-read on production, and `TestSandboxStorageMeterPG` covers the closing arithmetic.
  4. **Reaped, and only that.** After terminating the resumed sandbox its `snap-gen*` tags were deleted while the other sandbox's repository kept its tag; after terminating that one while suspended its tags were deleted too; `reap_pending` was 0 for the workspace and fleet-wide (0 before and after); the repository this run did not write (`bex-cnb-builder`, five tags) was byte-for-byte unchanged.
- **A leak the run found, filed as w22/017:** eight namespaces `snapshots/<ws>-sandbox/…` hold images although their workspace namespace no longer exists (earlier disposable workspaces and older ones): deleting a workspace with a suspended sandbox does not delete its image.
