-- w7/m53: durable Stripe export attempts, rejects, and old-ambiguity evidence.
-- No Stripe payload, API key, webhook secret, or payment detail is retained.

ALTER TABLE usage_hourly
    ADD COLUMN billing_export_state text NOT NULL DEFAULT 'pending'
        CHECK (billing_export_state IN ('pending', 'emitted', 'rejected', 'ambiguous')),
    ADD COLUMN billing_export_attempted_at timestamptz,
    ADD COLUMN billing_export_transaction_id text,
    ADD COLUMN billing_export_event_name text,
    ADD COLUMN billing_export_error_code text NOT NULL DEFAULT '',
    ADD COLUMN billing_export_error text NOT NULL DEFAULT '';

UPDATE usage_hourly
SET billing_export_state = 'emitted'
WHERE emitted_at IS NOT NULL;

DROP INDEX usage_hourly_unemitted_idx;
CREATE INDEX usage_hourly_billing_pending_idx
    ON usage_hourly (window_start)
    WHERE billing_export_state = 'pending';

CREATE TABLE billing_export_issues (
    transaction_id text PRIMARY KEY,
    workspace_id   text NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    resource_kind  text NOT NULL,
    service_id     text NOT NULL,
    kind           text NOT NULL,
    tier           text NOT NULL,
    window_start   timestamptz NOT NULL,
    event_name     text NOT NULL,
    issue_kind     text NOT NULL CHECK (issue_kind IN ('permanent_reject', 'stamp_ambiguity')),
    error_code     text NOT NULL DEFAULT '',
    error_message  text NOT NULL DEFAULT '',
    first_seen_at  timestamptz NOT NULL,
    last_seen_at   timestamptz NOT NULL,
    resolved_at    timestamptz,
    resolution     text NOT NULL DEFAULT '',
    actor          text NOT NULL DEFAULT '',
    reason         text NOT NULL DEFAULT ''
);

CREATE INDEX billing_export_issues_open_idx
    ON billing_export_issues (first_seen_at, transaction_id)
    WHERE resolved_at IS NULL;
