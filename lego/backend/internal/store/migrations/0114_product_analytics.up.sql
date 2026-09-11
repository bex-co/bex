-- Internal product analytics: closed facts, no names, URLs, credentials or bodies.
-- Resource deletion preserves the retained usage trail; workspace deletion removes it.
CREATE TABLE product_activity_events (
    source_key text PRIMARY KEY,
    workspace_id text NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    resource_id text NOT NULL,
    parent_id text NOT NULL DEFAULT '',
    resource_type text NOT NULL CHECK (resource_type IN ('workspace', 'web_service', 'static_site', 'private_service', 'background_worker', 'cron_job', 'postgres', 'keyvalue', 'domain')),
    event_type text NOT NULL CHECK (event_type IN ('created', 'deleted', 'domain_added', 'domain_verified', 'deploy_requested', 'deploy_finished')),
    at timestamptz NOT NULL,
    recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    actor_id text NOT NULL DEFAULT '',
    actor_type text NOT NULL DEFAULT 'unknown' CHECK (actor_type IN ('human', 'machine', 'system', 'unknown')),
    provenance text NOT NULL DEFAULT 'observed' CHECK (provenance IN ('observed', 'legacy')),
    outcome text NOT NULL DEFAULT '' CHECK (outcome IN ('', 'succeeded', 'failed', 'canceled')),
    duration_ms double precision CHECK (duration_ms >= 0)
);
CREATE INDEX product_activity_events_at_idx ON product_activity_events(at);
CREATE INDEX product_activity_events_workspace_at_idx ON product_activity_events(workspace_id, at);
CREATE INDEX product_activity_events_resource_idx ON product_activity_events(resource_id, event_type);
CREATE INDEX product_activity_events_actor_idx ON product_activity_events(actor_id) WHERE actor_id<>'';
CREATE TABLE product_analytics_collection (singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton), started_at timestamptz NOT NULL DEFAULT clock_timestamp());
INSERT INTO product_analytics_collection DEFAULT VALUES;

-- Explicit audience classification; never infer internal use from billing exemption.
CREATE TABLE product_analytics_audiences (
    workspace_id text PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    audience text NOT NULL CHECK (audience IN ('customer', 'internal', 'qa', 'canary'))
);
CREATE TABLE product_hosting_daily (
    workspace_id text NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    resource_id text NOT NULL,
    resource_type text NOT NULL,
    day date NOT NULL,
    observed_at timestamptz NOT NULL,
    live boolean NOT NULL,
    was_live boolean NOT NULL,
    PRIMARY KEY (resource_id, day)
);
CREATE INDEX product_hosting_daily_day_idx ON product_hosting_daily(day);
CREATE TABLE product_domain_observations (
    domain_id text PRIMARY KEY REFERENCES domains(id) ON DELETE CASCADE,
    observed_at timestamptz NOT NULL,
    tls_ready boolean NOT NULL,
    first_tls_ready_at timestamptz
);

-- Historical inventory is incomplete (deleted rows are gone); no creator is guessed.
INSERT INTO product_activity_events(source_key, workspace_id, resource_id, resource_type, event_type, at, provenance)
SELECT 'created:' || id, id, id, 'workspace', 'created', created_at, 'legacy' FROM tenants;
INSERT INTO product_activity_events(source_key, workspace_id, resource_id, resource_type, event_type, at, provenance)
SELECT 'created:' || id, tenant_id, id, COALESCE(NULLIF(type, ''), 'web_service'), 'created', created_at, 'legacy' FROM apps;

CREATE FUNCTION bex_product_deploy_event() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE ws text; kind text; result text;
BEGIN
    SELECT tenant_id, COALESCE(NULLIF(type, ''), 'web_service') INTO ws, kind FROM apps WHERE id = NEW.app_id;
    IF ws IS NULL THEN RETURN NEW; END IF;
    INSERT INTO product_activity_events(source_key, workspace_id, resource_id, parent_id, resource_type, event_type, at, actor_type)
    VALUES ('deploy_requested:' || NEW.id, ws, NEW.id, NEW.app_id, kind, 'deploy_requested', NEW.created_at, 'system')
    ON CONFLICT DO NOTHING;
    IF NEW.status NOT IN ('live', 'deactivated', 'build_failed', 'pre_deploy_failed', 'update_failed', 'canceled') OR NEW.finished_at IS NULL THEN RETURN NEW; END IF;
    result := CASE WHEN NEW.status IN ('live', 'deactivated') THEN 'succeeded' WHEN NEW.status = 'canceled' THEN 'canceled' ELSE 'failed' END;
    INSERT INTO product_activity_events(source_key, workspace_id, resource_id, parent_id, resource_type, event_type, at, actor_type, outcome, duration_ms)
    VALUES ('deploy_finished:' || NEW.id, ws, NEW.id, NEW.app_id, kind, 'deploy_finished', NEW.finished_at, 'system', result,
        CASE WHEN NEW.started_at IS NOT NULL AND NEW.finished_at >= NEW.started_at THEN EXTRACT(epoch FROM NEW.finished_at - NEW.started_at) * 1000 END)
    ON CONFLICT DO NOTHING;
    RETURN NEW;
END $$;
CREATE TRIGGER product_deploy_event AFTER INSERT OR UPDATE ON deploys FOR EACH ROW EXECUTE FUNCTION bex_product_deploy_event();
-- No UPDATE backfill: replaying production rows would also fire unrelated triggers.
INSERT INTO product_activity_events(source_key, workspace_id, resource_id, parent_id, resource_type, event_type, at, actor_type, provenance)
SELECT 'deploy_requested:' || d.id, a.tenant_id, d.id, a.id, COALESCE(NULLIF(a.type, ''), 'web_service'), 'deploy_requested', d.created_at, 'system', 'legacy' FROM deploys d JOIN apps a ON a.id = d.app_id;
INSERT INTO product_activity_events(source_key, workspace_id, resource_id, parent_id, resource_type, event_type, at, actor_type, provenance, outcome, duration_ms)
SELECT 'deploy_finished:' || d.id, a.tenant_id, d.id, a.id, COALESCE(NULLIF(a.type, ''), 'web_service'), 'deploy_finished', d.finished_at, 'system', 'legacy',
    CASE WHEN d.status IN ('live', 'deactivated') THEN 'succeeded' WHEN d.status = 'canceled' THEN 'canceled' ELSE 'failed' END,
    CASE WHEN d.started_at IS NOT NULL AND d.finished_at >= d.started_at THEN EXTRACT(epoch FROM d.finished_at - d.started_at) * 1000 END
FROM deploys d JOIN apps a ON a.id = d.app_id WHERE d.finished_at IS NOT NULL AND d.status IN ('live', 'deactivated', 'build_failed', 'pre_deploy_failed', 'update_failed', 'canceled');

CREATE FUNCTION bex_product_domain_event() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE ws text; added_at timestamptz;
BEGIN
    -- Generated redirect siblings do not count as another customer adoption.
    IF COALESCE(NEW.redirect_for_name, '') <> '' THEN RETURN NEW; END IF;
    SELECT tenant_id INTO ws FROM apps WHERE id = NEW.app_id;
    IF ws IS NULL THEN RETURN NEW; END IF;
    added_at := NEW.created_at;
    IF TG_OP = 'UPDATE' AND COALESCE(OLD.redirect_for_name, '') <> '' THEN added_at := clock_timestamp(); END IF;
    INSERT INTO product_activity_events(source_key, workspace_id, resource_id, parent_id, resource_type, event_type, at)
    VALUES ('domain_added:' || NEW.id, ws, NEW.id, NEW.app_id, 'domain', 'domain_added', added_at) ON CONFLICT DO NOTHING;
    IF NEW.claim_state = 'verified' AND NEW.verified_at IS NOT NULL THEN
        INSERT INTO product_activity_events(source_key, workspace_id, resource_id, parent_id, resource_type, event_type, at)
        VALUES ('domain_verified:' || NEW.id, ws, NEW.id, NEW.app_id, 'domain', 'domain_verified', NEW.verified_at) ON CONFLICT DO NOTHING;
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER product_domain_event AFTER INSERT OR UPDATE ON domains FOR EACH ROW EXECUTE FUNCTION bex_product_domain_event();
INSERT INTO product_activity_events(source_key, workspace_id, resource_id, parent_id, resource_type, event_type, at, provenance)
SELECT 'domain_added:' || d.id, a.tenant_id, d.id, a.id, 'domain', 'domain_added', d.created_at, 'legacy' FROM domains d JOIN apps a ON a.id = d.app_id WHERE COALESCE(d.redirect_for_name, '') = '';
INSERT INTO product_activity_events(source_key, workspace_id, resource_id, parent_id, resource_type, event_type, at, provenance)
SELECT 'domain_verified:' || d.id, a.tenant_id, d.id, a.id, 'domain', 'domain_verified', d.verified_at, 'legacy' FROM domains d JOIN apps a ON a.id = d.app_id WHERE COALESCE(d.redirect_for_name, '') = '' AND d.verified_at IS NOT NULL;

CREATE FUNCTION bex_product_app_delete() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    -- Creation history may already have expired; it is not lifecycle state.
    -- RollbackAppCreation explicitly discards a failed provision's facts.
    IF EXISTS (SELECT 1 FROM tenants WHERE id = OLD.tenant_id) THEN
        INSERT INTO product_activity_events(source_key, workspace_id, resource_id, resource_type, event_type, at, actor_type)
        VALUES ('deleted:' || OLD.id, OLD.tenant_id, OLD.id, COALESCE(NULLIF(OLD.type, ''), 'web_service'), 'deleted', clock_timestamp(), 'system') ON CONFLICT DO NOTHING;
    END IF;
    RETURN OLD;
END $$;
CREATE TRIGGER product_app_delete BEFORE DELETE ON apps FOR EACH ROW EXECUTE FUNCTION bex_product_app_delete();

CREATE FUNCTION bex_product_workspace_event() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO product_activity_events(source_key, workspace_id, resource_id, resource_type, event_type, at)
    VALUES ('created:' || NEW.id, NEW.id, NEW.id, 'workspace', 'created', NEW.created_at) ON CONFLICT DO NOTHING;
    RETURN NEW;
END $$;
CREATE TRIGGER product_workspace_event AFTER INSERT ON tenants FOR EACH ROW EXECUTE FUNCTION bex_product_workspace_event();
