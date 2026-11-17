# Render's Postgres point-in-time recovery window — live read-only capture

Captured 2026-11-17 (w7/075) with read-only `GET https://api.render.com/v1/postgres/{postgresId}/recovery` (`retrieve-postgres-recovery-info`) and `GET /v1/postgres/{postgresId}` against the three databases of the reference Render account that bex's earlier Render captures use. Two of them are paid `basic_256mb` databases in the `stargately` workspace, on the **Professional (Legacy)** plan ([workspace-plan-change.md](workspace-plan-change.md)), so their plan window is "Past 7 days". The account's other workspace, on Hobby, has no database. Database ids are shortened to `dpg-…`. Nothing was written: a `POST …/recovery` that Render accepts creates a billed database, so the refusal shapes are not captured here.

## Requests and answers

| Database | Read at (UTC) | `GET …/recovery` | Answer |
| --- | --- | --- | --- |
| `dpg-…` `basic_256mb`, `available`, Postgres 17, created 2025-09-03 | 2026-11-17T12:53:43Z | 200 | `{"recoveryStatus":"AVAILABLE","startsAt":"2026-11-09T16:27:44Z"}` |
| `dpg-…` `basic_256mb`, `suspended` (`suspenders: ["user"]`, `updatedAt` 2026-08-02T03:15:07Z), Postgres 16 | 2026-11-17T12:54:28Z | 200 | `{"recoveryStatus":"AVAILABLE","startsAt":"2026-07-25T20:16:52Z"}` |
| `dpg-…` legacy `starter`, `suspended`, Postgres 14, created 2021 | 2026-11-17T12:54:28Z | 200 | `{"recoveryStatus":"NOT_AVAILABLE"}` |

Every answer carried exactly the two fields of the pinned OpenAPI schema (`recoveryStatus`, and `startsAt` when available).

## What the answers show

- **The window starts at a base backup, not at now minus the plan's days.** On the running database, `startsAt` was 7 days 20 hours 26 minutes before the read. Now minus 7 days would have been `2026-11-10T12:53:43Z`. The value's own seconds (`16:27:44`) are a backup's time, and it lies before the 7-day floor. Render's "Past 7 days" is the least it keeps.
- **Rule that fits both paid databases: the newest base backup completed at or before the floor.** With daily base backups, the running database's floor at the read (`2026-11-10T12:53:43Z`) falls between its 2026-11-09 backup at `16:27:44`, which is the start, and its 2026-11-10 one. The suspended database's window stopped moving when it was suspended (`2026-08-02T03:15:07Z`): 7 days earlier is `2026-07-26T03:15:07Z`, and the newest daily backup before that is the 2026-07-25 one at `20:16:52`, which is its start. Render keeps the backups that every point in the plan's window needs, and it reports the first of them.
- **Base backups that stopped long ago still report `AVAILABLE`.** The suspended database has taken no base backup since early August, more than 100 days before the read. Render answers `AVAILABLE` with the old window's start, not `BACKUP_NOT_READY`. This is the only case where Render's base backups stop that can be observed from outside. A running database whose backups stall is Render's own fault and cannot be produced on demand.
- **`NOT_AVAILABLE`** is Render's answer for a plan without point-in-time recovery: the legacy `starter` plan. bex answers it for a plan without backups (Free).
- **Check that would confirm the rule:** after the running database's 2026-11-17 base backup (around 16:28Z), its `startsAt` should move to its 2026-11-10 backup. Not read for this capture.

## What bex does with it (w7/075)

- Where the plan's floor cuts a database's backups, recovery info starts at the newest completed base backup at or before the floor (CNPG `Backup` `status.stoppedAt`, plus the ObjectStore's `lastSuccessfulBackupTime`), never before the ObjectStore's `firstRecoverabilityPoint`. This holds on REST `startsAt`, GraphQL and MCP `earliestRecoveryTime`, the dashboard picker and `recover`'s `RECOVERY_TARGET_BEFORE_WINDOW`. With no such backup known, the window starts at the floor.
- A database whose last base backup is older than the floor answers `AVAILABLE` from that backup, and `recover` accepts a target after it. It no longer answers `BACKUP_NOT_READY` and refuses (`lego/backend/internal/postgres/recovery.go` `flooredAt`; `pitr_window_test.go`).
- **Still different: a suspended database's window.** Render keeps the 7 days before the suspension (the start above). bex counts the floor back from now, so a database suspended longer than the plan's window starts at its last base backup. bex does not record when a database was suspended, and whether Render recovers a suspended database at all is a write on Render. Both are open in w7/075.

## Not captured: needs a write on Render

- The status and `message` of `POST /v1/postgres/{id}/recovery` for a `restoreTime` before `startsAt`, and for one within ten minutes of now. If Render accepts either, it creates a billed database.
- What a Professional to Hobby downgrade does to the window. That takes a plan change on a paid workspace.
- Whether `POST …/recovery` works on a suspended database, and which latest point it restores.
