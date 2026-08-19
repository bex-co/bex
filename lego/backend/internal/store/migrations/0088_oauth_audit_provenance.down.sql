ALTER TABLE audit_events
    DROP CONSTRAINT IF EXISTS audit_events_relation_check,
    DROP CONSTRAINT IF EXISTS audit_events_oauth_client_id_check,
    DROP CONSTRAINT IF EXISTS audit_events_oauth_audience_check,
    DROP CONSTRAINT IF EXISTS audit_events_oauth_scopes_check;

ALTER TABLE audit_events
    DROP COLUMN IF EXISTS relation,
    DROP COLUMN IF EXISTS oauth_client_id,
    DROP COLUMN IF EXISTS oauth_audience,
    DROP COLUMN IF EXISTS oauth_scopes;
