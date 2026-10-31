# w4 · m210 — Keep capped cron history chronological

**Worker:** worker4 **Goal:** retain the newest ten cron executions in stable chronological order without reclassifying old completed Jobs as new **Status:** todo（2026-10-31 解除阻塞；已完成任务保持 DONE，详见文末“解除阻塞记录”）

**Estimate:** 2h55 across seven tasks. [Finding and compatibility audit](finding.md) · [Complete live evidence](../evidence/m210-cron-history-boundary.json).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Model history ordering, tombstones and legacy records | 25m | — — **DONE 2026-10-14** |
| [t002](done/t002.md) | Merge chronological observations before applying the cap | 45m | t001 — **DONE 2026-10-14** |
| [t003](done/t003.md) | Verify run adapters, paging and acknowledgement compatibility | 25m | t002 — **DONE 2026-10-14** |
| [t004](done/t004.md) | Render parity | 15m | t003 — **DONE 2026-10-14** |
| [t005](done/t005.md) | Simplify | 10m | t004 — **DONE 2026-10-14** |
| [t006](done/t006.md) | Test coverage | 45m | t004 — **DONE 2026-10-14** |
| [t007](t007.md) | Closeout | 10m | t005, t006 |

## Definition of done

- On an owned Free image cron with an annual future schedule, use native Trigger Run for one successful execution then ten unsuccessful executions, waiting for each terminal result. At the eleventh execution, complete GraphQL and REST histories contain precisely the newest ten run IDs in chronological order: run11 first, old success outside the retained set. Repeated unchanged reads retain that same set/order; no old Job regains priority merely because its status entry was evicted.
- Fresh Events pages at1440×1000 and390×1200 show newest failed runs before older retained ones; the five-row first page and Load more reflect API order. All retained statuses and timestamps remain accurate. A further successful native recovery appears first, displaces only the oldest retained row and advances the bounded success-timestamp derivation correctly.
- Preserve the observed first-failure timestamp control before the cap. lastSuccessfulRunAt continues to derive from the newest **retained** success; when none remains it keeps its documented empty/omitted behavior. Do not add an unbounded success history or fake a date.
- Complete t001/t003/t006's separately marked deleted/pre-start tombstone, legacy clock, tie, restart, cancellation/preemption, mixed scheduled/manual, cursor and adapter controls. No execution replay was observed in this hunt; preserve m108's independent acknowledgement guarantee.
- Required operator/type/codegen/lint and affected backend/dashboard checks pass. Delete the exact owned service, confirm service/runs404 and full baseline ID sets, remove observers/temp files and revoke the own session. Do not close on filing or local-only source changes.

## Source + Goal linkage

- **Source:** continuous functional $qa-find-bugs w4, 2026-10-14 a475; normal dashboard confirmations only. Old success becomes first after run11, reproduced by unchanged reads, fresh desktop/mobile and REST; full transactions and exact cleanup retained.
- **Goal linkage:** ADR008 dependable hosting diagnostics; ADR038 cron execution and ADR006/ADR018 coherent API/dashboard presentation.
- **Expected outcome:** Recent Runs and API pagination identify the actual newest retained executions after history fills.
- **Why now:** the original first-class run-history assumption confuses display eviction with a newly observed Job; the retained set itself is wrong, so a table sort cannot fix it. Existing cancellation/tombstone constraints require a modeled change.
- **Scope:** one operator history producer and optional internal CR status ordering metadata, with adapter/consumer regression verification. Preserve max10, IDs, statuses, failure reasons, execution/acknowledgement, UTC schedule, permissions and poll semantics. No product redesign or long-term history store.
- **Render parity:** included because public reads/UI reflect the producer. Bex GET history is a documented extension; compare primary trigger/cancel contracts and preserve field/wrapper/vocabulary compatibility without inventing upstream retention guarantees. Authenticated Render was not tested.
- **Limits:** only manual runs on one owned Free BusyBox cron were exercised. Scheduled mixes, cancellation, process restart, deleted tombstones, legacy migration, cursor tails, MCP/CLI and broad suites are verification tasks, not this pass's completed guarantees.

## CLI replay (2026-10-15 UTC)

[sweep254 evidence in w8](../../w8/evidence/cron-history-sweep254/README.md) adds a native Go cron created and maintained through published Bex, with both pinned clients reading its service metadata and logs. Bodyless API triggers run one success, ten failures and recovery sequentially. After run11 all REST/GraphQL/MCP histories still put the oldest success first and displace a newer failure; recovery puts the old success second. Both clients retrieve exactly12 execution messages and the success timestamp advances correctly on recovery. All owned resources are cleaned, service/runs404 and quota6/terminating0.

Current source6255ed8b2 contains f42f5d086 and all four focused new ordering tests pass. Deployed revision remains unknown, so this does not establish whether the source fix has reached the exercised controller or another case remains. t007 stays blocked; the original image/UI and broader DoD remain unverified by this CLI/manual-Go replay.

## 解除阻塞记录（2026-10-31，`/unblock w4`）

- **涉及：** w4/m210（同组 D3：w4/m174、w4/m181、w4/306、w4/316、w4/305）
- 来源：用户明确决定（2026-10-31，`/unblock w4`）——“i approve you to update bex-canary's subscription to paid and operate anything there but please clean up afterwards”；以及 2026-10-30：“qa-* 的数据库都是暂时的可以清除”。
- **结论（D3，付费 cron / pre-deploy）：** 允许在 bex-canary 创建短期 Starter（或更高）cron job / web service fixture（cron 与 pre-deploy 现需付费套餐，对齐 Render），验完立即删除。 bex-canary（`tea-daif693dqjvc73e7as3g`）计费状态为 `operator_excluded: structural exclusion`，不走 Stripe 收款（2026-10-29 只读核实 `workspaceBillingReadiness`），付费 fixture 与付费套餐不产生真实费用。
- 生产已部署 `25b99d8a373a`（`deploy/gitops/base/bex.yaml`，2026-10-30 pin），本项实现均早于此版本；部署不等于验收，以下仍是待验证工作。
- **worker 下一步：** 创建所需付费 fixture，按本项原 DoD 重放。
- **清理（用户要求，验收的一部分）：** 验收结束删除本次在 bex-canary 创建的全部资源；若为此把 bex-canary 切到付费套餐，所有依赖付费档的项做完后切回 Hobby；在本条记录清理结果（删除后 404 / 列表为空、套餐读回 Hobby）。
- **剩余阻塞：** 无（门槛已核实解除，交给 `/loopx w4`）。
- **验收：** 原 DoD / Acceptance 不变，未删减。
