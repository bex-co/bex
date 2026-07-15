-- w3/005: complete the per-member deploy lifecycle preferences with the
-- request-time "deploy started" event. Existing rows opt in by default, matching
-- the notification feature's no-row default (all three lifecycle events true).
ALTER TABLE notification_settings
    ADD COLUMN deploy_started boolean NOT NULL DEFAULT true;
