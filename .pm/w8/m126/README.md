# w8 · m126 — Free Postgres expires after 30 days and reports `expiresAt`, as on Render

**Worker:** worker8 **Goal:** A free managed Postgres reports Render's `expiresAt` on every surface, warns its owner before it expires, stops serving at expiry, and is deleted after Render's grace period unless upgraded. The ADR009 30-day TTL becomes real. **Status:** todo

## Tasks (in order)

| id   | title                                                                         | est | depends_on                 |
| ---- | ----------------------------------------------------------------------------- | --- | -------------------------- |
| t001 | Contract: pin Render's free-Postgres expiry, grace and upgrade semantics       | 30m | —                          |
| t002 | Store + API: `expiresAt` on free databases (REST, GraphQL, MCP)              | 60m | w8/m126/t001               |
| t003 | Control plane: expiry sweeper (warn, suspend at expiry, delete after grace)   | 90m | w8/m126/t002               |
| t004 | Upgrade clears the expiry; existing free databases get a dated rollout window | 45m | w8/m126/t003               |
| t005 | Dashboard: expiry date, warning banner and upgrade action on the database page | 45m | w8/m126/t002               |
| t006 | Render parity: expiry fields and lifecycle across REST, GraphQL, MCP, UI       | 20m | w8/m126/t004, w8/m126/t005 |
| t007 | Simplify the changed code                                                      | 15m | w8/m126/t006               |
| t008 | Test coverage: dates, sweeper transitions, upgrade, rollout window           | 45m | w8/m126/t006               |
| t009 | Close out the milestone                                                        | 30m | w8/m126/t007, w8/m126/t008 |

## Definition of done

Live on production after the release pipeline deploys bex-api and the dashboard:

- `bex pg create --plan free` → `GET /v1/postgres/{id}` returns `expiresAt` = creation + 30 days. GraphQL, MCP `get_postgres`/`list_postgres` and the dashboard database page show the same instant. A paid database has no `expiresAt`.
- Upgrading a free database to a paid plan removes `expiresAt` on every surface.
- On a test clock (or a database backdated in a dev environment), the sweeper sends the warning, makes the database unavailable at `expiresAt`, and deletes it after the grace period with audit/events. Each step happens exactly once across bex-api replicas.
- Free databases that existed before this milestone get an announced expiry no earlier than the rollout date + 30 days, never an immediate deletion.

## Source + Goal linkage

- **Source:** inbox note `w8/145` (`/qa-find-bugs-cli` sweep 1247, 2026-10-20), [kept in done/145.md](../done/145.md). User decision 2026-10-20: implement Render's expiry (option A), not record a divergence.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) Render alternative; [ADR009](../../../docs/ADR009-postgresql-management.md) plan table ("30-day TTL … else 'free forever' eats the cluster"); [ADR018](../../../docs/ADR018-render-parity.md) Postgres rows. Render's API carries `expiresAt` on Postgres and PostgresDetail (pinned CLI `pkg/client/types_gen.go:2624–2625`, `2674–2675`).
- **Expected outcome:** free databases stop accumulating forever on the cluster, users see the same lifecycle as on Render, and the ADR stops promising something the code does not do.
- **Why now:** the user decided it on 2026-10-20. Every free database created in the meantime adds to the rollout window t004 has to announce.
- **Render parity included:** the change adds a field and a lifecycle to REST, GraphQL, MCP and the dashboard.

## Notes for t001

- Read render.com/docs/free (Postgres section) and the pinned spec's `expiresAt` description, and record the exact rules: the 30-day clock, the grace period before deletion, what "expired" serves, whether a suspended or upgraded database resets the clock, and what notifications are sent. Write them into ADR009 before t002.
- Deletion must go through the existing Postgres delete path (backups and audit as for a user delete), never a raw CR delete.
