# Database connection guide (bex Managed Postgres)

Read [private-networking.md](private-networking.md) first when configuring or troubleshooting an internal connection.

## URL formats

- **Internal** (same region and workspace on bex; private network):

  `postgres://user:password@hostname/dbname`

  No SSL/TLS required for typical internal client settings.

- **External** (local dev, external tools, CI outside bex):

  `postgres://user:password@hostname/dbname?sslmode=require`

  TLS **1.2+** is required for external endpoints; see [bex database documentation](https://render.com/docs/databases) for current TLS and networking requirements.

## Finding URLs in the Dashboard

1. Open your bex dashboard.
2. Select the **database** instance.
3. Open the **Connect** tab.
4. Copy **Internal** or **External** connection strings as appropriate.

## Blueprint wiring

In `render.yaml`, expose credentials to services with `fromDatabase`:

- **`property: connectionString`** — single URL (ensure the app uses internal vs external appropriately at runtime if you override per environment), or
- **`property: connectionPoolString`** — the managed PgBouncer URL when integrated connection pooling is enabled, or
- Individual properties: **`host`**, **`port`**, **`user`**, **`password`**, **`database`**

See the **bex-blueprints** skill for full examples and immutable field rules.

## Connection pooling

bex provides integrated **PgBouncer** connection pooling for paid Postgres instances. It is not available for Free Postgres.

Enable it in a Blueprint and wire clients to its connection string:

```yaml
services:
  - type: web
    name: app
    envVars:
      - key: DATABASE_URL
        fromDatabase:
          name: db
          property: connectionPoolString

databases:
  - name: db
    plan: basic-256mb
    connectionPool: pgbouncer
```

Managed PgBouncer uses **transaction-level pooling**. Use the direct `connectionString` instead for clients that require session-level state or dedicated long-lived connections, including `LISTEN`/`NOTIFY`, temporary tables, and session-level advisory locks.

Enabling the managed pool restarts the database and causes a few minutes of unavailability. Application-side pools remain useful, but size them so total connections across all service instances stay within the database's enforced connection limit.

## Multiple logical databases

1. Connect with `psql` or your admin tool using the primary database credentials.
2. Run `CREATE DATABASE new_db;`
3. Point applications at the **same host and port** and **same user/password**, changing only the **path** (database name) in the URL:

   `postgres://user:password@hostname/new_db`

## SSL/TLS summary

| Path | TLS |
|------|-----|
| Internal (bex private network) | Not required |
| External | TLS 1.2+ required; use `sslmode=require` (or stricter) in clients |

## Common connection issues

| Symptom | Likely cause | Mitigation |
|---------|----------------|------------|
| Higher latency than expected | App on bex using **external** URL | Switch to **internal** URL for that service |
| SSL / certificate errors | External URL without TLS or wrong `sslmode` | Use `?sslmode=require` and a TLS-capable client |
| “Too many connections” / pool exhausted | Hitting **max_connections** | Reduce pool size per instance, add pooling, scale RAM/plan, or add **read replicas** for read-heavy load |
| Wrong database after `CREATE DATABASE` | URL still points at old DB name | Update connection string path to the new database name |
