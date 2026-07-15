CREATE TABLE ssh_sessions (
    id text PRIMARY KEY,
    subject text NOT NULL,
    workspace_id text NOT NULL,
    service_id text NOT NULL,
    instance_id text NOT NULL,
    remote_address text NOT NULL DEFAULT '',
    started_at timestamptz NOT NULL DEFAULT now(),
    ended_at timestamptz,
    result text NOT NULL DEFAULT 'active'
);

CREATE INDEX ssh_sessions_workspace_started_idx
    ON ssh_sessions (workspace_id, started_at DESC);

CREATE INDEX ssh_sessions_subject_started_idx
    ON ssh_sessions (subject, started_at DESC);

CREATE INDEX ssh_sessions_started_idx
    ON ssh_sessions (started_at);
