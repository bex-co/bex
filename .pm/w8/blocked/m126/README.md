# w8 · m126 — Free Postgres expires after 30 days and reports `expiresAt`, as on Render

**Worker:** worker8 **Goal:** A free managed Postgres reports Render's `expiresAt` on every surface, warns its owner before it expires, stops serving at expiry, and is deleted after Render's grace period unless upgraded. The ADR009 30-day TTL becomes real. **Status:** blocked — t001–t008 done 2026-10-20; t009 live check on production after the release pipeline deploys bex-api and the dashboard

## Tasks (in order)

| id   | title                                                                         | est | depends_on                 |
| ---- | ----------------------------------------------------------------------------- | --- | -------------------------- |
| t001 — **DONE** | Contract: pin Render's free-Postgres expiry, grace and upgrade semantics       | 30m | —                          |
| t002 — **DONE** | Store + API: `expiresAt` on free databases (REST, GraphQL, MCP)              | 60m | w8/m126/t001               |
| t003 — **DONE** | Control plane: expiry sweeper (warn, suspend at expiry, delete after grace)   | 90m | w8/m126/t002               |
| t004 — **DONE** | Upgrade clears the expiry; existing free databases get a dated rollout window | 45m | w8/m126/t003               |
| t005 — **DONE** | Dashboard: expiry date, warning banner and upgrade action on the database page | 45m | w8/m126/t002               |
| t006 — **DONE** | Render parity: expiry fields and lifecycle across REST, GraphQL, MCP, UI       | 20m | w8/m126/t004, w8/m126/t005 |
| t007 — **DONE** | Simplify the changed code                                                      | 15m | w8/m126/t006               |
| t008 — **DONE** | Test coverage: dates, sweeper transitions, upgrade, rollout window           | 45m | w8/m126/t006               |
| t009 | Close out the milestone                                                        | 30m | w8/m126/t007, w8/m126/t008 |

## Definition of done

Live on production after the release pipeline deploys bex-api and the dashboard:

- `bex pg create --plan free` → `GET /v1/postgres/{id}` returns `expiresAt` = creation + 30 days. GraphQL, MCP `get_postgres`/`list_postgres` and the dashboard database page show the same instant. A paid database has no `expiresAt`.
- Upgrading a free database to a paid plan removes `expiresAt` on every surface.
- On a test clock (or a database backdated in a dev environment), the sweeper sends the warning, makes the database unavailable at `expiresAt`, and deletes it after the grace period with audit/events. Each step happens exactly once across bex-api replicas.
- Free databases that existed before this milestone get an announced expiry no earlier than the rollout date + 30 days, never an immediate deletion.

## Source + Goal linkage

- **Source:** inbox note `w8/145` (`/qa-find-bugs-cli` sweep 1247, 2026-10-20), [kept in done/145.md](../../done/145.md). User decision 2026-10-20: implement Render's expiry (option A), not record a divergence.
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md) Render alternative; [ADR009](../../../../docs/ADR009-postgresql-management.md) plan table ("30-day TTL … else 'free forever' eats the cluster"); [ADR018](../../../../docs/ADR018-render-parity.md) Postgres rows. Render's API carries `expiresAt` on Postgres and PostgresDetail (pinned CLI `pkg/client/types_gen.go:2624–2625`, `2674–2675`).
- **Expected outcome:** free databases stop accumulating forever on the cluster, users see the same lifecycle as on Render, and the ADR stops promising something the code does not do.
- **Why now:** the user decided it on 2026-10-20. Every free database created in the meantime adds to the rollout window t004 has to announce.
- **Render parity included:** the change adds a field and a lifecycle to REST, GraphQL, MCP and the dashboard.

## Notes for t001

- Read render.com/docs/free (Postgres section) and the pinned spec's `expiresAt` description, and record the exact rules: the 30-day clock, the grace period before deletion, what "expired" serves, whether a suspended or upgraded database resets the clock, and what notifications are sent. Write them into ADR009 before t002.
- Deletion must go through the existing Postgres delete path (backups and audit as for a user delete), never a raw CR delete.

## Progress (2026-10-20)

- t001: Render's rules pinned in [ADR009 §Free-plan expiry](../../../../docs/ADR009-postgresql-management.md#free-plan-expiry-w8m126) from render.com/docs/free and the pinned spec.
- t002: `expiresAt` derives from the Database CR (creation, or `app.bex.co/free-since` after a change to free, clamped to the 2026-10-27 rollout, + 30 days). No store migration: the CR is the only state. REST, GraphQL `database.expiresAt` and MCP agree (`TestExpiresAtParityAcrossSurfaces`).
- t003: `postgres.FreeExpirySweeper` (5-minute `core.Poll`, started in `cmd/api/main.go`) claims each stage with an optimistic-concurrency `Update` (`app.bex.co/free-expiry: <stage>@<expiresAt>`), suspends at expiry, deletes after 14 days through `deleteDatabase` (usage retirement first, delete preconditioned on UID + resourceVersion), writes `postgres.FreeExpiry*` audit rows and mails owners through `notifications.DatabaseExpiryNotifier`.
- t004: every plan writer (`setPlan`, PATCH via `patchDatabaseObj`, Blueprint `applyDatabase`) runs `SyncFreeClock`: paid clears the clock and resumes an expiry suspension; paid → free starts a new clock. Resume of an expired database is refused (`POSTGRES_FREE_EXPIRED`). The rollout clamp keeps every pre-existing database until at least 2026-12-10.
- t005: Details card Expires row with a 7-day/expired warning linking to `#plan`, the expiry reason beside the status, and a list hint in the resource table; Vitest covers free/paid/soon/expired. The Details skeleton keeps modelling only the always-present rows plus Region, as it already does for the optional external-host row, so a free database's ready card is one row taller than its skeleton. The side-by-side visual check was not run: this machine has no cluster tooling.
- t006: ADR018 Postgres CRUD row updated.
- t007: `/simplify` applied (shared owner-mailing helper with billing's no-address and permanent-rejection handling, named reason-code constants, one tone helper in the dashboard).
- t008: `expiry_test.go` (dates, stage boundaries, every stage once, collapsed overdue stages, two-replica claims, upgrade and user-suspension, downgrade, rollout window), `expiry_cross_surface_test.go`, `notifications/database_expiry_test.go`, dashboard `expiry.test.ts` and `database-expiry.test.tsx`. Each fails on the pre-milestone code (the field, sweeper and notifier did not exist).

