-- w4/m511: Render's postgres_read_replica_stale for managed Postgres.
--
-- The operator polls each ready read replica's CNPG instance manager
-- (/pg/status) and flips Database.status.readReplicaStatuses[].syncing to
-- false (with syncStoppedAt pinned) once streaming or replay has been stopped
-- for 2 minutes. The control plane records one postgres_read_replica_stale
-- fact per (replica, syncStoppedAt) episode, keyed on that pair so concurrent
-- observers deduplicate onto one row. stale_replicas latches each currently
-- stale replica's episode timestamp on the checkpoint so steady-state resyncs
-- record nothing, recovery clears the entry silently (Render has no recovery
-- event), and a later break records a second event.

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
        'postgres_cluster_leader_changed',
        'postgres_read_replica_stale'
    ));

ALTER TABLE datastore_observed_checkpoints
    ADD COLUMN stale_replicas JSONB NOT NULL DEFAULT '{}'::jsonb;
