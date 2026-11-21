-- Run as the database administrator after migrations 0113, 0114 and 0116. The
-- reader receives only this view; product tables (including the audience labels
-- the view joins) and the unprojected telemetry row stay private.
BEGIN;
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'bex_cli_analytics') THEN
        CREATE ROLE bex_cli_analytics NOLOGIN;
    END IF;
END
$$;
ALTER ROLE bex_cli_analytics NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS CONNECTION LIMIT 6;
ALTER ROLE bex_cli_analytics SET default_transaction_read_only = on;
ALTER ROLE bex_cli_analytics SET statement_timeout = '10s';
ALTER ROLE bex_cli_analytics SET idle_in_transaction_session_timeout = '10s';
ALTER ROLE bex_cli_analytics SET timezone = 'UTC';
CREATE SCHEMA IF NOT EXISTS cli_analytics;
REVOKE ALL ON SCHEMA cli_analytics FROM PUBLIC;

CREATE OR REPLACE VIEW cli_analytics.events AS
SELECT
    e.received_at,
    NULLIF(btrim(e.installation_id), '') AS installation_id,
    NULLIF(btrim(e.subject), '') AS subject,
    NULLIF(btrim(e.workspace_id), '') AS workspace_id,
    COALESCE(NULLIF(btrim(e.command), ''), 'unknown') AS command,
    COALESCE(NULLIF(e.completion_kind, ''), 'unknown') AS outcome,
    CASE
        WHEN e.completion_kind IN ('success', 'explicit_exit') THEN e.exit_code <> 0
        WHEN e.completion_kind IN ('discovery_error', 'validation_error', 'setup_error', 'execution_error') THEN true
        ELSE NULL
    END AS failed,
    CASE WHEN e.duration_ms >= 0 AND e.completion_kind IN ('success', 'execution_error', 'explicit_exit')
        THEN e.duration_ms END AS duration_ms,
    CASE
        -- Commands that hold a session or wait on an agent for up to an hour
        -- (ADR113 D7). Flags are not collected, so `agent run` without --wait
        -- lands here too: keeping hour-long waits out of the non-interactive
        -- percentiles is worth that.
        WHEN e.command ~ '(^| )(logs|ssh|shell|psql)( |$)'
          OR e.command ~ '(^| )agent (chat|run|wait|send|logs|handoff)( |$)' THEN 'session / stream'
        WHEN e.launched_full_screen_tui IS TRUE OR e.is_stdin_tty OR e.is_stdout_tty THEN 'interactive'
        ELSE 'non-interactive'
    END AS duration_mode,
    COALESCE(NULLIF(e.cli_version, ''), 'unknown') AS upstream_version,
    COALESCE(NULLIF(e.os, ''), 'unknown') AS os,
    COALESCE(NULLIF(e.arch, ''), 'unknown') AS arch,
    COALESCE(NULLIF(e.output_format, ''), 'unknown') AS output_format,
    e.launched_full_screen_tui,
    string_to_array(e.agent_signals, ',') AS agent_signals,
    string_to_array(e.ci_signals, ',') AS ci_signals,
    -- Appended deliberately, and new columns must keep being appended:
    -- CREATE OR REPLACE VIEW can only add columns at the end, so inserting one
    -- mid-list fails on any cluster that already holds the previous generation
    -- of this view -- which is to say, on production.
    --
    -- The bex launcher's own release, a separate axis from the pinned upstream
    -- version above (w5/m94). 'unknown' covers an unmodified upstream `render`
    -- binary and every pre-m94 bex build; it is a real population, not a gap.
    COALESCE(NULLIF(e.bex_version, ''), 'unknown') AS bex_version,
    -- ADR113 D7/D9 (w3/m439), appended in this order.
    --
    -- A `bex agent` exit code is the outcome of the agent turn (3 failed,
    -- 4 canceled, 124 timed out), not a CLI failure; it needs the code itself.
    e.exit_code,
    -- Whether the CLI or the API failed, as opposed to the thing the user asked
    -- about (a deploy, an agent turn) reporting a failure through its exit
    -- code. Null where no attempt was classified (help, version, unknown).
    CASE
        WHEN e.completion_kind IN ('discovery_error', 'validation_error', 'setup_error', 'execution_error') THEN true
        WHEN e.completion_kind IN ('success', 'explicit_exit') THEN false
    END AS cli_fault,
    -- The product area, from the command names after the binary's own name
    -- (`bex` or `render`). A closed set; docs/runbooks/cli-analytics.md lists
    -- the mapping, and a new root command lands in 'other' until it is added.
    CASE
        WHEN c.verb = 'agent' AND c.sub = 'automations' THEN 'automation'
        WHEN c.verb = 'edit' OR (c.verb = 'agent' AND c.sub = 'open') THEN 'desktop'
        WHEN c.verb = 'agent' THEN 'agent'
        WHEN c.verb IN ('code', 'glm', 'muse', 'kimi', 'deepseek') THEN 'coding-agent'
        WHEN c.verb IN ('sandboxes', 'sandbox-groups')
          OR (c.verb = 'ea' AND c.sub IN ('sandboxes', 'sandbox-groups')) THEN 'sandbox'
        WHEN c.verb IN ('postgres', 'keyvalues', 'psql', 'pgcli', 'kv-cli', 's3')
          OR (c.verb = 'ea' AND c.sub = 'objects') THEN 'data'
        WHEN c.verb IN ('services', 'deploys', 'jobs', 'restart', 'logs', 'ssh',
                        'blueprints', 'environments', 'projects', 'workflows')
          OR (c.verb = 'ea' AND c.sub = 'blueprints') THEN 'hosting'
        WHEN c.verb IN ('workspaces', 'workspace', 'whoami', 'login', 'logout', 'upgrade', 'skills') THEN 'account'
        ELSE 'other'
    END AS command_family,
    e.started_at,
    -- Workspace audience labels (product_analytics_audiences, migration 0114),
    -- joined on the server-attributed workspace. The view owner's rights cover
    -- the join; the reader still cannot read the labels table itself.
    COALESCE(a.audience, 'unclassified') AS audience
FROM public.cli_telemetry_events e
-- Word 1 of the command path is the binary's own name.
CROSS JOIN LATERAL (SELECT w[2] AS verb, w[3] AS sub FROM string_to_array(btrim(e.command), ' ') AS w) c
LEFT JOIN public.product_analytics_audiences a ON a.workspace_id = NULLIF(btrim(e.workspace_id), '');

GRANT CONNECT ON DATABASE :"DBNAME" TO bex_cli_analytics;
GRANT USAGE ON SCHEMA cli_analytics TO bex_cli_analytics;
GRANT SELECT ON cli_analytics.events TO bex_cli_analytics;
COMMIT;
