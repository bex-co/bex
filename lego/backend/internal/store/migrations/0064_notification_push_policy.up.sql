-- w11/m5: native push extends the existing per-member notification_settings
-- row. JSONB keeps the nested schedule and exact per-service override document
-- atomic; the public API remains fully typed and validates before every write.
ALTER TABLE notification_settings
    ADD COLUMN push_policy jsonb NOT NULL DEFAULT '{
      "enabled": true,
      "events": ["deploy_failed", "server_failed", "cron_failed"],
      "minimumUrgency": "important",
      "timeZone": "UTC",
      "workingHours": [],
      "quietHours": [],
      "maxDeferralSeconds": 28800,
      "serviceOverrides": []
    }'::jsonb;

ALTER TABLE notification_settings
    ADD CONSTRAINT notification_settings_push_policy_object_check
    CHECK (jsonb_typeof(push_policy) = 'object');
