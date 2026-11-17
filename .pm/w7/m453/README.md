# w7 · m453 — A Postgres point-in-time recovery window follows the workspace plan, as on Render

**Worker:** worker7 **Goal:** a paid Postgres can be restored to any point in the past 3 days on a Hobby workspace and the past 7 days on Pro and higher, on every surface. The window starts no earlier than the plan allows, an upgrade does not backfill it, and a recovery outside it is refused. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [core + store: the PITR window per plan, and the floor a plan change keeps (no backfill on upgrade)](t001.md) | 1h15m | — |
| t002 | [postgres: recovery info offers the plan's window, and a recovery outside it is refused on REST, GraphQL and MCP](t002.md) | 1h30m | t001 |
| t003 | [Decide the stored retention, and guard it against the longest plan window](t003.md) | 1h | t001 |
| t004 | [Dashboard: the recovery picker offers the plan's window and says which plan keeps longer](t004.md) | 1h | t002 |
| t005 | [ADR009, ADR018, ADR030 and the plan-table guard state the rule](t005.md) | 30m | t002, t003 |
| t006 | [Production: the window and both refusals on a Hobby workspace's paid database](t006.md) | 45m | t004, t005 |
| t007 | [Render parity](t007.md) | 30m | t004, t005 |
| t008 | [Simplify](t008.md) | 20m | t007 |
| t009 | [Test coverage](t009.md) | 45m | t007 |
| t010 | [Closeout](t010.md) | 10m | t006, t009 |

## Definition of done

- REST `GET /v1/postgres/{id}/recovery` (`startsAt`), GraphQL `databaseRecoveryInfo` (`earliestRecoveryTime`), MCP `get_postgres_recovery_info` and the dashboard's recovery picker each start the window at the later of the ObjectStore's `firstRecoverabilityPoint` and the plan floor. The plan floor is now minus 3 days on Hobby and now minus 7 days on Pro, Scale and Enterprise.
- Upgrading from Hobby does not backfill the window: the floor stays at the old plan's floor from the upgrade, and the window grows forward from there, as Render documents. A downgrade narrows the window at once.
- `POST /v1/postgres/{id}/recovery`, GraphQL `recoverDatabase` and MCP `recover_postgres` refuse a target before the floored start with `RECOVERY_TARGET_BEFORE_WINDOW` naming the floored start. They also refuse a target within ten minutes of now, as Render does. Both refusals happen before any billing gate or `Database` create.
- The stored retention (`deploy/gitops/charts/barman-cloud-objectstores/tenant-postgres.yaml` `retentionPolicy`) is decided and recorded in ADR009. A test fails if it is ever shorter than the longest plan window.
- ADR009 § Backups + point-in-time recovery and ADR018's `Backups · PITR / recovery` row state the rule. ADR030 §9 lists the row as a retention window. `plan_table_test.go`'s `pricing: Point-in-time recovery window` row is a `retention` row checked against the numbers Render shows, and the `followUpPITRWindow` constant is gone.
- Production, once its image pin carries the change: a Hobby workspace's paid database reports a start no earlier than 3 days ago on all three API surfaces and in the picker. A recovery to 4 days ago is refused, and so is one to 5 minutes ago.

## Source + Goal linkage

- **Source:** promoted from [w7/068](../done/068.md) on 2026-11-16. That note was found by w7/m429 t005 while it mapped every row of Render's plan tables to a bex disposition: `lego/backend/internal/pricing/plan_table_test.go:242` names it for the row.
- **Render's behavior** (render.com/docs/postgresql-backups, read 2026-11-16): PITR covers the "Past 3 days" on Hobby and the "Past 7 days" on Pro or higher. "Upgrading from Hobby does not retroactively "backfill" your recovery window. Instead, your existing 3-day window extends to 7 days going forward." "You can't restore to a time that's within ten minutes of the current time." The page does not say what a downgrade does. bex narrows at once, as its log floor does (`core.LogsVisibleFromAfterPlanChange`). The pricing table says the same numbers: "Point-in-time recovery window: 3 days | 7 days | 7 days | 7 days" (`docs/render-artifacts/pricing.md:127`). Render's API contract is `recoveryStatus` + `startsAt` (`lego/backend/internal/api/openapi/render-public-api-1.json`, `retrieve-postgres-recovery-info`).
- **bex today:** the window is whatever the tenant ObjectStore keeps, `retentionPolicy: 30d` on every plan (`tenant-postgres.yaml:32`). `postgres.Service.recoveryWindow` (`lego/backend/internal/postgres/recovery.go:174`) reports CNPG's `firstRecoverabilityPoint` unchanged, and `Recover` refuses only a target before that point or in the future (`recovery.go:276-286`). So bex offers up to 30 days on every plan.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)), managed Postgres ([ADR009](../../../docs/ADR009-postgresql-management.md)), the plan table ([ADR018](../../../docs/ADR018-render-parity.md)). It is the last Postgres row of Render's plan table that bex does not apply.
- **Expected outcome:** the recovery picker, every API surface and the refusal all agree with Render's window for the workspace's plan, and a guard keeps the stored backups long enough for the longest window.
- **Why now:** w7/m429 left this row as a filed follow-up in the guard, and the log and metrics floors it should mirror (`core/plan_gate.go`) shipped in w9/m389 and w7/m427. The pattern is fresh and the change is confined to bex-api, the dashboard panel and one chart value.
- **Render parity included:** the change is visible on REST, GraphQL, MCP and the dashboard.
- **The stored retention is not a gate.** The bex-api floor alone gives Render's behavior in every workspace. Shortening the stored backups is a cost and operations choice (t003). If t003 finds that it needs the user, it files that variant as its own inbox note, and the milestone ships without it.
