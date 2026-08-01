# w7 · m68 — Tenant KeyValue off-cluster backups: nightly RDB snapshots to S3 for paid plans

**Worker:** worker7 **Goal:** paid-plan Valkey instances stop being PVC-only — a nightly RDB snapshot lands in S3 with pruned retention, purged on delete, with a drilled AOF-aware restore path. **Status:** done

## Tasks (in order)

| id   | title                                                                     | est | depends_on |
| ---- | ------------------------------------------------------------------------- | --- | ---------- |
| t001 | Env contract `BEX_KV_BACKUP_*` + docs/`.env.example` sync — **DONE**                  | 30m | —          |
| t002 | Operator: plan-gated nightly backup CronJob per KeyValue instance — **DONE**          | 60m | t001       |
| t003 | Delete teardown: purge `keyvalue/<id>/` S3 prefix + owned CronJob — **DONE**         | 45m | t002       |
| t004 | Backup staleness alert for KeyValue backup CronJobs — **DONE**                        | 30m | t002       |
| t005 | AOF-aware restore runbook + drill; amend ADR021 + ADR031 — **DONE**                   | 60m | t002       |
| t006 | Simplify — **DONE**                                                                   | 20m | t005       |
| t007 | Test coverage: envtest gating/teardown + snapshot-job behavior — **DONE**             | 60m | t005       |
| t008 | Closeout — **DONE**                                                                   | 15m | t007       |

## Definition of done

With `BEX_KV_BACKUP_*` set in prod, every paid-plan KeyValue instance gets a nightly gzip'd RDB snapshot at `s3://bex-tfstate/keyvalue/<id>/<RFC3339-utc>.rdb.gz` with the job pruning to the 7 most recent; Free-plan instances get no CronJob; with the env unset the operator's output is byte-identical to today (no CronJobs, no new RBAC used). Deleting a KeyValue purges its `keyvalue/<id>/` prefix and CronJob (finalizer path, verified live). A staleness alert fires when a paid instance's backup CronJob has no success >26h. A drill restored a snapshot into a fresh instance with the AOF-precedence pitfall handled and is recorded under `drills/`; ADR021 documents the design including the honest RPO statement (snapshot cadence, not PITR; Free = PVC-only) and ADR031's table gains the row.

## Source + Goal linkage

- **Source:** 2026-07-31 persistent-store backup audit (`/pm` handoff; no inbox note — materialized directly). `docs/ADR021-keyvalue-management.md` §Consequences explicitly defers backup; the audit rated it the second-worst gap after the auth DBs (customer data, PVC-only durability).
- **Goal linkage:** managed-datastore durability — the Render-alternative core (managed Key Value, ADR021) and ADR031's consolidated backup policy; sibling of w7/m67 completing the backup-gap pair.
- **Expected outcome:** paid tenants' Valkey data survives PVC/cluster loss with at-most-one-night RPO; the platform can state its Key Value durability honestly per plan.
- **Why now:** last tenant-data store with zero off-cluster copy; delete-teardown (w7/m12) and env-gated-feature (`BEX_DB_BACKUP_*`) patterns to clone are fresh; doing it alongside m67 amortizes the ADR031 amendment and drill session.
- **Render parity omitted:** no REST/GraphQL/MCP/UI surface change in scope — Render exposes no Key Value backup API either (its `/recovery` surfaces are Postgres-only), so backups are platform-side mechanism; surfacing restore verbs to tenants is deliberately deferred (see t005 out-of-scope).

## Directory-structure decision (from the audit conversation)

- Keyed by resource id alone (`keyvalue/<id>/`), no workspace level — ids are globally unique typed ids, matching how tenant Postgres is keyed, and a flat id prefix makes teardown a single prefix delete.
- RFC3339 UTC timestamps so lexicographic order is chronological — "latest" is the last key, no manifest file.
- Same `bex-tfstate` bucket as all other backups (established precedent; the "never in `bex-tfstate`" rule is about static *serving* content).
