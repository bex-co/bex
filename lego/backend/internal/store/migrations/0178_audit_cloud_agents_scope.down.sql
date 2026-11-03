ALTER TABLE audit_events DROP CONSTRAINT audit_events_oauth_scopes_check;
ALTER TABLE audit_events
    ADD CONSTRAINT audit_events_oauth_scopes_check
        CHECK (
            oauth_scopes IS NULL
            OR (
                cardinality(oauth_scopes) BETWEEN 1 AND 32
                AND oauth_scopes <@ ARRAY['bex.read', 'bex.write', 'bex.sensitive', 'bex.api']::text[]
            )
        );
