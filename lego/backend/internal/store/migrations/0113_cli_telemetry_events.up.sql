-- w5/m92: ingested CLI telemetry events (`POST /v1/cli-telemetry-events`).
-- One row per CLI invocation delivery: the imported Render CLI already speaks
-- this endpoint, bex-api is the collection side. subject/workspace_id are the
-- reporter's auth attribution (never the client's claim); reported_workspace_id
-- keeps the CLI's own current_workspace_id claim for join/debug. Stable
-- install UUIDs are personal data: rows fall under the audit-retention sweep
-- (PurgeCLITelemetryEvents), never an indefinite trail.
CREATE TABLE cli_telemetry_events (
    id text PRIMARY KEY,
    subject text NOT NULL DEFAULT '',
    workspace_id text NOT NULL DEFAULT '',
    reported_workspace_id text NOT NULL DEFAULT '',
    command text NOT NULL,
    duration_ms bigint NOT NULL DEFAULT 0,
    exit_code integer NOT NULL DEFAULT 0,
    completion_kind text NOT NULL DEFAULT '',
    cli_version text NOT NULL DEFAULT '',
    os text NOT NULL DEFAULT '',
    arch text NOT NULL DEFAULT '',
    output_format text NOT NULL DEFAULT '',
    installation_id text NOT NULL DEFAULT '',
    launched_full_screen_tui boolean,
    is_stdin_tty boolean NOT NULL DEFAULT false,
    is_stdout_tty boolean NOT NULL DEFAULT false,
    is_stderr_tty boolean NOT NULL DEFAULT false,
    is_term_dumb boolean NOT NULL DEFAULT false,
    agent_signals text NOT NULL DEFAULT '',
    ci_signals text NOT NULL DEFAULT '',
    started_at timestamptz,
    received_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX cli_telemetry_events_workspace_received_idx
    ON cli_telemetry_events (workspace_id, received_at DESC);

CREATE INDEX cli_telemetry_events_command_received_idx
    ON cli_telemetry_events (command, received_at DESC);

CREATE INDEX cli_telemetry_events_received_idx
    ON cli_telemetry_events (received_at);
