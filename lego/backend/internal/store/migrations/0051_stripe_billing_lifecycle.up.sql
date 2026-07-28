-- w7/m52: durable Stripe dunning intake and reversible workspace enforcement.
-- Raw webhook payloads and payment details are deliberately not retained.

CREATE TABLE billing_provider_mappings (
    workspace_id   text PRIMARY KEY REFERENCES tenants (id) ON DELETE CASCADE,
    customer_id    text NOT NULL UNIQUE,
    subscription_id text UNIQUE,
    livemode       boolean NOT NULL,
    updated_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE billing_lifecycles (
    workspace_id       text PRIMARY KEY REFERENCES tenants (id) ON DELETE CASCADE,
    status             text NOT NULL DEFAULT 'healthy'
                       CHECK (status IN ('healthy', 'grace', 'enforcing', 'enforced', 'recovering', 'excluded', 'comped')),
    reason             text NOT NULL DEFAULT '',
    grace_deadline     timestamptz,
    source_event_id    text NOT NULL DEFAULT '',
    source_event_at    timestamptz,
    source_event_outcome text NOT NULL DEFAULT '',
    invoice_id         text NOT NULL DEFAULT '',
    subscription_id    text NOT NULL DEFAULT '',
    transition_version bigint NOT NULL DEFAULT 0,
    retry_at           timestamptz,
    claimed_until      timestamptz,
    attempt_count      integer NOT NULL DEFAULT 0,
    last_error         text NOT NULL DEFAULT '',
    enforced_at        timestamptz,
    recovered_at       timestamptz,
    recovery_target    text NOT NULL DEFAULT 'healthy'
                       CHECK (recovery_target IN ('healthy', 'excluded', 'comped')),
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    CHECK ((status = 'grace' AND grace_deadline IS NOT NULL) OR status <> 'grace')
);

CREATE INDEX billing_lifecycles_due_idx
    ON billing_lifecycles (COALESCE(retry_at, grace_deadline), workspace_id)
    WHERE status IN ('grace', 'enforcing', 'recovering');

CREATE TABLE stripe_billing_events (
    event_id          text PRIMARY KEY,
    event_type        text NOT NULL,
    workspace_id      text NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    customer_id       text NOT NULL DEFAULT '',
    subscription_id   text NOT NULL DEFAULT '',
    object_id         text NOT NULL DEFAULT '',
    livemode          boolean NOT NULL,
    provider_created_at timestamptz NOT NULL,
    received_at       timestamptz NOT NULL DEFAULT now(),
    outcome           text NOT NULL CHECK (outcome IN ('failure', 'success')),
    reason            text NOT NULL DEFAULT '',
    applied_at        timestamptz
);

CREATE INDEX stripe_billing_events_received_idx
    ON stripe_billing_events (received_at);
CREATE INDEX stripe_billing_events_workspace_idx
    ON stripe_billing_events (workspace_id, provider_created_at DESC);

CREATE TABLE billing_enforcements (
    workspace_id text NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    resource_kind text NOT NULL CHECK (resource_kind IN ('service', 'postgres', 'key_value')),
    resource_name text NOT NULL,
    marker_id     text NOT NULL,
    enforced_at   timestamptz NOT NULL DEFAULT now(),
    recovered_at  timestamptz,
    PRIMARY KEY (workspace_id, resource_kind, resource_name)
);

CREATE INDEX billing_enforcements_active_idx
    ON billing_enforcements (workspace_id, resource_kind, resource_name)
    WHERE recovered_at IS NULL;

CREATE TABLE billing_notifications (
    workspace_id       text NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    transition_version bigint NOT NULL,
    status             text NOT NULL,
    reason             text NOT NULL DEFAULT '',
    grace_deadline     timestamptz,
    livemode           boolean NOT NULL,
    attempt_count      integer NOT NULL DEFAULT 0,
    next_attempt_at    timestamptz NOT NULL DEFAULT now(),
    claimed_until      timestamptz,
    delivered_at       timestamptz,
    last_error         text NOT NULL DEFAULT '',
    created_at         timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, transition_version)
);

CREATE INDEX billing_notifications_due_idx
    ON billing_notifications (next_attempt_at, workspace_id)
    WHERE delivered_at IS NULL;

ALTER TABLE tenants
    ADD COLUMN billing_comped boolean NOT NULL DEFAULT false;

ALTER TABLE audit_events
    ADD COLUMN billing_override_reason text,
    ADD COLUMN billing_status_from text,
    ADD COLUMN billing_status_to text;
