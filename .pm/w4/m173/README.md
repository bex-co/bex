# w4 · m173 — `instance_seconds` meters how many container series appeared in an hour, not how long instances ran

**Worker:** worker4 **Goal:** billed `instance_seconds` equals real instance running time: one instance per pod (not per container), prorated to the seconds it actually ran in the window, with replacement pods during a rollout counted only for their own lifetimes. **Status:** todo（2026-10-31 解除阻塞；已完成任务保持 DONE，详见文末“解除阻塞记录”）

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Prorated, per-pod instance-seconds query for Apps and datastores — **DONE** | 1h | — |
| t002 | Decide and apply the correction for already-metered hours | 45m | w4/m173/t001 |
| t003 | Render parity across usage surfaces — **DONE** | 30m | w4/m173/t002 |
| t004 | Simplify changed code — **DONE** | 20m | w4/m173/t003 |
| t005 | Test coverage for shipped behavior — **DONE** | 45m | w4/m173/t003 |
| t007 — **DONE** | Meter resources deleted before their hour is rolled up | 1h | w4/m173/t001 |
| t006 | Closeout | 15m | w4/m173/t005, w4/m173/t007 |

## Definition of done

Live replay on owned Free fixtures (deleted afterwards). Read `GET /v1/usage?ownerId=tea-daif693dqjvc73e7as3g` once the hour has rolled up:

- **Short life:** a single-container web service alive ~5 min within one hour reads ≈300 `instance_seconds` (±1 scrape interval), not 3600 or 0. A Free Postgres alive ~4 min reads `instance_seconds` consistent with its own `storage_gb_seconds` lifetime (today, for example, `qa-20261005-m137-pg`: `storage_gb_seconds` 236 vs `instance_seconds` **3600**; `qa-20261003-c11738-pg` 239 vs 3600; `qa-20261004-loop9-pg` 248 vs 3600).
- **Sidecars:** a Key Value (Valkey plus the redis_exporter sidecar) alive ~8 min reads ≈480, not one hour per container (today `qa-20261004-l10-kv`, alive about 02:00–02:06 on 2026-10-05, reads **7200**).
- **Full hour:** a single-replica service running the whole hour reads 3600 (control). A 2-replica service reads 7200.
- **Rollout:** one deploy within an hour adds only the overlap seconds the old and new pods actually co-ran, not an extra 3600.
- **No silent zeros:** a short-lived fixture that ran produces a positive row. Today several 2–10 minute fixtures (`qa-20261004-l8-pg`, `-l15-pg`, and the loop 13–29 image services) show 0 or no row.
- REST/GraphQL/MCP usage and the dashboard Billing "Charges" agree with the corrected rows.

## Source + Goal linkage

- **Source:** infinite `/qa-find-bugs` loop30 2026-10-05 UTC (muse.env, `bex-canary`), read-only on the Billing page and `GET /v1/usage`. The rows quoted in the DoD come straight from that response, including `qa-20261004-eb0114-stall` 3600, `qa-20261004-m172a-echo` 50400 and `qa-20261004-m172b-echo` 36000 for short-lived services.
- **Root cause:** `lego/backend/internal/usage/service.go:1113-1131` `queryInstanceSecondsByMatcher` evaluates `count(avg_over_time(container_memory_working_set_bytes{<matchers>}[<window>s]))` and multiplies by the window.
  - `count()` returns how many **container series** had any sample in the hour. The comment's "average number of running pods" is not what the expression computes.
  - Any pod present for one scrape is billed the full window, and each container in a pod (`container!=""`) is billed separately.
  - Replacement pods during a deploy or restart each add a full window.
  - Callers: Apps at `service.go:966` (`queryInstanceSeconds`) and datastores at `service.go:683` (`queryInstanceSecondsStateful`). `queryStorageGBSeconds` (`:1088-1110`) already prorates correctly with `avg_over_time`, which is why storage and instance disagree for the same database.
  - The zero/absent rows for some short-lived fixtures are a second symptom of the same query shape. Their exact cause is unverified (likely staleness of terminated series at the evaluation instant).
- **Governing docs:** ADR023 (usage metering: "pod count × window seconds") and ADR040 (Stripe meter export of `instance_seconds.<kind>.<tier>`).
- **Goal linkage:** ADR030/ADR040 pricing and billing correctness. Paid tiers are charged per instance-second from these rows.
- **Expected outcome:** instance billing matches running time. No full-hour charge for minutes of life, no multiplication by sidecars or rollouts, and no free time for short-lived paid resources.
- **Why now:** every paid instance that restarts, deploys, has a sidecar, or lives under an hour is mis-metered today, in both directions, and the rows feed Stripe meter events. Render parity task INCLUDED (t003): usage surfaces' numbers change.
- **Unverified:** paid-tier dollar impact (only Free fixtures probed, $0); the cause of the zero rows; whether `usage_monthly` compaction already sealed affected months.
- **Severity:** major.
- **Live replay 2026-10-05 (loop39, pin `f837cec46294`):** proration (t001) holds on the short-life bullet. `qa-20261005-l39-pro` (`srv-db1n2futdtps73dl0c9g`, Free, `traefik/whoami`) went Live at 09:43:40Z and was kept awake by a 2-minute pinger. After the ~10:55Z rollup tick, `GET /v1/usage?ownerId=tea-daif693dqjvc73e7as3g` read `instance_seconds` **990** for it (expected ≈980 for 09:43:30–10:00, against 3600 before t001). It was deleted at 10:59Z, before its 10:00 hour rolled up at ~11:55Z. Under t007 that hour (~3540 s) is expected to stay unmetered, which is a third replay of that gap. The sidecar, rollout and full-hour control bullets are still unprobed.
- **Related, 2026-10-05 code review:** t001's query still meters a pod that ends inside the window for ~285 s after its last sample. The subquery's instant selector looks back 5 minutes, and cAdvisor series get no staleness markers (w4/m110). In a promtool replay, a 5-minute pod read 585 s.
  - The DoD's "Rollout" bullet will fail until [w5/m111](../../w5/m111/README.md) lands.
  - t007's deleted-resource metering would add the same tail to every deleted resource.
  - t002's paid-exposure read should include this overcount.
  - w5/m111 owns only the query; enumeration, rollup timing and historical repair stay here.

## 解除阻塞记录（2026-10-31，`/unblock w4`）

- **涉及：** w4/m173
- 来源：现有先例 + 用户决定（D1/D3 的 bex-canary 授权）。
- **结论：t002 的只读量化交给 worker；是否修复历史数据仍待用户决定。** t006 的线上重放可在 bex-canary 用付费 fixture（含 2-replica 服务、Key Value）完成。 **生产只读查询依据（现有先例）：** w2/m96 已在 `hetzner-prod` 的 `bex-db` 上以 `BEGIN READ ONLY` 做过只读核查；worker 按同样方式只读查询，不打印凭据，只记录聚合结果与必要的 id。 bex-canary（`tea-daif693dqjvc73e7as3g`）计费状态为 `operator_excluded: structural exclusion`，不走 Stripe 收款（2026-10-29 只读核实 `workspaceBillingReadiness`），付费 fixture 与付费套餐不产生真实费用。
- 生产已部署 `25b99d8a373a`（`deploy/gitops/base/bex.yaml`，2026-10-30 pin），本项实现均早于此版本；部署不等于验收，以下仍是待验证工作。
- **worker 下一步：** (1) t002 第一步：只读量化付费档 `instance_seconds` 的多计量，以及已导出的 Stripe meter events（含 m173 记录的约 285s 尾巴）。若付费档影响为 0 → 记录“无需修复”，继续；若非 0 → 带金额和受影响 workspace 列表把 m173 停回 blocked，由用户决定是否重算并用 ADR040 `mark_repaired`。(2) t006 按 DoD 在 bex-canary 重放，等整点 rollup 后读 `GET /v1/usage`；验完清理。
- **清理（用户要求，验收的一部分）：** 验收结束删除本次在 bex-canary 创建的全部资源；若为此把 bex-canary 切到付费套餐，所有依赖付费档的项做完后切回 Hobby；在本条记录清理结果（删除后 404 / 列表为空、套餐读回 Hobby）。
- **剩余阻塞：** 需要用户决定的部分只在只读结果非零时出现（见上文“worker 下一步”）；在此之前可以开工。
- **验收：** 原 DoD / Acceptance 不变，未删减。
