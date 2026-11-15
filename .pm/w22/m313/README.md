# w22 · m313 — Sandbox storage and memory meters: a suspended sandbox pays snapshot storage, as on Render

**Worker:** worker22 **Goal:** a suspended sandbox is billed for its retained image and nothing else, and memory is metered on its own, as the first half of Render's sandbox billing model **Status:** todo — unblocked 2026-11-14 (t001–t005 and t007–t010 done and shipped; t006's production check and t011's closeout wait for a real suspended sandbox on production (deployed and checked against the registry 2026-11-11, run 42982188589); the live Stripe Price exists since 2026-11-10 and its `BEX_STRIPE_SANDBOX_STORAGE_METER` hold is gone)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Design: where snapshot bytes come from, and the storage meter's event contract — **DONE** | 1h | — |
| t002 | Memory meter: record sandbox_memory_gb_seconds beside the existing meter, not yet billed — **DONE** | 1h | t001 |
| t003 | Storage meter: store, suspend-path hooks, and a registration API for snapshots — **DONE** | 1h30m | t001 |
| t004 | Storage bounds and garbage collection (ADR104 D11) — **DONE** | 1h30m | t003 |
| t005 | Rates, usage surfaces and Stripe line for storage; memory shown but unbilled — **DONE** | 1h30m | t002, t003 |
| t006 | Production check: suspend, accrue, resume, compare with the registry | 1h | t005 |
| t007 | Amend ADR023, ADR030 and the ledgers — **DONE** | 30m | t006 |
| t008 | Render parity — **DONE** | 30m | t007 |
| t009 | Simplify — **DONE** | 20m | t008 |
| t010 | Test coverage — **DONE** | 40m | t008 |
| t011 | Closeout | 10m | t009, t010 |

## Definition of done

- A suspended sandbox accrues `sandbox_snapshot_gb_seconds` from commit success until its image is deleted, idempotently; the metered bytes match the registry within the documented tolerance on production
- Snapshot storage is priced at Render × 0.70 and billed; the memory kind is recorded and visible but not billed; `sandbox_compute_seconds` and the agent-session cost are unchanged
- Per-workspace storage cap, image garbage collection and a registry-growth alert exist
- ADR023, ADR030 and ADR104 D8 are amended; w32/m285 has a documented API to register snapshot objects

## Source + Goal linkage

- **Source:** [docs/ADR104-sandbox-render-parity.md](../../../docs/ADR104-sandbox-render-parity.md), whose gap table is the proposal (no separate `/pm-brainstorm` run), filed by `/pmx adr104 to w22` on 2026-11-10; gaps 8 (storage and memory meters) and 14 (storage part)
- **Goal linkage:** Render parity first (ADR008): Render's suspended sandboxes pay snapshot storage only; usage metering and billing (ADR023, ADR030, ADR040); pillar 5 sandboxes
- **Expected outcome:** A suspended sandbox shows a storage charge and no compute charge; the usage surfaces list the new kinds; registry growth from sandboxes is capped and alerted
- **Why now:** Today a suspended sandbox is free and bex absorbs its retained image for up to 30 days, and w32/m285 would add 30-day objects of up to 40 GB at no cost to the creator (ADR104 D8). It has no technical dependency, so it ships first.
- **Render parity:** included, usage surfaces change.
- **Recommendation, not a user decision:** ADR104 D8 recommends adopting Render's model; the parity rule makes it the default, so these tasks implement it. The user's sign-off is only needed if w22/m314's CPU spike fails.
- **Rates:** Render × 0.70 per the `pricing.yaml` convention; Render's $0.08 per GB-month comes from render.com/articles/render-sandboxes-vs-vercel-sandboxes (2026-11-06), not from Render's pricing page. Re-read it before shipping.
- **Shadow memory meter:** recorded here, billed by w22/m314, because the existing weighted charge cannot be split.

## 解阻记录 2026-11-14（/unblock w22）

- **来源：** 用户决定（D1，2026-11-14）加复核证据。
- **决定：** 用户授权 w22 的 worker 在生产上做短时取证：加探针并手动 `gh workflow run` 派发 `isolation-matrix.yml`（环境 `production-observe`），只用一次性、不计费的 workspace（ADR046 billing-exclude，与周跑相同），读 Zot 大小、Prometheus、kubelet 统计。m314 可以对一个 sandbox 加几分钟 CPU 负载，m313 可以挂起一个 sandbox 等至少一小时的汇总。授权只到这里：更长的负载、多个 sandbox、写生产业务数据都要再问用户。
- **已核实（访问不是计划，是已有的）：** (1) 该工作流的 `Get app cluster kubeconfig` 步骤用 `scripts/fetch-app-kubeconfig.sh` 取 `admin.conf`，所以 Zot 的 Secret、Prometheus、kubelet 统计都读得到；(2) `gh` 已登录，`workflow_dispatch` 已有成功运行（43577969524、43579027377、43580573152，2026-11-14）；(3) `scripts/qa-login.sh` 2026-11-14 实测通过（exit 0，已 `--logout`），w32/m284 记的"QA 登录坏了"已过时。
- **约束：** 与周跑共用 `concurrency: isolation-matrix`，不要和每周二 03:41 UTC 的 cron 撞；不给 `production-*` 环境加人工闸门（DO_NOT_DO #ENV-REVIEWERS）；证据只记摘要和 run id，原始日志不入库（#NO-ARTIFACTS-IN-GIT）；探针结束时必须清理一次性 workspace 和 sandbox。如果发现某项读不到，把缺口写成精确的新闸门再停靠，不要猜。

- **本项的状态：** t001–t005、t007–t010 已完成并发布；Stripe 的 Price 与 `BEX_STRIPE_SANDBOX_STORAGE_METER` 早已处理。t006 依赖 m321 t005 里 Zot 凭据修复已部署（已部署，见 m321）。
- **worker 的下一步（t006）：** 建一个一次性 sandbox、写入已知数据、挂起并记下 Zot 的 repository 大小；整点汇总有延迟，可以分两次派发（第一次建、挂起、记录，第二次在汇总之后读 `GET /v1/usage` 和发票预览，与 Zot 大小比较，再恢复、终止，确认 accrual 停止、镜像被删）。再做 t011 收尾。
- **验收：** t006 的三条验收（字节数在容差内、终止后 accrual 停止、挂起时用量只有存储没有 compute）都有生产证据。
