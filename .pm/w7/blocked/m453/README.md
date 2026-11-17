# w7 · m453 — A Postgres point-in-time recovery window follows the workspace plan, as on Render

**Worker:** worker7 **Goal:** a paid Postgres can be restored to any point in the past 3 days on a Hobby workspace and the past 7 days on Pro and higher, on every surface. Where the plan's floor cuts a database's backups, the window starts at the newest base backup completed at or before the floor, as Render's does (w7/075). An upgrade does not backfill it, and a recovery outside it is refused. **Status:** blocked (t001–t005, t007–t009 done; t006 waits for a production pin carrying `cef479863` and for its Hobby fixture to be 4 days old, 2026-11-21T19:00Z, then t010)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [core + store: the PITR window per plan, and the floor a plan change keeps (no backfill on upgrade)](done/t001.md) — **DONE** | 1h15m | — |
| t002 | [postgres: recovery info offers the plan's window, and a recovery outside it is refused on REST, GraphQL and MCP](done/t002.md) — **DONE** | 1h30m | t001 |
| t003 | [Decide the stored retention, and guard it against the longest plan window](done/t003.md) — **DONE** | 1h | t001 |
| t004 | [Dashboard: the recovery picker offers the plan's window and says which plan keeps longer](done/t004.md) — **DONE** | 1h | t002 |
| t005 | [ADR009, ADR018, ADR030 and the plan-table guard state the rule](done/t005.md) — **DONE** | 30m | t002, t003 |
| t006 | [Production: the window and both refusals on a Hobby workspace's paid database](t006.md) | 45m | t004, t005 |
| t007 | [Render parity](done/t007.md) — **DONE** | 30m | t004, t005 |
| t008 | [Simplify](done/t008.md) — **DONE** | 20m | t007 |
| t009 | [Test coverage](done/t009.md) — **DONE** | 45m | t007 |
| t010 | [Closeout](t010.md) | 10m | t006, t009 |

## Definition of done

- REST `GET /v1/postgres/{id}/recovery` (`startsAt`), GraphQL `databaseRecoveryInfo` (`earliestRecoveryTime`), MCP `get_postgres_recovery_info` and the dashboard's recovery picker each start the window at the same instant. The plan floor is now minus 3 days on Hobby and now minus 7 days on Pro, Scale and Enterprise. Where it cuts the ObjectStore's window, the start is the newest base backup completed at or before the floor (CNPG `Backup` `status.stoppedAt`, or the ObjectStore's `lastSuccessfulBackupTime`), never before `firstRecoverabilityPoint`, and the floor itself when no such backup is known. A database younger than the window keeps its own first point. A database whose base backups stopped before the floor stays `AVAILABLE` from its last one. (Amended 2026-11-17 from "the later of `firstRecoverabilityPoint` and the plan floor" to Render's captured start; see the record below.)
- Upgrading from Hobby does not backfill the window: the floor stays at the old plan's floor from the upgrade, and the window grows forward from there, as Render documents. A downgrade narrows the window at once.
- `POST /v1/postgres/{id}/recovery`, GraphQL `recoverDatabase` and MCP `recover_postgres` refuse a target before that start with `RECOVERY_TARGET_BEFORE_WINDOW` naming it. They also refuse a target within ten minutes of now, as Render does. Both refusals happen before any billing gate or `Database` create.
- The stored retention (`deploy/gitops/charts/barman-cloud-objectstores/tenant-postgres.yaml` `retentionPolicy`) is decided and recorded in ADR009. A test fails if it is ever shorter than the longest plan window.
- ADR009 § Backups + point-in-time recovery and ADR018's `Backups · PITR / recovery` row state the rule. ADR030 §9 lists the row as a retention window. `plan_table_test.go`'s `pricing: Point-in-time recovery window` row is a `retention` row checked against the numbers Render shows, and the `followUpPITRWindow` constant is gone.
- Production, once its image pin carries `94c8743a1` and `cef479863`: a Hobby workspace's paid database at least 4 days old reports the same start on all three API surfaces and in the picker, and on two reads a minute apart. That start is the newest base backup completed at or before now minus 3 days, so with nightly backups it lies between 3 and 4 days ago. A recovery to 4 days ago, inside its stored backups, is refused, and so is one to 5 minutes ago. A Scale database's start is the same for 7 days.

## Source + Goal linkage

- **Source:** promoted from [w7/068](../../done/068.md) on 2026-11-16. That note was found by w7/m429 t005 while it mapped every row of Render's plan tables to a bex disposition: `lego/backend/internal/pricing/plan_table_test.go:242` names it for the row.
- **Render's behavior** (render.com/docs/postgresql-backups, read 2026-11-16): PITR covers the "Past 3 days" on Hobby and the "Past 7 days" on Pro or higher. "Upgrading from Hobby does not retroactively "backfill" your recovery window. Instead, your existing 3-day window extends to 7 days going forward." "You can't restore to a time that's within ten minutes of the current time." The page does not say what a downgrade does. bex narrows at once, as its log floor does (`core.LogsVisibleFromAfterPlanChange`). The pricing table says the same numbers: "Point-in-time recovery window: 3 days | 7 days | 7 days | 7 days" (`docs/render-artifacts/pricing.md:127`). Render's API contract is `recoveryStatus` + `startsAt` (`lego/backend/internal/api/openapi/render-public-api-1.json`, `retrieve-postgres-recovery-info`).
- **bex today:** the window is whatever the tenant ObjectStore keeps, `retentionPolicy: 30d` on every plan (`tenant-postgres.yaml:32`). `postgres.Service.recoveryWindow` (`lego/backend/internal/postgres/recovery.go:174`) reports CNPG's `firstRecoverabilityPoint` unchanged, and `Recover` refuses only a target before that point or in the future (`recovery.go:276-286`). So bex offers up to 30 days on every plan.
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)), managed Postgres ([ADR009](../../../../docs/ADR009-postgresql-management.md)), the plan table ([ADR018](../../../../docs/ADR018-render-parity.md)). It is the last Postgres row of Render's plan table that bex does not apply.
- **Expected outcome:** the recovery picker, every API surface and the refusal all agree with Render's window for the workspace's plan, and a guard keeps the stored backups long enough for the longest window.
- **Why now:** w7/m429 left this row as a filed follow-up in the guard, and the log and metrics floors it should mirror (`core/plan_gate.go`) shipped in w9/m389 and w7/m427. The pattern is fresh and the change is confined to bex-api, the dashboard panel and one chart value.
- **Render parity included:** the change is visible on REST, GraphQL, MCP and the dashboard.
- **The stored retention is not a gate.** The bex-api floor alone gives Render's behavior in every workspace. Shortening the stored backups is a cost and operations choice (t003). If t003 finds that it needs the user, it files that variant as its own inbox note, and the milestone ships without it.

## Record

- **2026-11-16, t001–t005 and t007–t009 shipped** (one commit with this record).
  - The window per plan, the kept floor (migration 0290) and the reader are in core and the store.
  - Recovery info, the refusals and the `retention` hint are on REST, GraphQL and MCP. REST keeps Render's two fields.
  - The dashboard picker is bounded by the floored start and open time minus ten minutes. It shows the plan hint, and its pending state matches the ready panel at 1280px and 375px.
  - ADR009, ADR018 and ADR030 are updated, and the plan-table row is checked.
  - Every suite passed: the real-DB backend suite, `make lint`, dashboard lint and tests.
- **t003 decision: the stored retention stays `30d`.** The bex-api floor alone gives Render's window. The extra 23 days are ADR031's operator restore margin, and Wasabi's 90-day minimum storage duration means a shorter policy saves nothing. No user decision is needed, so nothing was filed. A backend test fails if the chart's `retentionPolicy` is ever shorter than `core.LongestPITRRetention()`. ADR009 records the reasoning.
- **Stalled-backup edge (t002):** a last base backup older than the plan floor means no established window (`BACKUP_NOT_READY`, recover refused before billing). ADR009 records it. Render's own answer is unverified and filed with two other undocumented points as [w7/075](../075.md). _Superseded 2026-11-17 by `cef479863` (w7/075): Render answers `AVAILABLE` there, so such a database now keeps a window from its last base backup._
- **Deviation:** t004's comparison and t007's walk did not use `dev-7`. The local cluster's API server was down, and a local stack has no backup store. They used the service-level tests on every surface and the dashboard against `local-bex.mjs` with a temporary recovery fixture (see t004, t007).
- **Gate (clears by itself):** t006 needs production to serve the change. Check `git merge-base --is-ancestor "$(git log -1 --format=%H --grep="w7/m453 t001" origin/main)" <the sha of the newest "pin platform images to" commit on main>`, then run t006's walk. t010 follows. _Cleared 2026-11-17 for `94c8743a1`; the gate now also needs `cef479863` (record below)._

## 2026-11-17, `/pm unblock` (verified evidence, no user decision): t006 follows `cef479863`, and the walk waits for it

- **Affected:** t006 (objective, gates, steps and acceptance), the definition of done's first, third and last bullets, and the Goal line. t001–t005 and t007–t009 stay done.
- **Source: verified evidence.**
  - The release gate above cleared for `94c8743a1`. The newest pin on origin/main, `fc4fbf28a chore(deploy): pin platform images to db48cde2bae9`, carries it: `git merge-base --is-ancestor 94c8743a1 db48cde2bae9` succeeds.
  - `cef479863` (w7/075, 2026-11-17) changed the rule this milestone walks. On Render, a running paid Pro database's `startsAt` was a base backup's time, 7 days 20 hours back: the newest backup at or before the 7-day floor, not now minus 7 days. A paid database whose backups stopped in August still answered `AVAILABLE` ([render-artifacts/postgres-recovery-window.md](../../../../docs/render-artifacts/postgres-recovery-window.md)). bex now starts a cut window the same way, and ADR009 and ADR018 say so.
  - That pin does not carry `cef479863`: `git merge-base --is-ancestor cef479863 db48cde2bae9` fails. So production still starts a cut window at now minus the plan's days.
- **Chosen behavior (Render parity first, ADR008).** The definition of done and t006 now check Render's start: the newest base backup completed at or before the floor, the same instant on every surface and on every read.
  - "No earlier than now minus 3 days" is gone. Against the shipped rule it would fail, because the start lies up to one backup interval before the floor.
  - The refusal checks stay. A 4-day target must now lie inside the fixture's stored backups, so that the floor refuses it and not the database's age.
- **Walked on the pin `db48cde2bae9`, 17:50–18:01Z.** Signed in as the QA user with `scripts/qa-login.sh`. REST, GraphQL and MCP were called with the session cookie, and the dashboard was read with a headless Chromium (the Playwright MCP server did not connect).
  - _Scale window (`bex`: `twenty-postgres` `dpg-dbnukjpngmjs739tgukg` and `beancount-forum-db` `dpg-d9nqg95cavls73fp8m20`, base backups daily at 03:00Z for more than 7 days)._ REST `startsAt` and GraphQL and MCP `earliestRecoveryTime` read now minus 7 days, rounded up to the second: `2026-11-10T17:51:04Z`, `…:05Z` and `…:05Z` for reads between 17:51:03.6Z and 17:51:04.3Z. That is the rule before `cef479863`, and the start moved with the clock. `retention` read `{days: 7, plan: scale}` with no upgrade half. With `cef479863`, the start should be the completion of the 2026-11-10 03:00Z backup.
  - _Scale refusals (`twenty-postgres`, restoring into `basic_256mb`)._ A target 8 days ago (`2026-11-09T17:51:24Z`, inside its stored backups) answered `RECOVERY_TARGET_BEFORE_WINDOW` on REST (400, `params.earliestRecoveryTime`), GraphQL (`extensions.code`) and MCP (`isError`). A target 5 minutes ago answered `RECOVERY_TARGET_TOO_RECENT` on all three, with `latestTargetTime` at now minus ten minutes. No database was created: the workspace still lists its four.
  - _Scale dashboard._ The panel's earliest point read "November 10, 2026 at 5:53 PM", and the hint read "Scale workspaces can restore to any point in the past 7 days." For a dialog opened at 17:53:09Z, the picker's `min` was `2026-11-10T17:53` and its `max` was `17:43`.
  - _Hobby fixture._ None of the QA user's Hobby workspaces had a Postgres, so t006's fallback applies. `qa-20261117-m453-hobby` (`dpg-dbu98feggh5s73emep10`, `basic_256mb`, in `tian-personal`, Hobby) was created at 17:51:57Z after a dry run. Its creation backup completed at 17:55:22Z.
    - From then on, all three surfaces read `AVAILABLE` from `2026-11-17T17:55:22Z`, with `retention {days: 3, plan: hobby, upgradeDays: 7, upgradePlan: pro}`.
    - The panel's hint reads "Hobby workspaces can restore to any point in the past 3 days. Pro keeps 7 days." The picker's `min` is that first point.
    - A target 4 days ago answers `RECOVERY_TARGET_BEFORE_WINDOW` naming 17:55:22Z. It lies before the database's first point, so it is not yet the floor's refusal.
    - At 18:00:41Z, a target 5 minutes ago (`17:55:40Z`, after the first point) answered `RECOVERY_TARGET_TOO_RECENT` on REST, GraphQL and MCP, with `latestTargetTime` at now minus ten minutes. No database was created: the workspace lists only the fixture.
    - For its first ten minutes, a new database has no valid target. The picker's `max` (17:46 for a dialog opened at 17:56:04Z) was before its `min` (17:55), and the ten-minute refusal names a time before the first point. Render's ten-minute rule implies the same, so this was not filed.
  - _Cleanup._ The fixture stays for t006 (about $0.20 a day on `tian-personal`). The QA session was logged out, and the API answered 401 to its cookie afterwards. The cookie jar and state file were deleted. No API key was minted, and no kubeconfig was fetched.
- **Remaining gate (clears by itself, no user action).** Both must hold:
  1. A production pin carries `cef479863`: `git merge-base --is-ancestor cef479863 <the sha in the newest "pin platform images to" commit on main>`. deploy.yml deploys at most every 6 hours after the last pin (12:08Z), so the next pin can come from about 18:08Z on 2026-11-17.
  2. The fixture is at least 4 days old: on or after 2026-11-21T19:00Z. By then the 3-day floor cuts its window, its start is its 2026-11-18 03:00Z backup, and a target 4 days ago lies inside its stored backups.
- **Next:** t006 as rewritten, on the fixture and a Scale database in `bex`, then delete the fixture. Then t010.
