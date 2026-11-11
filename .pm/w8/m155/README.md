# w8 · m155 — A Postgres user can create more databases in its instance, as on Render

**Worker:** worker8 **Goal:** the default user of a bex Postgres instance can run `CREATE DATABASE`, and connect to the new database through the instance's own URLs with the database name substituted, as Render documents; every credential that becomes the default keeps that ability. **Status:** todo

## Tasks (in order)

| id   | title                                                                         | est | depends_on |
| ---- | ----------------------------------------------------------------------------- | --- | ---------- |
| t001 | Decide the default user's role attributes, record them in ADR009 and ADR018   | 30m | —          |
| t002 | The owner and every default credential can create databases                   | 45m | t001       |
| t003 | Blast radius: existing and restored databases, retired users, exports, limits | 40m | t002       |
| t004 | Render parity                                                                 | 20m | t003       |
| t005 | Simplify                                                                      | 15m | t004       |
| t006 | Test coverage                                                                 | 30m | t004       |
| t007 | Closeout (live replay on production)                                          | 20m | t006       |

## Definition of done

1. On production, with the released `bex` CLI and a human login:

   ```text
   bex pg create --name <n> --plan free -o json --confirm
   bex psql <n> -c "CREATE DATABASE second_db" -o text
   bex psql <n> -c "select datname from pg_database where datname = 'second_db'" -o text
   ```

   The second command prints `CREATE DATABASE`; the third prints one row.

2. With an allow-list entry for the caller, the instance's external URL with `second_db` as its final component connects and answers a query (Render: "Use your instance's same internal and external URLs to connect, except substitute your new database's name as the final component").
3. A database created before this change gains the ability without being recreated, and its running connections are not dropped by the change.
4. A credential added afterwards (the new default user) can also `CREATE DATABASE`.
5. The user is still not a superuser (`rolsuper = f`), and `CREATE USER` behaves as t001 decided.
6. Fixtures deleted afterwards.

## Finding (2026-11-11 UTC, `/qa-find-bugs-cli` w8 loop sweep `833c`)

- **Severity:** major — a documented Render Postgres capability fails outright, and nothing on bex offers another way to get a second database in an instance. Tools that create a database as part of their normal run (a framework's `db:create` for a second database, a migration tool's scratch database) stop at the same error.
- **Versions:** released `bex v0.5.0` (Render CLI v2.35.0, `24ac2d60fdb2`); repo HEAD `261e2493b`; production platform pin `2f1c62c93f49` (the probes ran 07:43–07:45 UTC, before deploy run 42982188589 started).
- **Context:** `https://api.bex.co/v1/`, workspace `bex-canary` (Scale), QA human device login, non-TTY. A free PostgreSQL 18 instance inside a project environment, `status: available`.
- **Repro:**

  ```text
  $ bex pg create --name qa-20261110-833c-pg --plan free --environment production --project qa-20261110-833c-proj -o json --confirm
  → dpg-dbq1jfip92ls73cjjkc0, available

  $ bex psql qa-20261110-833c-pg -c "create database qa2_833c" -o text                  → exit 1
  Error: exit status 1: ERROR:  permission denied to create database

  $ bex psql qa-20261110-833c-pg -c "create user qa_u_833c with password '…'" -o text   → exit 1
  Error: exit status 1: ERROR:  permission denied to create role
  DETAIL:  Only roles with the CREATEROLE attribute may create roles.

  $ bex psql qa-20261110-833c-pg -c "select rolname, rolsuper, rolcreatedb, rolcreaterole, rolcanlogin from pg_roles where rolname !~ '^pg_' order by 1" -o text
   cnpg_metrics_exporter | f | f | f | t
   dpg_dbq1jf…_user      | f | f | f | t        ← the connecting (default) user
   postgres              | t | t | t | t
   streaming_replica     | f | f | f | t
  ```

  Controls in the same session, all succeeding: `CREATE TABLE`/`INSERT`/`SELECT`, `CREATE SCHEMA`, `CREATE EXTENSION pgcrypto`, `psql` passthrough flags, a query over TLS.

- **Expected (Render):** render.com/docs/postgresql-creating-connecting § "Adding multiple databases to a single instance" (read 2026-11-11): "You can create additional databases in your Render Postgres instance with the following steps:" … "Run `CREATE DATABASE <name>`, providing the name for your new database." … "Use your instance's same internal and external URLs to connect, except substitute your new database's name as the final component". The page states no plan restriction.
- **Actual:** `ERROR: permission denied to create database`, on every plan (the role projection takes no plan input).
- **Wire evidence:** not a REST failure. The pinned client reads `GET /v1/postgres/{id}/connection-info`, starts `psql` with the external connection string, and PostgreSQL itself refuses the statement.
- **Attribution:** infrastructure/runtime (the operator's CNPG role projection). The CLI and bex-api behave as upstream expects.
- **Root cause:** `managedRoles` (`lego/operator/internal/controller/database_controller.go:385-458`) projects the owner onto CNPG `spec.managed.roles` as `{name, ensure: present, login: true, inRoles: [pg_monitor]}` (`:405-410`) and each added credential as `{name, ensure: present, login: true[, passwordSecret][, inRoles]}` (`:436-443`). Neither entry sets `createdb`, and a managed role's omitted attribute is false, so CNPG keeps both `NOCREATEDB` (and would revert a manual `ALTER ROLE`). `bootstrap.initdb` (`:583-591`) creates the owner as a plain login role that owns the one database.
- **Fix:** set `createdb: true` on the owner entry and on every credential with `OwnerPrivileges` (the Render-style defaults; role attributes are not inherited through `inRoles`, so membership in the owner does not carry it). The projection is declarative, so existing databases converge at their next reconcile. t001 decides `createrole` (see Unverified) and what a retired (tombstoned) user keeps.
- **Blast radius:**
  - Every Database on every plan, new, existing and restored (the recovery bootstrap path uses the same `managedRoles`).
  - `spec_projection.go:83-90` merges CNPG-defaulted fields inside `roles` entries; check that an added key replaces a live entry's value and causes no reconcile loop.
  - Native auxiliary users (`POST …/users`, `OwnerPrivileges: false`) are "granted nothing" by design (`lego/types/v1alpha1/database_types.go:309-314`) and stay as they are.
  - What a second database is not covered by: a logical export dumps only the primary database (`database_exports.go:89`, `PGDATABASE` from the connection Secret, `:389`); REST, GraphQL, MCP and the dashboard show one `databaseName`; `pg_stat_statements` is created only in the primary database (`:589`). Physical backups, PITR, storage metering and suspend/resume are cluster-wide and need no change. t003 records each, and files what Render does differently.
  - Tenant isolation is unaffected: each instance is its own CNPG cluster, and `CREATEDB` grants nothing outside it.
- **Regression checks:** Definition of done 1–5; an operator unit test on `managedRoles` for the owner, a default credential, an auxiliary user and a retired user.
- **Unverified:**
  - Render's behavior was read from its documentation, not run on a Render account.
  - Whether Render's default user also has `CREATEROLE`. Its credentials page says "Users created directly with the `CREATE USER` command are _not_ managed by Render" (render.com/docs/postgresql-credentials), which implies the statement works, but no page lists the user's attributes. bex refuses it (above).
  - Paid plans and a credential added through `POST /postgres/{id}/credentials` were not exercised; the cause is read from the projection code.
- **Dedupe:** `rg -i "create database|createdb|multiple databases"` over `.pm` and `docs` finds no item or decision about a tenant creating databases. ADR009 does not mention it. `w4/m179` (done) recorded in passing that both roles have "no SUPERUSER, CREATEDB or CREATEROLE" while fixing user deletion, without comparing to Render.
- **Cleanup:** the fixture database, its project and the sweep's other fixtures are deleted with the sweep.
