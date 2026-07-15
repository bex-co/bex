-- One-off jobs (Render's /services/{id}/jobs, render CLI `render jobs`).
-- A job runs a startCommand in a service's container image and records the
-- outcome. service_name + tenant_id key the record so jobs work for both
-- store-managed Apps (where app_id tracks the row) and hand-applied CRs
-- (which have no store row). plan_id mirrors Render's optional plan field.
CREATE TABLE jobs (
    id           text        PRIMARY KEY,
    service_name text        NOT NULL,
    tenant_id    text        NOT NULL,
    start_command text       NOT NULL,
    plan_id      text        NOT NULL DEFAULT '',
    status       text        NOT NULL DEFAULT 'pending',
    created_at   timestamptz NOT NULL DEFAULT now(),
    started_at   timestamptz,
    finished_at  timestamptz
);

-- ListJobs reads newest-first per service; the tenant_id guard ensures cross-
-- workspace queries are impossible.
CREATE INDEX jobs_service_tenant_created_idx ON jobs (service_name, tenant_id, created_at DESC);
