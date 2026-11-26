-- w4/m671: the image a running service runs is no longer served by its
-- registry. Render "periodically checks image availability for active
-- services" and "can notify you even if your running services are
-- unaffected" (render.com/docs/deploying-an-image). The Image Pull Failed
-- email for it has its own lead, which says the service still runs, so it is
-- a stream of its own, apart from a deploy's failed pull.
ALTER TABLE alert_email_streams DROP CONSTRAINT alert_email_streams_event_check;
ALTER TABLE alert_email_streams ADD CONSTRAINT alert_email_streams_event_check CHECK (event IN (
    'server_failed', 'cron_failed', 'image_pull_failed', 'blueprint_sync_failed', 'service_suspended',
    'disk_usage_high', 'disk_usage_recovered', 'image_unavailable'));
