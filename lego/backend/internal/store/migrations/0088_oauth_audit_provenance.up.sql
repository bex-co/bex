-- w8/m27: bounded OAuth authorization provenance on the existing audit
-- trail. relation is the RelCan… the decision was made against; the three
-- oauth_* columns are the verified human-delegation facts from core.Identity
-- (never a bearer token or free-form metadata). Session, machine, system, and
-- pre-migration rows stay valid with NULL provenance.
--
-- The relation IN-list and oauth_scopes ARRAY must stay equal to
-- core.RelCanRelations() and core.ClosedOAuthScopes(). recordAudit swallows
-- sink errors, so a CHECK miss silently drops the row. Adding a RelCan… or
-- scope requires a follow-on migration (do not rewrite this file after it
-- has shipped); TestAuditRelationCheckMatchesRelCan pins the latest CHECK.
ALTER TABLE audit_events
    ADD COLUMN relation text,
    ADD COLUMN oauth_client_id text,
    ADD COLUMN oauth_audience text,
    ADD COLUMN oauth_scopes text[];

ALTER TABLE audit_events
    ADD CONSTRAINT audit_events_relation_check
        CHECK (relation IS NULL OR relation IN (
            'can_view',
            'can_view_logs',
            'can_operate',
            'can_create',
            'can_view_sensitive',
            'can_manage_keys',
            'can_manage_ssh_keys',
            'can_manage',
            'can_manage_billing'
        )),
    ADD CONSTRAINT audit_events_oauth_client_id_check
        CHECK (oauth_client_id IS NULL OR (char_length(oauth_client_id) BETWEEN 1 AND 128)),
    ADD CONSTRAINT audit_events_oauth_audience_check
        CHECK (oauth_audience IS NULL OR (char_length(oauth_audience) BETWEEN 1 AND 256)),
    ADD CONSTRAINT audit_events_oauth_scopes_check
        CHECK (
            oauth_scopes IS NULL
            OR (
                cardinality(oauth_scopes) BETWEEN 1 AND 32
                AND oauth_scopes <@ ARRAY['bex.read', 'bex.write', 'bex.sensitive', 'bex.api']::text[]
            )
        );
