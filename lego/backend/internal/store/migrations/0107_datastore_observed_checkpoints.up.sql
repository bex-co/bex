-- w3/m82: durable observed-lifecycle facts for managed datastores (Database /
-- KeyValue), the sibling of service_event_facts + service_event_checkpoints.
--
-- Datastores have no apps row, so these tables key on the CR's own immutable
-- dpg-/red- metadata.name and carry the owning workspace directly (from the
-- CR's bex.co/tenant label) instead of recovering it through a join. The
-- payload stays closed and typed for the same reason the App-side table does:
-- no JSON or free-text column can become a path for a credential value.
CREATE TABLE IF NOT EXISTS datastore_event_facts (
    10|    source_key TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    datastore_id TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('postgres', 'keyvalue')),
    fact_type TEXT NOT NULL CHECK (fact_type IN (
        'postgres_unavailable',
        'postgres_available',
        'key_value_unhealthy',
        'key_value_available',
        'postgres_backup_completed',
    20|        'postgres_backup_failed',
        'postgres_restore_succeeded',
        'postgres_restore_failed',
        'postgres_upgrade_started',
        'postgres_upgrade_succeeded',
        'postgres_upgrade_failed'
    )),
    at TIMESTAMPTZ NOT NULL,
    -- Webhook dispatch tails insertion order, not occurrence order (see the
    -- same column on service_event_facts).
    30|    recorded_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    reason_code TEXT NOT NULL DEFAULT '' CHECK (reason_code IN ('', 'readiness_failed')),
    -- Detail columns for the backup/restore/upgrade facts (t002); empty on
    -- every availability fact.
    backup_name TEXT NOT NULL DEFAULT '',
    backup_error TEXT NOT NULL DEFAULT '',
    version_from TEXT NOT NULL DEFAULT '',
    version_to TEXT NOT NULL DEFAULT '',
    scheduled BOOLEAN
);
    40|
CREATE INDEX IF NOT EXISTS datastore_event_facts_workspace_at_idx
    ON datastore_event_facts (workspace_id, at DESC, source_key DESC);

CREATE INDEX IF NOT EXISTS datastore_event_facts_datastore_at_idx
    ON datastore_event_facts (datastore_id, at DESC, source_key DESC);

CREATE INDEX IF NOT EXISTS datastore_event_facts_recorded_idx
    ON datastore_event_facts (recorded_at, source_key);

    50|-- Level-triggered Database/KeyValue reconciliation records edges relative to
-- this checkpoint, exactly as service_event_checkpoints does for Apps: the
-- first observation establishes a baseline and emits nothing.
--
-- availability latches 'unhealthy' only from 'healthy' (see
-- nextDatastoreAvailability in Go): a datastore that has never been Ready is
-- provisioning, not down, so the first Provisioned observation is what arms
-- the outage edge.
CREATE TABLE IF NOT EXISTS datastore_observed_checkpoints (
    datastore_id TEXT PRIMARY KEY,
    60|    workspace_id TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('postgres', 'keyvalue')),
    phase TEXT NOT NULL DEFAULT '',
    availability TEXT NOT NULL DEFAULT '' CHECK (availability IN ('', 'healthy', 'unhealthy')),
    suspended BOOLEAN NOT NULL DEFAULT FALSE,
    healthy_transition_at TIMESTAMPTZ,
    -- Backup/restore/upgrade edge tracking (t002).
    last_backup_name TEXT NOT NULL DEFAULT '',
    last_backup_phase TEXT NOT NULL DEFAULT '',
    restore_outcome TEXT NOT NULL DEFAULT '',
    70|    upgrade_key TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Datastore facts join the retrievable evt-... index on the same terms the
-- datastore AUDIT rows already do (migration 0083): the typed dpg-/red- id is
-- both the service id and the display name, and there is no apps row to point
-- at. The workspace comes off the fact itself.
CREATE FUNCTION bex_index_datastore_fact_service_event()
RETURNS trigger
    80|LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO service_event_index (
        workspace_id, event_key, source, source_row_id, phase,
        service_id, service_name, app_id
    ) VALUES (
        NEW.workspace_id, 'fact:' || NEW.source_key, 'fact', NEW.source_key, '',
        NEW.datastore_id, NEW.datastore_id, NULL
    )
    90|    ON CONFLICT ON CONSTRAINT service_event_index_source_key DO NOTHING;
    RETURN NEW;
END;
$$;

CREATE TRIGGER datastore_event_facts_event_index_insert
AFTER INSERT ON datastore_event_facts
FOR EACH ROW EXECUTE FUNCTION bex_index_datastore_fact_service_event();

-- Extend 0083's shared delete trigger with the new source table. Recreated
   100|-- whole rather than patched so the function body stays readable as one piece.
CREATE OR REPLACE FUNCTION bex_delete_service_event_index()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_TABLE_NAME = 'deploys' THEN
        DELETE FROM service_event_index
        WHERE source = 'deploy' AND source_row_id = OLD.id;
    ELSIF TG_TABLE_NAME = 'audit_events' THEN
   110|        DELETE FROM service_event_index
        WHERE source = 'audit' AND source_row_id = OLD.id;
    ELSIF TG_TABLE_NAME = 'service_event_facts' THEN
        DELETE FROM service_event_index
        WHERE source = 'fact' AND source_row_id = OLD.source_key;
    ELSIF TG_TABLE_NAME = 'datastore_event_facts' THEN
        DELETE FROM service_event_index
        WHERE source = 'fact' AND source_row_id = OLD.source_key;
    END IF;
    RETURN OLD;
   120|END;
$$;

CREATE TRIGGER datastore_event_facts_event_index_delete
AFTER DELETE ON datastore_event_facts
FOR EACH ROW EXECUTE FUNCTION bex_delete_service_event_index();
