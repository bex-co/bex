-- w2/m252: failure alerts by email, as on Render. Render emails a running
-- service becoming unhealthy, a failed cron run, a failed image pull and a
-- failed Blueprint sync (render.com/docs/notifications); until now bex emailed
-- deploys only and sent these to push alone.
--
-- alert_email_watermarks holds one cursor per source the worker tails: the
-- composed event feed ('feed') and failed Blueprint syncs ('blueprint_syncs').
CREATE TABLE alert_email_watermarks (
    name text PRIMARY KEY CHECK (name IN ('feed', 'blueprint_syncs')),
    at   timestamptz NOT NULL,
    key  text NOT NULL DEFAULT ''
);

-- alert_email_events records each source event once per recipient, so a feed
-- page projected twice (a stale replica cursor) cannot count a failure twice.
CREATE TABLE alert_email_events (
    tenant_id   text NOT NULL,
    subject     text NOT NULL,
    source_key  text NOT NULL,
    recorded_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, subject, source_key),
    FOREIGN KEY (tenant_id, subject)
        REFERENCES tenant_members (tenant_id, subject) ON DELETE CASCADE
);
CREATE INDEX alert_email_events_recorded_at ON alert_email_events (recorded_at);

-- alert_email_streams is one recipient's alert state for one resource and
-- event. The first failure is due at once; failures within the digest window
-- after a mail accumulate and go out together when it closes. occurrences
-- keeps the newest 50 pending times for the digest; pending_count is exact.
CREATE TABLE alert_email_streams (
    tenant_id      text NOT NULL,
    subject        text NOT NULL,
    resource_id    text NOT NULL,
    event          text NOT NULL CHECK (event IN (
        'server_failed', 'cron_failed', 'image_pull_failed', 'blueprint_sync_failed')),
    resource_name  text NOT NULL DEFAULT '',
    detail         text NOT NULL DEFAULT '',
    occurrences    timestamptz[] NOT NULL DEFAULT '{}',
    pending_count  integer NOT NULL DEFAULT 0 CHECK (pending_count >= 0),
    last_mailed_at timestamptz,
    due_at         timestamptz,
    claimed_until  timestamptz,
    attempts       integer NOT NULL DEFAULT 0,
    PRIMARY KEY (tenant_id, subject, resource_id, event),
    FOREIGN KEY (tenant_id, subject)
        REFERENCES tenant_members (tenant_id, subject) ON DELETE CASCADE
);
CREATE INDEX alert_email_streams_due ON alert_email_streams (due_at) WHERE pending_count > 0;

-- The failed-sync scan reads by completion time.
CREATE INDEX blueprint_syncs_failed_completed ON blueprint_syncs (completed_at, id) WHERE state = 'error';
