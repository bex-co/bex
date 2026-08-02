-- w11/m5: durable native-push inbox and per-device delivery queue.
--
-- push_notifications is one logical, token-free inbox item per
-- workspace-member and composed source event. push_deliveries fans that item
-- out once to each device that was active when the event was projected. The
-- device bearer capability remains solely in device_push_subscriptions.

CREATE TABLE push_notifications (
    tenant_id       text NOT NULL,
    subject         text NOT NULL,
    source_event_key text NOT NULL,
    event_id        text NOT NULL,
    event_type      text NOT NULL,
    title           text NOT NULL,
    body            text NOT NULL,
    urgency         text NOT NULL,
    resource_kind   text NOT NULL,
    resource_id     text NOT NULL,
    deep_link       text NOT NULL,
    occurred_at     timestamptz NOT NULL,
    deliver_at      timestamptz NOT NULL,
    read_at         timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, subject, source_event_key),
    UNIQUE (tenant_id, subject, event_id),
    FOREIGN KEY (tenant_id, subject)
        REFERENCES tenant_members (tenant_id, subject) ON DELETE CASCADE,
    CONSTRAINT push_notifications_source_key_check
        CHECK (char_length(source_event_key) BETWEEN 1 AND 512),
    CONSTRAINT push_notifications_event_id_check
        CHECK (event_id ~ '^evt-[0-9a-v]{20}$'),
    CONSTRAINT push_notifications_event_type_check
        CHECK (event_type IN (
            'deploy_started', 'deploy_succeeded', 'deploy_failed',
            'server_failed', 'cron_failed'
        )),
    CONSTRAINT push_notifications_title_check
        CHECK (char_length(title) BETWEEN 1 AND 120),
    CONSTRAINT push_notifications_body_check
        CHECK (char_length(body) BETWEEN 1 AND 1024),
    CONSTRAINT push_notifications_urgency_check
        CHECK (urgency IN ('routine', 'important', 'critical')),
    CONSTRAINT push_notifications_resource_check
        CHECK (resource_kind = 'service' AND resource_id ~ '^srv-[0-9a-v]{20}$'),
    CONSTRAINT push_notifications_deep_link_check
        CHECK (deep_link = '/services/' || resource_id)
);

CREATE INDEX push_notifications_member_inbox_idx
    ON push_notifications (tenant_id, subject, occurred_at DESC, event_id DESC);
CREATE INDEX push_notifications_unread_idx
    ON push_notifications (tenant_id, subject) WHERE read_at IS NULL;
CREATE INDEX push_notifications_due_idx
    ON push_notifications (deliver_at, tenant_id, subject, source_event_key);

CREATE TABLE push_deliveries (
    tenant_id       text NOT NULL,
    subject         text NOT NULL,
    device_id       text NOT NULL,
    source_event_key text NOT NULL,
    claimed_until   timestamptz,
    attempt_count   integer NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    last_error_code text NOT NULL DEFAULT '',
    last_attempted_at timestamptz,
    provider_ticket_id text NOT NULL DEFAULT '',
    accepted_token_digest text NOT NULL DEFAULT '',
    accepted_at     timestamptz,
    receipt_due_at  timestamptz,
    receipt_checked_at timestamptz,
    delivered_at    timestamptz,
    failed_at       timestamptz,
    ambiguous_at    timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, subject, device_id, source_event_key),
    FOREIGN KEY (tenant_id, subject, source_event_key)
        REFERENCES push_notifications (tenant_id, subject, source_event_key) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, subject, device_id)
        REFERENCES device_push_subscriptions (tenant_id, subject, device_id) ON DELETE CASCADE,
    CONSTRAINT push_deliveries_ticket_check
        CHECK (char_length(provider_ticket_id) <= 256),
    CONSTRAINT push_deliveries_token_digest_check
        CHECK (accepted_token_digest = '' OR accepted_token_digest ~ '^[0-9a-f]{64}$'),
    CONSTRAINT push_deliveries_error_code_check
        CHECK (last_error_code IN ('', 'transient', 'rate_limited', 'invalid_token', 'payload', 'permanent', 'receipt_pending', 'receipt_transient'))
);

CREATE INDEX push_deliveries_claimable_idx
    ON push_deliveries (next_attempt_at, claimed_until, tenant_id, subject, source_event_key)
    WHERE accepted_at IS NULL AND failed_at IS NULL;
CREATE INDEX push_deliveries_receipt_due_idx
    ON push_deliveries (receipt_due_at, claimed_until)
    WHERE accepted_at IS NOT NULL AND delivered_at IS NULL AND failed_at IS NULL AND ambiguous_at IS NULL;

-- A distinct singleton cursor lets push and webhooks consume the same composed
-- feed independently without either delivery channel advancing the other.
CREATE TABLE push_watermark (
    id  boolean PRIMARY KEY DEFAULT true CHECK (id),
    at  timestamptz NOT NULL,
    key text NOT NULL DEFAULT ''
);
