-- w2/m171: Render's postgres_cluster_leader_changed for managed Postgres.
--
-- The control plane records one fact per promotion of a new primary (an
-- automatic failover or the failover verb), observed from
-- Database.status.currentPrimary. The checkpoint latches the last primary and
-- CNPG's promotion time for it, so a resync replays nothing and an older
-- re-read from another bex-api replica cannot record a change backwards.
-- leader_id is the new primary, Render's details.leaderId. Existing
-- checkpoints start empty: their next pass latches the current primary
-- without recording a change.

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
        'postgres_wal_archive_failed',
        'postgres_cluster_leader_changed'
    ));

ALTER TABLE datastore_event_facts
    ADD COLUMN leader_id TEXT NOT NULL DEFAULT '';

ALTER TABLE datastore_observed_checkpoints
    ADD COLUMN current_primary TEXT NOT NULL DEFAULT '',
    ADD COLUMN current_primary_since TIMESTAMPTZ;
