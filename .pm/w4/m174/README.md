# w4 · m174 — Fast-exiting App containers intermittently lose stderr lines from their logs

**Worker:** worker4 **Goal:** every line a tenant App container writes, on stdout or stderr, reaches `GET /v1/logs` (and GraphQL/MCP/dashboard Logs), including containers that exit within a second: every cron run and every fast crash. **Status:** todo（2026-10-31 解除阻塞；已完成任务保持 DONE，详见文末“解除阻塞记录”）

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Pin the mechanism: why the API tailer drops stderr from a fast exit — **DONE** | 30m | — |
| t002 | Ship tenant App container logs from the node's CRI files | 1h | w4/m174/t001 |
| t007 — **DONE** | Find the residual one-line loss after the CRI-file switch | 45m | w4/m174/t002 |
| t008 | Verify delayed stdout prefixes at startup and history intake | 35m | w4/m174/t007 |
| t003 | Render parity | 20m | w4/m174/t002, w4/m174/t008 |
| t004 | Simplify — **DONE** | 15m | w4/m174/t003 |
| t005 | Test coverage — **DONE** | 45m | w4/m174/t003 |
| t006 | Closeout | 10m | w4/m174/t005, w4/m174/t007, w4/m174/t008 |

## Definition of done

Eight tasks. [a321 delayed stdout follow-up](delayed-stdout-a321.md) records one missing prefix in two successful 90-second runs, matching REST/GraphQL/UI and a fresh passing control. t008 adds this delayed verification; it does not replace any original DoD item. Root cause and live producer identity remain unverified; status stays blocked.

Live, on owned Free fixtures in `bex-canary` (deleted afterwards):

- **Fast-exit cron, all lines, every run.** Create cron `alpine:3` with start command `sh -c 'echo ALPHA-x one; echo BETA-x two; echo "error: GAMMA-x three" 1>&2; echo "{\"level\":\"warn\",\"msg\":\"DELTA-x\"}"; echo 100%-x under_score-x'` and schedule `0 0 1 1 *`. Trigger 6 manual runs about 35 s apart. `GET /v1/logs?ownerId=tea-daif693dqjvc73e7as3g&resource=<id>&limit=100` shows **5 lines per run, 30 total**, including `error: GAMMA-x three` for every run. Today it is missing in 3 of 6 runs.
- **Order and timestamps.** Each run's lines carry the container's own write timestamps (CRI), not the shipper's read time. The stderr line is not reordered after lines written later.
- **Control unchanged.** A cron that sleeps 3 s between writes (today complete: `plain-stderr`, `second-stderr`, `late-stderr` plus stdout all present) stays complete. A long-running web service's live tail, `level` parsing (`{"level":"warn"}` → `warning`, plain lines → `unknown`) and request-log pipeline behave as before.
- **No duplicates.** A service whose container restarts (crash loop) shows each line once per container lifetime, not re-shipped on every tailer reconnect.
- REST, GraphQL `logs`, MCP log tools and the dashboard Logs page return the same line set for the same filters.

## Source + Goal linkage

- **Source:** infinite `/qa-find-bugs` loop44, 2026-10-05 UTC, muse.env, `bex-canary`, pin `c7afefad5`.
  - Fixture `qa-20261005-l44-logs` (`srv-db1qokm7q7bs739ppc50`) ran the DoD command; its stored `startCommand` was verified through GraphQL.
  - Runs at 13:56:19, 14:00:42, 14:01:33, 14:03:43, 14:04:18 and 14:04:53 UTC (all `successful`). `GET /v1/logs …&limit=100` grouped by second returned all 5 lines at 14:00:42, 14:01:33 and 14:04:18. At 13:56:19, 14:03:43 and 14:04:53 it returned only the 4 stdout lines; `error: GAMMA-l44 three` was absent. `&text=GAMMA` was empty after the first run, so this is not a display issue.
  - Every stdout line arrived in every run.
  - Control fixture `qa-20261005-l44-err` (`srv-db1qq1u7q7bs739ppc6g`), with a 3 s sleep between writes, received all 6 lines, 3 of them on stderr. So stderr is collected in general, and a stdout line starting `error:` is kept.
- **Where it lives:**
  - `deploy/gitops/base/log-shipper.yaml:202-205`: tenant App pods ship through `loki.source.kubernetes "app_pods"`, the kubelet-API follow tailer.
  - The same file's header (`:42-56`, w6/m123) already records that this tailer loses lines from containers that exit inside its retry/backoff window. That is why build and pre-deploy pods were moved to **file** tailing (`alloy.mounts.varlog`, `stage.cri`, `:646-688`).
  - App pods were left on the API tailer. Cron Job pods are always short-lived, and fast crashes of web/worker/private services are too, so this is exactly where the error output a user needs is most likely to vanish.
  - The stderr-only pattern (stdout always complete) is **unverified**. Candidates for t001: per-stream copy ordering in the CRI log versus the tailer's resume-by-timestamp, or the tailer starting after part of the file was written.
- **Governing docs:** ADR010 (logs), ADR038 (cron runs), ADR018 log parity.
- **Goal linkage:** ADR008 reliable hosting. Logs are the primary debugging surface, and a failed cron run's stderr is the reason it failed.
- **Expected outcome:** complete per-run logs for cron jobs and crash output for fast-failing services.
- **Why now:** w4/m114 t007 just made cron failures say "exited with status N" and points users at the logs, which then miss the stderr line about half the time.
- **Render parity:** included (t003). The log line set users read on REST/GraphQL/MCP/UI changes, although no API shape changes.
- **Unverified:** web/worker crash-loop containers (reasoned from the shared pipeline, not probed); Loki ingestion as an alternative cause (stdout lines from the same instant arrive, which argues against it); multi-node placement.
- **Severity:** major (silent loss of the error output in a core journey).

## Production replay — 2026-10-06 (qa loop63, pin `e2e4d0d7e` ⊇ `55403e56d`; deploy run 37404121909 finished 03:02:19Z)

Owned Free cron `qa-20261006-l63-cron` (`srv-db269mk20ers738obg40`, `alpine:3`, the DoD command verbatim; deleted afterwards). `GET /v1/logs?ownerId=tea-daif693dqjvc73e7as3g&resource=<id>&limit=100`, grouped by run second:

- **Batch 1** (runs 03:03:27–03:06:23, starting ~1 min after the deploy finished): 29/30 lines. Runs 03:03:30, 03:04:04, 03:04:39, 03:05:50 and 03:06:25 complete; **03:05:14 lost only `error: GAMMA-x three`** (4 lines).
- **Batch 2** (runs 03:08:18–03:11:07): **30/30**, all six runs complete with the stderr line.
- Line order now follows CRI timestamps (the stderr line sometimes precedes `ALPHA`), consistent with the file tailer being active.
- Before the fix (loop44) the same probe lost the stderr line in 3 of 6 runs.
- **Open question for closeout:** the single miss fell ~3 min after the deploy finished. It may be the log-shipper DaemonSet still rolling node by node (a pod scheduled on a node whose Alloy had not yet restarted onto the CRI-file pipeline), or a residual race. Re-run the DoD's 6-run probe once more before closing; if any run misses, check which node ran it and that node's Alloy config generation.

## Production replay 2 — 2026-10-06 (qa loop64, ≥40 min after deploy run 37404121909)

Fresh owned cron `qa-20261006-l64-cron` (`srv-db26u3k20ers738obgfg`, DoD command verbatim; deleted afterwards). Six runs, 03:46:59–03:49:55: **29/30**. Run 03:48:45 has 4 lines and is missing `{"level":"warn","msg":"DELTA-x"}`; `GAMMA` is present this time. `&level=warning`, `&text=DELTA` and a 03:48:40–50Z window each return only five `DELTA` lines, so the line never reached Loki. This rules out "DaemonSet still rolling" for the loop63 miss: a residual drop of about one line in six runs remains, and it is not specific to stderr. Filed as t007; Closeout now depends on it.

## 解除阻塞记录（2026-10-31，`/unblock w4`）

- **涉及：** w4/m174（同组 D3：w4/m210、w4/m181、w4/306、w4/316、w4/305）
- 来源：用户明确决定（2026-10-31，`/unblock w4`）——“i approve you to update bex-canary's subscription to paid and operate anything there but please clean up afterwards”；以及 2026-10-30：“qa-* 的数据库都是暂时的可以清除”。
- **结论（D3，付费 cron / pre-deploy）：** 允许在 bex-canary 创建短期 Starter（或更高）cron job / web service fixture（cron 与 pre-deploy 现需付费套餐，对齐 Render），验完立即删除。 bex-canary（`tea-daif693dqjvc73e7as3g`）计费状态为 `operator_excluded: structural exclusion`，不走 Stripe 收款（2026-10-29 只读核实 `workspaceBillingReadiness`），付费 fixture 与付费套餐不产生真实费用。
- m174 另需先确认每个节点的 `alloy_build_info` 为 `v1.11.3-bex-cri-first-ts`（生产 log-shipper 镜像 `bex-alloy@sha256:4b3ddf00…`），由 worker 用 `hetzner-prod` 只读完成；不一致则带证据回 blocked。
- 生产已部署 `25b99d8a373a`（`deploy/gitops/base/bex.yaml`，2026-10-30 pin），本项实现均早于此版本；部署不等于验收，以下仍是待验证工作。
- **worker 下一步：** 创建所需付费 fixture，按本项原 DoD 重放。
- **清理（用户要求，验收的一部分）：** 验收结束删除本次在 bex-canary 创建的全部资源；若为此把 bex-canary 切到付费套餐，所有依赖付费档的项做完后切回 Hobby；在本条记录清理结果（删除后 404 / 列表为空、套餐读回 Hobby）。
- **剩余阻塞：** 无（门槛已核实解除，交给 `/loopx w4`）。
- **验收：** 原 DoD / Acceptance 不变，未删减。
