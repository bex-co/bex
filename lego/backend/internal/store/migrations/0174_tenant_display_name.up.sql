-- w3/m246: workspaces get Render's freeform display name. `name` stays the
-- unique DNS-label slug; `display_name` is what Render-shaped surfaces show
-- (empty = fall back to the slug). Trailing column with a default so
-- positional INSERTs keep working.
ALTER TABLE tenants ADD COLUMN display_name text NOT NULL DEFAULT '';

-- No backfill: an empty display name shows the slug. bex-api names a signup
-- workspace (slug = id) "<Name>'s Workspace" on its owner's next sign-in, the
-- only place the person's name is known.

-- The audit index composed service_id from the mutable slug, which went stale
-- after the first rename; the App's CR name is built from the tenant id
-- (store.CRName). The legacy `t.name` target match stays for old audit rows.
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
               a.tenant_id || '-' || a.name, a.name, a.id
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
