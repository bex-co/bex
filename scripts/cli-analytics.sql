-- Run as the database administrator after migrations 0113 and 0116. The reader
-- receives only this view; product tables and the unprojected telemetry row stay
-- private.
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
    received_at,
    NULLIF(btrim(installation_id), '') AS installation_id,
    NULLIF(btrim(subject), '') AS subject,
    NULLIF(btrim(workspace_id), '') AS workspace_id,
    COALESCE(NULLIF(btrim(command), ''), 'unknown') AS command,
    COALESCE(NULLIF(completion_kind, ''), 'unknown') AS outcome,
    CASE
        WHEN completion_kind IN ('success', 'explicit_exit') THEN exit_code <> 0
        WHEN completion_kind IN ('discovery_error', 'validation_error', 'setup_error', 'execution_error') THEN true
        ELSE NULL
    END AS failed,
    CASE WHEN duration_ms >= 0 AND completion_kind IN ('success', 'execution_error', 'explicit_exit')
        THEN duration_ms END AS duration_ms,
    CASE
        WHEN command ~ '(^| )(logs|ssh|shell|psql)( |$)' THEN 'session / stream'
        WHEN launched_full_screen_tui IS TRUE OR is_stdin_tty OR is_stdout_tty THEN 'interactive'
        ELSE 'non-interactive'
    END AS duration_mode,
    COALESCE(NULLIF(cli_version, ''), 'unknown') AS upstream_version,
    COALESCE(NULLIF(os, ''), 'unknown') AS os,
    COALESCE(NULLIF(arch, ''), 'unknown') AS arch,
    COALESCE(NULLIF(output_format, ''), 'unknown') AS output_format,
    launched_full_screen_tui,
    string_to_array(agent_signals, ',') AS agent_signals,
    string_to_array(ci_signals, ',') AS ci_signals,
    -- Appended deliberately, and new columns must keep being appended:
    -- CREATE OR REPLACE VIEW can only add columns at the end, so inserting one
    -- mid-list fails on any cluster that already holds the previous generation
    -- of this view -- which is to say, on production.
    --
    -- The bex launcher's own release, a separate axis from the pinned upstream
    -- version above (w5/m94). 'unknown' covers an unmodified upstream `render`
    -- binary and every pre-m94 bex build; it is a real population, not a gap.
    COALESCE(NULLIF(bex_version, ''), 'unknown') AS bex_version
FROM public.cli_telemetry_events;

GRANT CONNECT ON DATABASE :"DBNAME" TO bex_cli_analytics;
GRANT USAGE ON SCHEMA cli_analytics TO bex_cli_analytics;
GRANT SELECT ON cli_analytics.events TO bex_cli_analytics;
COMMIT;
