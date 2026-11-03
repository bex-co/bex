-- w21/m258 (ADR094 D9): the bex.cloud_agents OAuth capability joins the closed
-- vocabulary core.ClosedOAuthScopes(), so a denied or allowed cloud-agents
-- call keeps its provenance row. recordAudit swallows sink errors, so without
-- this a CHECK miss would silently drop the row.
ALTER TABLE audit_events DROP CONSTRAINT audit_events_oauth_scopes_check;
ALTER TABLE audit_events
    ADD CONSTRAINT audit_events_oauth_scopes_check
        CHECK (
            oauth_scopes IS NULL
            OR (
                cardinality(oauth_scopes) BETWEEN 1 AND 32
                AND oauth_scopes <@ ARRAY['bex.read', 'bex.write', 'bex.sensitive', 'bex.cloud_agents', 'bex.api']::text[]
            )
        );
