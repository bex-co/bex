-- w2/m76: browser Web Push (VAPID) destinations. Sibling of
-- device_push_subscriptions (0062), which CHECK-constrains provider='expo'.
-- A subscription belongs to one exact workspace member and browser; removing
-- the workspace or membership cascades the capability away.
CREATE TABLE webpush_subscriptions (
    tenant_id          text NOT NULL,
    subject            text NOT NULL,
    browser_id         text NOT NULL,
    endpoint           text NOT NULL,
    p256dh             text NOT NULL,
    auth               text NOT NULL,
    endpoint_digest    text NOT NULL,
    preference_id      text REFERENCES notification_settings (id) ON DELETE SET NULL,
    revoked_at         timestamptz,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    last_registered_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, subject, browser_id),
    FOREIGN KEY (tenant_id, subject)
        REFERENCES tenant_members (tenant_id, subject) ON DELETE CASCADE,
    CONSTRAINT webpush_subscriptions_browser_id_check
        CHECK (browser_id ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,199}$'),
    CONSTRAINT webpush_subscriptions_endpoint_check
        CHECK (char_length(endpoint) BETWEEN 8 AND 2048),
    CONSTRAINT webpush_subscriptions_p256dh_check
        CHECK (char_length(p256dh) BETWEEN 1 AND 256),
    CONSTRAINT webpush_subscriptions_auth_check
        CHECK (char_length(auth) BETWEEN 1 AND 256),
    CONSTRAINT webpush_subscriptions_endpoint_digest_check
        CHECK (endpoint_digest ~ '^[0-9a-f]{64}$')
);

-- An endpoint is a bearer capability and may route to only one active member
-- /browser. Re-registering it after account switching revokes the old owner.
CREATE UNIQUE INDEX webpush_subscriptions_active_endpoint_idx
    ON webpush_subscriptions (endpoint_digest)
    WHERE revoked_at IS NULL;

CREATE INDEX webpush_subscriptions_active_member_idx
    ON webpush_subscriptions (tenant_id, subject, updated_at DESC)
    WHERE revoked_at IS NULL;

-- push_deliveries.device_id now names either an Expo installation or a browser
-- id. The Expo-only FK would reject web-push fan-out rows; membership cascade
-- plus SweepPushRetention reclaim orphans (same as a revoked Expo token).
ALTER TABLE push_deliveries
    DROP CONSTRAINT IF EXISTS push_deliveries_tenant_id_subject_device_id_fkey;
