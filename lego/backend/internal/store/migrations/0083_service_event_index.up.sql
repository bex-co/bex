-- Make the stable evt-... identity of every outbound-webhook source
-- directly retrievable. Service events are projections rather than independent
-- rows: deploys project a started and (eventually) ended transition, while an
-- audit row or typed fact projects one event. Keep that source identity here
-- instead of copying the event details into a second mutable payload store.

-- Match internal/id.Derive(id.Event, eventKey) exactly. PostgreSQL already
-- provides sha256(bytea) (used by migration 0048); the small immutable wrapper
-- maps the first 100 digest bits onto Go's lowercase base32hex alphabet.
CREATE FUNCTION bex_service_event_id(event_key text)
RETURNS text
LANGUAGE plpgsql
IMMUTABLE
STRICT
PARALLEL SAFE
AS $$
DECLARE
    digest_bits bit(256);
    alphabet constant text := '0123456789abcdefghijklmnopqrstuv';
    derived text := 'evt-';
    digit integer;
BEGIN
    digest_bits := ('x' || encode(sha256(convert_to(event_key, 'UTF8')), 'hex'))::bit(256);
    FOR digit IN 0..19 LOOP
        derived := derived || substr(
            alphabet,
            substring(digest_bits FROM digit * 5 + 1 FOR 5)::integer + 1,
            1
        );
    END LOOP;
    RETURN derived;
END;
$$;

CREATE TABLE service_event_index (
    workspace_id text NOT NULL,
    event_key text NOT NULL,
    event_id text GENERATED ALWAYS AS (bex_service_event_id(event_key)) STORED,
    source text NOT NULL CHECK (source IN ('deploy', 'audit', 'fact')),
    source_row_id text NOT NULL,
    phase text NOT NULL DEFAULT '',
    -- service_id/service_name are the exact identities the webhook projection
    -- carries. app_id is retained separately because service_id is the
    -- Render-compatible <workspace-name>-<app-name> address, not the apps PK.
    service_id text NOT NULL,
    service_name text NOT NULL,
    app_id text REFERENCES apps(id) ON DELETE CASCADE,
    CONSTRAINT service_event_index_pkey PRIMARY KEY (workspace_id, event_id),
    CONSTRAINT service_event_index_source_key UNIQUE (source, source_row_id, phase, workspace_id),
    CONSTRAINT service_event_index_event_id_check CHECK (event_id ~ '^evt-[0-9a-v]{20}$'),
    CONSTRAINT service_event_index_phase_check CHECK (
        (source = 'deploy' AND phase IN ('started', 'ended')) OR
        (source <> 'deploy' AND phase = '')
    )
);

-- Source-table triggers are installed before the historical backfill. In a
-- rolling deployment this closes the gap between the migration snapshot and
-- an older bex-api replica still writing source rows; trigger work is in the
-- same transaction as the source mutation.
CREATE FUNCTION bex_index_deploy_service_events()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        INSERT INTO service_event_index (
            workspace_id, event_key, source, source_row_id, phase,
            service_id, service_name, app_id
        )
        SELECT a.tenant_id, NEW.id || ':started', 'deploy', NEW.id, 'started',
               t.name || '-' || a.name, a.name, a.id
        FROM apps a
        JOIN tenants t ON t.id = a.tenant_id
        WHERE a.id = NEW.app_id
        ON CONFLICT ON CONSTRAINT service_event_index_source_key DO NOTHING;
    END IF;

    IF NEW.finished_at IS NOT NULL THEN
        INSERT INTO service_event_index (
            workspace_id, event_key, source, source_row_id, phase,
            service_id, service_name, app_id
        )
        SELECT a.tenant_id, NEW.id || ':ended', 'deploy', NEW.id, 'ended',
               t.name || '-' || a.name, a.name, a.id
        FROM apps a
        JOIN tenants t ON t.id = a.tenant_id
        WHERE a.id = NEW.app_id
        ON CONFLICT ON CONSTRAINT service_event_index_source_key DO NOTHING;
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER deploys_service_event_index_insert
AFTER INSERT ON deploys
FOR EACH ROW EXECUTE FUNCTION bex_index_deploy_service_events();

CREATE TRIGGER deploys_service_event_index_finish
AFTER UPDATE OF finished_at ON deploys
FOR EACH ROW
WHEN (OLD.finished_at IS NULL AND NEW.finished_at IS NOT NULL)
EXECUTE FUNCTION bex_index_deploy_service_events();

CREATE FUNCTION bex_index_audit_service_event()
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

CREATE TRIGGER audit_events_service_event_index_insert
AFTER INSERT ON audit_events
FOR EACH ROW EXECUTE FUNCTION bex_index_audit_service_event();

CREATE FUNCTION bex_index_fact_service_event()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO service_event_index (
        workspace_id, event_key, source, source_row_id, phase,
        service_id, service_name, app_id
    )
    SELECT a.tenant_id, 'fact:' || NEW.source_key, 'fact', NEW.source_key, '',
           t.name || '-' || a.name, a.name, a.id
    FROM apps a
    JOIN tenants t ON t.id = a.tenant_id
    WHERE a.id = NEW.app_id
    ON CONFLICT ON CONSTRAINT service_event_index_source_key DO NOTHING;
    RETURN NEW;
END;
$$;

CREATE TRIGGER service_event_facts_event_index_insert
AFTER INSERT ON service_event_facts
FOR EACH ROW EXECUTE FUNCTION bex_index_fact_service_event();

CREATE FUNCTION bex_delete_service_event_index()
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

CREATE TRIGGER deploys_service_event_index_delete
AFTER DELETE ON deploys
FOR EACH ROW EXECUTE FUNCTION bex_delete_service_event_index();

CREATE TRIGGER audit_events_service_event_index_delete
AFTER DELETE ON audit_events
FOR EACH ROW EXECUTE FUNCTION bex_delete_service_event_index();

CREATE TRIGGER service_event_facts_event_index_delete
AFTER DELETE ON service_event_facts
FOR EACH ROW EXECUTE FUNCTION bex_delete_service_event_index();

-- Deterministic historical backfill. Deliberately do not use a catch-all
-- ON CONFLICT DO NOTHING: a collision in the 100-bit public id must fail the
-- migration visibly rather than silently making one source unreachable.
INSERT INTO service_event_index (
    workspace_id, event_key, source, source_row_id, phase,
    service_id, service_name, app_id
)
SELECT a.tenant_id, d.id || ':started', 'deploy', d.id, 'started',
       t.name || '-' || a.name, a.name, a.id
FROM deploys d
JOIN apps a ON a.id = d.app_id
JOIN tenants t ON t.id = a.tenant_id;

INSERT INTO service_event_index (
    workspace_id, event_key, source, source_row_id, phase,
    service_id, service_name, app_id
)
SELECT a.tenant_id, d.id || ':ended', 'deploy', d.id, 'ended',
       t.name || '-' || a.name, a.name, a.id
FROM deploys d
JOIN apps a ON a.id = d.app_id
JOIN tenants t ON t.id = a.tenant_id
WHERE d.finished_at IS NOT NULL;

INSERT INTO service_event_index (
    workspace_id, event_key, source, source_row_id, phase,
    service_id, service_name, app_id
)
SELECT a.tenant_id, e.id || ':', 'audit', e.id, '',
       t.name || '-' || a.name, a.name, a.id
FROM audit_events e
JOIN apps a ON e.workspace_id = a.tenant_id OR e.workspace_id = 'default'
JOIN tenants t ON t.id = a.tenant_id
WHERE e.outcome = 'allowed'
  AND e.target IN (
      'service:' || a.tenant_id || '-' || a.name,
      'service:' || t.name || '-' || a.name,
      'service:' || a.name
  );

INSERT INTO service_event_index (
    workspace_id, event_key, source, source_row_id, phase,
    service_id, service_name, app_id
)
SELECT e.workspace_id, e.id || ':', 'audit', e.id, '',
       split_part(e.target, ':', 2),
       COALESCE(NULLIF(e.target_name, ''), split_part(e.target, ':', 2)),
       NULL
FROM audit_events e
WHERE e.outcome = 'allowed'
  AND (e.target LIKE 'database:%' OR e.target LIKE 'keyvalue:%');

INSERT INTO service_event_index (
    workspace_id, event_key, source, source_row_id, phase,
    service_id, service_name, app_id
)
SELECT a.tenant_id, 'fact:' || f.source_key, 'fact', f.source_key, '',
       t.name || '-' || a.name, a.name, a.id
FROM service_event_facts f
JOIN apps a ON a.id = f.app_id
JOIN tenants t ON t.id = a.tenant_id;
