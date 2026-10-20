# w5 · m141 — Postgres backup and archiving events mean what Render's mean

**Worker:** worker5 **Goal:** a scheduled base backup emits Render's `postgres_pitr_checkpoint_*` events, a tenant export emits `postgres_backup_completed`/`_failed`, and a broken WAL archive emits one `postgres_wal_archive_failed` — on REST, GraphQL and MCP events, webhooks and the dashboard. **Status:** blocked (t001–t004, t006–t008 done 2026-10-20; t005's live walk needs a backup-enabled Postgres)

## Tasks (in order)

| id   | title                                                                                    | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Operator reports the in-flight scheduled backup and a `WALArchiving` condition — **DONE** | 60m | —          |
| t002 | Scheduled backups record `pitr_checkpoint_*`; exports record `postgres_backup_*` ends — **DONE** | 60m | t001       |
| t003 | `postgres_wal_archive_failed`, latched per failure; one migration for the four new facts — **DONE** | 45m | t002       |
| t004 | Vocabulary, webhooks and push carry the new names; existing subscriptions are migrated — **DONE** | 45m | t003       |
| t005 | Dashboard catalog, docs, and the dev-5 walk                                              | 60m | t004       |
| t006 | Render parity — **DONE** | 30m | t005       |
| t007 | Simplify — **DONE** | 20m | t006       |
| t008 | Test coverage — **DONE** | 45m | t006, t007 |
| t009 | Closeout                                                                                 | 10m | t008       |

Total: **9 tasks, about 6h 15m**.

## Definition of done

- On dev-5, a paid database's scheduled backup emits `postgres_pitr_checkpoint_started` then `_completed` (and `_failed` when it fails), and no `postgres_backup_*` event.
- A tenant export emits `postgres_backup_started` and then `postgres_backup_completed`, or `postgres_backup_failed` with its reason.
- Breaking the WAL archive emits exactly one `postgres_wal_archive_failed`; restoring it re-arms the edge so a second break emits again. A database without backups never emits it.
- Each event appears on REST, GraphQL and MCP events, is delivered to a subscribed webhook, and is labelled in the dashboard (en/zh). A webhook that subscribed to `postgres_backup_*` before the change also receives the checkpoint events.
- `docs/render-artifacts/datastore-webhook-events.md`, the vocabulary fixture, `webhooks-api.md` and ADR018 row 263 describe the shipped vocabulary.

## Source + Goal linkage

- **Source:** fourth `/pm-brainstorm for w5` (2026-10-20), item 1, chosen by the user the same day. It folds in round 2's WAL-archive candidate (same files).
  - Render (render.com/docs/webhooks): `postgres_backup_*` is "a manually triggered export started/completed/failed"; `postgres_pitr_checkpoint_*` is point-in-time recovery's daily checkpoint; `postgres_wal_archive_failed` is "Point-in-time recovery failed a WAL archive".
  - bex: `postgres_backup_started` correctly comes from `postgres.CreateExport` (`lego/backend/internal/core/audit.go:224`), but `postgres_backup_completed/_failed` come from `status.lastBackup`, the newest finished CNPG base backup — nightly or post-upgrade (`lego/backend/internal/store/datastore_lifecycle_facts.go:41-76`, `lego/operator/internal/controller/database_controller.go:994-1080`). So every database with backups emits a nightly `postgres_backup_completed`, and an export never says it finished, although `status.exports[].phase`/`CompletedAt`/`FailureReason` exist (`lego/types/v1alpha1/database_types.go:344-376`) and the observer never reads them (`store/datastore_observations.go:196`).
  - WAL archiving is read only into a platform metric (`lego/operator/internal/controller/datastore_metrics.go:105-123`) and a platform alert (`deploy/gitops/base/prometheus.yaml:1242-1249`). Paid plans offer tenant point-in-time recovery (`tiers.yaml` `backup: true`; `recoverDatabase`), so an archive failure silently stops `latestRecoveryTime` advancing; w7/040 found three production databases that had archived nothing.
  - No board owner: w3/m82 t002 assumed `backup_started` meant an on-demand base backup; `datastore-webhook-events.md:99` lists the checkpoint and archive events as omissions.
- **Goal linkage:** ADR009 managed Postgres (backups and recovery); ADR018 Render parity (row 263, datastore webhook events); ADR052 outbound webhooks.
- **Expected outcome:** webhook subscribers get the events Render's names promise; a tenant learns when an export finishes and when their recovery window stops advancing.
- **Why now:** subscribers receive a mislabelled event every night today, and the recovery-window failure is invisible to tenants. w5/m138 just finished in the same event files, so they are free.
- **Behavior change:** existing `postgres_backup_*` subscribers stop receiving a nightly `postgres_backup_completed`; t004 migrates their subscriptions to include the checkpoint names so nothing is silently lost.
- **Render parity included:** this changes the event vocabulary on REST, GraphQL and MCP, webhook payloads and the dashboard catalog.

## Progress (2026-10-20)

- **Shipped behaviour.** The operator projects `status.activeBackup` (the running CNPG base backup, from the Backup List it already reads) and a `WALArchiving` condition mirroring CNPG's `ContinuousArchiving` on backup-enabled databases (removed otherwise; CNPG's `walArchivingFailing` backup phase counts as a failed checkpoint). The control plane records `postgres_pitr_checkpoint_started/_completed/_failed` for every base backup, `postgres_backup_completed/_failed` for exports reaching `available`/`failed` (floored below exports already visible on a database's first post-0157 pass, with a 5-minute grace), and one `postgres_wal_archive_failed` per failure episode (silent while suspended, upgrading or restoring). Migration 0157 widens the fact CHECK, adds three checkpoint columns, and appends the checkpoint names to webhook endpoints that filtered on the old nightly names. Webhooks, the events index and the dashboard picker (en/zh) carry the four names; the vocabulary fixture lists them as supported.
- **Push decision.** The existing "backup failed" push category covers export, checkpoint and archiving failures (titles "Export failed", "Backup failed", "Backups failing"), so saved preferences keep covering the nightly failure they covered before and the push schema is unchanged. Rows already queued at deploy from a nightly failure will read "Export failed" once.
- **Evidence.** Operator unit tests (`database_backup_status_test.go`) and the full `make test`; memStore tests for start-once, outcome routing, export floor (incl. a late-recorded export), WAL latch/suppression/re-arm and baseline silence; real-Postgres `TestPGPointInTimeRecoveryEdges` and `TestPITREventSubscriptionMigration`; push projection, webhook vocabulary and dashboard picker tests; `make lint` clean.
- **Remaining (t005 live walk).** dev-N stacks have no backup object store, so they have no base backups, WAL archiving or exports, and QA's `bex-canary` fixtures are Free (no backups). The walk needs a backup-enabled Postgres: a paid QA fixture in `bex-canary` (user approval for the spend), or an object store wired into a dev-N stack. Then: watch one nightly (or manual) base backup emit start + completed, run one export, break and restore archiving (revoke the backup credential), and record each event on REST and a webhook receiver.
