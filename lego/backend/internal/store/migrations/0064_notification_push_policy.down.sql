ALTER TABLE notification_settings
    DROP CONSTRAINT notification_settings_push_policy_object_check,
    DROP COLUMN push_policy;
