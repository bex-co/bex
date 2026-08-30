CREATE TABLE account_deletions (
    subject text PRIMARY KEY,
    deleted_marker text NOT NULL,
    state text NOT NULL DEFAULT 'pending'
        CHECK (state IN ('pending', 'cleaning', 'identity', 'done')),
    workspace_plan jsonb NOT NULL DEFAULT '[]'::jsonb,
    attempts integer NOT NULL DEFAULT 0,
    claimed_until timestamptz,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    last_error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);

CREATE INDEX account_deletions_pending_idx
    ON account_deletions (next_attempt_at, created_at)
    WHERE state != 'done';

-- Account cleanup addresses identity/provenance columns directly instead of
-- through their ordinary workspace-first access paths. Keep those rare but
-- potentially large updates index-backed.
CREATE INDEX notification_settings_subject_idx ON notification_settings (subject);
CREATE INDEX github_connect_transactions_subject_idx ON github_connect_transactions (subject);
CREATE INDEX tenant_invites_invited_by_idx ON tenant_invites (invited_by);
CREATE INDEX tenant_invites_lower_email_idx ON tenant_invites (lower(email));
CREATE INDEX registry_credentials_created_by_idx ON registry_credentials (created_by);
CREATE INDEX webhook_endpoints_created_by_idx ON webhook_endpoints (created_by);
CREATE INDEX audit_events_caller_idx ON audit_events (caller);
CREATE INDEX audit_events_lower_target_name_idx ON audit_events (lower(target_name));
