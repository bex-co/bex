DELETE FROM alert_email_streams WHERE event = 'service_suspended';
ALTER TABLE alert_email_streams DROP CONSTRAINT alert_email_streams_event_check;
ALTER TABLE alert_email_streams ADD CONSTRAINT alert_email_streams_event_check CHECK (event IN (
    'server_failed', 'cron_failed', 'image_pull_failed', 'blueprint_sync_failed'));
