-- Restore the former schema defaults. The data migration is intentionally not
-- reversed because a failure-only row is also a valid explicit preference.
ALTER TABLE notification_settings
    ALTER COLUMN deploy_started SET DEFAULT true,
    ALTER COLUMN deploy_succeeded SET DEFAULT true,
    ALTER COLUMN deploy_failed SET DEFAULT true;
