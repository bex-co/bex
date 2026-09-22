-- Jobs and event projections are workspace state, not retained audit evidence.
-- Remove legacy orphans before validating the same cascade used by other
-- workspace-owned tables. Dispatch recovery intents deliberately stay separate.

DELETE FROM jobs
WHERE NOT EXISTS (SELECT 1 FROM tenants WHERE tenants.id = jobs.tenant_id);

DELETE FROM datastore_event_facts
WHERE NOT EXISTS (SELECT 1 FROM tenants WHERE tenants.id = datastore_event_facts.workspace_id);

DELETE FROM datastore_observed_checkpoints
WHERE NOT EXISTS (SELECT 1 FROM tenants WHERE tenants.id = datastore_observed_checkpoints.workspace_id);

DELETE FROM service_event_index
WHERE NOT EXISTS (SELECT 1 FROM tenants WHERE tenants.id = service_event_index.workspace_id);

ALTER TABLE jobs
    ADD CONSTRAINT jobs_workspace_fk
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE;

ALTER TABLE datastore_event_facts
    ADD CONSTRAINT datastore_event_facts_workspace_fk
    FOREIGN KEY (workspace_id) REFERENCES tenants(id) ON DELETE CASCADE;

ALTER TABLE datastore_observed_checkpoints
    ADD CONSTRAINT datastore_observed_checkpoints_workspace_fk
    FOREIGN KEY (workspace_id) REFERENCES tenants(id) ON DELETE CASCADE;

ALTER TABLE service_event_index
    ADD CONSTRAINT service_event_index_workspace_fk
    FOREIGN KEY (workspace_id) REFERENCES tenants(id) ON DELETE CASCADE;

CREATE INDEX jobs_tenant_id_idx ON jobs (tenant_id);
CREATE INDEX datastore_observed_checkpoints_workspace_idx
    ON datastore_observed_checkpoints (workspace_id);

CREATE OR REPLACE FUNCTION bex_index_audit_service_event()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.outcome <> 'allowed' THEN
        RETURN NEW;
    END IF;

    IF NEW.target LIKE 'service:%' THEN
        -- Managed writers carry the immutable srv-id. Old replicas and bare
        -- CRs still use one of three names; never attach an older generation's
        -- row to an app that did not exist when the operation was recorded.
        INSERT INTO service_event_index (
            workspace_id, event_key, source, source_row_id, phase,
            service_id, service_name, app_id
        )
        SELECT a.tenant_id, NEW.id || ':', 'audit', NEW.id, '',
               a.tenant_name || '-' || a.name, a.name, a.id
        FROM (
            -- Keep immutable-id lookups indexable independently of legacy names.
            -- Lock in subqueries: PostgreSQL cannot row-lock a UNION result.
            SELECT * FROM (
                SELECT a.id, a.tenant_id, a.name, t.name AS tenant_name
                FROM apps a
                JOIN tenants t ON t.id = a.tenant_id
                WHERE a.id = split_part(NEW.target, ':', 2)
                  AND (NEW.workspace_id = a.tenant_id OR NEW.workspace_id = 'default')
                FOR KEY SHARE OF t, a
            ) immutable_match
          UNION ALL
            SELECT * FROM (
                SELECT a.id, a.tenant_id, a.name, t.name AS tenant_name
                FROM apps a
                JOIN tenants t ON t.id = a.tenant_id
                WHERE NEW.target !~ '^service:srv-[0-9a-v]{20}$'
                  AND (NEW.workspace_id = a.tenant_id OR NEW.workspace_id = 'default')
                  AND NEW.at >= a.created_at
                  AND NEW.target IN (
                      'service:' || a.tenant_id || '-' || a.name,
                      'service:' || t.name || '-' || a.name,
                      'service:' || a.name
                  )
                FOR KEY SHARE OF t, a
            ) legacy_match
        ) a
        ON CONFLICT ON CONSTRAINT service_event_index_source_key DO NOTHING;
    ELSIF NEW.target LIKE 'database:%' OR NEW.target LIKE 'keyvalue:%' THEN
        -- Audit evidence survives workspace deletion; its workspace projection
        -- does not. Lock a live tenant through insertion so concurrent teardown
        -- cannot make this derived index reject the retained audit source.
        INSERT INTO service_event_index (
            workspace_id, event_key, source, source_row_id, phase,
            service_id, service_name, app_id
        )
        SELECT NEW.workspace_id, NEW.id || ':', 'audit', NEW.id, '',
               split_part(NEW.target, ':', 2),
               COALESCE(NULLIF(NEW.target_name, ''), split_part(NEW.target, ':', 2)),
               NULL
        FROM tenants
        WHERE tenants.id = NEW.workspace_id
        FOR KEY SHARE OF tenants
        ON CONFLICT ON CONSTRAINT service_event_index_source_key DO NOTHING;
    END IF;
    RETURN NEW;
END;
$$;
