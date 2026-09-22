DROP INDEX service_event_index_app_source_idx;

CREATE OR REPLACE FUNCTION bex_index_audit_service_event()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.outcome <> 'allowed' THEN
        RETURN NEW;
    END IF;

    IF NEW.target LIKE 'service:%' THEN
        -- Current CRs use <tenant-id>-<app>; legacy CRs can retain either the
        -- old <tenant-name>-<app> spelling or the bare public app name. Scope
        -- every spelling to the owning workspace. workspace:default rows are
        -- deliberately attributable to each matching owner, matching the
        -- existing per-service feed's bootstrap/authz-off semantics.
        INSERT INTO service_event_index (
            workspace_id, event_key, source, source_row_id, phase,
            service_id, service_name, app_id
        )
        SELECT a.tenant_id, NEW.id || ':', 'audit', NEW.id, '',
               t.name || '-' || a.name, a.name, a.id
        FROM apps a
        JOIN tenants t ON t.id = a.tenant_id
        WHERE (NEW.workspace_id = a.tenant_id OR NEW.workspace_id = 'default')
          AND NEW.target IN (
              'service:' || a.tenant_id || '-' || a.name,
              'service:' || t.name || '-' || a.name,
              'service:' || a.name
          )
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
