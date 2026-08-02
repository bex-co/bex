-- w11/m5: native push destinations. A subscription belongs to one exact
-- workspace member and app-installation device. Removing either the workspace
-- or membership cascades the capability away; no orphan can keep receiving.
CREATE TABLE device_push_subscriptions (
    tenant_id       text NOT NULL,
    subject         text NOT NULL,
    device_id       text NOT NULL,
    provider        text NOT NULL,
    platform        text NOT NULL,
    token           text NOT NULL,
    token_digest    text NOT NULL,
    preference_id   text REFERENCES notification_settings (id) ON DELETE SET NULL,
    revoked_at      timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    last_registered_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, subject, device_id),
    FOREIGN KEY (tenant_id, subject)
        REFERENCES tenant_members (tenant_id, subject) ON DELETE CASCADE,
    CONSTRAINT device_push_subscriptions_provider_check
        CHECK (provider = 'expo'),
    CONSTRAINT device_push_subscriptions_platform_check
        CHECK (platform IN ('ios', 'android')),
    CONSTRAINT device_push_subscriptions_device_id_check
        CHECK (device_id ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,199}$'),
    CONSTRAINT device_push_subscriptions_token_check
        CHECK (char_length(token) BETWEEN 1 AND 4096),
    CONSTRAINT device_push_subscriptions_token_digest_check
        CHECK (token_digest ~ '^[0-9a-f]{64}$')
);

-- A provider token is a bearer capability and may route to only one active
-- member/device. Registering it after account switching revokes the old owner.
CREATE UNIQUE INDEX device_push_subscriptions_active_token_idx
    ON device_push_subscriptions (provider, token_digest)
    WHERE revoked_at IS NULL;

CREATE INDEX device_push_subscriptions_active_member_idx
    ON device_push_subscriptions (tenant_id, subject, updated_at DESC)
    WHERE revoked_at IS NULL;
