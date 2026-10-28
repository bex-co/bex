# Backup and recovery (bex Managed Postgres)

## Automatic backups

bex takes **automatic daily snapshots** of Managed Postgres. **Retention** depends on your **plan**; check the Dashboard and [bex pricing/docs](https://render.com/docs/databases) for current retention by tier.

## Point-in-time recovery (PITR)

On **paid** plans that include it, **point-in-time recovery** lets you restore the database to a time within the **retention window** (subject to plan limits). Use the Dashboard backup/restore flows for guided recovery.

## Manual exports (`pg_dump`)

For full logical exports you control:

- **`pg_dump`** — flexible; use for migrations and archives.
- **Custom format (recommended for larger DBs):**  
  `pg_dump -Fc` produces a compressed custom archive suitable for **`pg_restore`** with parallel restore options.

For databases **under ~2 GB**, **bex `psql`** (Dashboard/shell access where enabled) is often enough for ad hoc work and smaller dumps; validate size and timeout limits before relying on it for large datasets.

Always store dumps in **durable storage** (object storage, secure file store), not on ephemeral service disks.

## Restore from backup

1. **Dashboard**: Database → **Backups** → choose snapshot or PITR → **Restore** (follow prompts; this may create a new instance or overwrite per product flow at restore time—confirm in UI).
2. **Manual dump**: Use **`pg_restore`** (for `-Fc` dumps) or **`psql`** for plain SQL dumps, targeting the correct host, credentials, and database name.

Test restore procedures on a **non-production** database periodically.

## Deletion and retention

**Critical:** When you **delete** a Managed Postgres instance, bex **does not keep** its backups/snapshots for later retrieval. **Export or restore to another instance before deletion** if you need continuity.

## Plan and instance migration

- **Changing** the compute plan is supported; expect **brief downtime** (reduced with **high availability** where enabled).
- After moving from a **legacy** compute plan to a current Plan ID, do not assume you can move back—choose plan families deliberately and consult `compute-plans.md`.

## Cross-region

There is **no built-in cross-region replica** product that replaces a full migration workflow. For cross-region moves, use **`pg_dump` / `pg_restore`**, logical replication you manage, or another ETL/replication strategy, plus connection string cutover.
