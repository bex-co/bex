-- w4/631: who triggered and who canceled a cron run, as Render's
-- cronJobRun.triggeredBy/canceledBy and cron_job_run_ended.user name them.
-- bex-api writes a row when a user triggers or cancels a run, keyed by the
-- run's public crr- id (derived from its Job name, so known before the Job
-- exists). A scheduled run gets no row unless a user cancels it, and so
-- never names a trigger. The values are identity subjects; reads resolve
-- them to the person.
CREATE TABLE IF NOT EXISTS cron_run_actors (
    app_id TEXT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    run_id TEXT NOT NULL,
    triggered_by TEXT NOT NULL DEFAULT '',
    canceled_by TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (app_id, run_id)
);
