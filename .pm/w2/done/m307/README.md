# w2 · m307 — Postgres: turn on connection pooling and grow storage from the dashboard, as on Render

**Worker:** worker2 **Goal:** a dashboard user can enable or disable PgBouncer connection pooling and grow a database's storage, as on Render. Both are GraphQL writes with the same semantics and errors as REST and MCP, and the parity ledger stops claiming a dashboard pooler that does not exist. **Status:** done

## Tasks (in order)

| id   | title | est | depends_on |
| ---- | ----- | --- | ---------- |
| t001 | GraphQL `updateDatabaseConnectionPool` with REST/MCP's semantics and errors — **DONE** | 45m | — |
| t002 | GraphQL `updateDatabaseDiskSize` sharing REST's grow-only guard — **DONE** | 45m | — |
| t003 | Dashboard connection-pool toggle with Render's confirmation, replacing w4/107's "use the API" copy — **DONE** | 60m | t001 |
| t004 | Dashboard storage edit (grow-only) with a converging state — **DONE** | 60m | t002 |
| t005 | Amend ADR018 rows 177 and 190 — **DONE** | 15m | t003, t004 |
| t006 | Render parity — **DONE** | 20m | t005 |
| t007 | Simplify — **DONE** | 15m | t006 |
| t008 | Test coverage — **DONE** | 30m | t007 |
| t009 | Closeout — **DONE** | 10m | t008 |

## Definition of done

- On dev-2, on a paid database:
  - The dashboard enables connection pooling after a confirmation that names the restart, or the failover when HA is on.
  - The internal and external pooled strings (port 6432) then appear.
  - Disabling works the same way.
- On a free database, the control is unavailable and shows Render's reason.
- On dev-2, the dashboard grows storage. A smaller value is refused with the same named 400 that REST returns. The new size shows at once while CNPG converges.
- GraphQL `updateDatabaseConnectionPool` and `updateDatabaseDiskSize` return the same results and errors as REST `PATCH /postgres/{id}` and MCP `update_postgres` for the same inputs.
- `docs/ADR018-render-parity.md` rows 177 and 190 describe the shipped state.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2`, 2026-11-10, item 3. Evidence:
  - Render, connection pooling ([docs](https://render.com/docs/postgresql-connection-pooling)):
    - The control is on the database's **Info** page, under **General > Connection Pool**: a toggle, then a confirmation modal, then **Enable connection pool**.
    - Enabling restarts the database (a few minutes of downtime). With HA it fails over instead.
    - The internal and external pooled URLs use port 6432, and the pool runs in transaction mode.
    - It is not available for free databases.
  - Render, storage ([docs](https://render.com/docs/postgresql-creating-connecting)): "From your database's **Plan** page … scroll down to the **Storage** section and click **Edit**."
  - bex REST already serves both writes: `connectionPool` (`lego/backend/internal/postgres/rest.go:478`) and `diskSizeGB` (`service.go:1287`, w2/m60's grow-only guard).
  - bex MCP `update_postgres` serves both too (`mcp.go:329-331`).
  - bex GraphQL accepts `connectionPool` and `diskSizeGB` only at create (`graphql.go:547,556`). The dashboard's update mutations are per field (`dashboard/src/features/databases/api/databases.graphql:336-353`: `UpdateDatabasePlan`, `UpdateDatabaseVersion`, `UpdateDatabaseDiskAutoscaling`), and none covers the pool or the disk.
  - The dashboard's pooler section (`dashboard/src/features/databases/components/access-control-panel.tsx:197`) shows `databases.accessPoolerDisabled` (`locales/en.ts:983`). Since w4/107 that copy says the pooler can be enabled only through the API.
  - `w4/done/107.md:48,66`: w4/107 shipped option (b), the honest copy, and recommended option (a), the toggle, "as a separate milestone". It was never filed.
  - The ledger is wrong today. ADR018:190 marks the pooler UI ✅. ADR018:177 records w2/m60 declining a GraphQL/MCP disk write "that Render itself does not expose", but Render's dashboard does expose it and bex's dashboard is a GraphQL client.
- **Goal linkage:** Render parity first (ADR008 § Render parity first, AGENTS.md rule 1), V0 roadmap item 4 (PostgreSQL), and the ADR018 rows above.
- **Expected outcome:** a paying Postgres customer who hits the connection limit, or runs low on storage, fixes it from the dashboard as on Render, instead of reaching a dead end that points at the API.
- **Why now:** the parity ledger claims a dashboard capability that does not exist. w2/m171, m245 and m247 just made paid and HA Postgres the plans customers buy, and pooling is Render's documented answer when a database approaches its connection limit. The dead-end copy is live on every paid database page.
- **Decision state:** no open divergence. Amending ADR018:177's w2/m60 rationale follows AGENTS.md ("when an ADR … conflicts with parity, amend that ADR"); it needs no separate sign-off.
- **Render parity included:** GraphQL and dashboard change; REST and MCP must be re-checked for identical semantics.

## dev-2 walk — 2026-11-10 (`/loopx w2`): passed

- **Setup:** bex-api rebuilt from the tree (`dev-env.sh 2 up`). Throwaway identity `m307-walk-1794314313@example.com`, email verified through Mailpit. Paid `basic-256mb` database `dpg-dbph0qpjg4rb8daumoo0` in `tea-dbph0o9jg4rb8daumong`.
  - Its PVC first sat Pending: dev-N had no `hcloud-volumes` class. Fixed as [w2/081](../done/081.md), after which it went `available`.
- **GraphQL:**
  - `updateDatabaseConnectionPool(pgbouncer)` returned `poolerEnabled: true`, `status: config_restart` (the restart, as on Render). The CNPG `Pooler` went `active` (1/1), and `connection-info` carried the pooled URI on the `-pooler` host.
  - `updateDatabaseDiskSize(10)` returned 10. Spec, allocated size and the CNPG `storage.size` all read 10.
  - A shrink to 5 was refused with the identical message on GraphQL and REST: "Postgres storage is grow-only: requested 5 GB is below the allocated 10 GB".
- **Dashboard** (`yarn dev` against dev-2, Playwright):
  - The switch showed on, and Reveal listed the internal and external pooled strings.
  - Switching off showed "Disable connection pooling for m307-walk?" (restart, pooled strings stop working). Confirming removed the `Pooler` and `spec.pooler`, and the panel then said "Turn on connection pooling above…".
  - Switching on showed "Enable connection pooling for m307-walk?" with **Enable connection pool**, and the `Pooler` came back `active`.
  - **Edit storage** opened prefilled at 15 ("any multiple of 5 GB greater than the current 10 GB"). 12 was refused client-side ("Enter a whole multiple of 5 GB."), with Save disabled. Saving 15 showed 15 GB in the metadata card, the disk header and the Storage row, with "Increasing storage to 15 GB…". Spec and the CNPG `storage.size` read 15.
- **Local limitation:** the local-path alias cannot expand volumes, so the PVC stayed 1Gi. Production's `hcloud-volumes` expands.
- **Not walked live:** the free-plan refusals for pool and storage, and the HA failover copy. Both are covered by the REST/GraphQL/MCP and dashboard tests.
- **Cleanup:** database deleted (204), dashboard dev server stopped.
