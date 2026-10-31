# w4 · m167 — Loop5: redact secrets from Postgres Insights query text

**Worker:** worker4 **Goal:** Database Insights never publishes credential material — no password verifier or secret literal reaches any surface **Status:** todo（2026-10-31 解除阻塞；已完成任务保持 DONE，详见文末“解除阻塞记录”）

## Tasks (in order)

| id   | title                                              | est | depends_on |
| ---- | -------------------------------------------------- | --- | ---------- |
| t001 — **DONE** | Redact secret-bearing statements in Insights reads | 1h  | —          |
| t002 — **DONE** | Render parity across the Insights surfaces         | 30m | w4/m167/t001 |
| t003 — **DONE** | Simplify the touched backend code                  | 30m | w4/m167/t002 |
| t004 — **DONE** | Test coverage for Insights redaction               | 1h  | w4/m167/t002 |
| t005 | Closeout                                           | 15m | w4/m167/t004 |

## Definition of done

- Top Queries and Active Processes redact statements carrying secrets (at minimum `PASSWORD`/`ENCRYPTED PASSWORD` DDL verifiers) before they leave `bex-api`, on REST, GraphQL, and MCP alike.
- Ordinary query text (SELECT/INSERT/DDL without secrets) still renders verbatim; the existing `<insufficient privilege>` masking still holds.
- No dashboard change required beyond what the redacted payload already yields.

## Source + Goal linkage

- **Source:** infinite `/qa-find-bugs` loop5 2026-10-04 UTC (muse.env). Live evidence: fresh database `dpg-…9scg` (since deleted) showed `ALTER ROLE "dpg_…_user" WITH PASSWORD 'SCRAM-SHA-256$4096:…'` verbatim in Insights → Top queries — the user's SCRAM verifier, which enables offline brute-force of the database password, visible to every Insights reader.
- **Goal linkage:** tenant-data security (auth/credential hygiene); a credential leak in a routine observability table.
- **Expected outcome:** Insights query text is safe to show: secrets redacted at the read, on every surface.
- **Why now:** the leak is live on every fresh database (provisioning runs the `ALTER ROLE` itself, so the verifier sits in `pg_stat_statements` from minute one). Render parity task included (t002): user-facing observability surface with REST/GraphQL/MCP equivalents to check.

## 解除阻塞记录（2026-10-31，`/unblock w4`）

- **涉及：** w4/m167（同组 D1：w4/m223、w4/m215、w4/m214、w4/m179、w4/m170、w4/m208、w4/325、w4/177、w4/364）
- 来源：用户明确决定（2026-10-31，`/unblock w4`）——“i approve you to update bex-canary's subscription to paid and operate anything there but please clean up afterwards”；以及 2026-10-30：“qa-* 的数据库都是暂时的可以清除”。
- **结论（D1，Free Postgres 名额）：** 允许删除 `qa-20261017-64552e-pg14`（`dpg-db9r654flioc739mcpbg`，2026-10-29 只读确认仍 available、占用 bex-canary 唯一 Free 名额；w8/w2 只把它当只读基线）。DoD 明确要求 Free 的检查使用腾出的 Free 名额（同一时间只能一个项占用，验完立即删除再交给下一项）；DoD 未限定 Free、或需要多个库的检查可用短期付费 Postgres。 bex-canary（`tea-daif693dqjvc73e7as3g`）计费状态为 `operator_excluded: structural exclusion`，不走 Stripe 收款（2026-10-29 只读核实 `workspaceBillingReadiness`），付费 fixture 与付费套餐不产生真实费用。
- 生产已部署 `25b99d8a373a`（`deploy/gitops/base/bex.yaml`，2026-10-30 pin），本项实现均早于此版本；部署不等于验收，以下仍是待验证工作。
- **worker 下一步：** 先读 `GET /v1/postgres?ownerId=tea-daif693dqjvc73e7as3g` 确认 pg14 是否仍在（他人可能已删）；仍在则删除并确认 404；然后按本项原 DoD 重放。
- **清理（用户要求，验收的一部分）：** 验收结束删除本次在 bex-canary 创建的全部资源；若为此把 bex-canary 切到付费套餐，所有依赖付费档的项做完后切回 Hobby；在本条记录清理结果（删除后 404 / 列表为空、套餐读回 Hobby）。
- **剩余阻塞：** 无（门槛已核实解除，交给 `/loopx w4`）。
- **验收：** 原 DoD / Acceptance 不变，未删减。
