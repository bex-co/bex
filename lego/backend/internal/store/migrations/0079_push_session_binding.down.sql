ALTER TABLE device_push_subscriptions
    DROP CONSTRAINT IF EXISTS device_push_subscriptions_session_id_check;

ALTER TABLE device_push_subscriptions
    DROP COLUMN IF EXISTS session_id;
