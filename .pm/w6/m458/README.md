# w6 · m458 — Managed PgBouncer runs with Render's settings, and read replicas are pooled too

**Worker:** worker6 **Goal:** a database with connection pooling accepts up to 30,000 clients on its pooled URL, keeps at most `max_connections` − 10 server connections, closes a client idle for a day and retries a failed server login after 2 s, as Render's managed PgBouncer does, and its read replicas are pooled as Render pools each replica. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Record what the pooler runs today](t001.md) | 30m | — |
| t002 | [`poolerSpec` sets Render's PgBouncer settings](t002.md) | 1h | t001, w6/134 |
| t003 | [The pooler pod can hold 30,000 clients](t003.md) | 45m | t002 |
| t004 | [Read replicas are pooled, as Render pools each replica](t004.md) | 1h30m | t002 |
| t005 | [Verify 1,000 pooled clients on production; ADR009 and ADR018](t005.md) | 45m | t003, t004 |
| t006 | [Render parity](t006.md) | 30m | t005 |
| t007 | [Simplify](t007.md) | 15m | t006 |
| t008 | [Test coverage](t008.md) | 30m | t006 |
| t009 | [Closeout](t009.md) | 10m | t007, t008 |

## Definition of done

- On production, a `basic-1gb` probe database with pooling accepts 1,000 concurrent clients on its pooled URL while `SHOW POOLS` holds at most 90 server connections, and PgBouncer's `SHOW CONFIG` reads `max_client_conn` 30000, `default_pool_size` and `max_db_connections` 90, `client_idle_timeout` 86400 and `server_login_retry` 2.
- A plan change rewrites the pool size without dropping connected clients.
- A database with a read replica has pooled replica connections in the form t004 records from Render's contract, consistently on REST, GraphQL, MCP and the dashboard.
- ADR009 and ADR018 record the settings.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w6` 2026-11-17, item 4, filed by the user's `/pm them to w6`. Render: [Postgres connection pooling](https://render.com/docs/postgresql-connection-pooling), read 2026-11-17: transaction pooling; `max_client_conn` 30000; `max_db_connections` / `default_pool_size` "`max_connections` minus 10" ("Render Postgres reserves 10 connections for direct database connections and internal operations"); `client_idle_timeout` 86400; `server_login_retry` 2; port 6432; pooling is enabled for every read replica, each with its own pool.
- **bex today:** `poolerSpec` (`lego/operator/internal/controller/database_controller.go:694`) sets one PgBouncer instance with `type: rw` and `poolMode: transaction`, and no other setting, so the pooler runs CNPG's or PgBouncer's defaults (PgBouncer's are `max_client_conn` 100 and `default_pool_size` 20; t001 reads what actually runs). Replica endpoints have no pooler.
- **Goal linkage:** ADR008 § Render parity first.
- **Expected outcome:** pooling does what tenants turn it on for, many clients on few server connections, with Render's numbers.
- **Why now:** if t001 confirms the defaults, a pooled database refuses its 101st client where Render accepts 30,000, which is the case pooling exists for. w6/134 supplies the pool size's input. Free databases already cannot pool (w8/m50).
- **Render parity task:** included. t004 may add pooled replica strings to connection info on every surface.
