-- w2/m253: a service the platform suspended for a crash loop emails its
-- workspace, as Render does ("Render suspends a service that repeatedly fails
-- to start and become healthy over a 24-hour period": Email ✓).
ALTER TABLE alert_email_streams DROP CONSTRAINT alert_email_streams_event_check;
ALTER TABLE alert_email_streams ADD CONSTRAINT alert_email_streams_event_check CHECK (event IN (
    'server_failed', 'cron_failed', 'image_pull_failed', 'blueprint_sync_failed', 'service_suspended'));
