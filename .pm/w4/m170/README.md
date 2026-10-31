# w4 · m170 — Refuse reserved Postgres identifiers so a database name or user can't break or wedge a database

**Worker:** worker4 **Goal:** reserved PostgreSQL names are refused with a 400 at create and at add-user. A database whose reconcile fails says why, and a tenant can always undo a user add. **Status:** todo（2026-10-31 解除阻塞；已完成任务保持 DONE，详见文末“解除阻塞记录”）

## Tasks (in order)

| id   | title                                                               | est | depends_on                 |
| ---- | ------------------------------------------------------------------- | --- | -------------------------- |
| t001 — **DONE** | Refuse reserved database names and roles at every write path        | 1h  | —                          |
| t002 — **DONE** | Recover databases already wedged by a reserved managed role         | 45m | w4/m170/t001               |
| t003 — **DONE** | Surface the failure reason of an unavailable database               | 1h  | —                          |
| t004 — **DONE** | Render parity across Postgres create/user/read surfaces             | 30m | w4/m170/t002, w4/m170/t003 |
| t005 — **DONE** | Simplify changed code                                               | 20m | w4/m170/t004               |
| t006 — **DONE** | Test coverage for shipped behavior                                  | 45m | w4/m170/t004               |
| t007 | Closeout                                                            | 15m | w4/m170/t006               |

## Definition of done

Each bullet re-runs a probe from the hunt (dashboard origin, page `fetch` with credentials, owned Free `qa-` fixtures, deleted afterwards):

- **Create, reserved user:** the New Postgres dialog with Database user `postgres` blocks Create with a reserved-name message. `POST /v1/postgres` (and GraphQL/MCP create) with `databaseUser: "postgres"` returns 400 `POSTGRES_IDENTIFIER_RESERVED` naming the field. Today the create is accepted and the database goes `unavailable` within about 30 s, forever, with no reason shown.
- **Create, reserved database name:** `databaseName: "template1"` returns 400 (today: `available`, but `current_database()` is the system `template1` owned by `postgres`, and the tenant user has `has_database_privilege(…,'CREATE') = false` and `has_schema_privilege(…,'public','CREATE') = false`, so not one table can be created). The same holds for `postgres` and `template0`.
- **Add user:** `POST /v1/postgres/<id>/users {name:"postgres"}` returns 400. Today it returns 201 with a one-time password, the healthy database flips to `unavailable`, and `DELETE …/users/postgres` (204) does not bring it back, because the `ensure: absent` tombstone keeps the reconcile failing. All the while `select 1` still works.
- **Control:** `qa_owner` as user and `orders_data` as database name still create and become `available`. Adding user `qa_extra` still works.
- **Reason visible:** while a database is `unavailable` because its reconcile failed, REST/GraphQL/MCP carry a value-free reason, and the dashboard's Details shows it next to the status.

## Source + Goal linkage

- **Source:** infinite `/qa-find-bugs` loop8 2026-10-04 UTC (muse.env, `bex-canary`), journey 11. Fixtures, all deleted to 404:
  - `dpg-db1ep7fhb1uc73ebigs0`: user `postgres` → unavailable.
  - `dpg-db1erf1j8bls738hh140`: database `template1` → available but unusable.
  - `dpg-db1esrpj8bls738hh190`: healthy, then add-user `postgres` → unavailable, then delete-user → still unavailable.

  Evidence (local): `.playwright-mcp/qa-pg-reserved-user-1.png`, `.playwright-mcp/qa-pg-reserved-user-wedged-1.png`. Governing ADR: ADR009 (PostgreSQL management).
- **Root cause:**
  - `lego/types/v1alpha1/database_types.go:39` `ValidPostgresIdentifier` checks only syntax. It is used by create (`lego/backend/internal/postgres/service.go:300,370-377`), add-user (`internal/postgres/access.go:138`) and Blueprint databases (`internal/apps/deploy.go:2431,2434`). No layer reserves `postgres`/`template0`/`template1` as database names, nor `postgres`, `streaming_replica` or the `pg_` prefix as roles.
  - The operator projects the owner and users onto CNPG `spec.managed.roles`, with deleted users as `ensure: absent` tombstones (`lego/operator/internal/controller/database_controller.go:365-400`). A reserved role therefore fails the reconcile on add and again after delete.
  - `dbFail` sets `Phase=Failed` with the reason only on the Ready condition (`database_controller.go:1484-1488`). `dbStatus` maps that to `unavailable`, and no read surface exposes the condition message: the GraphQL `Database` type has no reason field.
- **Goal linkage:** ADR008 reliable managed data, and honest status.
- **Expected outcome:** a tenant cannot create or reconfigure a database into a broken or wedged state with a name the platform can't honor, and a failed database explains itself.
- **Why now:** any tenant reaches this from the create dialog or the add-user API. The add-user path takes a working production database permanently out of reconcile (status lies while it still serves, and later plan/IP changes can't apply). Render parity task INCLUDED (t004): REST/GraphQL/MCP/UI surfaces change.
- **Unverified:**
  - The exact rejection text, inferred to come from the CNPG webhook or controller refusing to manage the reserved role; t003 must read it from the Database condition.
  - Whether later spec changes (plan, IP allowlist) indeed fail to apply on a wedged database.
  - `pg_`-prefixed roles and `streaming_replica` (reasoned, not probed).
  - Blueprint `databases[].databaseName/user` (same validator, not probed).
  - The tenant does **not** appear to get superuser access: the role stayed CNPG-unmanaged. The returned password was deliberately never read.
- **Severity:** major.

## 解除阻塞记录（2026-10-31，`/unblock w4`）

- **涉及：** w4/m170（同组 D1：w4/m223、w4/m215、w4/m214、w4/m179、w4/m167、w4/m208、w4/325、w4/177、w4/364）
- 来源：用户明确决定（2026-10-31，`/unblock w4`）——“i approve you to update bex-canary's subscription to paid and operate anything there but please clean up afterwards”；以及 2026-10-30：“qa-* 的数据库都是暂时的可以清除”。
- **结论（D1，Free Postgres 名额）：** 允许删除 `qa-20261017-64552e-pg14`（`dpg-db9r654flioc739mcpbg`，2026-10-29 只读确认仍 available、占用 bex-canary 唯一 Free 名额；w8/w2 只把它当只读基线）。DoD 明确要求 Free 的检查使用腾出的 Free 名额（同一时间只能一个项占用，验完立即删除再交给下一项）；DoD 未限定 Free、或需要多个库的检查可用短期付费 Postgres。 bex-canary（`tea-daif693dqjvc73e7as3g`）计费状态为 `operator_excluded: structural exclusion`，不走 Stripe 收款（2026-10-29 只读核实 `workspaceBillingReadiness`），付费 fixture 与付费套餐不产生真实费用。
- 生产已部署 `25b99d8a373a`（`deploy/gitops/base/bex.yaml`，2026-10-30 pin），本项实现均早于此版本；部署不等于验收，以下仍是待验证工作。
- **worker 下一步：** 先读 `GET /v1/postgres?ownerId=tea-daif693dqjvc73e7as3g` 确认 pg14 是否仍在（他人可能已删）；仍在则删除并确认 404；然后按本项原 DoD 重放。
- **清理（用户要求，验收的一部分）：** 验收结束删除本次在 bex-canary 创建的全部资源；若为此把 bex-canary 切到付费套餐，所有依赖付费档的项做完后切回 Hobby；在本条记录清理结果（删除后 404 / 列表为空、套餐读回 Hobby）。
- **剩余阻塞：** 无（门槛已核实解除，交给 `/loopx w4`）。
- **验收：** 原 DoD / Acceptance 不变，未删减。
