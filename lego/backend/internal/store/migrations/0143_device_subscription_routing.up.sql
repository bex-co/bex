-- w5/m115: DELETE /v1/notification-device-subscriptions/{deviceId} with no
-- ownerId resolves the device's own workspace first.
-- DevicePushSubscriptionWorkspaces reads tenant_id by the caller's
-- (subject, device_id) over active rows; the table's keys lead with tenant_id,
-- so a lookup without it would otherwise scan (the 0140 pattern for the
-- inbox's routing read).
CREATE INDEX IF NOT EXISTS device_push_subscriptions_subject_device_idx
    ON device_push_subscriptions (subject, device_id)
    WHERE revoked_at IS NULL;
