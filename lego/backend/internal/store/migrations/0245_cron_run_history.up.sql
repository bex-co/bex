-- w3/m366: a cron job's run history beyond App.status.runs' ten entries is
-- read back from its cron_job_run_started/_ended facts, addressed by run id.
-- Facts recorded before 0141 (2026-10-05) carry no run_id. Derive it exactly
-- as id.Derive(id.CronRun, <job name>) does. Derive hashes only the parts, so
-- this is 0083's bex_service_event_id(<job name>) with the crr- prefix. The
-- job name is the source key "cron:<app_id>:<job name>:started|ended" without
-- its prefix and suffix. cron_run_history_pg_test.go checks this against Go.
UPDATE service_event_facts f
SET run_id = 'crr-' || substr(bex_service_event_id(substr(f.source_key,
        length('cron:' || f.app_id || ':') + 1,
        length(f.source_key) - length('cron:' || f.app_id || ':')
          - CASE f.fact_type WHEN 'cron_job_run_started' THEN length(':started') ELSE length(':ended') END)), 5)
WHERE f.fact_type IN ('cron_job_run_started', 'cron_job_run_ended')
  AND f.run_id = ''
  AND left(f.source_key, length('cron:' || f.app_id || ':')) = 'cron:' || f.app_id || ':';

-- GetCronRun and a cursor past the status window resolve a run id to its facts.
CREATE INDEX IF NOT EXISTS service_event_facts_cron_run_idx
    ON service_event_facts (app_id, run_id)
    WHERE run_id <> '';
