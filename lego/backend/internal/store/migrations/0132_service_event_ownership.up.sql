-- Resolve ownership when the audit row is inserted, never again by name at read time.
CREATE INDEX service_event_index_app_source_idx
    ON service_event_index (app_id, source, source_row_id) WHERE app_id IS NOT NULL;

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
            SELECT a.id, a.tenant_id, a.name, t.name AS tenant_name
            FROM apps a
            JOIN tenants t ON t.id = a.tenant_id
            WHERE a.id = split_part(NEW.target, ':', 2)
              AND (NEW.workspace_id = a.tenant_id OR NEW.workspace_id = 'default')
          UNION ALL
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
        ) a
        ON CONFLICT ON CONSTRAINT service_event_index_source_key DO NOTHING;
    ELSIF NEW.target LIKE 'database:%' OR NEW.target LIKE 'keyvalue:%' THEN
        -- Datastore webhook events have no apps row and no service-scoped list
        -- home. Their typed target is already the immutable dpg-/red- identity.
        INSERT INTO service_event_index (
            workspace_id, event_key, source, source_row_id, phase,
            service_id, service_name, app_id
        ) VALUES (
            NEW.workspace_id, NEW.id || ':', 'audit', NEW.id, '',
            split_part(NEW.target, ':', 2),
            COALESCE(NULLIF(NEW.target_name, ''), split_part(NEW.target, ':', 2)),
            NULL
        )
        ON CONFLICT ON CONSTRAINT service_event_index_source_key DO NOTHING;
    END IF;
    RETURN NEW;
END;
$$;

-- Migration 0083's name-only historical backfill could bind old audit evidence
-- to a replacement app. Discard only impossible associations, not source rows.
-- Do not replay unindexed names: their original owner may no longer exist.
DELETE FROM service_event_index i
USING audit_events e, apps a
WHERE i.source = 'audit' AND i.source_row_id = e.id AND i.app_id = a.id
  AND e.at < a.created_at
  AND e.target <> 'service:' || a.id;
