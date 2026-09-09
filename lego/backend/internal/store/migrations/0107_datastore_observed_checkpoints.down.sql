-- DROP TABLE does not fire row-level delete triggers, so the index rows the
-- insert trigger wrote have to be reclaimed explicitly before the source table
-- goes away — otherwise they outlive their source and stay retrievable.
DO $$
BEGIN
    IF to_regclass('public.datastore_event_facts') IS NOT NULL THEN
        DELETE FROM service_event_index
        WHERE source = 'fact'
          AND source_row_id IN (SELECT source_key FROM datastore_event_facts);
    END IF;
END
$$;

DROP TRIGGER IF EXISTS datastore_event_facts_event_index_delete ON datastore_event_facts;
DROP TRIGGER IF EXISTS datastore_event_facts_event_index_insert ON datastore_event_facts;
    DROP FUNCTION IF EXISTS bex_index_datastore_fact_service_event();

-- Restore migration 0083's body of the shared delete trigger.
CREATE OR REPLACE FUNCTION bex_delete_service_event_index()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_TABLE_NAME = 'deploys' THEN
        DELETE FROM service_event_index
           WHERE source = 'deploy' AND source_row_id = OLD.id;
    ELSIF TG_TABLE_NAME = 'audit_events' THEN
        DELETE FROM service_event_index
        WHERE source = 'audit' AND source_row_id = OLD.id;
    ELSIF TG_TABLE_NAME = 'service_event_facts' THEN
        DELETE FROM service_event_index
        WHERE source = 'fact' AND source_row_id = OLD.source_key;
    END IF;
    RETURN OLD;
END;
    $$;

DROP TABLE IF EXISTS datastore_observed_checkpoints;
DROP TABLE IF EXISTS datastore_event_facts;
