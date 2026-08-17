-- Bind every provider delivery to the exact mobile login epoch that registered
-- the destination. Pre-migration rows have no trustworthy session binding, so
-- revoke them (and cascade their queued deliveries) before making the field
-- mandatory. Current clients repair registration on their next authenticated
-- foreground pass.
ALTER TABLE device_push_subscriptions
    ADD COLUMN session_id text;

DELETE FROM device_push_subscriptions;

ALTER TABLE device_push_subscriptions
    ALTER COLUMN session_id SET NOT NULL;

ALTER TABLE device_push_subscriptions
    ADD CONSTRAINT device_push_subscriptions_session_id_check
        CHECK (session_id ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,199}$');
