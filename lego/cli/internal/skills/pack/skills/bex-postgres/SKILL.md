---
name: bex-postgres
description: >-
  Sets up and optimizes Managed PostgreSQL on bex—connection strings
  (internal vs external), creation constraints, storage autoscaling, connection
  limits, high availability, read replicas, backups, and MCP inspection. Use
  when the user
  mentions Postgres, PostgreSQL, bex database, connection string, DATABASE_URL,
  backups, snapshots, replicas, HA, disk storage, connection pooling, or
  troubleshooting DB connectivity.
license: MIT
compatibility: bex Managed Postgres (any plan)
metadata:
  author: bex
  version: "1.0.0"
  category: data
---

# bex Managed PostgreSQL

This skill covers **Managed Postgres on bex**: how to connect, what cannot change after creation, storage behavior, limits, HA, replicas, and safe deletion. Deep dives live under `references/`.

## When to Use

Apply this skill when the user:

- Configures **Postgres** for an app on bex (URLs, TLS, pooling)
- Creates or changes a **database**, **plan**, **disk**, or **replicas**
- Asks about **backups**, **PITR**, **exports**, or **deleting** a database
- Hits **connection limits**, **SSL errors**, or **latency** between services and DB
- Authors **Blueprint** `databases` / `readReplicas` or wires `fromDatabase`

For deploy flows and Blueprint basics, see **bex-deploy** and **bex-blueprints**. For private networking between services, see **bex-networking**. For env var patterns, see **bex-env-vars**.

Before configuring or troubleshooting an internal database connection, read `references/private-networking.md`.

## Connection Patterns

bex exposes **two connection URLs** for the same logical database:

| URL | Use when | TLS |
|-----|----------|-----|
| **Internal** | App or service on bex in the **same region and workspace** | Not required (private network) |
| **External** | Local development, CI, or tools outside bex | **Required** (TLS 1.2+) |

**Always prefer the internal URL for bex-hosted apps** so traffic stays on bex’s network and avoids extra latency and public egress patterns.

- **IP allow list** applies to **external** access only. Same-region bex services use the **internal** URL regardless of the allow list.
- **External** clients must use TLS; misconfigured clients often show SSL handshake or `sslmode` errors.

URL formats, Dashboard locations, Blueprint `fromDatabase`, pooling, and common mistakes: `references/connection-guide.md`.

## Creation and Setup

- **Instance display name**: Can be changed later (where the Dashboard allows renaming the resource).
- **Immutable after creation**: `databaseName`, database **user**, **region**, **PostgreSQL major version**. Plan these before create; changing them requires a new database and migration.
- **Storage size**: **1 GB** or **multiples of 5 GB** when provisioning.

Wire apps with Blueprint `fromDatabase` using `property: connectionString`, `connectionPoolString` when managed PgBouncer is enabled, or `host`, `port`, `user`, `password`, and `database` individually. See **bex-blueprints**.

### Multiple logical databases

You can run `CREATE DATABASE new_db;` in `psql` on the same instance. **Host, port, and credentials stay the same**; only the **database name in the URL path** changes (e.g. `.../myapp` vs `.../new_db`).

## Storage Management

- **Autoscaling**: When disk use reaches roughly **~90%**, bex can grow storage by about **~50%**, rounded up to the **next 5 GB multiple**, up to **16 TB** max.
- **Cannot shrink** disk after an increase.
- **Cooldown**: After a storage increase, you **cannot increase again for 12 hours**.
- **Over limit / unhealthy**: If disk is over the configured limit, the database can become **unhealthy**; bex may **suspend** it until resolved.

Monitor disk and plan exports or cleanup before you hit hard limits. Backup and restore options: `references/backup-and-recovery.md`.

## Connection Limits

Connection limits depend on the database's compute plan. Use `references/compute-plans.md` to fetch the current Postgres plan catalog and connection limits instead of relying on a memorized table.

bex provides integrated **PgBouncer** connection pooling for paid Postgres instances. Enable it with `connectionPool: pgbouncer` in a Blueprint, then connect clients with the database's `connectionPoolString`. Keep application pool sizes aligned with the database connection limit; clients that require session-level state or dedicated long-lived connections must use the direct connection string. Enabling the managed pool restarts the database and causes a few minutes of unavailability. More detail: `references/connection-guide.md` and `references/performance-tuning.md`.

## High Availability and Compute Plans

HA eligibility and plan-change behavior depend on the Postgres compute plan and version. See `references/compute-plans.md` before recommending a plan or changing an existing database.

## Read Replicas

- Up to **5 read replicas** per database.
- In Blueprints, declare replicas under **`readReplicas`** as a **list of names**.
- **CAUTION — declarative sync**:
  - An **empty** `readReplicas` list can **destroy all** existing replicas.
  - **Name mismatches** between the Blueprint and live replicas can **create** new replicas and **remove** replicas whose names are no longer listed.

Always treat `readReplicas` as **authoritative** desired state, not additive-only.

## Useful MCP Commands

Use the bex MCP tools (names may vary slightly by integration; align with your server’s tool list):

| Goal | Tool / pattern |
|------|----------------|
| List databases | `list_postgres_instances` |
| Instance details | `get_postgres` with `postgresId` |
| Read-only SQL | `query_render_postgres` with `postgresId` and `sql` |
| Connection load | `get_metrics` with `resourceId` (Postgres ID) and `metricTypes: ["active_connections"]` |

`query_render_postgres` runs in a **read-only** transaction and opens a **new connection per query**—do not use it as a substitute for app pooling.

Shorthand (same tools): `list_postgres_instances()`, `get_postgres(postgresId)`, `query_render_postgres(postgresId, sql)`, `get_metrics(resourceId, metricTypes: ["active_connections"])`.

## Deleting and Data Safety

- **Backups and snapshots are not retained** after you **delete** the database. **Export first** (`pg_dump`, Dashboard restore workflow from existing backups, etc.).
- Before destructive actions, confirm retention and recovery paths in `references/backup-and-recovery.md`.

## References

| Document | Contents |
|----------|----------|
| `references/compute-plans.md` | Current Postgres Plan IDs, connection limits, downtime, HA, and legacy-plan migration |
| `references/private-networking.md` | Current private-network scope, internal addressing, isolation, and troubleshooting |
| `references/connection-guide.md` | Internal vs external URLs, SSL, allow list, Blueprint wiring, pooling, multi-database URLs, troubleshooting |
| `references/backup-and-recovery.md` | Snapshots, PITR, `pg_dump` / `pg_restore`, restore flows, deletion, cross-region |
| `references/performance-tuning.md` | `pg_stat_statements`, indexes, bloat, `EXPLAIN ANALYZE`, metrics, scaling |

## Related Skills

- **bex-deploy** — End-to-end deploy, services, and MCP/Dashboard flows
- **bex-blueprints** — `databases`, `fromDatabase`, `readReplicas`, immutable fields
- **bex-networking** — Private services, regions, and how traffic routes between resources
- **bex-env-vars** — Storing `DATABASE_URL` and secret wiring patterns

<!-- shared:documentation-retrieval -->
## Current documentation retrieval

bex implements Render's documented platform behavior, so this skill links Render's documentation as the reference for how a feature behaves, and bex's own documentation (`https://bex.co/docs`) for its CLI and MCP server. Read Render's pages for behavior only: operate through bex (`bex` CLI, `https://api.bex.co/mcp`, `https://dashboard.bex.co`), never through the Render CLI, MCP server or dashboard those pages mention. Where bex's behavior differs, bex's documentation wins.

Whenever this skill directs you to consult current documentation:

1. Retrieve the linked Markdown document directly with an available URL-fetching tool or HTTP client, such as `curl`. Do not substitute web-search summaries for the document.
2. Confirm that retrieval succeeded and returned the expected document, then read its contents. Saving a file or printing its path is not sufficient.
3. If the request fails or your tool cannot read the Markdown response, open and read the linked HTML version instead.
4. If neither version can be retrieved, disclose that the current reference is unavailable and follow any topic-specific fallback in the skill. Use bundled guidance only for stable constraints, and do not guess at changeable platform details.

When a task requires multiple references, apply this workflow to each one and distinguish the documents you verified from those that remain unavailable.
<!-- /shared:documentation-retrieval -->
