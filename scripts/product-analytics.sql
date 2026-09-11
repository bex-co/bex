-- Restricted product-only reader. Run after migration 0115 as database admin.
BEGIN;
DO $$ BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname='bex_product_analytics') THEN
        CREATE ROLE bex_product_analytics NOLOGIN;
    END IF;
END $$;
ALTER ROLE bex_product_analytics NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS CONNECTION LIMIT 6;
ALTER ROLE bex_product_analytics SET default_transaction_read_only=on;
ALTER ROLE bex_product_analytics SET statement_timeout='10s';
ALTER ROLE bex_product_analytics SET idle_in_transaction_session_timeout='10s';
ALTER ROLE bex_product_analytics SET timezone='UTC';

CREATE SCHEMA IF NOT EXISTS product_analytics;
REVOKE ALL ON SCHEMA product_analytics FROM PUBLIC;

-- Explicit projections are the privacy boundary. New source columns must not
-- become readable merely because this bootstrap is rerun.
CREATE OR REPLACE VIEW product_analytics.events AS
SELECT e.source_key, e.workspace_id, e.resource_id, e.parent_id,
       e.resource_type, e.event_type, e.at, e.recorded_at, e.actor_id,
       e.actor_type, e.provenance, e.outcome, e.duration_ms,
       COALESCE(a.audience,'unclassified') AS audience,
       -- Appended, and new columns must keep being appended: CREATE OR REPLACE
       -- VIEW can only add at the end, so inserting one mid-list fails against
       -- any cluster already holding the previous generation -- production
       -- included, where the bootstrap would abort mid-transaction (learned in
       -- w5/m94, guarded by the append-only test).
       --
       -- Which surface created the resource (w5/m97): a different axis from
       -- actor_type beside it, which records human-vs-machine credential and
       -- cannot tell a dashboard click from a CLI invocation.
       e.surface
FROM public.product_activity_events e
LEFT JOIN public.product_analytics_audiences a USING(workspace_id);

CREATE OR REPLACE VIEW product_analytics.workspaces AS
SELECT t.id AS workspace_id, t.created_at,
       COALESCE(a.audience,'unclassified') AS audience
FROM public.tenants t
LEFT JOIN public.product_analytics_audiences a ON a.workspace_id=t.id;

CREATE OR REPLACE VIEW product_analytics.hosting AS
SELECT h.workspace_id, h.resource_id, h.resource_type, h.day,
       h.observed_at, h.live, h.was_live,
       COALESCE(a.audience,'unclassified') AS audience
FROM public.product_hosting_daily h
LEFT JOIN public.product_analytics_audiences a USING(workspace_id);

CREATE OR REPLACE VIEW product_analytics.domains AS
SELECT d.id AS domain_id, d.app_id AS resource_id, a.tenant_id AS workspace_id,
       COALESCE(NULLIF(a.type,''),'web_service') AS resource_type,
       d.created_at, d.claim_state, d.verified_at, d.verification_attempts,
       o.observed_at,
       CASE WHEN o.observed_at>now()-interval '30 minutes' THEN o.tls_ready END AS tls_ready,
       o.first_tls_ready_at, COALESCE(c.audience,'unclassified') AS audience
FROM public.domains d
JOIN public.apps a ON a.id=d.app_id
LEFT JOIN public.product_domain_observations o ON o.domain_id=d.id
LEFT JOIN public.product_analytics_audiences c ON c.workspace_id=a.tenant_id
WHERE COALESCE(d.redirect_for_name,'')='';

CREATE OR REPLACE VIEW product_analytics.collection AS
SELECT started_at, inventory_started_at, events_retained_from,
       -- Appended, not inserted: CREATE OR REPLACE VIEW can only add at the end
       -- (w5/m94).
       surface_started_at
FROM public.product_analytics_collection;

CREATE OR REPLACE VIEW product_analytics.inventory_batches AS
SELECT source, bucket, observed_at, complete
FROM public.product_inventory_batches;

CREATE OR REPLACE VIEW product_analytics.inventory AS
SELECT i.source, i.bucket, i.workspace_id, i.resource_type, i.state, i.resources,
       COALESCE(a.audience,'unclassified') AS audience
FROM public.product_inventory_counts i
LEFT JOIN public.product_analytics_audiences a USING(workspace_id);

CREATE OR REPLACE VIEW product_analytics.provisioning AS
SELECT l.resource_id, l.workspace_id, l.source, l.resource_type,
       l.created_at, l.first_seen_at, l.last_seen_at, l.first_ready_at,
       l.removed_at, l.state, COALESCE(a.audience,'unclassified') AS audience
FROM public.product_inventory_lifecycle l
LEFT JOIN public.product_analytics_audiences a USING(workspace_id);

GRANT CONNECT ON DATABASE :"DBNAME" TO bex_product_analytics;
GRANT USAGE ON SCHEMA product_analytics TO bex_product_analytics;
GRANT SELECT ON
    product_analytics.events,
    product_analytics.workspaces,
    product_analytics.hosting,
    product_analytics.domains,
    product_analytics.collection,
    product_analytics.inventory_batches,
    product_analytics.inventory,
    product_analytics.provisioning
TO bex_product_analytics;
COMMIT;
