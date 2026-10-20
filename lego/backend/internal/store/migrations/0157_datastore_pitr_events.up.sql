-- w5/m141: Render's point-in-time-recovery events for managed Postgres.
--
-- Render's postgres_backup_* means a manually triggered export, while its PITR
-- base backups are postgres_pitr_checkpoint_* and an archiving failure is
-- postgres_wal_archive_failed. CNPG base backups move to the checkpoint names,
-- exports take the backup names, and three checkpoint columns carry the new
-- edges: the base backup whose start was reported, the latched WAL-archiving
-- state, and the floor below which a finished export is history.

ALTER TABLE datastore_event_facts DROP CONSTRAINT datastore_event_facts_fact_type_check;
ALTER TABLE datastore_event_facts ADD CONSTRAINT datastore_event_facts_fact_type_check
    CHECK (fact_type IN (
        'postgres_unavailable',
        'postgres_available',
        'key_value_unhealthy',
        'key_value_available',
        'postgres_backup_completed',
        'postgres_backup_failed',
        'postgres_restore_succeeded',
        'postgres_restore_failed',
        'postgres_upgrade_started',
        'postgres_upgrade_succeeded',
        'postgres_upgrade_failed',
        'postgres_pitr_checkpoint_started',
        'postgres_pitr_checkpoint_completed',
        'postgres_pitr_checkpoint_failed',
        'postgres_wal_archive_failed'
    ));

ALTER TABLE datastore_observed_checkpoints
    ADD COLUMN started_backup TEXT NOT NULL DEFAULT '',
    ADD COLUMN wal_archiving TEXT NOT NULL DEFAULT '' CHECK (wal_archiving IN ('', 'ok', 'failed')),
    ADD COLUMN exports_since TIMESTAMPTZ;

-- A subscription to the nightly signal keeps receiving it under its new name:
-- an endpoint filtering on postgres_backup_completed/_failed also gets the
-- matching checkpoint event, appended after its existing order (0106's
-- rewrite pattern). Recorded facts keep the names they were recorded under.
UPDATE webhook_endpoints
SET event_types = event_types
    || CASE WHEN 'postgres_backup_completed' = ANY(event_types)
                 AND NOT 'postgres_pitr_checkpoint_completed' = ANY(event_types)
            THEN ARRAY['postgres_pitr_checkpoint_completed'] ELSE ARRAY[]::text[] END
    || CASE WHEN 'postgres_backup_failed' = ANY(event_types)
                 AND NOT 'postgres_pitr_checkpoint_failed' = ANY(event_types)
            THEN ARRAY['postgres_pitr_checkpoint_failed'] ELSE ARRAY[]::text[] END
WHERE event_types && ARRAY['postgres_backup_completed', 'postgres_backup_failed']::text[];
