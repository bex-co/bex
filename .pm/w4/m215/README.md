# w4 · m215 — Keep accepted Postgres upgrades non-Ready

**Worker:** worker4 **Goal:** An accepted major upgrade never returns to Available before the target major is healthy, and the dashboard follows its progress without a reload. **Status:** todo（2026-10-31 解除阻塞；已完成任务保持 DONE，详见文末“解除阻塞记录”）

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Hold readiness while an accepted major upgrade awaits CNPG acknowledgement — **DONE** | 45m | — |
| t002 | Replay the native upgrade status sequence and recovery controls | 35m | t001 |
| t003 | Render parity — **DONE** | 15m | t002 |
| t004 | Simplify — **DONE** | 10m | t003 |
| t005 | Test coverage — **DONE** | 25m | t003 |
| t006 | Closeout | 10m | t004, t005 |

Total: **140m, 6 tasks**. [Researched finding](finding.md), [durable observations](evidence.json), [isolated diagnostic](diagnostic.go.txt), [diagnostic output](diagnostic.log.txt).

## Definition of done

- After an accepted supported upgrade, neither the API nor the open dashboard returns to Available before the target major is observed and the required instances are ready. The sequence may use Restarting before Upgrading; the old observed version remains truthful until it changes.
- A fresh desktop and 390px native upgrade shows the transition through completion without reloading. Repeat with another supported source/target pair; verify the process version and retained fixture data, then remove every owned resource and session.
- First provision, unchanged healthy databases, manual restart, suspension/resume, failed-upgrade revert and missing/unknown version observations keep their existing explicit meanings. A rejected or reverted target cannot leave a perpetual Upgrading state.
- Meaningful operator and public status regressions fail on the old implementation. Run the relevant operator suite/lint, backend status tests and dashboard checks required by any changed code.
- Finish the standing tasks and move this milestone and its tasks to done only after those observations hold.

## Source + Goal linkage

- **Source:** Functional `$qa-find-bugs w4`, a712/a713, 2026-10-16 UTC. Two fresh native a713 transitions reproduce the issue; a712 independently first exposed the badge lag. See the finding for the exact capture and source limits.
- **Goal linkage:** ADR008 usable Render-alternative hosting; ADR009 truthful upgrade state; ADR018 cross-surface status parity.
- **Expected outcome:** The owner can trust upgrade progress instead of seeing Available while the offline upgrade runs.
- **Why now:** Accepted upgrades already work, but old healthy CNPG status can satisfy the new Database generation before CNPG acknowledges the target. It also cancels fast dashboard polling. Fix this gap separately from the completed manual-restart acknowledgement work in w4/m137/t010 and the SQL execution gate in w4/177.
- **Render parity included:** This changes an operator-produced status consumed by REST, GraphQL, MCP and the dashboard. Preserve the existing Bex vocabulary and validation; review the documented Render upgrade lifecycle.

## Implementation record (2026-10-16)

- **t001:** `reconcileDatabaseReadiness` (`lego/operator/internal/controller/database_controller.go`) gained `majorUpgradePending`: a known `spec.version` ahead of the recorded major holds `Upgrading`/Ready=False (`MajorVersionUpgrade`) before CNPG announces its phase. It runs after the failed and running branches, so a failed upgrade's revert still wins. A second, latent gap was found while testing: CNPG reports the target PGDATA major before its instances are ready. Recording that early hid the version change from the healthy pass, which then took no post-upgrade base backup and re-pointed archiving at the source generation's prefix. A new major is now recorded only once its cluster is healthy (or the upgrade failed, keeping the w5/110 revert). The `CurrentVersion` doc and CRD description say so.
- **t003/t004:** REST/GraphQL/MCP and the dashboard consume the operator status unchanged. ADR009 documents the before-phase hold and the adoption rule; ADR018's upgrade row records `upgrading` from acceptance until healthy. The `upgrading` vocabulary drift is retained, and authenticated Render timing was not compared. The change is a small decision helper plus one adoption condition, reviewed inline.
- **t005:** `TestAcceptedMajorUpgradeStaysNonReadyUntilTheTargetIsHealthy` covers four frames: old major healthy before acknowledgement, CNPG Upgrading, target observed but not ready, and target healthy. It asserts phase, Ready, recorded version, the post-upgrade backup and the pg17 archive server. The old controller fails at the first frame, and the old adoption alone fails at the third. `TestPendingMajorUpgradeLeavesOtherReadinessAlone` covers an unchanged major, nothing yet observed, and a failed upgrade reverted then Ready. `TestMajorUpgradePending` is the decision table. The backend's `TestPendingMajorUpgradeReadsUpgradingWithTheSourceVersion` checks the public status and version. Existing restart/suspend/failed-upgrade tests pass. Operator `make test` (codegen + envtest) and `make lint` pass. **Not run:** the dashboard live transition and a deployed CNPG, which are t002.

**Live progress, 2026-10-17:** [t002's a892 replay](t002.md#additional-live-replay--a892-2026-10-17-utc) adds desktop 13→16 and fresh 390px 16→18 with preserved rows/sequences, sampled native status/UI receipts and complete cleanup. The original a719 already covered 16→17→18. The remaining gate is t002 step 1's isolated recovery/unknown-version observations, followed by t006; no task or milestone is moved to done by this filing. No implementation suites were rerun.

## 解除阻塞记录（2026-10-31，`/unblock w4`）

- **涉及：** w4/m215（同组 D1：w4/m223、w4/m214、w4/m179、w4/m167、w4/m170、w4/m208、w4/325、w4/177、w4/364）
- 来源：用户明确决定（2026-10-31，`/unblock w4`）——“i approve you to update bex-canary's subscription to paid and operate anything there but please clean up afterwards”；以及 2026-10-30：“qa-* 的数据库都是暂时的可以清除”。
- **结论（D1，Free Postgres 名额）：** 允许删除 `qa-20261017-64552e-pg14`（`dpg-db9r654flioc739mcpbg`，2026-10-29 只读确认仍 available、占用 bex-canary 唯一 Free 名额；w8/w2 只把它当只读基线）。DoD 明确要求 Free 的检查使用腾出的 Free 名额（同一时间只能一个项占用，验完立即删除再交给下一项）；DoD 未限定 Free、或需要多个库的检查可用短期付费 Postgres。 bex-canary（`tea-daif693dqjvc73e7as3g`）计费状态为 `operator_excluded: structural exclusion`，不走 Stripe 收款（2026-10-29 只读核实 `workspaceBillingReadiness`），付费 fixture 与付费套餐不产生真实费用。
- 生产已部署 `25b99d8a373a`（`deploy/gitops/base/bex.yaml`，2026-10-30 pin），本项实现均早于此版本；部署不等于验收，以下仍是待验证工作。
- **worker 下一步：** 先读 `GET /v1/postgres?ownerId=tea-daif693dqjvc73e7as3g` 确认 pg14 是否仍在（他人可能已删）；仍在则删除并确认 404；然后按本项原 DoD 重放。
- **清理（用户要求，验收的一部分）：** 验收结束删除本次在 bex-canary 创建的全部资源；若为此把 bex-canary 切到付费套餐，所有依赖付费档的项做完后切回 Hobby；在本条记录清理结果（删除后 404 / 列表为空、套餐读回 Hobby）。
- **剩余阻塞：** 无（门槛已核实解除，交给 `/loopx w4`）。
- **验收：** 原 DoD / Acceptance 不变，未删减。
