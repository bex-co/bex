-- Reverse 0157: drop the checkpoint columns and the PITR fact types. Facts
-- recorded under the new names are removed first so the narrower CHECK can be
-- reinstated; endpoint filters keep the extra names (unknown types match
-- nothing and are harmless).
DELETE FROM datastore_event_facts
WHERE fact_type IN ('postgres_pitr_checkpoint_started', 'postgres_pitr_checkpoint_completed',
                    'postgres_pitr_checkpoint_failed', 'postgres_wal_archive_failed');

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
        'postgres_upgrade_failed'
    ));

ALTER TABLE datastore_observed_checkpoints
    DROP COLUMN started_backup,
    DROP COLUMN wal_archiving,
    DROP COLUMN exports_since;
