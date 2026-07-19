-- w3/m19: durable operator-observed and source-control event facts. The
-- payload is deliberately closed and typed: no JSON or generic message/value
-- column can become a path for an env or secret value.
CREATE TABLE IF NOT EXISTS service_event_facts (
    source_key TEXT PRIMARY KEY,
    app_id TEXT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    fact_type TEXT NOT NULL CHECK (fact_type IN (
        'image_pull_failed',
        'service_suspended',
        'service_resumed',
        'server_failed',
        'server_available',
        'branch_changed',
        'commit_ignored',
        'autoscaling_started',
        'autoscaling_ended'
    )),
    at TIMESTAMPTZ NOT NULL,
    -- Webhook dispatch tails insertion order, not occurrence order. Operator
    -- observations can be persisted well after their original transition time;
    -- a separate clock prevents those late facts falling behind its watermark.
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    deploy_id TEXT NOT NULL DEFAULT '',
    image TEXT NOT NULL DEFAULT '',
    reason_code TEXT NOT NULL DEFAULT '' CHECK (reason_code IN (
        '',
        'image_pull_backoff',
        'readiness_failed',
        'root_directory',
        'build_filter',
        'skip_phrase'
    )),
    instance_id TEXT NOT NULL DEFAULT '',
    from_count INTEGER,
    to_count INTEGER,
    branch_from TEXT NOT NULL DEFAULT '',
    branch_to TEXT NOT NULL DEFAULT '',
    commit_id TEXT NOT NULL DEFAULT '',
    commit_url TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS service_event_facts_app_at_idx
    ON service_event_facts (app_id, at DESC, source_key DESC);

CREATE INDEX IF NOT EXISTS service_event_facts_recorded_idx
    ON service_event_facts (recorded_at, source_key);

-- Level-triggered App reconciliation records edges relative to this checkpoint.
-- The first observation establishes a baseline and emits nothing; subsequent
-- observations advance the checkpoint and append their edge in one transaction.
CREATE TABLE IF NOT EXISTS service_event_checkpoints (
    app_id TEXT PRIMARY KEY REFERENCES apps(id) ON DELETE CASCADE,
    service_phase TEXT NOT NULL DEFAULT '',
    availability TEXT NOT NULL DEFAULT '' CHECK (availability IN ('', 'healthy', 'unhealthy')),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
